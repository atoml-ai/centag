-- Migration: 009_plugin_market_registry
-- Description: 创建插件市场注册表和评分表，避免与流水线节点 plugin_registry 表冲突
-- Date: 2026-05-04

-- 插件市场注册表（插件包/分发元数据）
CREATE TABLE IF NOT EXISTS plugin_market_registry (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    description TEXT,
    author VARCHAR(255),
    email VARCHAR(255),
    url VARCHAR(500),
    category VARCHAR(100),
    tags JSON,
    permissions JSON,
    dependencies JSON,
    download_url VARCHAR(500) NOT NULL,
    checksum VARCHAR(128) NOT NULL,
    signature VARCHAR(500),
    size BIGINT DEFAULT 0,
    download_count INT DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0.00,
    rating_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(name, version)
);

-- 插件评分表
CREATE TABLE IF NOT EXISTS plugin_market_ratings (
    id VARCHAR(255) PRIMARY KEY,
    plugin_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    score INT NOT NULL CHECK (score >= 1 AND score <= 5),
    comment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(plugin_id, user_id),
    FOREIGN KEY (plugin_id) REFERENCES plugin_market_registry(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_name ON plugin_market_registry(name);
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_category ON plugin_market_registry(category);
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_author ON plugin_market_registry(author);
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_rating ON plugin_market_registry(rating DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_download_count ON plugin_market_registry(download_count DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_created_at ON plugin_market_registry(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_market_ratings_plugin_id ON plugin_market_ratings(plugin_id);

-- PostgreSQL 特定: 全文搜索索引 (SQLite 不支持)
-- CREATE INDEX IF NOT EXISTS idx_plugin_market_registry_search ON plugin_market_registry USING gin(to_tsvector('english', name || ' ' || coalesce(description, '')));

