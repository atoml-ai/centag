-- MITM domain whitelist (C-tier, §5.2)
-- Full table coverage with protection (format/dedup/limits/deletion circuit breaker)
CREATE TABLE IF NOT EXISTS mitm_domains (
    domain    TEXT PRIMARY KEY,
    category  TEXT NOT NULL DEFAULT '',
    enabled   INTEGER NOT NULL DEFAULT 1,
    remark    TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
