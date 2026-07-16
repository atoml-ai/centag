-- v2.1: Add user quota fields to users table
-- Each ADD COLUMN uses IF NOT EXISTS for idempotency

ALTER TABLE users ADD COLUMN default_pipeline_id TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN daily_token_limit INTEGER DEFAULT 0;  -- 0=unlimited
ALTER TABLE users ADD COLUMN monthly_token_limit INTEGER DEFAULT 0; -- 0=unlimited
ALTER TABLE users ADD COLUMN daily_token_used INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN monthly_token_used INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN quota_reset_date TEXT DEFAULT NULL;
