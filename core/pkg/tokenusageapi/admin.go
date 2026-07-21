// Package tokenusageapi exposes admin token-usage / quota APIs for Team plugins.
package tokenusageapi

import (
	"context"
	"time"

	"centag/core/internal/tokenusage"
)

// UsageStats is an aggregate usage snapshot.
type UsageStats = tokenusage.UsageStats

// UserQuota is a per-user quota row plus live usage.
type UserQuota = tokenusage.UserQuota

// UserRank is one row of the admin usage ranking.
type UserRank struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	TotalTokens int    `json:"total_tokens"`
}

// AdminService is the Team admin surface over token usage / quotas.
type AdminService interface {
	GetAllUsersUsage(ctx context.Context, from, to time.Time) (*UsageStats, error)
	GetUserRanking(ctx context.Context, limit, days int) ([]UserRank, error)
	SetQuota(ctx context.Context, userID int64, dailyLimit, monthlyLimit int) error
	GetUserQuota(ctx context.Context, userID int64) (*UserQuota, error)
	ResetQuota(ctx context.Context, userID int64) error
}

type adapter struct {
	s *tokenusage.Service
}

// Wrap adapts the internal tokenusage.Service to AdminService.
func Wrap(s *tokenusage.Service) AdminService {
	if s == nil {
		s = tokenusage.DefaultService()
	}
	return &adapter{s: s}
}

// Default returns AdminService backed by tokenusage.DefaultService().
func Default() AdminService { return Wrap(tokenusage.DefaultService()) }

func (a *adapter) GetAllUsersUsage(ctx context.Context, from, to time.Time) (*UsageStats, error) {
	return a.s.GetAllUsersUsage(ctx, from, to)
}

func (a *adapter) GetUserRanking(ctx context.Context, limit, days int) ([]UserRank, error) {
	rows, err := a.s.GetUserRanking(ctx, limit, days)
	if err != nil {
		return nil, err
	}
	out := make([]UserRank, 0, len(rows))
	for _, r := range rows {
		out = append(out, UserRank{
			UserID:      r.UserID,
			Username:    r.Username,
			TotalTokens: r.TotalTokens,
		})
	}
	return out, nil
}

func (a *adapter) SetQuota(ctx context.Context, userID int64, dailyLimit, monthlyLimit int) error {
	return a.s.SetQuota(ctx, userID, dailyLimit, monthlyLimit)
}

func (a *adapter) GetUserQuota(ctx context.Context, userID int64) (*UserQuota, error) {
	return a.s.GetUserQuota(ctx, userID)
}

func (a *adapter) ResetQuota(ctx context.Context, userID int64) error {
	return a.s.ResetQuota(ctx, userID)
}
