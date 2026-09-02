-- Pipeline templates from remote public configsync snapshot
CREATE TABLE IF NOT EXISTS pipeline_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    shortcut_code TEXT,
    schema_version TEXT,
    version TEXT,
    edition TEXT DEFAULT 'all',
    nodes TEXT NOT NULL DEFAULT '[]',
    global_config TEXT,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipeline_templates_shortcut ON pipeline_templates(shortcut_code);
CREATE INDEX IF NOT EXISTS idx_pipeline_templates_edition ON pipeline_templates(edition);
