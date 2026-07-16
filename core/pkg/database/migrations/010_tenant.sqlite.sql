-- +migrate Up
-- Migration: 010_tenant
-- Description: 创建租户表（一用户一租户）
-- Date: 2026-05-04

CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenants_user_id ON tenants(user_id);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);

-- +migrate Down
DROP TABLE IF EXISTS tenants;