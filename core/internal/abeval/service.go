package abeval

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Record A/B evaluation result persisted by the score aggregator.
type Record struct {
	PipelineID     string
	RequestID      string
	Question       string
	Strategy       string
	WinnerNode     string
	CandidateANode string
	CandidateBNode string
	ModelA         string
	ModelB         string
	ScoreA         float64
	ScoreB         float64
	LatencyAMs     int64
	LatencyBMs     int64
	CostAUSD       float64
	CostBUSD       float64
}

// Result row returned by list API.
type Result struct {
	ID             int64     `json:"id"`
	PipelineID     string    `json:"pipeline_id"`
	RequestID      string    `json:"request_id"`
	Question       string    `json:"question"`
	Strategy       string    `json:"strategy"`
	WinnerNode     string    `json:"winner_node"`
	CandidateANode string    `json:"candidate_a_node"`
	CandidateBNode string    `json:"candidate_b_node"`
	ModelA         string    `json:"model_a"`
	ModelB         string    `json:"model_b"`
	ScoreA         float64   `json:"score_a"`
	ScoreB         float64   `json:"score_b"`
	LatencyAMs     int64     `json:"latency_a_ms"`
	LatencyBMs     int64     `json:"latency_b_ms"`
	CostAUSD       float64   `json:"cost_a_usd"`
	CostBUSD       float64   `json:"cost_b_usd"`
	CreatedAt      time.Time `json:"created_at"`
}

// Summary aggregates win rate and averages for dashboards.
type Summary struct {
	TotalComparisons int                `json:"total_comparisons"`
	From             string             `json:"from"`
	To               string             `json:"to"`
	ModelWins        []ModelWinStat     `json:"model_wins"`
	AvgScoreByModel  []ModelScoreStat   `json:"avg_score_by_model"`
	AvgLatencyByModel []ModelLatencyStat `json:"avg_latency_by_model"`
	AvgCostByModel   []ModelCostStat    `json:"avg_cost_by_model"`
}

type ModelWinStat struct {
	Model    string  `json:"model"`
	Wins     int     `json:"wins"`
	WinRate  float64 `json:"win_rate"`
}

type ModelScoreStat struct {
	Model    string  `json:"model"`
	AvgScore float64 `json:"avg_score"`
}

