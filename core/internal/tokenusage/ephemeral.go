package tokenusage

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// ephemeralSchema is a minimal SQLite schema for in-process metering (restart clears).
const ephemeralSchema = `
CREATE TABLE IF NOT EXISTS token_usage (
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
	success INTEGER NOT NULL DEFAULT 1,
	dept_tag TEXT,
	agent_type TEXT
);
CREATE TABLE IF NOT EXISTS token_usage_daily (
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
	request_count INTEGER DEFAULT 0,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, backend_id, model, agent_type, date)
);
`

// NewEphemeralService creates an in-memory SQLite-backed Service.
// Data lives only for the process lifetime (cleared on restart).
func NewEphemeralService() (*Service, error) {
	dsn := fmt.Sprintf("file:centag_ephemeral_token_%d?mode=memory&cache=shared", os.Getpid())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open ephemeral sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(ephemeralSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ephemeral schema: %w", err)
	}
	return NewService(db, "sqlite"), nil
}
