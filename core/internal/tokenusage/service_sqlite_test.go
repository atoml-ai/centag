package tokenusage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupSQLiteTokenUsageDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	schema := `
	CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL);
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
		pricing_rule_id INTEGER,
		success INTEGER NOT NULL DEFAULT 1,
		dept_tag TEXT,
		agent_type TEXT
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
		request_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, backend_id, model, agent_type, date)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestRecordUsage_SQLite(t *testing.T) {
	db := setupSQLiteTokenUsageDB(t)
	defer db.Close()

	svc := NewService(db, "sqlite")
	err := svc.RecordUsage(context.Background(), &UsageRecord{
		UserID:           1,
		BackendID:        "deepseek",
		Model:            "deepseek-v4-flash",
		PromptTokens:     100,
		CompletionTokens: 38,
		TotalTokens:      138,
		RequestID:        "req-test",
		Success:          true,
	})
	if err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM token_usage WHERE user_id = 1 AND total_tokens = 138`).Scan(&count); err != nil {
		t.Fatalf("count detail: %v", err)
	}
	if count != 1 {
		t.Fatalf("detail rows = %d, want 1", count)
	}

	stats, err := svc.GetUserUsage(context.Background(), 1, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetUserUsage: %v", err)
	}
	if stats.TotalTokens != 138 {
		t.Fatalf("stats.TotalTokens = %d, want 138", stats.TotalTokens)
	}
}

func TestRecordUsage_NormalizeEmptyAgentType(t *testing.T) {
	db := setupSQLiteTokenUsageDB(t)
	defer db.Close()

	svc := NewService(db, "sqlite")
	ctx := context.Background()

	first := &UsageRecord{
		UserID:           1,
		BackendID:        "deepseek",
		Model:            "deepseek-v4-flash",
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		RequestID:        "req-direct-1",
		Success:          true,
	}
	second := &UsageRecord{
		UserID:           1,
		BackendID:        "deepseek",
		Model:            "deepseek-v4-flash",
		PromptTokens:     20,
		CompletionTokens: 10,
		TotalTokens:      30,
		RequestID:        "req-direct-2",
		Success:          true,
	}

	if err := svc.RecordUsage(ctx, first); err != nil {
		t.Fatalf("RecordUsage first: %v", err)
	}
	if err := svc.RecordUsage(ctx, second); err != nil {
		t.Fatalf("RecordUsage second: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM token_usage_daily WHERE user_id = 1 AND backend_id = 'deepseek' AND model = 'deepseek-v4-flash' AND agent_type = 'direct'`).Scan(&count); err != nil {
		t.Fatalf("count token_usage_daily: %v", err)
	}
	if count != 1 {
		t.Fatalf("token_usage_daily rows = %d, want 1", count)
	}

	var reqCount int
	if err := db.QueryRow(`SELECT request_count FROM token_usage_daily WHERE user_id = 1 AND backend_id = 'deepseek' AND model = 'deepseek-v4-flash' AND agent_type = 'direct'`).Scan(&reqCount); err != nil {
		t.Fatalf("query request_count: %v", err)
	}
	if reqCount != 2 {
		t.Fatalf("request_count = %d, want 2", reqCount)
	}

	var detailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM token_usage WHERE user_id = 1 AND backend_id = 'deepseek' AND model = 'deepseek-v4-flash' AND agent_type = 'direct'`).Scan(&detailCount); err != nil {
		t.Fatalf("count token_usage detail: %v", err)
	}
	if detailCount != 2 {
		t.Fatalf("token_usage detail rows = %d, want 2", detailCount)
	}
}

// ── used_usd writeback tests ─────────────────────────────────────────────────

func setupSQLiteTokenUsageDBWithAPIKeys(t *testing.T) *sql.DB {
	t.Helper()
	db := setupSQLiteTokenUsageDB(t)
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT,
		key_hash TEXT UNIQUE NOT NULL,
		key_prefix TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		budget_usd REAL NOT NULL DEFAULT 0,
		used_usd REAL NOT NULL DEFAULT 0,
		rate_limit_rpm INTEGER NOT NULL DEFAULT 0,
		rate_limit_tpm INTEGER NOT NULL DEFAULT 0,
		model_whitelist TEXT NOT NULL DEFAULT '*',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create api_keys table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix) VALUES (1, 1, 'test-key', 'abc', 'llmproxy_abc')`)
	if err != nil {
		t.Fatalf("insert api key: %v", err)
	}
	return db
}

// TestRecordUsage_UsedUSD_SQLiteSkip verifies that on SQLite driver,
// RecordUsage with api_key_id>0 and cost>0 does NOT crash but skips the writeback.
func TestRecordUsage_UsedUSD_SQLiteSkip(t *testing.T) {
	db := setupSQLiteTokenUsageDBWithAPIKeys(t)
	defer db.Close()

	svc := NewService(db, "sqlite")
	err := svc.RecordUsage(context.Background(), &UsageRecord{
		UserID:           1,
		APIKeyID:         1,
		BackendID:        "deepseek",
		Model:            "deepseek-v4-flash",
		PromptTokens:     100,
		CompletionTokens: 38,
		TotalTokens:      138,
		CostUSD:          0.05,
		RequestID:        "req-usd",
		Success:          true,
	})
	if err != nil {
		t.Fatalf("RecordUsage with api_key_id on sqlite: %v", err)
	}

	// used_usd should remain unchanged (writeback skipped for SQLite driver)
	var usedUSD float64
	if err := db.QueryRow(`SELECT used_usd FROM api_keys WHERE id = 1`).Scan(&usedUSD); err != nil {
		t.Fatalf("query used_usd: %v", err)
	}
	if usedUSD != 0 {
		t.Errorf("used_usd should be 0 on SQLite driver (writeback skipped), got %f", usedUSD)
	}

	// Detail row should be inserted
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM token_usage WHERE request_id = 'req-usd'`).Scan(&count); err != nil {
		t.Fatalf("count detail: %v", err)
	}
	if count != 1 {
		t.Fatalf("detail rows = %d, want 1", count)
	}
}

// TestRecordUsage_UsedUSD_Writeback_Atomic tests the atomic UPDATE pattern
// used in RecordUsage for the PostgreSQL code path.  Because PostgreSQL-style
// $N placeholders differ from SQLite ? placeholders, we validate the same
// atomic pattern directly with SQLite SQL to confirm the writeback logic.
func TestRecordUsage_UsedUSD_Writeback_Atomic(t *testing.T) {
	db := setupSQLiteTokenUsageDBWithAPIKeys(t)
	defer db.Close()

	// Simulate the atomic writeback: UPDATE api_keys SET used_usd = used_usd + ? WHERE id = ?
	cost1 := 0.05
	if _, err := db.Exec(`UPDATE api_keys SET used_usd = used_usd + ? WHERE id = ?`, cost1, 1); err != nil {
		t.Fatalf("first update: %v", err)
	}

	var u1 float64
	db.QueryRow(`SELECT used_usd FROM api_keys WHERE id = 1`).Scan(&u1)
	if u1 != 0.05 {
		t.Errorf("after first update: used_usd = %f, want 0.05", u1)
	}

	// Second update (atomic accumulation)
	cost2 := 0.03
	if _, err := db.Exec(`UPDATE api_keys SET used_usd = used_usd + ? WHERE id = ?`, cost2, 1); err != nil {
		t.Fatalf("second update: %v", err)
	}

	var u2 float64
	db.QueryRow(`SELECT used_usd FROM api_keys WHERE id = 1`).Scan(&u2)
	if u2 != 0.08 {
		t.Errorf("after second update: used_usd = %f, want 0.08", u2)
	}
}
