package tokenusage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// TeamQuotaService provides team-level quota management
type TeamQuotaService struct {
	db     *sql.DB
	driver string

	// In-memory cache for team quotas
	cache   map[string]*TeamQuotaCacheEntry
	cacheMu sync.RWMutex
	cacheTS time.Time
}

// TeamQuotaCacheEntry represents a cached team quota entry
type TeamQuotaCacheEntry struct {
	TenantID         string
	DailyTokenLimit   int64
	MonthlyTokenLimit int64
	DailyTokenUsed   int64
	MonthlyTokenUsed int64
}

// TeamQuotaStatus represents the current quota status for a team
type TeamQuotaStatus struct {
	HasQuota          bool   `json:"has_quota"`
	TenantID         string `json:"tenant_id"`
	DailyTokenLimit   int64  `json:"daily_token_limit"`
	MonthlyTokenLimit int64  `json:"monthly_token_limit"`
	DailyTokenUsed   int64  `json:"daily_token_used"`
	MonthlyTokenUsed int64  `json:"monthly_token_used"`
	DailyRemaining   int64  `json:"daily_remaining"`
	MonthlyRemaining int64  `json:"monthly_remaining"`
}

// NewTeamQuotaService creates a new TeamQuotaService
func NewTeamQuotaService(db *sql.DB, driver string) *TeamQuotaService {
	return &TeamQuotaService{
		db:     db,
		driver: driver,
		cache:  make(map[string]*TeamQuotaCacheEntry),
	}
}

// GetTeamQuotaStatus gets the current quota status for a team
func (s *TeamQuotaService) GetTeamQuotaStatus(ctx context.Context, tenantID string) (*TeamQuotaStatus, error) {
	entry, err := s.getTeamQuotaEntry(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	status := &TeamQuotaStatus{
		HasQuota:          entry.DailyTokenLimit > 0 || entry.MonthlyTokenLimit > 0,
		TenantID:         entry.TenantID,
		DailyTokenLimit:   entry.DailyTokenLimit,
		MonthlyTokenLimit: entry.MonthlyTokenLimit,
		DailyTokenUsed:   entry.DailyTokenUsed,
		MonthlyTokenUsed: entry.MonthlyTokenUsed,
	}

	// Calculate remaining
	if entry.DailyTokenLimit > 0 {
		status.DailyRemaining = entry.DailyTokenLimit - entry.DailyTokenUsed
		if status.DailyRemaining < 0 {
			status.DailyRemaining = 0
		}
	}
	if entry.MonthlyTokenLimit > 0 {
		status.MonthlyRemaining = entry.MonthlyTokenLimit - entry.MonthlyTokenUsed
		if status.MonthlyRemaining < 0 {
			status.MonthlyRemaining = 0
		}
	}

	return status, nil
}

// CheckTeamQuota checks if a team has exceeded their quota
// Returns nil if quota is not exceeded, error otherwise
func (s *TeamQuotaService) CheckTeamQuota(ctx context.Context, tenantID string) error {
	// Empty tenantID means single-user mode, skip team quota check
	if tenantID == "" {
		return nil
	}

	entry, err := s.getTeamQuotaEntry(ctx, tenantID)
	if err != nil {
		return err
	}

	// No quota set
	if entry.DailyTokenLimit <= 0 && entry.MonthlyTokenLimit <= 0 {
		return nil
	}

	// Check daily limit
	if entry.DailyTokenLimit > 0 && entry.DailyTokenUsed >= entry.DailyTokenLimit {
		return fmt.Errorf("team daily token quota exceeded: %d/%d", entry.DailyTokenUsed, entry.DailyTokenLimit)
	}

	// Check monthly limit
	if entry.MonthlyTokenLimit > 0 && entry.MonthlyTokenUsed >= entry.MonthlyTokenLimit {
		return fmt.Errorf("team monthly token quota exceeded: %d/%d", entry.MonthlyTokenUsed, entry.MonthlyTokenLimit)
	}

	return nil
}

// RecordTeamUsage records token usage for a team and updates their quota
func (s *TeamQuotaService) RecordTeamUsage(ctx context.Context, tenantID string, tokens int64) error {
	if tokens <= 0 || tenantID == "" {
		return nil
	}

	var query string
	if s.driver == "postgresql" {
		query = `
			INSERT INTO team_quota (tenant_id, daily_token_used, monthly_token_used, created_at, updated_at)
			VALUES ($1, $2, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				daily_token_used = team_quota.daily_token_used + $2,
				monthly_token_used = team_quota.monthly_token_used + $2,
				updated_at = CURRENT_TIMESTAMP
		`
	} else {
		query = `
			INSERT INTO team_quota (tenant_id, daily_token_used, monthly_token_used, created_at, updated_at)
			VALUES (?, ?, ?, datetime('now'), datetime('now'))
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				daily_token_used = team_quota.daily_token_used + excluded.daily_token_used,
				monthly_token_used = team_quota.monthly_token_used + excluded.monthly_token_used,
				updated_at = datetime('now')
		`
	}

	_, err := s.db.ExecContext(ctx, query, tenantID, tokens)
	return err
}

// SetTeamQuota sets the quota limits for a team
func (s *TeamQuotaService) SetTeamQuota(ctx context.Context, tenantID string, dailyLimit, monthlyLimit int64) error {
	var query string
	if s.driver == "postgresql" {
		query = `
			INSERT INTO team_quota (tenant_id, daily_token_limit, monthly_token_limit, created_at, updated_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				daily_token_limit = $2,
				monthly_token_limit = $3,
				updated_at = CURRENT_TIMESTAMP
		`
	} else {
		query = `
			INSERT INTO team_quota (tenant_id, daily_token_limit, monthly_token_limit, created_at, updated_at)
			VALUES (?, ?, ?, datetime('now'), datetime('now'))
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				daily_token_limit = excluded.daily_token_limit,
				monthly_token_limit = excluded.monthly_token_limit,
				updated_at = datetime('now')
		`
	}

	_, err := s.db.ExecContext(ctx, query, tenantID, dailyLimit, monthlyLimit)
	if err != nil {
		return fmt.Errorf("set team quota: %w", err)
	}

	// Invalidate cache
	s.cacheMu.Lock()
	delete(s.cache, tenantID)
	s.cacheMu.Unlock()

	return nil
}

// ResetTeamDailyUsage resets the daily usage for a team
func (s *TeamQuotaService) ResetTeamDailyUsage(ctx context.Context, tenantID string) error {
	var query string
	if s.driver == "postgresql" {
		query = `UPDATE team_quota SET daily_token_used = 0, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = $1`
	} else {
		query = `UPDATE team_quota SET daily_token_used = 0, updated_at = datetime('now') WHERE tenant_id = ?`
	}

	_, err := s.db.ExecContext(ctx, query, tenantID)
	return err
}

// ResetTeamMonthlyUsage resets the monthly usage for a team
func (s *TeamQuotaService) ResetTeamMonthlyUsage(ctx context.Context, tenantID string) error {
	var query string
	if s.driver == "postgresql" {
		query = `UPDATE team_quota SET monthly_token_used = 0, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = $1`
	} else {
		query = `UPDATE team_quota SET monthly_token_used = 0, updated_at = datetime('now') WHERE tenant_id = ?`
	}

	_, err := s.db.ExecContext(ctx, query, tenantID)
	return err
}

// RefreshCache refreshes the in-memory cache from the database
func (s *TeamQuotaService) RefreshCache(ctx context.Context) error {
	query := `SELECT tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used 
	          FROM team_quota`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("refresh team quota cache: %w", err)
	}
	defer rows.Close()

	newCache := make(map[string]*TeamQuotaCacheEntry)
	for rows.Next() {
		var entry TeamQuotaCacheEntry
		if err := rows.Scan(
			&entry.TenantID, &entry.DailyTokenLimit, &entry.MonthlyTokenLimit,
			&entry.DailyTokenUsed, &entry.MonthlyTokenUsed,
		); err != nil {
			return fmt.Errorf("refresh team quota cache scan: %w", err)
		}
		newCache[entry.TenantID] = &entry
	}

	s.cacheMu.Lock()
	s.cache = newCache
	s.cacheTS = time.Now()
	s.cacheMu.Unlock()

	return nil
}

