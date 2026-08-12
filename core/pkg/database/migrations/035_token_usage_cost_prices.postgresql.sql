-- +migrate Up
-- Migration: 035_token_usage_cost_prices
-- Description: Add cost-side unit prices (USD per 1M tokens) to token_usage and token_usage_daily
-- Date: 2026-08-12

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS cost_input_price DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS cost_output_price DECIMAL(12,6) DEFAULT 0;

ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS cost_input_price DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage_daily ADD COLUMN IF NOT EXISTS cost_output_price DECIMAL(12,6) DEFAULT 0;

-- +migrate Down
ALTER TABLE token_usage_daily DROP COLUMN IF EXISTS cost_output_price;
ALTER TABLE token_usage_daily DROP COLUMN IF EXISTS cost_input_price;
ALTER TABLE token_usage DROP COLUMN IF EXISTS cost_output_price;
ALTER TABLE token_usage DROP COLUMN IF EXISTS cost_input_price;
