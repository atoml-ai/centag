-- +migrate Up
-- Migration: 019_cost_and_success
-- Description: Add cost attribution and request outcome columns to token usage tables
-- Date: 2026-06-12

ALTER TABLE token_usage ADD COLUMN cost_usd REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN success INTEGER NOT NULL DEFAULT 1;

ALTER TABLE token_usage_daily ADD COLUMN total_cost_usd REAL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_token_usage_backend_success
    ON token_usage(backend_id, success, created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_token_usage_backend_success;

ALTER TABLE token_usage_daily DROP COLUMN total_cost_usd;
ALTER TABLE token_usage DROP COLUMN success;
ALTER TABLE token_usage DROP COLUMN cost_usd;