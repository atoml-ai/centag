-- +migrate Up
-- Migration: 031_pricing_rules
-- Description: Create pricing_rules table for configurable model pricing
-- Date: 2026-07-18

CREATE TABLE IF NOT EXISTS pricing_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    backend_id TEXT NOT NULL,
    model TEXT NOT NULL,
    input_price_per_m REAL NOT NULL,
    output_price_per_m REAL NOT NULL,
    currency TEXT DEFAULT 'USD',
    priority INTEGER DEFAULT 0,
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_pricing_rules_backend_model
    ON pricing_rules(backend_id, model);

-- +migrate Down
DROP INDEX IF EXISTS idx_pricing_rules_backend_model;
DROP TABLE IF EXISTS pricing_rules;
