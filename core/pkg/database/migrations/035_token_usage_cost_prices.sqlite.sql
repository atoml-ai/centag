-- +migrate Up
-- Migration: 035_token_usage_cost_prices
-- Description: Add cost-side unit prices (USD per 1M tokens) to token_usage and token_usage_daily
-- Date: 2026-08-12
-- Note: SQLite ADD COLUMN is not idempotent; migrator tracks version so this runs once.

ALTER TABLE token_usage ADD COLUMN cost_input_price REAL DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN cost_output_price REAL DEFAULT 0;

ALTER TABLE token_usage_daily ADD COLUMN cost_input_price REAL DEFAULT 0;
ALTER TABLE token_usage_daily ADD COLUMN cost_output_price REAL DEFAULT 0;

-- +migrate Down
-- SQLite cannot DROP COLUMN reliably on older versions; leave columns in place on rollback.
