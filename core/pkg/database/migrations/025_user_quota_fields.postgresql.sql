-- v2.1: Add user quota fields to users table
-- Each ADD COLUMN uses IF NOT EXISTS for idempotency

ALTER TABLE users ADD COLUMN IF NOT EXISTS default_pipeline_id VARCHAR(255) DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_token_limit BIGINT DEFAULT 0;  -- 0=unlimited
ALTER TABLE users ADD COLUMN IF NOT EXISTS monthly_token_limit BIGINT DEFAULT 0; -- 0=unlimited
ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_token_used BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS monthly_token_used BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS quota_reset_date TIMESTAMP DEFAULT NULL;
