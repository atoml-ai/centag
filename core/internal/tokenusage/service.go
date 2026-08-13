package tokenusage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	billingpkg "centag/core/pkg/billing"
)

// Service Token 使用计量服务
type Service struct {
	db     *sql.DB
	driver string // database.Init 的插件名：postgresql

	groupColMu      sync.Mutex
	groupColMissing bool // cached: users.group_id absent (open-core), skip future lookups
}

// groupIDForUser resolves the user's group (036). Returns "" on any failure so
// metering never blocks (open-core has no users.group_id column).
func (s *Service) groupIDForUser(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", nil
	}
	s.groupColMu.Lock()
	missing := s.groupColMissing
	s.groupColMu.Unlock()
	if missing {
		return "", nil
	}

	query := s.q(`SELECT group_id FROM users WHERE id = $1`)
	var groupID sql.NullString
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&groupID)
	if err != nil {
		// open-core / pre-036 schema: fail open, remember once to avoid per-request errors
		if isMissingColumn(err) {
			s.groupColMu.Lock()
			s.groupColMissing = true
			s.groupColMu.Unlock()
			return "", nil
		}
		return "", err
	}
	return groupID.String, nil
}

func isMissingColumn(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such column") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "column") && strings.Contains(msg, "does not exist")
}

// UsageRecord Token 使用记录
type UsageRecord struct {
	UserID             int64   `json:"user_id"`
	APIKeyID           int64   `json:"api_key_id"`
	BackendID          string  `json:"backend_id"`
	Model              string  `json:"model"`
	PromptTokens       int     `json:"prompt_tokens"`
	CompletionTokens   int     `json:"completion_tokens"`
	TotalTokens        int     `json:"total_tokens"`
	CostUSD            float64 `json:"cost_usd"` // 成本侧金额（列名历史遗留 *_usd）
	InputCost          float64 `json:"input_cost"`
	OutputCost         float64 `json:"output_cost"`
	CostInputPrice     float64 `json:"cost_input_price"`     // 成本侧 input 单价（USD per 1M tokens）
	CostOutputPrice    float64 `json:"cost_output_price"`    // 成本侧 output 单价（USD per 1M tokens）
	RevenueUSD         float64 `json:"revenue_usd"`          // 收益侧金额
	RevenueInputPrice  float64 `json:"revenue_input_price"`  // 收益侧 input 单价（/1M）
	RevenueOutputPrice float64 `json:"revenue_output_price"` // 收益侧 output 单价（/1M）
	PricingRuleID      int64   `json:"pricing_rule_id"`
	Success            bool    `json:"success"`
	TenantID           string  `json:"tenant_id"`
	GroupID            string  `json:"group_id"` // 036: resolved group (shared metering pool), "" when single-user
	DeptTag            string  `json:"dept_tag"`
	RequestID          string  `json:"request_id"`
	ClientIP           string  `json:"client_ip"`
	AgentType          string  `json:"agent_type"`
	Source             string  `json:"source"`     // 038: "real" or "estimated" (token usage estimation)
	SessionID          string  `json:"session_id"` // 039: 会话 ID（对话记录关联，支持按会话查询计量计价明细）
}

// UsageStats 使用统计
type UsageStats struct {
	TotalPromptTokens     int     `json:"total_prompt_tokens"`
	TotalCompletionTokens int     `json:"total_completion_tokens"`
	TotalTokens           int     `json:"total_tokens"`
	RequestCount          int     `json:"request_count"`
	TotalCostUSD          float64 `json:"total_cost_usd"` // historical name; amount in Currency
	Currency              string  `json:"currency"`
}

// DailyStats 每日统计
type DailyStats struct {
	Date         string `json:"date"`
	TotalTokens  int    `json:"total_tokens"`
	PromptTokens int    `json:"prompt_tokens"`
	CompTokens   int    `json:"completion_tokens"`
	RequestCount int    `json:"request_count"`
	UniqueUsers  int    `json:"unique_users"`
	UniqueModels int    `json:"unique_models"`
}

// ModelStats 模型使用统计
type ModelStats struct {
	Model        string  `json:"model"`
	TotalTokens  int     `json:"total_tokens"`
	RequestCount int     `json:"request_count"`
	AvgTokens    float64 `json:"avg_tokens"`
}

