-- +migrate Up
-- Migration: 034_token_usage_revenue
-- Description: Dual-ledger columns for cost vs revenue statements
-- Date: 2026-08-05

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS revenue_usd DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS revenue_input_price DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS revenue_output_price DECIMAL(12,6) DEFAULT 0;

ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS total_revenue_usd DECIMAL(12,6) DEFAULT 0;

-- +migrate Down
ALTER TABLE token_usage_daily DROP COLUMN IF EXISTS total_revenue_usd;
ALTER TABLE token_usage DROP COLUMN IF EXISTS revenue_output_price;
ALTER TABLE token_usage DROP COLUMN IF EXISTS revenue_input_price;
ALTER TABLE token_usage DROP COLUMN IF EXISTS revenue_usd;
