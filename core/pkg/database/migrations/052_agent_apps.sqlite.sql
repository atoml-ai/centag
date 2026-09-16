-- Agent app catalog (C-tier, §5.4)
-- Overlay on TemplateRegistry: only display/guide/sort fields are synced.
CREATE TABLE IF NOT EXISTS agent_apps (
    type_id     TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    vendor       TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',  -- cli|tui|web|desktop
    enabled      INTEGER NOT NULL DEFAULT 1,
    sort         INTEGER NOT NULL DEFAULT 0,
    install_url  TEXT NOT NULL DEFAULT '',
    install_hint TEXT NOT NULL DEFAULT '',
    meta_json    TEXT NOT NULL DEFAULT '{}', -- ui_guide, companion_cli(install only), verified_*
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
