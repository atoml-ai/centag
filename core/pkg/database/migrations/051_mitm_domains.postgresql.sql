-- MITM domain whitelist (C-tier, §5.2)
CREATE TABLE IF NOT EXISTS mitm_domains (
    domain    TEXT PRIMARY KEY,
    category  TEXT NOT NULL DEFAULT '',
    enabled   BOOLEAN NOT NULL DEFAULT TRUE,
    remark    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
