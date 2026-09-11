-- 自进化提案日志（self-evolution-paradigm / agent_evolution_log）
-- 首版字段：id, session_id, proposal(json), status, applied_at, effect_measure(json), rollback_of
CREATE TABLE IF NOT EXISTS agent_evolution_log (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    target TEXT NOT NULL,
    proposal TEXT NOT NULL,
    status TEXT NOT NULL,
    applied_at TEXT,
    effect_measure TEXT,
    rollback_of TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_evolution_log_session ON agent_evolution_log(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_evolution_log_status ON agent_evolution_log(status);
