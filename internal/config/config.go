// Package config reads the service configuration from the environment.
// Crawl and display thresholds are product rules in internal/policy, not
// settings; see docs/design/project-layout.md section 4.
package config

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Config is the service configuration.
type Config struct {
	DatabaseURL          string
	HTTPAddr             string
	PublicURL            string
	TrustedProxies       []string
	WorkerConcurrency    int
	AllowPrivateNetworks bool
	LogLevel             slog.Level
	Tagger               Tagger
	OIDC                 OIDC
	AppLinks             AppLinks
}

// OIDC configures sign-in with the Kite Plus identity provider. It is off
// until the deployment names an issuer; see docs/design/accounts.md
// section 7.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// AccountURL is the provider's page where readers manage and delete their
	// account; it defaults to ZITADEL's under the issuer.
	AccountURL string
	// WebhookSecret signs the user events the provider pushes; empty turns
	// the webhook off. It needs APIToken, which confirms each event.
	WebhookSecret string
	// APIToken reads the provider's accounts and their events, for the
	// webhook and the hourly reconciliation; empty turns both off.
	APIToken string
}

// Enabled reports whether readers can sign in with the provider.
func (o OIDC) Enabled() bool { return o.Issuer != "" }

func (o *OIDC) validate() error {
	if o.Issuer == "" {
		if o.ClientID != "" || o.ClientSecret != "" || o.AccountURL != "" || o.WebhookSecret != "" || o.APIToken != "" {
			return errors.New("EXPLORE_OIDC_ISSUER is not set: name the issuer to sign in with it, or leave the other OIDC and IdP settings empty")
		}
		return nil
	}
	if !httpURL(o.Issuer) {
		return fmt.Errorf("EXPLORE_OIDC_ISSUER: %q is not an http or https URL", o.Issuer)
	}
	if o.ClientID == "" || o.ClientSecret == "" {
		return errors.New("EXPLORE_OIDC_CLIENT_ID and EXPLORE_OIDC_CLIENT_SECRET are required with EXPLORE_OIDC_ISSUER")
	}
	if o.WebhookSecret != "" && o.APIToken == "" {
		return errors.New("EXPLORE_IDP_WEBHOOK_SECRET needs EXPLORE_IDP_API_TOKEN: a pushed event is applied only once the provider confirms it")
	}
	if o.AccountURL == "" {
		o.AccountURL = strings.TrimRight(o.Issuer, "/") + "/ui/console/users/me"
	} else if !httpURL(o.AccountURL) {
		return fmt.Errorf("EXPLORE_OIDC_ACCOUNT_URL: %q is not an http or https URL", o.AccountURL)
	}
	return nil
}

// AppLinks are what the apps need to claim this site's addresses for the
// sign-in callback; each file is served only when its values are set. See
// docs/design/accounts.md section 1.1.
type AppLinks struct {
	// AppleAppIDs are "TEAMID.bundle.id" values for apple-app-site-association.
	AppleAppIDs []string
	// AndroidPackage and AndroidCertSHA256 go into assetlinks.json.
	AndroidPackage    string
	AndroidCertSHA256 []string
}

