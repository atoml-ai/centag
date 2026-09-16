-- System config KV table (B-tier, §4.6)
-- Handles existing table from migration 001 (columns: config_key, config_value)
-- by creating a new table, migrating data, and dropping the old one.

-- Step 1: Create new table with correct schema
CREATE TABLE IF NOT EXISTS system_config_new (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    value_type  TEXT NOT NULL DEFAULT 'string',  -- number|int|bool|string|json
    scope       TEXT NOT NULL DEFAULT 'core',    -- core|pro
    enabled     INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Migrate data from old table if it exists
INSERT OR IGNORE INTO system_config_new (key, value, value_type, scope, description, updated_at)
SELECT config_key, config_value, 'string', 'core', COALESCE(description, ''), updated_at
FROM system_config
WHERE config_key IS NOT NULL;

-- Step 3: Drop old table
DROP TABLE IF EXISTS system_config;

-- Step 4: Rename new table
ALTER TABLE system_config_new RENAME TO system_config;

-- Step 5: Seed built-in keys with defaults (only if not already present)
INSERT OR IGNORE INTO system_config (key, value, value_type, scope, description) VALUES
    ('billing.usd_to_cny',            '7.2',   'number', 'core', 'USD to CNY exchange rate'),
    ('scheduler.price_weight',        '20',    'int',    'core', 'Scheduler price weight'),
    ('scheduler.performance_weight',  '20',    'int',    'core', 'Scheduler performance weight'),
    ('scheduler.quality_weight',      '25',    'int',    'core', 'Scheduler quality weight'),
    ('scheduler.latency_weight',      '15',    'int',    'core', 'Scheduler latency weight'),
    ('scheduler.privacy_weight',      '10',    'int',    'core', 'Scheduler privacy weight'),
    ('scheduler.match_weight',        '10',    'int',    'core', 'Scheduler match weight'),
    ('pipeline.default_timeout_ms',   '30000', 'int',    'core', 'Default pipeline timeout in ms'),
    ('retry.codes',                   '',      'string', 'core', 'Comma-separated retryable HTTP status codes');
