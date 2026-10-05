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

	if u, err := url.Parse(c.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
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
