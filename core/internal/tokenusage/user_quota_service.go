package tokenusage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// UserQuotaService provides user-level quota management
type UserQuotaService struct {
	db     *sql.DB
	driver string

	// In-memory cache for user quotas (refreshed periodically)
	cache   map[int64]*UserQuotaEntry
	cacheMu sync.RWMutex
	cacheTS time.Time
}

// UserQuotaEntry represents a cached user quota entry
type UserQuotaEntry struct {
	UserID           int64
	DefaultPipelineID string
	DailyTokenLimit   int64
	MonthlyTokenLimit int64
	DailyTokenUsed   int64
	MonthlyTokenUsed int64
	QuotaResetDate   *time.Time
}

// UserQuotaStatus represents the current quota status for a user
type UserQuotaStatus struct {
	HasQuota         bool   `json:"has_quota"`
	DefaultPipelineID string `json:"default_pipeline_id"`
	DailyTokenLimit   int64  `json:"daily_token_limit"`
	MonthlyTokenLimit int64  `json:"monthly_token_limit"`
	DailyTokenUsed   int64  `json:"daily_token_used"`
	MonthlyTokenUsed int64  `json:"monthly_token_used"`
	DailyRemaining   int64  `json:"daily_remaining"`
	MonthlyRemaining int64  `json:"monthly_remaining"`
}

// NewUserQuotaService creates a new UserQuotaService
func NewUserQuotaService(db *sql.DB, driver string) *UserQuotaService {
	return &UserQuotaService{
		db:     db,
		driver: driver,
		cache:  make(map[int64]*UserQuotaEntry),
	}
}

