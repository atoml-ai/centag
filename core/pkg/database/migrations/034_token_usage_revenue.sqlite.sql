-- +migrate Up
-- Migration: 034_token_usage_revenue
-- Description: Dual-ledger columns for cost vs revenue statements
-- Date: 2026-08-05
-- Note: SQLite ADD COLUMN is not idempotent; migrator tracks version so this runs once.

ALTER TABLE token_usage ADD COLUMN revenue_usd REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN revenue_input_price REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN revenue_output_price REAL DEFAULT 0;

ALTER TABLE token_usage_daily ADD COLUMN total_revenue_usd REAL DEFAULT 0;

-- +migrate Down
-- SQLite cannot DROP COLUMN reliably on older versions; leave columns in place on rollback.
