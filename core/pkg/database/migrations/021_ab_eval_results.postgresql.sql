-- +migrate Up
-- Migration: 021_ab_eval_results
-- Description: A/B aggregator score evaluation history
-- Date: 2026-06-12

CREATE TABLE IF NOT EXISTS ab_eval_results (
    id BIGSERIAL PRIMARY KEY,
    pipeline_id VARCHAR(128),
    request_id VARCHAR(128),
    question TEXT,
    strategy VARCHAR(32) NOT NULL DEFAULT 'score',
    winner_node VARCHAR(64),
    candidate_a_node VARCHAR(64),
    candidate_b_node VARCHAR(64),
    model_a VARCHAR(128),
    model_b VARCHAR(128),
    score_a DECIMAL(8,4),
    score_b DECIMAL(8,4),
    latency_a_ms BIGINT DEFAULT 0,
    latency_b_ms BIGINT DEFAULT 0,
    cost_a_usd DECIMAL(12,6) DEFAULT 0,
    cost_b_usd DECIMAL(12,6) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ab_eval_results_created_at ON ab_eval_results(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ab_eval_results_pipeline ON ab_eval_results(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_ab_eval_results_winner ON ab_eval_results(winner_node);

-- +migrate Down
DROP INDEX IF EXISTS idx_ab_eval_results_winner;
DROP INDEX IF EXISTS idx_ab_eval_results_pipeline;
DROP INDEX IF EXISTS idx_ab_eval_results_created_at;
DROP TABLE IF EXISTS ab_eval_results;