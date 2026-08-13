package server

import (
	"context"
	"database/sql"
	"testing"

	"centag/core/internal/tokenusage"
	"centag/core/pkg/backend"
	"centag/core/pkg/hooks"
	_ "modernc.org/sqlite"
)

// setupTokenUsageDB mirrors the tokenusage package's SQLite schema so the
// adapter's RecordUsage path can be exercised in-process.
func setupTokenUsageDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	schema := `
	CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL, group_id TEXT);
	INSERT INTO users (id, username) VALUES (1, 'admin');
	CREATE TABLE token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		api_key_id INTEGER,
		backend_id TEXT NOT NULL,
		model TEXT NOT NULL,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		request_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		tenant_id TEXT,
		cost_usd REAL DEFAULT 0,
		input_cost REAL DEFAULT 0,
		output_cost REAL DEFAULT 0,
		cost_input_price REAL DEFAULT 0,
		cost_output_price REAL DEFAULT 0,
		revenue_usd REAL DEFAULT 0,
		revenue_input_price REAL DEFAULT 0,
		revenue_output_price REAL DEFAULT 0,
		pricing_rule_id INTEGER,
		success INTEGER NOT NULL DEFAULT 1,
		dept_tag TEXT,
		agent_type TEXT,
		group_id TEXT,
		source TEXT NOT NULL DEFAULT 'real',
		session_id TEXT
	);
	CREATE TABLE token_usage_daily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		backend_id TEXT NOT NULL,
		model TEXT NOT NULL,
		agent_type TEXT,
		date DATE NOT NULL,
		total_prompt_tokens INTEGER DEFAULT 0,
		total_completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0,
		cost_input_price REAL DEFAULT 0,
		cost_output_price REAL DEFAULT 0,
		total_revenue_usd REAL DEFAULT 0,
		request_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		group_id TEXT,
		UNIQUE(user_id, backend_id, model, agent_type, date)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

// TestTokenUsageHookAdapter_UserOwnedBackendScope verifies that usage recorded
// through the adapter against a user-created backend is scoped to the user
// (TenantID/GroupID = user scope) so team/group quota pools never count it.
func TestTokenUsageHookAdapter_UserOwnedBackendScope(t *testing.T) {
	orig := backend.GetManager()
	m := backend.NewManager()
	if err := m.Add(&backend.BackendConfig{ID: "user-backend", Name: "u", TenantID: "user:7"}); err != nil {
		t.Fatalf("add user backend: %v", err)
	}
	backend.SetManagerForTest(m)
	t.Cleanup(func() { backend.SetManagerForTest(orig) })

	db := setupTokenUsageDB(t)
	defer db.Close()

	svc := tokenusage.NewService(db, "sqlite")
	a := newTokenUsageHookAdapter(svc)

	err := a.OnTokenUsed(context.Background(), &hooks.TokenUsage{
		UserID:       7,
		Backend:      "user-backend",
		Model:        "m",
		InputTokens:  100,
		OutputTokens: 40,
		TotalTokens:  140,
		Success:      true,
		TenantID:     "team:1",
		GroupID:      "group:9",
	})
	if err != nil {
		t.Fatalf("OnTokenUsed: %v", err)
	}

	var tenantID, groupID string
	if err := db.QueryRow(`SELECT tenant_id, group_id FROM token_usage WHERE backend_id = 'user-backend'`).Scan(&tenantID, &groupID); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if tenantID != "user:7" {
		t.Errorf("tenant_id = %q, want user scope %q", tenantID, "user:7")
	}
	if groupID != "user:7" {
		t.Errorf("group_id = %q, want user scope %q", groupID, "user:7")
	}
}

// TestTokenUsageHookAdapter_TeamBackendKeepsScope ensures team/system backends
// retain the caller's tenant and resolved group.
func TestTokenUsageHookAdapter_TeamBackendKeepsScope(t *testing.T) {
	orig := backend.GetManager()
	m := backend.NewManager()
	if err := m.Add(&backend.BackendConfig{ID: "team-backend", Name: "t"}); err != nil {
		t.Fatalf("add team backend: %v", err)
	}
	backend.SetManagerForTest(m)
	t.Cleanup(func() { backend.SetManagerForTest(orig) })

	db := setupTokenUsageDB(t)
	defer db.Close()

	svc := tokenusage.NewService(db, "sqlite")
	a := newTokenUsageHookAdapter(svc)

	err := a.OnTokenUsed(context.Background(), &hooks.TokenUsage{
		UserID:       7,
		Backend:      "team-backend",
		Model:        "m",
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
		Success:      true,
		TenantID:     "team:1",
		GroupID:      "group:9",
	})
	if err != nil {
		t.Fatalf("OnTokenUsed: %v", err)
	}

	var tenantID, groupID string
	if err := db.QueryRow(`SELECT tenant_id, group_id FROM token_usage WHERE backend_id = 'team-backend'`).Scan(&tenantID, &groupID); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if tenantID != "team:1" {
		t.Errorf("tenant_id = %q, want %q", tenantID, "team:1")
	}
	if groupID != "group:9" {
		t.Errorf("group_id = %q, want %q", groupID, "group:9")
	}
}
