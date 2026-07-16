package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TeamQuotaRepository provides CRUD operations for TeamQuota
type TeamQuotaRepository struct {
	db *sql.DB
}

// NewTeamQuotaRepository creates a new TeamQuotaRepository
func NewTeamQuotaRepository() *TeamQuotaRepository {
	mgr := Get()
	if mgr == nil {
		return nil
	}
	db := mgr.GetDB()
	if db == nil {
		return nil
	}
	return &TeamQuotaRepository{db: db}
}

// GetByTenantID retrieves a team quota by tenant ID
func (r *TeamQuotaRepository) GetByTenantID(ctx context.Context, tenantID string) (*TeamQuota, error) {
	query := `SELECT id, tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, created_at, updated_at 
	          FROM team_quota WHERE tenant_id = ?`

	var tq TeamQuota
	err := r.db.QueryRowContext(ctx, query, tenantID).Scan(
		&tq.ID, &tq.TenantID, &tq.DailyTokenLimit, &tq.MonthlyTokenLimit,
		&tq.DailyTokenUsed, &tq.MonthlyTokenUsed, &tq.CreatedAt, &tq.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get team quota by tenant_id: %w", err)
	}
	return &tq, nil
}

// GetByID retrieves a team quota by ID
func (r *TeamQuotaRepository) GetByID(ctx context.Context, id int64) (*TeamQuota, error) {
	query := `SELECT id, tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, created_at, updated_at 
	          FROM team_quota WHERE id = ?`

	var tq TeamQuota
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tq.ID, &tq.TenantID, &tq.DailyTokenLimit, &tq.MonthlyTokenLimit,
		&tq.DailyTokenUsed, &tq.MonthlyTokenUsed, &tq.CreatedAt, &tq.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get team quota by id: %w", err)
	}
	return &tq, nil
}

// Create creates a new team quota
func (r *TeamQuotaRepository) Create(ctx context.Context, tq *TeamQuota) error {
	query := `INSERT INTO team_quota (tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	tq.CreatedAt = now
	tq.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, query,
		tq.TenantID, tq.DailyTokenLimit, tq.MonthlyTokenLimit,
		tq.DailyTokenUsed, tq.MonthlyTokenUsed, tq.CreatedAt, tq.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create team quota: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create team quota last insert id: %w", err)
	}
	tq.ID = id
	return nil
}

// Update updates an existing team quota
func (r *TeamQuotaRepository) Update(ctx context.Context, tq *TeamQuota) error {
	query := `UPDATE team_quota SET 
	          daily_token_limit = ?, monthly_token_limit = ?,
	          daily_token_used = ?, monthly_token_used = ?,
	          updated_at = ?
	          WHERE id = ?`

	tq.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		tq.DailyTokenLimit, tq.MonthlyTokenLimit,
		tq.DailyTokenUsed, tq.MonthlyTokenUsed,
		tq.UpdatedAt, tq.ID,
	)
	if err != nil {
		return fmt.Errorf("update team quota: %w", err)
	}
	return nil
}

// Upsert creates or updates a team quota by tenant ID
func (r *TeamQuotaRepository) Upsert(ctx context.Context, tq *TeamQuota) error {
	existing, err := r.GetByTenantID(ctx, tq.TenantID)
	if err != nil && err != ErrNotFound {
		return err
	}
	if existing == nil {
		// Not found, create new
		return r.Create(ctx, tq)
	}
	// Found, update
	tq.ID = existing.ID
	return r.Update(ctx, tq)
}

// Delete deletes a team quota by ID
func (r *TeamQuotaRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM team_quota WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete team quota: %w", err)
	}
	return nil
}

// List retrieves all team quotas
func (r *TeamQuotaRepository) List(ctx context.Context) ([]*TeamQuota, error) {
	query := `SELECT id, tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, created_at, updated_at 
	          FROM team_quota ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list team quotas: %w", err)
	}
	defer rows.Close()

	var quotas []*TeamQuota
	for rows.Next() {
		var tq TeamQuota
		if err := rows.Scan(
			&tq.ID, &tq.TenantID, &tq.DailyTokenLimit, &tq.MonthlyTokenLimit,
			&tq.DailyTokenUsed, &tq.MonthlyTokenUsed, &tq.CreatedAt, &tq.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list team quotas scan: %w", err)
		}
		quotas = append(quotas, &tq)
	}
	return quotas, nil
}
