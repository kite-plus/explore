// Package storetest gives each integration test a migrated store in its own
// PostgreSQL schema, so tests run in parallel against one database.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/store"
)

// EnvURL names the variable that points tests at a database.
const EnvURL = "EXPLORE_TEST_DATABASE_URL"

// New returns a migrated store in a fresh schema, dropped when the test
// ends. Without EXPLORE_TEST_DATABASE_URL the test is skipped.
func New(t testing.TB) *store.Store {
	t.Helper()
	url := os.Getenv(EnvURL)
	if url == "" {
		t.Skip(EnvURL + " is not set")
	}
	ctx := context.Background()

	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(b[:])

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to %s: %v", EnvURL, err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	s, err := store.OpenSchema(ctx, url, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close(context.Background())
	})
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}
