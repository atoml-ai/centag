package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

var pgPlaceholderRe = regexp.MustCompile(`\$\d+`)

type UnifiedTenantService struct {
	db      *sql.DB
	dialect Dialect
}

func NewUnifiedTenantService(db *sql.DB, dialect Dialect) *UnifiedTenantService {
	return &UnifiedTenantService{db: db, dialect: dialect}
}

func (s *UnifiedTenantService) replacePlaceholders(query string) string {
	matches := pgPlaceholderRe.FindAllString(query, -1)
	if len(matches) == 0 {
		return query
	}
	seen := make(map[string]string, len(matches))
	replace := func(match string) string {
		if repl, ok := seen[match]; ok {
			return repl
		}
		var n int
		fmt.Sscanf(match, "$%d", &n)
		repl := s.dialect.Placeholder(n)
		seen[match] = repl
		return repl
	}
	return pgPlaceholderRe.ReplaceAllStringFunc(query, replace)
}

func (s *UnifiedTenantService) CreateTenant(ctx context.Context, tenant *Tenant) error {
	query := s.replacePlaceholders(`
		INSERT INTO tenants (id, user_id, name, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	_, err := s.db.ExecContext(ctx, query,
		tenant.ID, tenant.UserID, tenant.Name, tenant.Description,
		tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	return err
}

func (s *UnifiedTenantService) GetTenantByUserID(ctx context.Context, userID int64) (*Tenant, error) {
	query := s.replacePlaceholders(`
		SELECT id, user_id, name, description, status, created_at, updated_at
		FROM tenants
		WHERE user_id = $1
	`)
	return s.scanTenant(ctx, query, userID)
}

func (s *UnifiedTenantService) GetTenantByID(ctx context.Context, tenantID string) (*Tenant, error) {
	query := s.replacePlaceholders(`
		SELECT id, user_id, name, description, status, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`)
	return s.scanTenant(ctx, query, tenantID)
}

func (s *UnifiedTenantService) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	query := s.replacePlaceholders(`
		UPDATE tenants
		SET name = $2, description = $3, status = $4, updated_at = $5
		WHERE id = $1
	`)
	_, err := s.db.ExecContext(ctx, query,
		tenant.ID, tenant.Name, tenant.Description, tenant.Status, time.Now().UTC(),
	)
	return err
}

func (s *UnifiedTenantService) DeleteTenant(ctx context.Context, tenantID string) error {
	query := s.replacePlaceholders(`DELETE FROM tenants WHERE id = $1`)
	_, err := s.db.ExecContext(ctx, query, tenantID)
	return err
}

func (s *UnifiedTenantService) ListTenants(ctx context.Context) ([]*Tenant, error) {
	query := `
		SELECT id, user_id, name, description, status, created_at, updated_at
		FROM tenants
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		tenant, err := s.scanTenantRow(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}
	return tenants, rows.Err()
}

func (s *UnifiedTenantService) GetTenantQuota(ctx context.Context, tenantID string) (*TenantQuota, error) {
	query := s.replacePlaceholders(`
		SELECT tenant_id, daily_limit, monthly_limit, used_today, used_this_month,
		       daily_request_limit, monthly_request_limit, used_today_requests, used_this_month_requests,
		       max_backends, max_api_keys, reset_date, updated_at
		FROM tenant_quotas
		WHERE tenant_id = $1
	`)
	quota := &TenantQuota{}
	var resetDateRaw interface{}
	err := s.db.QueryRowContext(ctx, query, tenantID).Scan(
		&quota.TenantID, &quota.DailyTokenLimit, &quota.MonthlyTokenLimit,
		&quota.UsedTodayTokens, &quota.UsedMonthTokens,
		&quota.DailyRequestLimit, &quota.MonthlyRequestLimit,
		&quota.UsedTodayRequests, &quota.UsedMonthRequests,
		&quota.MaxBackends, &quota.MaxAPIKeys,
		&resetDateRaw, &quota.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if t, err := s.dialect.ParseTime(resetDateRaw); err == nil && !t.IsZero() {
		quota.ResetDate = t
	}
	return quota, nil
}

func (s *UnifiedTenantService) SetTenantQuota(ctx context.Context, quota *TenantQuota) error {
	query := s.replacePlaceholders(s.dialect.UpsertSQL(
		"tenant_quotas",
		"tenant_id, daily_limit, monthly_limit, used_today, used_this_month, daily_request_limit, monthly_request_limit, used_today_requests, used_this_month_requests, max_backends, max_api_keys, reset_date, updated_at",
		"tenant_id",
	))
	_, err := s.db.ExecContext(ctx, query,
		quota.TenantID, quota.DailyTokenLimit, quota.MonthlyTokenLimit,
		quota.UsedTodayTokens, quota.UsedMonthTokens,
		quota.DailyRequestLimit, quota.MonthlyRequestLimit,
		quota.UsedTodayRequests, quota.UsedMonthRequests,
		quota.MaxBackends, quota.MaxAPIKeys,
		quota.ResetDate, time.Now().UTC(),
	)
	return err
}

func (s *UnifiedTenantService) scanTenant(ctx context.Context, query string, args ...interface{}) (*Tenant, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	return s.scanTenantRow(row)
}

func (s *UnifiedTenantService) scanTenantRow(row interface{ Scan(dest ...interface{}) error }) (*Tenant, error) {
	tenant := &Tenant{}
	var createdAtRaw, updatedAtRaw interface{}
	err := row.Scan(
		&tenant.ID, &tenant.UserID, &tenant.Name, &tenant.Description,
		&tenant.Status, &createdAtRaw, &updatedAtRaw,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if t, err := s.dialect.ParseTime(createdAtRaw); err == nil && !t.IsZero() {
		tenant.CreatedAt = t
	}
	if t, err := s.dialect.ParseTime(updatedAtRaw); err == nil && !t.IsZero() {
		tenant.UpdatedAt = t
	}
	return tenant, nil
}
