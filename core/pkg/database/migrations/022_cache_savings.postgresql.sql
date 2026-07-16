-- +migrate Up
-- Migration: 022_cache_savings
-- Description: Cache hit cost savings for cost dashboard
-- Date: 2026-06-12

CREATE TABLE IF NOT EXISTS cache_savings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    backend_id TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    saved_usd DECIMAL(12, 6) NOT NULL DEFAULT 0,
    cache_layer VARCHAR(8) NOT NULL DEFAULT 'L1',
    tenant_id VARCHAR(64),
    dept_tag VARCHAR(64),
    request_id TEXT,
    pipeline_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cache_savings_created_at ON cache_savings(created_at);
CREATE INDEX IF NOT EXISTS idx_cache_savings_tenant_created ON cache_savings(tenant_id, created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_cache_savings_tenant_created;
DROP INDEX IF EXISTS idx_cache_savings_created_at;
DROP TABLE IF EXISTS cache_savings;