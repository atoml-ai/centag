-- Remote agent skill rows synced from Feishu (R13)
CREATE TABLE IF NOT EXISTS remote_skills (
    name        TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT '',
    tools       TEXT NOT NULL DEFAULT '[]',     -- JSON array
    steps       TEXT NOT NULL DEFAULT '[]',     -- JSON array
    system_prompt TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT '',
    edition     TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
