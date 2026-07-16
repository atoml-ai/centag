package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DecisionLogService provides scheduler decision logging
type DecisionLogService struct {
	db     *sql.DB
	driver string

	// Sampling rate (0.0 to 1.0, 1.0 = log all)
	sampleRate float64
}

// DecisionLogEntry represents a scheduler decision log entry
type DecisionLogEntry struct {
	ID        int64
	RequestID string
	UserID    int64
	TenantID  string
	Model     string
	Backend   string
	Strategy  string
	Score     float64
	Reason    string
	CreatedAt time.Time
}

// NewDecisionLogService creates a new DecisionLogService
func NewDecisionLogService(db *sql.DB, driver string) *DecisionLogService {
	return &DecisionLogService{
		db:         db,
		driver:     driver,
		sampleRate: 0.1, // Default 10% sampling
	}
}

// SetSampleRate sets the sampling rate for decision logging
func (s *DecisionLogService) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	s.sampleRate = rate
}

// LogDecision logs a scheduler decision
func (s *DecisionLogService) LogDecision(ctx context.Context, entry *DecisionLogEntry) error {
	// Apply sampling
	if s.sampleRate < 1.0 && entry.UserID%100 > int64(s.sampleRate*100) {
		return nil
	}

	query := s.buildInsertQuery()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	args := []interface{}{
		entry.RequestID, entry.UserID, entry.TenantID, entry.Model,
		entry.Backend, entry.Strategy, entry.Score, entry.Reason, entry.CreatedAt,
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("log scheduler decision: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("log scheduler decision last insert id: %w", err)
	}
	entry.ID = id

	return nil
}

// QueryByRequestID queries decisions by request ID
func (s *DecisionLogService) QueryByRequestID(ctx context.Context, requestID string) ([]*DecisionLogEntry, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE request_id = ? 
	          ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("query decisions by request_id: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// QueryByTenantID queries decisions by tenant ID with pagination
func (s *DecisionLogService) QueryByTenantID(ctx context.Context, tenantID string, limit, offset int) ([]*DecisionLogEntry, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE tenant_id = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query decisions by tenant_id: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// QueryByTimeRange queries decisions within a time range
func (s *DecisionLogService) QueryByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*DecisionLogEntry, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE created_at >= ? AND created_at <= ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query decisions by time range: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// QueryByModel queries decisions by model
func (s *DecisionLogService) QueryByModel(ctx context.Context, model string, limit, offset int) ([]*DecisionLogEntry, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE model = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, model, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query decisions by model: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// DeleteOldDecisions deletes decisions older than the specified duration
func (s *DecisionLogService) DeleteOldDecisions(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM scheduler_decisions WHERE created_at < ?`
	cutoff := time.Now().Add(-olderThan)

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old decisions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete old decisions rows affected: %w", err)
	}
	return rowsAffected, nil
}

// GetDecisionStats gets statistics about scheduler decisions
func (s *DecisionLogService) GetDecisionStats(ctx context.Context, start, end time.Time) (*DecisionStats, error) {
	query := `SELECT 
	          COUNT(*) as total_decisions,
	          COUNT(DISTINCT model) as unique_models,
	          COUNT(DISTINCT backend) as unique_backends,
	          AVG(score) as avg_score
	          FROM scheduler_decisions 
	          WHERE created_at >= ? AND created_at <= ?`

	stats := &DecisionStats{}
	err := s.db.QueryRowContext(ctx, query, start, end).Scan(
		&stats.TotalDecisions, &stats.UniqueModels,
		&stats.UniqueBackends, &stats.AvgScore,
	)
	if err != nil {
		return nil, fmt.Errorf("get decision stats: %w", err)
	}

	return stats, nil
}

// DecisionStats represents scheduler decision statistics
type DecisionStats struct {
	TotalDecisions int     `json:"total_decisions"`
	UniqueModels   int     `json:"unique_models"`
	UniqueBackends int     `json:"unique_backends"`
	AvgScore       float64 `json:"avg_score"`
}

// buildInsertQuery builds the INSERT query
func (s *DecisionLogService) buildInsertQuery() string {
	return `INSERT INTO scheduler_decisions 
		(request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

// scanEntries scans rows into DecisionLogEntry slice
func (s *DecisionLogService) scanEntries(rows *sql.Rows) ([]*DecisionLogEntry, error) {
	var entries []*DecisionLogEntry

	for rows.Next() {
		var entry DecisionLogEntry
		if err := rows.Scan(
			&entry.ID, &entry.RequestID, &entry.UserID, &entry.TenantID,
			&entry.Model, &entry.Backend, &entry.Strategy, &entry.Score,
			&entry.Reason, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan decision log entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}
