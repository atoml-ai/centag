-- +migrate Up
-- Migration: 036_token_usage_group_id
-- Description: Add group_id (resolved shared metering pool) to token_usage and token_usage_daily
-- Date: 2026-08-12

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS group_id VARCHAR(64);

ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS group_id VARCHAR(64);

-- +migrate Down
ALTER TABLE token_usage_daily DROP COLUMN IF EXISTS group_id;
ALTER TABLE token_usage DROP COLUMN IF EXISTS group_id;
