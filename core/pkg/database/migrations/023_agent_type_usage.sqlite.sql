-- 023: Add agent_type column to token_usage and token_usage_daily for per-agent usage tracking.
-- SQLite requires table recreation to change unique constraints.

-- 1. Add agent_type to token_usage (simple ALTER ADD COLUMN).
ALTER TABLE token_usage ADD COLUMN agent_type TEXT;

-- 2. Rebuild token_usage_daily with new unique constraint.
CREATE TABLE IF NOT EXISTS token_usage_daily_new (
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
    tenant_id TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, backend_id, model, agent_type, date)
);

INSERT INTO token_usage_daily_new
    (user_id, backend_id, model, date, total_prompt_tokens, total_completion_tokens, total_tokens, total_cost_usd, request_count, tenant_id, updated_at)
SELECT
    user_id, backend_id, model, date, total_prompt_tokens, total_completion_tokens, total_tokens, total_cost_usd, request_count, tenant_id, updated_at
FROM token_usage_daily;

DROP TABLE token_usage_daily;
ALTER TABLE token_usage_daily_new RENAME TO token_usage_daily;

CREATE INDEX IF NOT EXISTS idx_token_usage_agent_type ON token_usage (agent_type);
