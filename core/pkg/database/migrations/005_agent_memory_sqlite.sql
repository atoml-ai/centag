-- ============================================
-- Migration: 005_agent_memory
-- Description: Agent Memory 文档表（SQLite 基础版）
-- Note: SQLite 不支持向量类型，仅存储文档元数据
--       向量召回功能需要 PostgreSQL + pgvector
-- ============================================

-- +migrate Up
CREATE TABLE IF NOT EXISTS agent_memory_docs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL DEFAULT 'main',
    path TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_rev INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    CONSTRAINT uq_agent_memory_docs_user_ns_path UNIQUE (user_id, namespace, path)
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_docs_user_ns
    ON agent_memory_docs (user_id, namespace)
    WHERE deleted_at IS NULL;

-- 注意：SQLite 不支持向量类型
-- agent_memory_chunks 表仅在 PostgreSQL 中创建

-- +migrate Down
DROP TABLE IF EXISTS agent_memory_docs;
