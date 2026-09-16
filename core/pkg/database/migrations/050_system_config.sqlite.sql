-- System config KV table (B-tier, §4.6)
-- Low-frequency scalar parameters: exchange rate, scheduler weights, timeouts, etc.
CREATE TABLE IF NOT EXISTS system_config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    value_type  TEXT NOT NULL DEFAULT 'string',  -- number|int|bool|string|json
    scope       TEXT NOT NULL DEFAULT 'core',    -- core|pro
    enabled     INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed built-in keys with defaults
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
