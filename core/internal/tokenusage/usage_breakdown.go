package tokenusage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UsageBreakdownRecord is a per-(backend_id, model) metering + billing row.
// cost_input_price / cost_output_price are USD per 1M tokens.
type UsageBreakdownRecord struct {
	BackendID       string  `json:"backend_id"`
	Model           string  `json:"model"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	CostInputPrice  float64 `json:"cost_input_price"`
	CostOutputPrice float64 `json:"cost_output_price"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	TotalCost       float64 `json:"total_cost"`
}

// UsageBreakdownSummary aggregates totals across records.
type UsageBreakdownSummary struct {
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
}

// UsageBreakdown is the response shape for user-scoped detailed metering/billing.
type UsageBreakdown struct {
	Records []UsageBreakdownRecord `json:"records"`
	Summary UsageBreakdownSummary  `json:"summary"`
	From    string                 `json:"from"`
	To      string                 `json:"to"`
}

// GetUsageBreakdown returns per-(backend_id, model) metering + billing rows for a
// user within [from, to]. Success-only rows are aggregated; unit prices are the
// highest rate recorded in the window for that (backend, model).
func (s *Service) GetUsageBreakdown(ctx context.Context, userID int64, from, to time.Time) (*UsageBreakdown, error) {
	successFilter := "COALESCE(success, TRUE) = TRUE"
	if !s.isPostgres() {
		successFilter = "COALESCE(success, 1) = 1"
	}
	query := s.q(fmt.Sprintf(`
		SELECT
			backend_id,
			model,
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(MAX(cost_input_price), 0),
			COALESCE(MAX(cost_output_price), 0),
			COALESCE(SUM(input_cost), 0),
			COALESCE(SUM(output_cost), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM token_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND %s
		GROUP BY backend_id, model
		ORDER BY SUM(cost_usd) DESC, SUM(total_tokens) DESC
	`, successFilter))

	rows, err := s.db.QueryContext(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query usage breakdown: %w", err)
	}
	defer rows.Close()

	out := &UsageBreakdown{
		Records: []UsageBreakdownRecord{},
		From:    from.Format("2006-01-02"),
		To:      to.Format("2006-01-02"),
	}
	var totalInput, totalOutput, totalTokens int64
	var totalCost float64
	for rows.Next() {
		var r UsageBreakdownRecord
		if err := rows.Scan(
			&r.BackendID, &r.Model, &r.RequestCount,
			&r.InputTokens, &r.OutputTokens, &r.TotalTokens,
			&r.CostInputPrice, &r.CostOutputPrice,
			&r.InputCost, &r.OutputCost, &r.TotalCost,
		); err != nil {
			return nil, fmt.Errorf("scan usage breakdown: %w", err)
		}
		out.Records = append(out.Records, r)
		totalInput += r.InputTokens
		totalOutput += r.OutputTokens
		totalTokens += r.TotalTokens
		totalCost += r.TotalCost
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage breakdown: %w", err)
	}

	out.Summary = UsageBreakdownSummary{
		TotalInputTokens:  totalInput,
		TotalOutputTokens: totalOutput,
		TotalTokens:       totalTokens,
		TotalCost:         totalCost,
	}
	return out, nil
}

// SessionUsageSummary aggregates one session's metering + billing rows.
// Unit prices are the highest rate recorded within the session (consistent with
// GetUsageBreakdown); costs are summed.
type SessionUsageSummary struct {
	SessionID       string  `json:"session_id"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	CostInputPrice  float64 `json:"cost_input_price"`
	CostOutputPrice float64 `json:"cost_output_price"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	TotalCost       float64 `json:"total_cost"`
}

// GetSessionsUsageBreakdown returns per-session metering + billing summaries for
// the given session IDs (success-only). Sessions with no usage are absent from the map.
func (s *Service) GetSessionsUsageBreakdown(ctx context.Context, userID int64, sessionIDs []string) (map[string]*SessionUsageSummary, error) {
	out := make(map[string]*SessionUsageSummary, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return out, nil
	}
	successFilter := "COALESCE(success, TRUE) = TRUE"
	if !s.isPostgres() {
		successFilter = "COALESCE(success, 1) = 1"
	}
	placeholders := make([]string, len(sessionIDs))
	args := make([]any, 0, len(sessionIDs)+1)
	args = append(args, userID)
	for i, id := range sessionIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}
	query := s.q(fmt.Sprintf(`
		SELECT
			session_id,
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(MAX(cost_input_price), 0),
			COALESCE(MAX(cost_output_price), 0),
			COALESCE(SUM(input_cost), 0),
			COALESCE(SUM(output_cost), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM token_usage
		WHERE user_id = $1 AND session_id IN (%s) AND %s
		GROUP BY session_id
	`, strings.Join(placeholders, ","), successFilter))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions usage breakdown: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r SessionUsageSummary
		if err := rows.Scan(
			&r.SessionID, &r.RequestCount,
			&r.InputTokens, &r.OutputTokens, &r.TotalTokens,
			&r.CostInputPrice, &r.CostOutputPrice,
			&r.InputCost, &r.OutputCost, &r.TotalCost,
		); err != nil {
			return nil, fmt.Errorf("scan sessions usage breakdown: %w", err)
		}
		out[r.SessionID] = &r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions usage breakdown: %w", err)
	}
	return out, nil
}
