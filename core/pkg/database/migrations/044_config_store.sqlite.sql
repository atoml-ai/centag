-- Config data from remote configsync (Feishu)
CREATE TABLE IF NOT EXISTS config_store (
    config_key TEXT PRIMARY KEY,
    config_value TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_config_store_key ON config_store(config_key);
