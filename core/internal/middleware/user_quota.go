// Package middleware provides the UserQuotaMiddleware for user-level resource limiting.
//
// Legacy users.daily/monthly_token_* enforcement has been removed; Team quotas
// are enforced exclusively via UserPlan (centag-pro). This middleware is a
// no-op kept so route wiring in open-core does not need a conditional.
package middleware

import (
	"sync"
	"time"

	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
)

// UserQuotaMiddleware is retained for API compatibility; checks are no-ops.
type UserQuotaMiddleware struct {
	db      *database.Manager
	windows sync.Map
}

type userQuotaWindow struct {
	mu      sync.RWMutex
	tokens  int64
	requests int64
	resetAt time.Time
}

// NewUserQuotaMiddleware creates a no-op user quota checker.
func NewUserQuotaMiddleware(db *database.Manager) *UserQuotaMiddleware {
	return &UserQuotaMiddleware{db: db}
}

// Middleware always allows the request (plan enforcer owns Team limits).
func (m *UserQuotaMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// RecordTokens is a no-op (usage is recorded in token_usage / plan windows).
func (m *UserQuotaMiddleware) RecordTokens(userID int64, tokens int64) {}

// RecordRequest is a no-op.
func (m *UserQuotaMiddleware) RecordRequest(userID int64) {}

// GetWindowStats returns zeros.
func (m *UserQuotaMiddleware) GetWindowStats(userID int64) (tokens int64, requests int64, resetAt time.Time) {
	return 0, 0, time.Time{}
}

// ResetWindow is a no-op.
func (m *UserQuotaMiddleware) ResetWindow(userID int64) {}
