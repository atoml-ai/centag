-- +migrate Up
-- Migration: 019_cost_and_success
-- Description: Add cost attribution and request outcome columns to token usage tables
-- Date: 2026-06-12

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS cost_usd DECIMAL(12, 6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS success BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS total_cost_usd DECIMAL(14, 6) DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_token_usage_backend_success
    ON token_usage(backend_id, success, created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_token_usage_backend_success;

ALTER TABLE token_usage_daily DROP COLUMN IF EXISTS total_cost_usd;
ALTER TABLE token_usage DROP COLUMN IF EXISTS success;
ALTER TABLE token_usage DROP COLUMN IF EXISTS cost_usd;