-- +migrate Up
-- Migration: 031_pricing_rules
-- Description: Create pricing_rules table for configurable model pricing
-- Date: 2026-07-18

CREATE TABLE IF NOT EXISTS pricing_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    backend_id VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    input_price_per_m DECIMAL(12,6) NOT NULL,
    output_price_per_m DECIMAL(12,6) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pricing_rules_backend_model
    ON pricing_rules(backend_id, model);

-- +migrate Down
DROP INDEX IF EXISTS idx_pricing_rules_backend_model;
DROP TABLE IF EXISTS pricing_rules;
