package tokenusage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// UserQuotaService looks up per-user defaults (e.g. default pipeline).
// Daily/monthly token limits on users were removed; enforcement lives on
// EffectivePlan (user_plans / group_plans) via UserPlanEnforcer.
type UserQuotaService struct {
	db     *sql.DB
	driver string

	cache   map[int64]*UserQuotaEntry
	cacheMu sync.RWMutex
	cacheTS time.Time
}

// UserQuotaEntry caches the user's default pipeline.
type UserQuotaEntry struct {
	UserID            int64
	DefaultPipelineID string
	// Deprecated legacy fields kept zeroed for API compatibility.
	DailyTokenLimit   int64
	MonthlyTokenLimit int64
	DailyTokenUsed    int64
	MonthlyTokenUsed  int64
	QuotaResetDate    *time.Time
}

// UserQuotaStatus is a compatibility DTO; HasQuota is always false after the
// UserPlan single-source migration.
type UserQuotaStatus struct {
	HasQuota          bool   `json:"has_quota"`
	DefaultPipelineID string `json:"default_pipeline_id"`
	DailyTokenLimit   int64  `json:"daily_token_limit"`
	MonthlyTokenLimit int64  `json:"monthly_token_limit"`
	DailyTokenUsed    int64  `json:"daily_token_used"`
	MonthlyTokenUsed  int64  `json:"monthly_token_used"`
	DailyRemaining    int64  `json:"daily_remaining"`
	MonthlyRemaining  int64  `json:"monthly_remaining"`
}

// NewUserQuotaService creates a new UserQuotaService
func NewUserQuotaService(db *sql.DB, driver string) *UserQuotaService {
	return &UserQuotaService{
		db:     db,
		driver: driver,
		cache:  make(map[int64]*UserQuotaEntry),
	}
}

// GetUserQuotaStatus returns default-pipeline info; token quotas are unused.
func (s *UserQuotaService) GetUserQuotaStatus(ctx context.Context, userID int64) (*UserQuotaStatus, error) {
	entry, err := s.getUserQuotaEntry(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserQuotaStatus{
		HasQuota:          false,
		DefaultPipelineID: entry.DefaultPipelineID,
	}, nil
}

// CheckUserQuota is a no-op; plan enforcer owns quota checks.
func (s *UserQuotaService) CheckUserQuota(ctx context.Context, userID int64) error {
	return nil
}

// RecordUserUsage is a no-op; usage is recorded in token_usage.
func (s *UserQuotaService) RecordUserUsage(ctx context.Context, userID int64, tokens int64) error {
	return nil
}

// GetUserDefaultPipelineID gets the default pipeline ID for a user
func (s *UserQuotaService) GetUserDefaultPipelineID(ctx context.Context, userID int64) (string, error) {
	entry, err := s.getUserQuotaEntry(ctx, userID)
	if err != nil {
		return "", err
	}
	return entry.DefaultPipelineID, nil
}

// SetUserQuota updates only default_pipeline_id (legacy limit args ignored).
func (s *UserQuotaService) SetUserQuota(ctx context.Context, userID int64, dailyLimit, monthlyLimit int64, defaultPipelineID string) error {
	_ = dailyLimit
	_ = monthlyLimit
	var query string
	if s.driver == "postgresql" {
		query = `UPDATE users SET default_pipeline_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	} else {
		query = `UPDATE users SET default_pipeline_id = ?, updated_at = datetime('now') WHERE id = ?`
	}
	var err error
	if s.driver == "postgresql" {
		_, err = s.db.ExecContext(ctx, query, userID, defaultPipelineID)
	} else {
		_, err = s.db.ExecContext(ctx, query, defaultPipelineID, userID)
	}
	if err != nil {
		return fmt.Errorf("set user default pipeline: %w", err)
	}
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
	return nil
}

// ResetUserDailyUsage is a no-op (legacy counters removed).
func (s *UserQuotaService) ResetUserDailyUsage(ctx context.Context, userID int64) error {
	return nil
}

// ResetUserMonthlyUsage is a no-op (legacy counters removed).
func (s *UserQuotaService) ResetUserMonthlyUsage(ctx context.Context, userID int64) error {
	return nil
}

// RefreshCache refreshes the in-memory cache from the database
func (s *UserQuotaService) RefreshCache(ctx context.Context) error {
	enabledPred := "enabled = 1"
	if s.driver == "postgresql" {
		enabledPred = "enabled = TRUE"
	}
	query := `SELECT id, COALESCE(default_pipeline_id, '') FROM users WHERE ` + enabledPred

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("refresh user quota cache: %w", err)
	}
	defer rows.Close()

	newCache := make(map[int64]*UserQuotaEntry)
	for rows.Next() {
		var entry UserQuotaEntry
		if err := rows.Scan(&entry.UserID, &entry.DefaultPipelineID); err != nil {
			return fmt.Errorf("refresh user quota cache scan: %w", err)
		}
		newCache[entry.UserID] = &entry
	}

	s.cacheMu.Lock()
	s.cache = newCache
	s.cacheTS = time.Now()
	s.cacheMu.Unlock()

	return nil
}

func (s *UserQuotaService) getUserQuotaEntry(ctx context.Context, userID int64) (*UserQuotaEntry, error) {
	s.cacheMu.RLock()
	if entry, ok := s.cache[userID]; ok && time.Since(s.cacheTS) < time.Minute {
		s.cacheMu.RUnlock()
		return entry, nil
	}
	s.cacheMu.RUnlock()

	entry, err := s.queryUserQuotaEntry(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[userID] = entry
	s.cacheMu.Unlock()

	return entry, nil
}

func (s *UserQuotaService) queryUserQuotaEntry(ctx context.Context, userID int64) (*UserQuotaEntry, error) {
	query := `SELECT id, COALESCE(default_pipeline_id, '') FROM users WHERE id = ?`
	var entry UserQuotaEntry
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&entry.UserID, &entry.DefaultPipelineID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &UserQuotaEntry{UserID: userID}, nil
		}
		return nil, fmt.Errorf("query user quota entry: %w", err)
	}
	return &entry, nil
}
