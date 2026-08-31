-- +migrate Up
-- Migration: 042_pricing_rules_price_type
-- Description: Add price_type column to pricing_rules for cost/revenue separation
-- Date: 2026-08-31

ALTER TABLE pricing_rules ADD COLUMN price_type VARCHAR(20) NOT NULL DEFAULT 'cost';

-- +migrate Down
ALTER TABLE pricing_rules DROP COLUMN IF EXISTS price_type;
