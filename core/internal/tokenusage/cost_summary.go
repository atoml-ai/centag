package tokenusage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"centag/core/internal/billing"
)

// CostSummaryQuery 成本聚合查询参数。
type CostSummaryQuery struct {
	GroupBy  string
	From     time.Time
	To       time.Time
	TenantID string
}

// CostGroup 分组成本条目。
type CostGroup struct {
	Key          string  `json:"key"`
	CostUSD      float64 `json:"cost_usd"`
	Tokens       int64   `json:"tokens"`
	RequestCount int64   `json:"request_count"`
}

// CostSummary 成本汇总响应。
type CostSummary struct {
	TotalCostUSD  float64     `json:"total_cost_usd"`
	TotalTokens   int64       `json:"total_tokens"`
	CacheSavedUSD float64     `json:"cache_saved_usd"`
	Currency      string      `json:"currency"`    // storage unit: always USD
	USDToCNY      float64     `json:"usd_to_cny"`  // display FX; UI may convert to CNY
	Groups        []CostGroup `json:"groups"`
	From          string      `json:"from"`
	To            string      `json:"to"`
	GroupBy       string      `json:"group_by"`
}

func normalizeCostGroupBy(groupBy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(groupBy)) {
	case "", "model":
		return "model", nil
	case "backend", "tenant", "date", "dept", "agent_type":
		return strings.ToLower(strings.TrimSpace(groupBy)), nil
	default:
		return "", fmt.Errorf("unsupported group_by %q", groupBy)
	}
}

func costGroupColumn(groupBy string) (string, error) {
	switch groupBy {
	case "tenant":
		return "COALESCE(NULLIF(tenant_id, ''), 'unknown')", nil
	case "backend":
		return "backend_id", nil
	case "model":
		return "model", nil
	case "date":
		return "", nil // handled via exprDay
	case "dept":
		return "COALESCE(NULLIF(dept_tag, ''), 'unassigned')", nil
	case "agent_type":
		return "COALESCE(NULLIF(agent_type, ''), 'direct')", nil
	default:
		return "", fmt.Errorf("unsupported group_by %q", groupBy)
	}
}

// GetCostSummary 按维度聚合 token_usage 成本。
func (s *Service) GetCostSummary(ctx context.Context, q CostSummaryQuery) (*CostSummary, error) {
	groupBy, err := normalizeCostGroupBy(q.GroupBy)
	if err != nil {
		return nil, err
	}

	from := q.From
	to := q.To
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -30)
	}

	successFilter := "COALESCE(success, TRUE) = TRUE"
	if !s.isPostgres() {
		successFilter = "COALESCE(success, 1) = 1"
	}
	where := []string{"created_at >= $1", "created_at <= $2", successFilter}
	args := []interface{}{from, to}
	argN := 3
	if tid := strings.TrimSpace(q.TenantID); tid != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", argN))
		args = append(args, tid)
		argN++
	}
	whereSQL := strings.Join(where, " AND ")

	totalQuery := s.q(fmt.Sprintf(`
		SELECT
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(total_tokens), 0)
		FROM token_usage
		WHERE %s
	`, whereSQL))

	summary := &CostSummary{
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		GroupBy:  groupBy,
		Currency: billing.DefaultPricingCurrency,
		USDToCNY: billing.USDToCNY(),
		Groups:   []CostGroup{},
	}
	if err := s.db.QueryRowContext(ctx, totalQuery, args...).Scan(&summary.TotalCostUSD, &summary.TotalTokens); err != nil {
		return nil, err
	}

	groupCol, err := costGroupColumn(groupBy)
	if err != nil {
		return nil, err
	}

	var groupQuery string
	if groupBy == "date" {
		dayCol := s.exprDay("created_at")
		groupQuery = s.q(fmt.Sprintf(`
			SELECT
				%s AS grp,
				COALESCE(SUM(cost_usd), 0),
				COALESCE(SUM(total_tokens), 0),
				COUNT(*)
			FROM token_usage
			WHERE %s
			GROUP BY 1
			ORDER BY 1 ASC
		`, dayCol, whereSQL))
	} else {
		groupQuery = s.q(fmt.Sprintf(`
			SELECT
				%s AS grp,
				COALESCE(SUM(cost_usd), 0),
				COALESCE(SUM(total_tokens), 0),
				COUNT(*)
			FROM token_usage
			WHERE %s
			GROUP BY 1
			ORDER BY 2 DESC
		`, groupCol, whereSQL))
	}

	rows, err := s.db.QueryContext(ctx, groupQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var g CostGroup
		if err := rows.Scan(&g.Key, &g.CostUSD, &g.Tokens, &g.RequestCount); err != nil {
			return nil, err
		}
		summary.Groups = append(summary.Groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if saved, err := s.SumCacheSavingsUSD(ctx, from, to, q.TenantID); err == nil {
		summary.CacheSavedUSD = saved
	}
	return summary, nil
}
