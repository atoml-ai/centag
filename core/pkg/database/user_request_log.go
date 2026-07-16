package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserRequestLogRepository provides CRUD operations for UserRequestLog
type UserRequestLogRepository struct {
	db *sql.DB
}

// NewUserRequestLogRepository creates a new UserRequestLogRepository
func NewUserRequestLogRepository() *UserRequestLogRepository {
	mgr := Get()
	if mgr == nil {
		return nil
	}
	db := mgr.GetDB()
	if db == nil {
		return nil
	}
	return &UserRequestLogRepository{db: db}
}

// Create creates a new user request log entry
func (r *UserRequestLogRepository) Create(ctx context.Context, log *UserRequestLog) error {
	query := `INSERT INTO user_request_logs (user_id, tenant_id, request_id, model, backend, 
	          pipeline, input_tokens, output_tokens, latency_ms, status_code, 
	          request_body, response_body, created_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	result, err := r.db.ExecContext(ctx, query,
		log.UserID, log.TenantID, log.RequestID, log.Model, log.Backend,
		log.Pipeline, log.InputTokens, log.OutputTokens, log.LatencyMs, log.StatusCode,
		log.RequestBody, log.ResponseBody, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user request log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create user request log last insert id: %w", err)
	}
	log.ID = id
	return nil
}

// GetByID retrieves a user request log by ID
func (r *UserRequestLogRepository) GetByID(ctx context.Context, id int64) (*UserRequestLog, error) {
	query := `SELECT id, user_id, tenant_id, request_id, model, backend, pipeline,
	          input_tokens, output_tokens, latency_ms, status_code, 
	          request_body, response_body, created_at 
	          FROM user_request_logs WHERE id = ?`

	var log UserRequestLog
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID, &log.UserID, &log.TenantID, &log.RequestID, &log.Model, &log.Backend,
		&log.Pipeline, &log.InputTokens, &log.OutputTokens, &log.LatencyMs, &log.StatusCode,
		&log.RequestBody, &log.ResponseBody, &log.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user request log by id: %w", err)
	}
	return &log, nil
}

// ListByUserID retrieves user request logs by user ID with pagination
func (r *UserRequestLogRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*UserRequestLog, error) {
	query := `SELECT id, user_id, tenant_id, request_id, model, backend, pipeline,
	          input_tokens, output_tokens, latency_ms, status_code, 
	          request_body, response_body, created_at 
	          FROM user_request_logs 
	          WHERE user_id = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user request logs by user_id: %w", err)
	}
	defer rows.Close()

	var logs []*UserRequestLog
	for rows.Next() {
		var log UserRequestLog
		if err := rows.Scan(
			&log.ID, &log.UserID, &log.TenantID, &log.RequestID, &log.Model, &log.Backend,
			&log.Pipeline, &log.InputTokens, &log.OutputTokens, &log.LatencyMs, &log.StatusCode,
			&log.RequestBody, &log.ResponseBody, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list user request logs scan: %w", err)
		}
		logs = append(logs, &log)
	}
	return logs, nil
}

// ListByTenantID retrieves user request logs by tenant ID with pagination
func (r *UserRequestLogRepository) ListByTenantID(ctx context.Context, tenantID string, limit, offset int) ([]*UserRequestLog, error) {
	query := `SELECT id, user_id, tenant_id, request_id, model, backend, pipeline,
	          input_tokens, output_tokens, latency_ms, status_code, 
	          request_body, response_body, created_at 
	          FROM user_request_logs 
	          WHERE tenant_id = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user request logs by tenant_id: %w", err)
	}
	defer rows.Close()

	var logs []*UserRequestLog
	for rows.Next() {
		var log UserRequestLog
		if err := rows.Scan(
			&log.ID, &log.UserID, &log.TenantID, &log.RequestID, &log.Model, &log.Backend,
			&log.Pipeline, &log.InputTokens, &log.OutputTokens, &log.LatencyMs, &log.StatusCode,
			&log.RequestBody, &log.ResponseBody, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list user request logs scan: %w", err)
		}
		logs = append(logs, &log)
	}
	return logs, nil
}

// ListByTimeRange retrieves user request logs within a time range
func (r *UserRequestLogRepository) ListByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*UserRequestLog, error) {
	query := `SELECT id, user_id, tenant_id, request_id, model, backend, pipeline,
	          input_tokens, output_tokens, latency_ms, status_code, 
	          request_body, response_body, created_at 
	          FROM user_request_logs 
	          WHERE created_at >= ? AND created_at <= ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user request logs by time range: %w", err)
	}
	defer rows.Close()

	var logs []*UserRequestLog
	for rows.Next() {
		var log UserRequestLog
		if err := rows.Scan(
			&log.ID, &log.UserID, &log.TenantID, &log.RequestID, &log.Model, &log.Backend,
			&log.Pipeline, &log.InputTokens, &log.OutputTokens, &log.LatencyMs, &log.StatusCode,
			&log.RequestBody, &log.ResponseBody, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list user request logs scan: %w", err)
		}
		logs = append(logs, &log)
	}
	return logs, nil
}

// DeleteOldLogs deletes logs older than the specified duration
func (r *UserRequestLogRepository) DeleteOldLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM user_request_logs WHERE created_at < ?`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old user request logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete old user request logs rows affected: %w", err)
	}
	return rowsAffected, nil
}

// CountByUserID counts request logs for a user
func (r *UserRequestLogRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM user_request_logs WHERE user_id = ?`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user request logs by user_id: %w", err)
	}
	return count, nil
}
