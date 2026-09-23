package config

import (
	"crypto/sha256"
	"encoding/hex"
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
		c.WorkerConcurrency != 16 || c.AllowPrivateNetworks || c.LogLevel != slog.LevelInfo || len(c.AdminTokens) != 0 {
		t.Errorf("defaults = %+v", c)
	}
	if err := c.RequireDatabase(); err == nil {
		t.Error("RequireDatabase passed without a URL")
	}
}

func TestValues(t *testing.T) {
	sum := sha256.Sum256([]byte("secret"))
	c, err := Load(env(map[string]string{
		"EXPLORE_DATABASE_URL":           "postgres://x",
		"EXPLORE_PUBLIC_URL":             "https://explore.example.com/",
		"EXPLORE_TRUSTED_PROXIES":        "10.0.0.2, 10.0.0.3",
		"EXPLORE_ADMIN_TOKENS":           "alice:" + hex.EncodeToString(sum[:]),
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
	if len(c.AdminTokens) != 1 || c.AdminTokens[0].Name != "alice" || c.AdminTokens[0].Hash != sum {
		t.Errorf("tokens = %+v", c.AdminTokens)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := map[string]string{
		"EXPLORE_PUBLIC_URL":             "explore.kite.plus",
		"EXPLORE_WORKER_CONCURRENCY":     "0",
		"EXPLORE_ALLOW_PRIVATE_NETWORKS": "maybe",
		"EXPLORE_LOG_LEVEL":              "loud",
		"EXPLORE_ADMIN_TOKENS":           "alice:not-hex",
	}
	for k, v := range cases {
		if _, err := Load(env(map[string]string{k: v})); err == nil {
			t.Errorf("%s=%q accepted", k, v)
		}
	}
}

func TestTokenErrorsDoNotEchoSecrets(t *testing.T) {
	_, err := Load(env(map[string]string{"EXPLORE_ADMIN_TOKENS": "alice:plain-secret-token"}))
	if err == nil || strings.Contains(err.Error(), "plain-secret-token") {
		t.Fatalf("err = %v", err)
	}
}

func TestDuplicateTokenNames(t *testing.T) {
	sum := sha256.Sum256([]byte("x"))
	h := hex.EncodeToString(sum[:])
	if _, err := Load(env(map[string]string{"EXPLORE_ADMIN_TOKENS": "a:" + h + ",a:" + h})); err == nil {
		t.Error("duplicate names accepted")
	}
}
