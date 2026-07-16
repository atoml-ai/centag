-- +migrate Up
-- Migration: 021_ab_eval_results
-- Description: A/B aggregator score evaluation history
-- Date: 2026-06-12

CREATE TABLE IF NOT EXISTS ab_eval_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id TEXT,
    request_id TEXT,
    question TEXT,
    strategy TEXT NOT NULL DEFAULT 'score',
    winner_node TEXT,
    candidate_a_node TEXT,
    candidate_b_node TEXT,
    model_a TEXT,
    model_b TEXT,
    score_a REAL,
    score_b REAL,
    latency_a_ms INTEGER DEFAULT 0,
    latency_b_ms INTEGER DEFAULT 0,
    cost_a_usd REAL DEFAULT 0,
    cost_b_usd REAL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ab_eval_results_created_at ON ab_eval_results(created_at);
CREATE INDEX IF NOT EXISTS idx_ab_eval_results_pipeline ON ab_eval_results(pipeline_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_ab_eval_results_pipeline;
DROP INDEX IF EXISTS idx_ab_eval_results_created_at;
DROP TABLE IF EXISTS ab_eval_results;