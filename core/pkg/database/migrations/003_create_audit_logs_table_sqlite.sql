-- ============================================
-- Migration: 003_create_audit_logs_table
-- Description: 创建审计日志表
-- ============================================

-- +migrate Up
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id INTEGER,
    details TEXT,
    ip_address TEXT,
    user_agent TEXT,
    status TEXT DEFAULT 'success',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);

-- +migrate Down
DROP TABLE IF EXISTS audit_logs;
