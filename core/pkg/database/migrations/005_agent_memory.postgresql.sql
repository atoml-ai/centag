-- ============================================
-- Migration: 005_agent_memory
-- Description: OpenClaw 云记忆文档表 + pgvector 分块表（与语义缓存 vector_cache 隔离）
-- ============================================

-- +migrate Up
CREATE TABLE IF NOT EXISTS agent_memory_docs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL DEFAULT 'main',
    path TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_rev BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_agent_memory_docs_user_ns_path UNIQUE (user_id, namespace, path)
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_docs_user_ns
    ON agent_memory_docs (user_id, namespace)
    WHERE deleted_at IS NULL;

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS agent_memory_chunks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL DEFAULT 'main',
    path TEXT NOT NULL,
    chunk_index INT NOT NULL,
    line_start INT NOT NULL DEFAULT 1,
    line_end INT NOT NULL DEFAULT 1,
    chunk_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_chunks_user_ns_path
    ON agent_memory_chunks (user_id, namespace, path);

CREATE INDEX IF NOT EXISTS idx_agent_memory_chunks_hnsw
    ON agent_memory_chunks USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);

-- +migrate Down
DROP TABLE IF EXISTS agent_memory_chunks;
DROP TABLE IF EXISTS agent_memory_docs;
