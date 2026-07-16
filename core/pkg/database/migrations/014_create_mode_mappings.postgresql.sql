-- 创建模式映射表
-- 用于配置快捷码、模式名与流水线ID的映射关系

CREATE TABLE IF NOT EXISTS mode_mappings (
    id SERIAL PRIMARY KEY,
    shortcut_code VARCHAR(10) NOT NULL UNIQUE,
    mode_name VARCHAR(50) NOT NULL UNIQUE,
    pipeline_id VARCHAR(100) NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 插入默认映射数据
INSERT INTO mode_mappings (shortcut_code, mode_name, pipeline_id, description, enabled, sort_order) VALUES
('#d', 'direct-backend', 'direct-backend', '直连后端', TRUE, 1),
('#s', 'smart-scheduling', 'smart-scheduling', '系统调度/智能调度', TRUE, 2),
('#f', 'fallback', 'fallback-mode', '降级模式', TRUE, 3),
('#o', 'optimize', 'optimize-mode', '优化模式', TRUE, 4),
('#a', 'audit', 'audit-mode', '审核模式', TRUE, 5),
('#m', 'model-matching', 'model-matching', '模型匹配', TRUE, 6),
('#r', 'router', 'router-mode', '路由模式', TRUE, 7),
('#ag', 'aggregator', 'aggregator-mode', '聚合模式', TRUE, 8),
('#l', 'translate', 'translate-mode', '翻译模式', TRUE, 9),
('#p', 'pipeline', 'pipeline-mode', '通用流水线', TRUE, 10),
('#t', 'transparent', 'transparent-proxy', '透明代理', TRUE, 11),
('#c', 'intent', 'router-mode', '意图分类', TRUE, 12)
ON CONFLICT (shortcut_code) DO NOTHING;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_mode_mappings_mode_name ON mode_mappings(mode_name);
CREATE INDEX IF NOT EXISTS idx_mode_mappings_pipeline_id ON mode_mappings(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_mode_mappings_enabled ON mode_mappings(enabled);
