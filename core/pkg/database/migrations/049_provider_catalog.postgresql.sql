-- Provider catalog persistence (R14)
CREATE TABLE IF NOT EXISTS provider_catalog (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL DEFAULT '',
    provider_type   TEXT NOT NULL DEFAULT '',
    base_url        TEXT NOT NULL DEFAULT '',
    env_key         TEXT NOT NULL DEFAULT '',
    icon            TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    default_models  TEXT NOT NULL DEFAULT '[]',   -- JSON array
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
