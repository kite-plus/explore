package config

import (
	"log/slog"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != "127.0.0.1:8080" || c.PublicURL != "https://explore.kite.plus" ||
		c.WorkerConcurrency != 16 || c.AllowPrivateNetworks || c.LogLevel != slog.LevelInfo {
		t.Errorf("defaults = %+v", c)
	}
	if err := c.RequireDatabase(); err == nil {
		t.Error("RequireDatabase passed without a URL")
	}
}

func TestValues(t *testing.T) {
	c, err := Load(env(map[string]string{
		"EXPLORE_DATABASE_URL":           "postgres://x",
		"EXPLORE_PUBLIC_URL":             "https://explore.example.com/",
		"EXPLORE_TRUSTED_PROXIES":        "10.0.0.2, 10.0.0.3",
		"EXPLORE_WORKER_CONCURRENCY":     "4",
		"EXPLORE_ALLOW_PRIVATE_NETWORKS": "true",
		"EXPLORE_LOG_LEVEL":              "debug",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicURL != "https://explore.example.com" || len(c.TrustedProxies) != 2 || c.WorkerConcurrency != 4 ||
		!c.AllowPrivateNetworks || c.LogLevel != slog.LevelDebug {
		t.Errorf("config = %+v", c)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := map[string]string{
		"EXPLORE_PUBLIC_URL":             "explore.kite.plus",
		"EXPLORE_WORKER_CONCURRENCY":     "0",
		"EXPLORE_ALLOW_PRIVATE_NETWORKS": "maybe",
		"EXPLORE_LOG_LEVEL":              "loud",
	}
	for k, v := range cases {
		if _, err := Load(env(map[string]string{k: v})); err == nil {
			t.Errorf("%s=%q accepted", k, v)
		}
	}
}

func TestTagger(t *testing.T) {
	c, err := Load(env(map[string]string{}))
	if err != nil || c.Tagger.Enabled() {
		t.Fatalf("tagging must be off by default: %+v, %v", c.Tagger, err)
	}
	c, err = Load(env(map[string]string{
		"EXPLORE_TAGGER_MODEL": "claude-opus-5", "EXPLORE_ANTHROPIC_API_KEY": "sk-secret", "EXPLORE_TAGGER_EFFORT": "low",
	}))
	if err != nil || !c.Tagger.Enabled() || c.Tagger.Effort != "low" {
		t.Fatalf("tagger = %+v, %v", c.Tagger, err)
	}
	for _, m := range []map[string]string{
		{"EXPLORE_ANTHROPIC_API_KEY": "sk-secret"},
		{"EXPLORE_TAGGER_MODEL": "claude-opus-5"},
		{"EXPLORE_TAGGER_MODEL": "claude-opus-5", "EXPLORE_ANTHROPIC_API_KEY": "sk-secret", "EXPLORE_TAGGER_EFFORT": "turbo"},
	} {
		_, err := Load(env(m))
		if err == nil {
			t.Errorf("%v: want an error", m)
		} else if strings.Contains(err.Error(), "sk-secret") {
			t.Errorf("the error echoes the key: %v", err)
		}
	}
}
