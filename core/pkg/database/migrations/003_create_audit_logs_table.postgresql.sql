-- ============================================
-- Migration: 003_create_audit_logs_table
-- Description: 创建审计日志表，记录所有管理操作
-- Created: 2026-03-22
-- Author: centag team
-- ============================================

-- +migrate Up
-- 创建审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,  -- CREATE, UPDATE, DELETE, LOGIN, LOGOUT
    resource_type VARCHAR(50),     -- USER, API_KEY, BACKEND, CONFIG
    resource_id BIGINT,
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    status VARCHAR(20) DEFAULT 'success',  -- success, failure
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);

-- 添加注释
COMMENT ON TABLE audit_logs IS '审计日志表，记录所有管理操作';
COMMENT ON COLUMN audit_logs.action IS '操作类型：CREATE, UPDATE, DELETE, LOGIN, LOGOUT';
COMMENT ON COLUMN audit_logs.resource_type IS '资源类型：USER, API_KEY, BACKEND, CONFIG';
COMMENT ON COLUMN audit_logs.status IS '操作状态：success, failure';

-- +migrate Down
-- 回滚：删除审计日志表
DROP TABLE IF EXISTS audit_logs CASCADE;
