-- +migrate Up
-- Migration: 036_token_usage_group_id
-- Description: Add group_id (resolved shared metering pool) to token_usage and token_usage_daily
-- Date: 2026-08-12
-- Note: SQLite ADD COLUMN is not idempotent; migrator tracks version so this runs once.

ALTER TABLE token_usage ADD COLUMN group_id TEXT;

ALTER TABLE token_usage_daily ADD COLUMN group_id TEXT;

-- +migrate Down
-- SQLite cannot DROP COLUMN reliably on older versions; leave columns in place on rollback.
