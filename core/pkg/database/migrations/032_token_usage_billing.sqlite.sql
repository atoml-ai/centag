-- +migrate Up
-- Migration: 032_token_usage_billing
-- Description: Add cost breakdown columns to token_usage (no FK on pricing_rule_id)
-- Date: 2026-07-18
-- Note: SQLite ADD COLUMN is not idempotent; migrator tracks version so this runs once.

ALTER TABLE token_usage ADD COLUMN input_cost REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN output_cost REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN pricing_rule_id INTEGER;

-- +migrate Down
-- SQLite cannot DROP COLUMN reliably on older versions; leave columns in place on rollback.
