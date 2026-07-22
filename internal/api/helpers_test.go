package api

import (
	"context"
	"os"
	"testing"

	"kova/internal/db"
	"kova/internal/extract"
	"kova/internal/secretbox"
	"kova/internal/store"
)

// testStore connects to TEST_DATABASE_URL and returns a clean store. DB-backed
// api tests skip when it isn't set.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run DB-backed api tests")
	}
	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		t.Skipf("cannot connect to test db: %v", err)
	}
	_, _ = pool.Exec(context.Background(),
		`TRUNCATE users, sessions, workspaces, workspace_members, api_keys, requests, usage_events, password_resets CASCADE`)
	t.Cleanup(pool.Close)
	return store.New(pool)
}

func newServer(t *testing.T, ex extract.Extractor) *Server {
	box, err := secretbox.New("test-encryption-key")
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	return New(ex, testStore(t), GitHubConfig{}, nil, box)
}
