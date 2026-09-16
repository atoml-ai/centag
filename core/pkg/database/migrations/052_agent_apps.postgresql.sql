-- Agent app catalog (C-tier, §5.4)
CREATE TABLE IF NOT EXISTS agent_apps (
    type_id     TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    vendor       TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    sort         INTEGER NOT NULL DEFAULT 0,
    install_url  TEXT NOT NULL DEFAULT '',
    install_hint TEXT NOT NULL DEFAULT '',
    meta_json    TEXT NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
