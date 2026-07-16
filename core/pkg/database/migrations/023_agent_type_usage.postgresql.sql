-- 023: Add agent_type column to token_usage and token_usage_daily for per-agent usage tracking.

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS agent_type TEXT;
ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS agent_type TEXT;

-- Rebuild token_usage_daily unique index to include agent_type.
-- The original UNIQUE is (user_id, backend_id, model, date).
-- New: (user_id, backend_id, model, agent_type, date).
ALTER TABLE token_usage_daily DROP CONSTRAINT IF EXISTS token_usage_daily_user_id_backend_id_model_date_key;
CREATE UNIQUE INDEX IF NOT EXISTS token_usage_daily_user_agent_backend_model_date_key
    ON token_usage_daily (user_id, backend_id, model, agent_type, date);

CREATE INDEX IF NOT EXISTS idx_token_usage_agent_type ON token_usage (agent_type);