// BackendStats 后端使用统计
type BackendStats struct {
	BackendID    string   `json:"backend_id"`
	TotalTokens  int      `json:"total_tokens"`
	RequestCount int      `json:"request_count"`
	SuccessRate  *float64 `json:"success_rate,omitempty"`
}

// NewService 创建 Token 计量服务。driver 须为 "postgresql"。
func NewService(db *sql.DB, driver string) *Service {
	return &Service{db: db, driver: driver}
}

func (s *Service) isPostgres() bool { return s.driver == "postgresql" }

// cutoffDaysAgo 返回「最近 N 个自然日」窗口起点（含当天则 N 天内从 N-1 天前 0 点算起，这里用「从当前时刻往前推 N 天」与原先 INTERVAL 语义接近）。
func (s *Service) cutoffDaysAgo(days int) time.Time {
	if days < 1 {
		days = 30
	}
	return time.Now().AddDate(0, 0, -days)
}

// exprDay 将时间列截成日历日，便于 GROUP BY（SQLite / PostgreSQL 语法不同）。
func (s *Service) exprDay(column string) string {
	if s.isPostgres() {
		return fmt.Sprintf("(%s::date)", column)
	}
	return fmt.Sprintf("date(%s)", column)
}

// RecordUsage 记录 Token 使用（异步调用）
func (s *Service) RecordUsage(ctx context.Context, record *UsageRecord) error {
	if record == nil {
		return nil
	}
	// Token-bearing rows are successful completions; zero-token rows may record explicit failures.
	success := true
	if record.TotalTokens == 0 {
		success = record.Success
		if success {
			return nil
		}
	}
	if record.TotalTokens > 0 && (record.CostUSD == 0 || record.RevenueUSD == 0) {
		dual := EstimateDualPricing(
			record.BackendID,
			record.Model,
			record.PromptTokens,
			record.CompletionTokens,
		)
		// Team: apply user/group pricing overrides when registered.
		if record.UserID > 0 {
			if applier := billingpkg.GetUserDualPricer(); applier != nil {
				costBD := dual.CostBreakdown.toPkg()
				revBD := dual.RevenueBreakdown.toPkg()
				applier.ApplyOverrides(ctx, record.UserID, record.BackendID, record.Model,
					record.PromptTokens, record.CompletionTokens, &costBD, &revBD)
				dual.CostBreakdown = fromPkgBreakdown(costBD)
				dual.RevenueBreakdown = fromPkgBreakdown(revBD)
			}
		}
		if record.CostUSD == 0 {
			bd := dual.CostBreakdown
			record.CostUSD = bd.TotalCost
			if record.InputCost == 0 {
				record.InputCost = bd.InputCost
			}
			if record.OutputCost == 0 {
				record.OutputCost = bd.OutputCost
			}
			if record.PricingRuleID == 0 {
				record.PricingRuleID = bd.PricingRuleID
			}
			if record.CostInputPrice == 0 {
				record.CostInputPrice = bd.InputPricePerM
			}
			if record.CostOutputPrice == 0 {
				record.CostOutputPrice = bd.OutputPricePerM
			}
		}
		if record.RevenueUSD == 0 {
			rb := dual.RevenueBreakdown
			record.RevenueUSD = rb.TotalCost
			if record.RevenueInputPrice == 0 {
				record.RevenueInputPrice = rb.InputPricePerM
			}
			if record.RevenueOutputPrice == 0 {
				record.RevenueOutputPrice = rb.OutputPricePerM
			}
		}
	}

	normalizedAgentType := normalizeAgentType(record.AgentType)

	// D1 (036): each row records the resolved group so group cost = SUM(cost_usd)
	// WHERE group_id and shared-pool gates can sum per group. Resolve before the
	// tx to avoid holding a pool connection (SQLite single-conn pools deadlock).
	groupID := record.GroupID
	if groupID == "" {
		groupID, _ = s.groupIDForUser(ctx, record.UserID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 插入明细记录（client_ip 列尚未迁移，暂不写入）
	// pricing_rule_id: use NULL when 0 (no rule / legacy fallback)
	// source: 038 添加字段区分真实/估算 token 用量
	var ruleID interface{}
	if record.PricingRuleID > 0 {
		ruleID = record.PricingRuleID
	}
	insertQuery := s.q(`
		INSERT INTO token_usage 
		(user_id, api_key_id, backend_id, model, prompt_tokens, completion_tokens, total_tokens,
		 cost_usd, input_cost, output_cost, cost_input_price, cost_output_price,
		 revenue_usd, revenue_input_price, revenue_output_price,
		 pricing_rule_id, success, tenant_id, group_id, dept_tag, request_id, agent_type, source, session_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`)
	_, err = tx.ExecContext(ctx, insertQuery,
		record.UserID, record.APIKeyID, record.BackendID, record.Model,
		record.PromptTokens, record.CompletionTokens, record.TotalTokens,
		record.CostUSD, record.InputCost, record.OutputCost,
		record.CostInputPrice, record.CostOutputPrice,
		record.RevenueUSD, record.RevenueInputPrice, record.RevenueOutputPrice,
		ruleID, success, nullIfEmpty(record.TenantID), nullIfEmpty(groupID),
		nullIfEmpty(record.DeptTag),
		record.RequestID, normalizedAgentType,
		sourceOrDefault(record.Source), // 038: 数据来源，空值默认为 "real"
		nullIfEmpty(record.SessionID),  // 039: 会话 ID，空值存 NULL
	)
	if err != nil {
		return err
	}

	// 2. 更新统计表（按天聚合）
	var dateStr string
	if time.Now().Hour() < 8 {
		// 如果当前时间小于 8 点，算作前一天
		dateStr = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	} else {
		dateStr = time.Now().Format("2006-01-02")
	}

	upsertQuery := s.recordUsageDailyUpsertSQL()
	_, err = tx.ExecContext(ctx, upsertQuery,
		record.UserID, record.BackendID, record.Model, normalizedAgentType, dateStr,
		record.PromptTokens, record.CompletionTokens, record.TotalTokens,
		record.CostUSD, record.CostInputPrice, record.CostOutputPrice,
		record.RevenueUSD,
		nullIfEmpty(groupID),
	)
	if err != nil {
		return err
	}

	// 3. 回写虚拟 Key 已用额度（金额口径，SQLite/PostgreSQL 均生效）
	// 注意：占位符须按文本出现顺序递增（$1/$2），q() 在 SQLite 下按顺序转成 ? 才不致错位
	if record.APIKeyID > 0 && record.CostUSD > 0 {
		updateQuery := s.q(`UPDATE api_keys SET used_usd = used_usd + $1 WHERE id = $2`)
		_, err = tx.ExecContext(ctx, updateQuery, record.CostUSD, record.APIKeyID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) recordUsageDailyUpsertSQL() string {
	if s.isPostgres() {
		return `
		INSERT INTO token_usage_daily 
		(user_id, backend_id, model, agent_type, date, total_prompt_tokens, total_completion_tokens, total_tokens,
		 total_cost_usd, cost_input_price, cost_output_price, total_revenue_usd, group_id, request_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1)
		ON CONFLICT (user_id, backend_id, model, agent_type, date)
		DO UPDATE SET
			total_prompt_tokens = token_usage_daily.total_prompt_tokens + $6,
			total_completion_tokens = token_usage_daily.total_completion_tokens + $7,
			total_tokens = token_usage_daily.total_tokens + $8,
			total_cost_usd = token_usage_daily.total_cost_usd + $9,
			cost_input_price = token_usage_daily.cost_input_price + $10,
			cost_output_price = token_usage_daily.cost_output_price + $11,
			total_revenue_usd = token_usage_daily.total_revenue_usd + $12,
			request_count = token_usage_daily.request_count + 1,
			updated_at = CURRENT_TIMESTAMP
	`
	}
	return `
		INSERT INTO token_usage_daily 
		(user_id, backend_id, model, agent_type, date, total_prompt_tokens, total_completion_tokens, total_tokens,
		 total_cost_usd, cost_input_price, cost_output_price, total_revenue_usd, group_id, request_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT (user_id, backend_id, model, agent_type, date)
		DO UPDATE SET
			total_prompt_tokens = token_usage_daily.total_prompt_tokens + excluded.total_prompt_tokens,
			total_completion_tokens = token_usage_daily.total_completion_tokens + excluded.total_completion_tokens,
			total_tokens = token_usage_daily.total_tokens + excluded.total_tokens,
			total_cost_usd = token_usage_daily.total_cost_usd + excluded.total_cost_usd,
			cost_input_price = token_usage_daily.cost_input_price + excluded.cost_input_price,
			cost_output_price = token_usage_daily.cost_output_price + excluded.cost_output_price,
			total_revenue_usd = token_usage_daily.total_revenue_usd + excluded.total_revenue_usd,
			request_count = token_usage_daily.request_count + 1,
			updated_at = CURRENT_TIMESTAMP
	`
}

// GetUserUsage 获取用户使用情况
func (s *Service) GetUserUsage(ctx context.Context, userID int64, from, to time.Time) (*UsageStats, error) {
	query := s.q(`
		SELECT 
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COUNT(*),
			COALESCE(SUM(cost_usd), 0)
		FROM token_usage
		WHERE user_id = $1 AND created_at BETWEEN $2 AND $3
	`)

	stats := &UsageStats{Currency: "USD"}
	err := s.db.QueryRowContext(ctx, query, userID, from, to).Scan(
		&stats.TotalPromptTokens, &stats.TotalCompletionTokens,
		&stats.TotalTokens, &stats.RequestCount, &stats.TotalCostUSD,
	)

	return stats, err
}

// GetAggregateUsage sums usage across all users (minimal/personal process-wide view).
func (s *Service) GetAggregateUsage(ctx context.Context, from, to time.Time) (*UsageStats, error) {
	query := s.q(`
		SELECT 
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COUNT(*),
			COALESCE(SUM(cost_usd), 0)
		FROM token_usage
		WHERE created_at BETWEEN $1 AND $2
	`)
	stats := &UsageStats{Currency: "USD"}
	err := s.db.QueryRowContext(ctx, query, from, to).Scan(
		&stats.TotalPromptTokens, &stats.TotalCompletionTokens,
		&stats.TotalTokens, &stats.RequestCount, &stats.TotalCostUSD,
	)
	return stats, err
}

// GetDailyUsage 获取每日使用情况
func (s *Service) GetDailyUsage(ctx context.Context, userID int64, days int) ([]DailyStats, error) {
	cutoff := s.cutoffDaysAgo(days)
	dayCol := s.exprDay("created_at")
	query := s.q(fmt.Sprintf(`
		SELECT 
			%s AS stat_date,
			SUM(total_tokens) as total_tokens,
			SUM(prompt_tokens) as prompt_tokens,
			SUM(completion_tokens) as comp_tokens,
			COUNT(*) as request_count,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT model) as unique_models
		FROM token_usage
		WHERE user_id = $1 AND created_at >= $2
		GROUP BY 1
		ORDER BY 1 DESC
	`, dayCol))

	rows, err := s.db.QueryContext(ctx, query, userID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DailyStats
	for rows.Next() {
		stat := DailyStats{}
		if err := rows.Scan(
			&stat.Date, &stat.TotalTokens, &stat.PromptTokens,
			&stat.CompTokens, &stat.RequestCount, &stat.UniqueUsers, &stat.UniqueModels,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetModelStats 获取模型使用统计
func (s *Service) GetModelStats(ctx context.Context, userID int64, days int) ([]ModelStats, error) {
	cutoff := s.cutoffDaysAgo(days)
	query := s.q(`
		SELECT 
			model,
			SUM(total_tokens) as total_tokens,
			COUNT(*) as request_count,
			AVG(total_tokens) as avg_tokens
		FROM token_usage
		WHERE user_id = $1 AND created_at >= $2
		GROUP BY model
		ORDER BY total_tokens DESC
	`)

	rows, err := s.db.QueryContext(ctx, query, userID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		stat := ModelStats{}
		if err := rows.Scan(&stat.Model, &stat.TotalTokens, &stat.RequestCount, &stat.AvgTokens); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetBackendStats 获取后端使用统计
func (s *Service) GetBackendStats(ctx context.Context, userID int64, days int) ([]BackendStats, error) {
	cutoff := s.cutoffDaysAgo(days)
	successExpr := "1"
	if s.isPostgres() {
		successExpr = "TRUE"
	}
	query := s.q(fmt.Sprintf(`
		SELECT 
			backend_id,
			SUM(total_tokens) as total_tokens,
			COUNT(*) as request_count,
			CASE
				WHEN COUNT(*) = 0 THEN NULL
				ELSE CAST(SUM(CASE WHEN COALESCE(success, %s) THEN 1 ELSE 0 END) AS REAL) / COUNT(*)
			END as success_rate
		FROM token_usage
		WHERE user_id = $1 AND created_at >= $2
		GROUP BY backend_id
		ORDER BY total_tokens DESC
	`, successExpr))

	rows, err := s.db.QueryContext(ctx, query, userID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []BackendStats
	for rows.Next() {
		stat := BackendStats{}
		var successRate sql.NullFloat64
		if err := rows.Scan(&stat.BackendID, &stat.TotalTokens, &stat.RequestCount, &successRate); err != nil {
			return nil, err
		}
		if successRate.Valid {
			rate := successRate.Float64
			stat.SuccessRate = &rate
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetAllUsersUsage 管理员获取所有用户使用情况
func (s *Service) GetAllUsersUsage(ctx context.Context, from, to time.Time) (*UsageStats, error) {
	query := s.q(`
		SELECT 
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COUNT(*)
		FROM token_usage
		WHERE created_at BETWEEN $1 AND $2
	`)

	stats := &UsageStats{}
	err := s.db.QueryRowContext(ctx, query, from, to).Scan(
		&stats.TotalPromptTokens, &stats.TotalCompletionTokens,
		&stats.TotalTokens, &stats.RequestCount,
	)

	return stats, err
}

// GetUserRanking 获取用户 Token 使用排行
func (s *Service) GetUserRanking(ctx context.Context, limit int, days int) ([]struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	TotalTokens int    `json:"total_tokens"`
}, error) {
	if limit < 1 {
		limit = 10
	}
	cutoff := s.cutoffDaysAgo(days)
	query := s.q(`
		SELECT 
			tu.user_id,
			u.username,
			SUM(tu.total_tokens) as total_tokens
		FROM token_usage tu
		JOIN users u ON tu.user_id = u.id
		WHERE tu.created_at >= $1
		GROUP BY tu.user_id, u.username
		ORDER BY total_tokens DESC
		LIMIT $2
	`)

	rows, err := s.db.QueryContext(ctx, query, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rankings []struct {
		UserID      int64  `json:"user_id"`
		Username    string `json:"username"`
		TotalTokens int    `json:"total_tokens"`
	}

	for rows.Next() {
		ranking := struct {
			UserID      int64  `json:"user_id"`
			Username    string `json:"username"`
			TotalTokens int    `json:"total_tokens"`
		}{}
		if err := rows.Scan(&ranking.UserID, &ranking.Username, &ranking.TotalTokens); err != nil {
			return nil, err
		}
		rankings = append(rankings, ranking)
	}

	return rankings, nil
}

// CheckQuota 检查配额限制（用量从 token_usage 实时汇总，不依赖 token_quotas.used_* 是否与定时任务同步）
func (s *Service) CheckQuota(ctx context.Context, userID int64) error {
	q := s.q(`SELECT daily_limit, monthly_limit FROM token_quotas WHERE user_id = $1`)
	var dailyLimit, monthlyLimit int
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&dailyLimit, &monthlyLimit)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if dailyLimit <= 0 && monthlyLimit <= 0 {
		return nil
	}

	now := time.Now()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextDay := startDay.AddDate(0, 0, 1)
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := startMonth.AddDate(0, 1, 0)

	sumQ := s.q(`SELECT COALESCE(SUM(total_tokens), 0) FROM token_usage WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`)

	if dailyLimit > 0 {
		var used int
		if err := s.db.QueryRowContext(ctx, sumQ, userID, startDay, nextDay).Scan(&used); err != nil {
			return err
		}
		if used >= dailyLimit {
			return fmt.Errorf("daily quota exceeded: %d/%d", used, dailyLimit)
		}
	}

	if monthlyLimit > 0 {
		var used int
		if err := s.db.QueryRowContext(ctx, sumQ, userID, startMonth, nextMonth).Scan(&used); err != nil {
			return err
		}
		if used >= monthlyLimit {
			return fmt.Errorf("monthly quota exceeded: %d/%d", used, monthlyLimit)
		}
	}

	return nil
}

// UserQuota 用户配额与当前用量（token_quotas 行）
type UserQuota struct {
	DailyLimit    int  `json:"daily_limit"`
	MonthlyLimit  int  `json:"monthly_limit"`
	UsedToday     int  `json:"used_today"`
	UsedThisMonth int  `json:"used_this_month"`
	HasQuota      bool `json:"has_quota"`
}

// GetUserQuota 读取限额并在有配额行时返回当日/当月实际用量（来自 token_usage）。
func (s *Service) GetUserQuota(ctx context.Context, userID int64) (*UserQuota, error) {
	q := s.q(`SELECT daily_limit, monthly_limit FROM token_quotas WHERE user_id = $1`)
	var row UserQuota
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&row.DailyLimit, &row.MonthlyLimit)
	if err == sql.ErrNoRows {
		return &UserQuota{HasQuota: false}, nil
	}
	if err != nil {
		return nil, err
	}
	row.HasQuota = true

	now := time.Now()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextDay := startDay.AddDate(0, 0, 1)
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := startMonth.AddDate(0, 1, 0)
	sumQ := s.q(`SELECT COALESCE(SUM(total_tokens), 0) FROM token_usage WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`)
	if err := s.db.QueryRowContext(ctx, sumQ, userID, startDay, nextDay).Scan(&row.UsedToday); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, sumQ, userID, startMonth, nextMonth).Scan(&row.UsedThisMonth); err != nil {
		return nil, err
	}
	return &row, nil
}

// SetQuota 设置用户配额
func (s *Service) SetQuota(ctx context.Context, userID int64, dailyLimit, monthlyLimit int) error {
	var query string
	if s.isPostgres() {
		query = `
		INSERT INTO token_quotas (user_id, daily_limit, monthly_limit)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			daily_limit = $2,
			monthly_limit = $3,
			updated_at = CURRENT_TIMESTAMP
	`
	} else {
		query = `
		INSERT INTO token_quotas (user_id, daily_limit, monthly_limit)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			daily_limit = excluded.daily_limit,
			monthly_limit = excluded.monthly_limit,
			updated_at = CURRENT_TIMESTAMP
	`
	}

	_, err := s.db.ExecContext(ctx, query, userID, dailyLimit, monthlyLimit)
	return err
}

// ResetQuota 重置用户配额
func (s *Service) ResetQuota(ctx context.Context, userID int64) error {
	query := s.q(`
		UPDATE token_quotas
		SET used_today = 0, used_this_month = 0, reset_date = CURRENT_DATE
		WHERE user_id = $1
	`)

	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// UpdateQuotaUsage 更新配额使用量（定时任务调用）
func (s *Service) UpdateQuotaUsage(ctx context.Context) error {
	var queryToday, queryMonth string
	if s.isPostgres() {
		queryToday = `
		UPDATE token_quotas tq
		SET used_today = (
			SELECT COALESCE(SUM(total_tokens), 0)
			FROM token_usage tu
			WHERE tu.user_id = tq.user_id
			AND (tu.created_at::date) = CURRENT_DATE
		)
	`
		queryMonth = `
		UPDATE token_quotas tq
		SET used_this_month = (
			SELECT COALESCE(SUM(total_tokens), 0)
			FROM token_usage tu
			WHERE tu.user_id = tq.user_id
			AND DATE_TRUNC('month', tu.created_at) = DATE_TRUNC('month', CURRENT_TIMESTAMP)
		)
	`
	} else {
		queryToday = `
		UPDATE token_quotas tq
		SET used_today = (
			SELECT COALESCE(SUM(total_tokens), 0)
			FROM token_usage tu
			WHERE tu.user_id = tq.user_id
			AND date(tu.created_at) = date('now')
		)
	`
		queryMonth = `
		UPDATE token_quotas tq
		SET used_this_month = (
			SELECT COALESCE(SUM(total_tokens), 0)
			FROM token_usage tu
			WHERE tu.user_id = tq.user_id
			AND strftime('%Y-%m', tu.created_at) = strftime('%Y-%m', 'now')
		)
	`
	}

	if _, err := s.db.ExecContext(ctx, queryToday); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, queryMonth)
	return err
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

// sourceOrDefault returns the source string, defaulting to "real" if empty.
func sourceOrDefault(s string) string {
	if strings.TrimSpace(s) == "" {
		return "real"
	}
	return strings.TrimSpace(s)
}

func normalizeAgentType(agentType string) string {
	if strings.TrimSpace(agentType) == "" {
		return "direct"
	}
	return strings.TrimSpace(agentType)
}
