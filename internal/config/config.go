// Package config reads the service configuration from the environment.
// Crawl and display thresholds are product rules in internal/policy, not
// settings; see docs/design/project-layout.md section 4.
package config

import (
	"crypto/sha256"
	"encoding/hex"
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
	AdminTokens          []AdminToken
	WorkerConcurrency    int
	AllowPrivateNetworks bool
	LogLevel             slog.Level
}

// AdminToken is a maintainer credential. Only the SHA-256 of the token is
// configured, so the environment never holds the secret itself.
type AdminToken struct {
	Name string
	Hash [sha256.Size]byte
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
	tokens, err := parseTokens(getenv("EXPLORE_ADMIN_TOKENS"))
	if err != nil {
		return Config{}, err
	}
	c.AdminTokens = tokens
	return c, nil
}

// RequireDatabase fails when no database is configured.
func (c Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return ErrNoDatabase
	}
	return nil
}

func parseTokens(v string) ([]AdminToken, error) {
	var out []AdminToken
	seen := make(map[string]bool)
	for _, item := range list(v) {
		name, sum, ok := strings.Cut(item, ":")
		name = strings.TrimSpace(name)
		raw, err := hex.DecodeString(strings.TrimSpace(sum))
		if !ok || name == "" || err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("EXPLORE_ADMIN_TOKENS: each entry must be name:sha256-hex, got %q", redact(item))
		}
		if seen[name] {
			return nil, fmt.Errorf("EXPLORE_ADMIN_TOKENS: name %q appears twice", name)
		}
		seen[name] = true
		var t AdminToken
		t.Name = name
		copy(t.Hash[:], raw)
		out = append(out, t)
	}
	return out, nil
}

// redact keeps a malformed entry recognizable without echoing a secret
// someone pasted by mistake.
func redact(item string) string {
	name, _, _ := strings.Cut(item, ":")
	return strings.TrimSpace(name) + ":…"
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