type ModelLatencyStat struct {
	Model       string  `json:"model"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type ModelCostStat struct {
	Model      string  `json:"model"`
	AvgCostUSD float64 `json:"avg_cost_usd"`
}

// Service persists and queries A/B evaluation history.
type Service struct {
	db     *sql.DB
	driver string
}

func NewService(db *sql.DB, driver string) *Service {
	return &Service{db: db, driver: driver}
}

func (s *Service) isPostgres() bool { return s.driver == "postgresql" }

// RecordResult inserts one comparison outcome.
func (s *Service) RecordResult(ctx context.Context, rec *Record) error {
	if s == nil || s.db == nil || rec == nil {
		return nil
	}
	strategy := rec.Strategy
	if strategy == "" {
		strategy = "score"
	}
	var q string
	if s.isPostgres() {
		q = `
			INSERT INTO ab_eval_results (
				pipeline_id, request_id, question, strategy,
				winner_node, candidate_a_node, candidate_b_node,
				model_a, model_b, score_a, score_b,
				latency_a_ms, latency_b_ms, cost_a_usd, cost_b_usd
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`
	} else {
		q = `
			INSERT INTO ab_eval_results (
				pipeline_id, request_id, question, strategy,
				winner_node, candidate_a_node, candidate_b_node,
				model_a, model_b, score_a, score_b,
				latency_a_ms, latency_b_ms, cost_a_usd, cost_b_usd
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	}
	_, err := s.db.ExecContext(ctx, q,
		rec.PipelineID, rec.RequestID, rec.Question, strategy,
		rec.WinnerNode, rec.CandidateANode, rec.CandidateBNode,
		rec.ModelA, rec.ModelB, rec.ScoreA, rec.ScoreB,
		rec.LatencyAMs, rec.LatencyBMs, rec.CostAUSD, rec.CostBUSD,
	)
	return err
}

// ListResults returns recent comparison rows.
func (s *Service) ListResults(ctx context.Context, from, to time.Time, limit int) ([]Result, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("ab eval service unavailable")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var q string
	if s.isPostgres() {
		q = `
			SELECT id, pipeline_id, request_id, question, strategy,
				winner_node, candidate_a_node, candidate_b_node,
				model_a, model_b, score_a, score_b,
				latency_a_ms, latency_b_ms, cost_a_usd, cost_b_usd, created_at
			FROM ab_eval_results
			WHERE created_at >= $1 AND created_at <= $2
			ORDER BY created_at DESC
			LIMIT $3`
	} else {
		q = `
			SELECT id, pipeline_id, request_id, question, strategy,
				winner_node, candidate_a_node, candidate_b_node,
				model_a, model_b, score_a, score_b,
				latency_a_ms, latency_b_ms, cost_a_usd, cost_b_usd, created_at
			FROM ab_eval_results
			WHERE created_at >= ? AND created_at <= ?
			ORDER BY created_at DESC
			LIMIT ?`
	}
	rows, err := s.db.QueryContext(ctx, q, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Result, 0)
	for rows.Next() {
		var r Result
		if err := rows.Scan(
			&r.ID, &r.PipelineID, &r.RequestID, &r.Question, &r.Strategy,
			&r.WinnerNode, &r.CandidateANode, &r.CandidateBNode,
			&r.ModelA, &r.ModelB, &r.ScoreA, &r.ScoreB,
			&r.LatencyAMs, &r.LatencyBMs, &r.CostAUSD, &r.CostBUSD, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetSummary aggregates win rate and averages for the date window.
func (s *Service) GetSummary(ctx context.Context, from, to time.Time) (*Summary, error) {
	results, err := s.ListResults(ctx, from, to, 500)
	if err != nil {
		return nil, err
	}
	summary := &Summary{
		TotalComparisons: len(results),
		From:             from.Format("2006-01-02"),
		To:               to.Format("2006-01-02"),
	}

	wins := map[string]int{}
	scores := map[string][]float64{}
	latencies := map[string][]int64{}
	costs := map[string][]float64{}

	for _, r := range results {
		if r.WinnerNode == r.CandidateANode && r.ModelA != "" {
			wins[r.ModelA]++
		} else if r.WinnerNode == r.CandidateBNode && r.ModelB != "" {
			wins[r.ModelB]++
		}
		if r.ModelA != "" {
			scores[r.ModelA] = append(scores[r.ModelA], r.ScoreA)
			latencies[r.ModelA] = append(latencies[r.ModelA], r.LatencyAMs)
			costs[r.ModelA] = append(costs[r.ModelA], r.CostAUSD)
		}
		if r.ModelB != "" {
			scores[r.ModelB] = append(scores[r.ModelB], r.ScoreB)
			latencies[r.ModelB] = append(latencies[r.ModelB], r.LatencyBMs)
			costs[r.ModelB] = append(costs[r.ModelB], r.CostBUSD)
		}
	}

	totalWins := 0
	for _, c := range wins {
		totalWins += c
	}
	for model, c := range wins {
		rate := 0.0
		if totalWins > 0 {
			rate = float64(c) / float64(totalWins)
		}
		summary.ModelWins = append(summary.ModelWins, ModelWinStat{
			Model: model, Wins: c, WinRate: rate,
		})
	}
	for model, vals := range scores {
		summary.AvgScoreByModel = append(summary.AvgScoreByModel, ModelScoreStat{
			Model: model, AvgScore: avgFloat(vals),
		})
	}
	for model, vals := range latencies {
		summary.AvgLatencyByModel = append(summary.AvgLatencyByModel, ModelLatencyStat{
			Model: model, AvgLatencyMs: avgInt64(vals),
		})
	}
	for model, vals := range costs {
		summary.AvgCostByModel = append(summary.AvgCostByModel, ModelCostStat{
			Model: model, AvgCostUSD: avgFloat(vals),
		})
	}
	return summary, nil
}

func avgFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func avgInt64(vals []int64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