var (
	appleAppID  = regexp.MustCompile(`^[A-Z0-9]{10}\.[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
	androidPkg  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)
	fingerprint = regexp.MustCompile(`^([0-9A-F]{2}:){31}[0-9A-F]{2}$`)
)

func (a *AppLinks) validate() error {
	for _, id := range a.AppleAppIDs {
		if !appleAppID.MatchString(id) {
			return fmt.Errorf("EXPLORE_APPLE_APP_IDS: %q is not TEAMID.bundle.id", id)
		}
	}
	if !androidPkg.MatchString(a.AndroidPackage) {
		return fmt.Errorf("EXPLORE_ANDROID_PACKAGE: %q is not an Android package name", a.AndroidPackage)
	}
	for i, f := range a.AndroidCertSHA256 {
		a.AndroidCertSHA256[i] = strings.ToUpper(f)
		if !fingerprint.MatchString(a.AndroidCertSHA256[i]) {
			return fmt.Errorf("EXPLORE_ANDROID_CERT_SHA256: %q is not a SHA-256 fingerprint of 32 hex pairs joined by colons", f)
		}
	}
	return nil
}

func httpURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Tagger configures the model that tags entries. Tagging is off until the
// deployment names a model; see docs/design/project-layout.md section 4.
type Tagger struct {
	Provider  string // "anthropic", or "openai" for any OpenAI-compatible API
	BaseURL   string // empty uses the provider's own
	Model     string
	APIKey    string
	Effort    string         // empty sends none, for models that reject it
	ExtraBody map[string]any // top-level fields added to every request
}

// Enabled reports whether entries get tagged.
func (t Tagger) Enabled() bool { return t.Model != "" }

func (t Tagger) validate() error {
	if t.Provider != "anthropic" && t.Provider != "openai" {
		return fmt.Errorf("EXPLORE_TAGGER_PROVIDER: %q is not anthropic or openai", t.Provider)
	}
	if t.BaseURL != "" {
		if u, err := url.Parse(t.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("EXPLORE_TAGGER_BASE_URL is not an http or https URL")
		}
	}
	if t.Model == "" {
		if t.APIKey != "" || t.BaseURL != "" || t.ExtraBody != nil {
			return errors.New("EXPLORE_TAGGER_MODEL is not set: name a model to tag entries, or leave the other tagger settings empty")
		}
		return nil
	}
	// A local OpenAI-compatible server usually takes no key.
	if t.APIKey == "" && (t.Provider == "anthropic" || t.BaseURL == "") {
		return errors.New("EXPLORE_TAGGER_API_KEY is not set: only an openai provider with its own base URL may go without one")
	}
	if t.Provider == "openai" {
		if strings.ContainsFunc(t.Effort, func(r rune) bool { return r < 'a' || r > 'z' }) {
			return fmt.Errorf("EXPLORE_TAGGER_EFFORT: %q is not a reasoning effort such as low or high", t.Effort)
		}
		return nil
	}
	switch t.Effort {
	case "", "low", "medium", "high", "xhigh", "max":
		return nil
	}
	return fmt.Errorf("EXPLORE_TAGGER_EFFORT: %q is not low, medium, high, xhigh or max", t.Effort)
}

// ErrNoDatabase is returned by RequireDatabase.
var ErrNoDatabase = errors.New("EXPLORE_DATABASE_URL is not set")

// Load reads the configuration through getenv, usually os.Getenv.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DatabaseURL:       strings.TrimSpace(getenv("EXPLORE_DATABASE_URL")),
		HTTPAddr:          or(getenv("EXPLORE_HTTP_ADDR"), "127.0.0.1:8080"),
		PublicURL:         strings.TrimRight(or(getenv("EXPLORE_PUBLIC_URL"), "https://explore.kite.plus"), "/"),
		TrustedProxies:    list(getenv("EXPLORE_TRUSTED_PROXIES")),
		WorkerConcurrency: 16,
		LogLevel:          slog.LevelInfo,
	}

	if !httpURL(c.PublicURL) {
		return Config{}, fmt.Errorf("EXPLORE_PUBLIC_URL: %q is not an http or https URL", c.PublicURL)
	}
	if v := strings.TrimSpace(getenv("EXPLORE_WORKER_CONCURRENCY")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 256 {
			return Config{}, fmt.Errorf("EXPLORE_WORKER_CONCURRENCY: %q is not a number from 1 to 256", v)
		}
		c.WorkerConcurrency = n
	}
	if v := strings.TrimSpace(getenv("EXPLORE_ALLOW_PRIVATE_NETWORKS")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("EXPLORE_ALLOW_PRIVATE_NETWORKS: %q is not true or false", v)
		}
		c.AllowPrivateNetworks = b
	}
	if v := strings.TrimSpace(getenv("EXPLORE_LOG_LEVEL")); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("EXPLORE_LOG_LEVEL: %q is not debug, info, warn or error", v)
		}
	}
	c.Tagger = Tagger{
		Provider: strings.ToLower(or(getenv("EXPLORE_TAGGER_PROVIDER"), "anthropic")),
		BaseURL:  strings.TrimRight(strings.TrimSpace(getenv("EXPLORE_TAGGER_BASE_URL")), "/"),
		Model:    strings.TrimSpace(getenv("EXPLORE_TAGGER_MODEL")),
		// EXPLORE_ANTHROPIC_API_KEY is the key's name from before other providers.
		APIKey: cmp.Or(strings.TrimSpace(getenv("EXPLORE_TAGGER_API_KEY")), strings.TrimSpace(getenv("EXPLORE_ANTHROPIC_API_KEY"))),
		Effort: strings.TrimSpace(getenv("EXPLORE_TAGGER_EFFORT")),
	}
	if v := strings.TrimSpace(getenv("EXPLORE_TAGGER_EXTRA_BODY")); v != "" {
		// The value is not echoed: it may carry a credential.
		if err := json.Unmarshal([]byte(v), &c.Tagger.ExtraBody); err != nil || c.Tagger.ExtraBody == nil {
			return Config{}, errors.New("EXPLORE_TAGGER_EXTRA_BODY is not a JSON object")
		}
	}
	if err := c.Tagger.validate(); err != nil {
		return Config{}, err
	}
	// Secrets are not echoed in errors.
	c.OIDC = OIDC{
		Issuer:        strings.TrimSpace(getenv("EXPLORE_OIDC_ISSUER")),
		ClientID:      strings.TrimSpace(getenv("EXPLORE_OIDC_CLIENT_ID")),
		ClientSecret:  strings.TrimSpace(getenv("EXPLORE_OIDC_CLIENT_SECRET")),
		AccountURL:    strings.TrimSpace(getenv("EXPLORE_OIDC_ACCOUNT_URL")),
		WebhookSecret: strings.TrimSpace(getenv("EXPLORE_IDP_WEBHOOK_SECRET")),
		APIToken:      strings.TrimSpace(getenv("EXPLORE_IDP_API_TOKEN")),
	}
	if err := c.OIDC.validate(); err != nil {
		return Config{}, err
	}
	// A public site trusts only an issuer it reaches over TLS; plain http
	// is for development, where private networks are allowed anyway.
	if c.OIDC.Enabled() && strings.HasPrefix(c.PublicURL, "https://") && !strings.HasPrefix(c.OIDC.Issuer, "https://") &&
		!c.AllowPrivateNetworks {
		return Config{}, fmt.Errorf("EXPLORE_OIDC_ISSUER: %q is not https, which a site at an https EXPLORE_PUBLIC_URL needs unless EXPLORE_ALLOW_PRIVATE_NETWORKS is set", c.OIDC.Issuer)
	}
	c.AppLinks = AppLinks{
		AppleAppIDs:       list(getenv("EXPLORE_APPLE_APP_IDS")),
		AndroidPackage:    or(getenv("EXPLORE_ANDROID_PACKAGE"), "plus.kite.explore"),
		AndroidCertSHA256: list(getenv("EXPLORE_ANDROID_CERT_SHA256")),
	}
	if err := c.AppLinks.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// RequireDatabase fails when no database is configured.
func (c Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return ErrNoDatabase
	}
	return nil
}

func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func or(v, fallback string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return fallback
}
