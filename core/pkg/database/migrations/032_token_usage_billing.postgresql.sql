-- +migrate Up
-- Migration: 032_token_usage_billing
-- Description: Add cost breakdown columns to token_usage (no FK on pricing_rule_id)
-- Date: 2026-07-18

ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS input_cost DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS output_cost DECIMAL(12,6) DEFAULT 0;
ALTER TABLE token_usage ADD COLUMN IF NOT EXISTS pricing_rule_id BIGINT;

-- +migrate Down
ALTER TABLE token_usage DROP COLUMN IF EXISTS pricing_rule_id;
ALTER TABLE token_usage DROP COLUMN IF EXISTS output_cost;
ALTER TABLE token_usage DROP COLUMN IF EXISTS input_cost;
