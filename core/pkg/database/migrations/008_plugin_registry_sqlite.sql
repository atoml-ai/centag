-- +migrate Up
-- 插件注册表
CREATE TABLE IF NOT EXISTS plugin_registry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    implementation TEXT UNIQUE NOT NULL,
    kind TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    descriptor_json JSON,
    source TEXT NOT NULL DEFAULT 'unknown',
    enabled BOOLEAN DEFAULT TRUE,
    signature_status TEXT DEFAULT 'none',
    last_health_check TIMESTAMP,
    health_status TEXT DEFAULT 'unknown',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_plugin_registry_enabled ON plugin_registry(enabled);
CREATE INDEX IF NOT EXISTS idx_plugin_registry_implementation ON plugin_registry(implementation);
CREATE INDEX IF NOT EXISTS idx_plugin_registry_health ON plugin_registry(health_status);

-- +migrate Down
DROP INDEX IF EXISTS idx_plugin_registry_health;
DROP INDEX IF EXISTS idx_plugin_registry_implementation;
DROP INDEX IF EXISTS idx_plugin_registry_enabled;
DROP TABLE IF EXISTS plugin_registry;