// getTeamQuotaEntry gets a team quota entry from cache or database
func (s *TeamQuotaService) getTeamQuotaEntry(ctx context.Context, tenantID string) (*TeamQuotaCacheEntry, error) {
	// Try cache first (with read lock)
	s.cacheMu.RLock()
	if entry, ok := s.cache[tenantID]; ok && time.Since(s.cacheTS) < time.Minute {
		s.cacheMu.RUnlock()
		return entry, nil
	}
	s.cacheMu.RUnlock()

	// Cache miss or expired, query database
	entry, err := s.queryTeamQuotaEntry(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Update cache (with write lock)
	s.cacheMu.Lock()
	s.cache[tenantID] = entry
	s.cacheMu.Unlock()

	return entry, nil
}

// queryTeamQuotaEntry queries the database for a team quota entry
func (s *TeamQuotaService) queryTeamQuotaEntry(ctx context.Context, tenantID string) (*TeamQuotaCacheEntry, error) {
	query := `SELECT tenant_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used 
	          FROM team_quota WHERE tenant_id = ?`

	var entry TeamQuotaCacheEntry
	err := s.db.QueryRowContext(ctx, query, tenantID).Scan(
		&entry.TenantID, &entry.DailyTokenLimit, &entry.MonthlyTokenLimit,
		&entry.DailyTokenUsed, &entry.MonthlyTokenUsed,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return &TeamQuotaCacheEntry{TenantID: tenantID}, nil
		}
		return nil, fmt.Errorf("query team quota entry: %w", err)
	}
	return &entry, nil
}
