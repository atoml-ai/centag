package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SchedulerDecisionRepository provides CRUD operations for SchedulerDecision
type SchedulerDecisionRepository struct {
	db *sql.DB
}

// NewSchedulerDecisionRepository creates a new SchedulerDecisionRepository
func NewSchedulerDecisionRepository() *SchedulerDecisionRepository {
	mgr := Get()
	if mgr == nil {
		return nil
	}
	db := mgr.GetDB()
	if db == nil {
		return nil
	}
	return &SchedulerDecisionRepository{db: db}
}

// Create creates a new scheduler decision log entry
func (r *SchedulerDecisionRepository) Create(ctx context.Context, decision *SchedulerDecision) error {
	query := `INSERT INTO scheduler_decisions (request_id, user_id, tenant_id, model, backend, 
	          strategy, score, reason, created_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now()
	}

	result, err := r.db.ExecContext(ctx, query,
		decision.RequestID, decision.UserID, decision.TenantID, decision.Model, decision.Backend,
		decision.Strategy, decision.Score, decision.Reason, decision.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create scheduler decision: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create scheduler decision last insert id: %w", err)
	}
	decision.ID = id
	return nil
}

// GetByID retrieves a scheduler decision by ID
func (r *SchedulerDecisionRepository) GetByID(ctx context.Context, id int64) (*SchedulerDecision, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions WHERE id = ?`

	var d SchedulerDecision
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&d.ID, &d.RequestID, &d.UserID, &d.TenantID, &d.Model, &d.Backend,
		&d.Strategy, &d.Score, &d.Reason, &d.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get scheduler decision by id: %w", err)
	}
	return &d, nil
}

// ListByRequestID retrieves scheduler decisions by request ID
func (r *SchedulerDecisionRepository) ListByRequestID(ctx context.Context, requestID string) ([]*SchedulerDecision, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE request_id = ? 
	          ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("list scheduler decisions by request_id: %w", err)
	}
	defer rows.Close()

	var decisions []*SchedulerDecision
	for rows.Next() {
		var d SchedulerDecision
		if err := rows.Scan(
			&d.ID, &d.RequestID, &d.UserID, &d.TenantID, &d.Model, &d.Backend,
			&d.Strategy, &d.Score, &d.Reason, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list scheduler decisions scan: %w", err)
		}
		decisions = append(decisions, &d)
	}
	return decisions, nil
}

// ListByTenantID retrieves scheduler decisions by tenant ID with pagination
func (r *SchedulerDecisionRepository) ListByTenantID(ctx context.Context, tenantID string, limit, offset int) ([]*SchedulerDecision, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE tenant_id = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list scheduler decisions by tenant_id: %w", err)
	}
	defer rows.Close()

	var decisions []*SchedulerDecision
	for rows.Next() {
		var d SchedulerDecision
		if err := rows.Scan(
			&d.ID, &d.RequestID, &d.UserID, &d.TenantID, &d.Model, &d.Backend,
			&d.Strategy, &d.Score, &d.Reason, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list scheduler decisions scan: %w", err)
		}
		decisions = append(decisions, &d)
	}
	return decisions, nil
}

// ListByTimeRange retrieves scheduler decisions within a time range
func (r *SchedulerDecisionRepository) ListByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*SchedulerDecision, error) {
	query := `SELECT id, request_id, user_id, tenant_id, model, backend, strategy, score, reason, created_at 
	          FROM scheduler_decisions 
	          WHERE created_at >= ? AND created_at <= ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list scheduler decisions by time range: %w", err)
	}
	defer rows.Close()

	var decisions []*SchedulerDecision
	for rows.Next() {
		var d SchedulerDecision
		if err := rows.Scan(
			&d.ID, &d.RequestID, &d.UserID, &d.TenantID, &d.Model, &d.Backend,
			&d.Strategy, &d.Score, &d.Reason, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list scheduler decisions scan: %w", err)
		}
		decisions = append(decisions, &d)
	}
	return decisions, nil
}

// DeleteOldDecisions deletes decisions older than the specified duration
func (r *SchedulerDecisionRepository) DeleteOldDecisions(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM scheduler_decisions WHERE created_at < ?`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old scheduler decisions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete old scheduler decisions rows affected: %w", err)
	}
	return rowsAffected, nil
}