// GetUserQuotaStatus gets the current quota status for a user
func (s *UserQuotaService) GetUserQuotaStatus(ctx context.Context, userID int64) (*UserQuotaStatus, error) {
	entry, err := s.getUserQuotaEntry(ctx, userID)
	if err != nil {
		return nil, err
	}

	status := &UserQuotaStatus{
		HasQuota:         entry.DailyTokenLimit > 0 || entry.MonthlyTokenLimit > 0,
		DefaultPipelineID: entry.DefaultPipelineID,
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

// CheckUserQuota checks if a user has exceeded their quota
// Returns nil if quota is not exceeded, error otherwise
func (s *UserQuotaService) CheckUserQuota(ctx context.Context, userID int64) error {
	entry, err := s.getUserQuotaEntry(ctx, userID)
	if err != nil {
		return err
	}

	// No quota set
	if entry.DailyTokenLimit <= 0 && entry.MonthlyTokenLimit <= 0 {
		return nil
	}

	// Check daily limit
	if entry.DailyTokenLimit > 0 && entry.DailyTokenUsed >= entry.DailyTokenLimit {
		return fmt.Errorf("daily token quota exceeded: %d/%d", entry.DailyTokenUsed, entry.DailyTokenLimit)
	}

	// Check monthly limit
	if entry.MonthlyTokenLimit > 0 && entry.MonthlyTokenUsed >= entry.MonthlyTokenLimit {
		return fmt.Errorf("monthly token quota exceeded: %d/%d", entry.MonthlyTokenUsed, entry.MonthlyTokenLimit)
	}

	return nil
}

// RecordUserUsage records token usage for a user and updates their quota
func (s *UserQuotaService) RecordUserUsage(ctx context.Context, userID int64, tokens int64) error {
	if tokens <= 0 {
		return nil
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	monthStr := now.Format("2006-01")

	var query string
	if s.driver == "postgresql" {
		query = `
			UPDATE users SET
				daily_token_used = CASE 
				 WHEN $2 = daily_token_used::text THEN $3
				 ELSE daily_token_used + $4
				END,
				monthly_token_used = CASE
				 WHEN $5 = monthly_token_used::text THEN $6
				 ELSE monthly_token_used + $4
				END,
				updated_at = CURRENT_TIMESTAMP
			 WHERE id = $1
		`
	} else {
		query = `
			UPDATE users SET
				daily_token_used = daily_token_used + ?,
				monthly_token_used = monthly_token_used + ?,
				updated_at = datetime('now')
			 WHERE id = ?
		`
	}

	// For simplicity, we just increment the counters
	// In production, you'd want to handle daily/monthly reset logic
	if s.driver == "postgresql" {
		_, err := s.db.ExecContext(ctx, query, userID, dateStr, 0, tokens, monthStr, 0)
		return err
	}

	_, err := s.db.ExecContext(ctx, query, tokens, tokens, userID)
	return err
}

// GetUserDefaultPipelineID gets the default pipeline ID for a user
func (s *UserQuotaService) GetUserDefaultPipelineID(ctx context.Context, userID int64) (string, error) {
	entry, err := s.getUserQuotaEntry(ctx, userID)
	if err != nil {
		return "", err
	}
	return entry.DefaultPipelineID, nil
}

// SetUserQuota sets the quota limits for a user
func (s *UserQuotaService) SetUserQuota(ctx context.Context, userID int64, dailyLimit, monthlyLimit int64, defaultPipelineID string) error {
	var query string
	if s.driver == "postgresql" {
		query = `
			UPDATE users SET
				daily_token_limit = $2,
				monthly_token_limit = $3,
				default_pipeline_id = $4,
				updated_at = CURRENT_TIMESTAMP
			 WHERE id = $1
		`
	} else {
		query = `
			UPDATE users SET
				daily_token_limit = ?,
				monthly_token_limit = ?,
				default_pipeline_id = ?,
				updated_at = datetime('now')
			 WHERE id = ?
		`
	}

	_, err := s.db.ExecContext(ctx, query, userID, dailyLimit, monthlyLimit, defaultPipelineID)
	if err != nil {
		return fmt.Errorf("set user quota: %w", err)
	}

	// Invalidate cache
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()

	return nil
}

// ResetUserDailyUsage resets the daily usage for a user
func (s *UserQuotaService) ResetUserDailyUsage(ctx context.Context, userID int64) error {
	var query string
	if s.driver == "postgresql" {
		query = `UPDATE users SET daily_token_used = 0, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	} else {
		query = `UPDATE users SET daily_token_used = 0, updated_at = datetime('now') WHERE id = ?`
	}

	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// ResetUserMonthlyUsage resets the monthly usage for a user
func (s *UserQuotaService) ResetUserMonthlyUsage(ctx context.Context, userID int64) error {
	var query string
	if s.driver == "postgresql" {
		query = `UPDATE users SET monthly_token_used = 0, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	} else {
		query = `UPDATE users SET monthly_token_used = 0, updated_at = datetime('now') WHERE id = ?`
	}

	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// RefreshCache refreshes the in-memory cache from the database
func (s *UserQuotaService) RefreshCache(ctx context.Context) error {
	enabledPred := "enabled = 1"
	if s.driver == "postgresql" {
		enabledPred = "enabled = TRUE"
	}
	query := `SELECT id, default_pipeline_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, quota_reset_date 
	          FROM users WHERE ` + enabledPred

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("refresh user quota cache: %w", err)
	}
	defer rows.Close()

	newCache := make(map[int64]*UserQuotaEntry)
	for rows.Next() {
		var entry UserQuotaEntry
		if err := rows.Scan(
			&entry.UserID, &entry.DefaultPipelineID, &entry.DailyTokenLimit, &entry.MonthlyTokenLimit,
			&entry.DailyTokenUsed, &entry.MonthlyTokenUsed, &entry.QuotaResetDate,
		); err != nil {
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

// getUserQuotaEntry gets a user quota entry from cache or database
func (s *UserQuotaService) getUserQuotaEntry(ctx context.Context, userID int64) (*UserQuotaEntry, error) {
	// Try cache first (with read lock)
	s.cacheMu.RLock()
	if entry, ok := s.cache[userID]; ok && time.Since(s.cacheTS) < time.Minute {
		s.cacheMu.RUnlock()
		return entry, nil
	}
	s.cacheMu.RUnlock()

	// Cache miss or expired, query database
	entry, err := s.queryUserQuotaEntry(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Update cache (with write lock)
	s.cacheMu.Lock()
	s.cache[userID] = entry
	s.cacheMu.Unlock()

	return entry, nil
}

// queryUserQuotaEntry queries the database for a user quota entry
func (s *UserQuotaService) queryUserQuotaEntry(ctx context.Context, userID int64) (*UserQuotaEntry, error) {
	query := `SELECT id, default_pipeline_id, daily_token_limit, monthly_token_limit, 
	          daily_token_used, monthly_token_used, quota_reset_date 
	          FROM users WHERE id = ?`

	var entry UserQuotaEntry
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&entry.UserID, &entry.DefaultPipelineID, &entry.DailyTokenLimit, &entry.MonthlyTokenLimit,
		&entry.DailyTokenUsed, &entry.MonthlyTokenUsed, &entry.QuotaResetDate,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return &UserQuotaEntry{UserID: userID}, nil
		}
		return nil, fmt.Errorf("query user quota entry: %w", err)
	}
	return &entry, nil
}
