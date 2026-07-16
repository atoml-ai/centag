-- Migration: 010_tenant
-- Description: 创建租户表和 API Key 表
-- Date: 2026-05-04

-- 租户表
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'active', -- active, suspended, deleted
    quota JSON,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 租户 API Key 表
CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    description TEXT,
    status VARCHAR(50) DEFAULT 'active', -- active, revoked
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 租户使用量统计表
CREATE TABLE IF NOT EXISTS tenant_usage (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    requests_count INT DEFAULT 0,
    tokens_used BIGINT DEFAULT 0,
    concurrent_requests INT DEFAULT 0,
    storage_used_mb BIGINT DEFAULT 0,
    cpu_usage_percent DECIMAL(5,2) DEFAULT 0.00,
    memory_usage_mb BIGINT DEFAULT 0,
    
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_tenant_id ON tenant_api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_api_key ON tenant_api_keys(api_key);
CREATE INDEX IF NOT EXISTS idx_tenant_usage_tenant_id ON tenant_usage(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_usage_timestamp ON tenant_usage(timestamp);

-- 更新时间触发器
CREATE TRIGGER IF NOT EXISTS update_tenants_updated_at
AFTER UPDATE ON tenants
FOR EACH ROW
BEGIN
    UPDATE tenants SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_tenant_api_keys_updated_at
AFTER UPDATE ON tenant_api_keys
FOR EACH ROW
BEGIN
    UPDATE tenant_api_keys SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
