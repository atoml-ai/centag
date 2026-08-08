package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestAPIKeyDeleteReleasesUsageFK guards the FK fix: deleting an API key that
// has token_usage rows referencing it must succeed and preserve the usage rows
// (api_key_id set to NULL) instead of failing with a foreign-key violation.
func TestAPIKeyDeleteReleasesUsageFK(t *testing.T) {
	db, err := sql.Open("sqlite", "file:apikey_delete_fk_test?mode=memory&cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT UNIQUE NOT NULL)`,
		`CREATE TABLE api_keys (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(100), key_hash VARCHAR(64) UNIQUE NOT NULL, key_prefix VARCHAR(64) NOT NULL,
			enabled BOOLEAN DEFAULT true)`,
		`CREATE TABLE token_usage (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			api_key_id INTEGER REFERENCES api_keys(id),
			backend_id VARCHAR(100) NOT NULL,
			model VARCHAR(100) NOT NULL,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO users (id, username) VALUES (1, 'u1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix) VALUES (10, 1, 'k', 'hash', 'llmproxy_x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO token_usage (user_id, api_key_id, backend_id, model, total_tokens) VALUES (1, 10, 'zen', 'm', 100)`); err != nil {
		t.Fatal(err)
	}

	s := &sqliteAPIKeyStore{db: db}
	if err := s.Delete(context.Background(), 10); err != nil {
		t.Fatalf("delete key with usage rows failed: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE id = 10`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("key not deleted (n=%d err=%v)", n, err)
	}
	var apiKeyID sql.NullInt64
	if err := db.QueryRow(`SELECT api_key_id FROM token_usage WHERE id = 1`).Scan(&apiKeyID); err != nil {
		t.Fatal(err)
	}
	if apiKeyID.Valid {
		t.Fatalf("token_usage.api_key_id should be NULL after key delete, got %d", apiKeyID.Int64)
	}
}
