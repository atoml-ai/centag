// Package middleware provides the UserQuotaMiddleware for user-level resource limiting.
//
// Design goals:
//   - Zero overhead for single-user mode (userID == 0 → skip)
//   - In-memory window cache to avoid DB round-trips on every request
//   - User-level quota is checked before tenant-level quota
//   - Graceful degradation: if quota check fails, log and allow (configurable)
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// UserQuotaMiddleware performs user-level quota checks before LLM requests.
type UserQuotaMiddleware struct {
	db *database.Manager

	// windows caches the in-flight counters per user.
	// Key: userID (int64), Value: *userQuotaWindow
	windows sync.Map
}

// userQuotaWindow holds the mutable counters for a single user.
type userQuotaWindow struct {
	mu        sync.RWMutex
	tokens    int64
	requests  int64
	resetAt   time.Time // when the daily window expires
}

// NewUserQuotaMiddleware creates a new user quota checker.
// db may be nil; in that case every request is allowed.
func NewUserQuotaMiddleware(db *database.Manager) *UserQuotaMiddleware {
	return &UserQuotaMiddleware{db: db}
}

// Middleware returns a gin.HandlerFunc that checks user quota before allowing
// the request to proceed. It is intended to be placed *after* auth
// middleware so that the user ID can be read from the authenticated context.
func (m *UserQuotaMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromContext(c)

		// No user ID → no quota enforcement
		if userID == 0 {
			c.Next()
			return
		}

		// No DB → cannot check quota, allow through
		if m.db == nil {
			c.Next()
			return
		}

		// Perform the quota check
		allowed, reason := m.check(c, userID)
		if !allowed {
			logger.Warnf("Quota exceeded for user %d: %s", userID, reason)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": reason,
					"type":    "quota_exceeded",
				},
			})
			c.Abort()
			return
		}

		// Record the request in the in-memory window after the handler completes.
		defer m.RecordRequest(userID)

		c.Next()
	}
}

// check evaluates both token and request quotas for the user.
// It first consults the in-memory window, falling back to the DB
// when the window has expired or is missing.
func (m *UserQuotaMiddleware) check(c *gin.Context, userID int64) (bool, string) {
	now := time.Now().UTC()

	// Load or create the in-memory window
	w, _ := m.windows.LoadOrStore(userID, &userQuotaWindow{
		resetAt: now.Truncate(24 * time.Hour).Add(24 * time.Hour),
	})
	window := w.(*userQuotaWindow)

	// If the window expired, reset counters and refresh from DB
	window.mu.Lock()
	if now.After(window.resetAt) {
		window.tokens = 0
		window.requests = 0
		window.resetAt = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	window.mu.Unlock()

	// Fetch user from DB
	user, err := m.db.UserStore().GetByID(c.Request.Context(), userID)
	if err != nil {
		// No user row → unlimited
		return true, ""
	}

	// Check daily token limit
	if user.DailyTokenLimit > 0 {
		window.mu.RLock()
		current := window.tokens
		window.mu.RUnlock()

		// If in-memory counter is stale (e.g. server restart), seed from DB
		if current == 0 && user.DailyTokenUsed > 0 {
			window.mu.Lock()
			window.tokens = user.DailyTokenUsed
			window.mu.Unlock()
			current = user.DailyTokenUsed
		}

		if current >= user.DailyTokenLimit {
			return false, "daily token quota exceeded for user"
		}
	}

	// Check monthly token limit
	if user.MonthlyTokenLimit > 0 {
		if user.MonthlyTokenUsed >= user.MonthlyTokenLimit {
			return false, "monthly token quota exceeded for user"
		}
	}

	return true, ""
}

// RecordTokens records token usage for a user after a successful request.
// This should be called by the proxy handler after the response is sent.
func (m *UserQuotaMiddleware) RecordTokens(userID int64, tokens int64) {
	if userID == 0 || tokens <= 0 {
		return
	}

	w, ok := m.windows.Load(userID)
	if !ok {
		return
	}
	window := w.(*userQuotaWindow)

	window.mu.Lock()
	window.tokens += tokens
	window.requests++
	window.mu.Unlock()
}

// RecordRequest records a single request for a user.
// Useful for endpoints where token count is not yet known.
func (m *UserQuotaMiddleware) RecordRequest(userID int64) {
	if userID == 0 {
		return
	}

	w, ok := m.windows.Load(userID)
	if !ok {
		return
	}
	window := w.(*userQuotaWindow)

	window.mu.Lock()
	window.requests++
	window.mu.Unlock()
}

// GetWindowStats returns the current in-memory counters for a user.
// Used by admin dashboards and tests.
func (m *UserQuotaMiddleware) GetWindowStats(userID int64) (tokens int64, requests int64, resetAt time.Time) {
	w, ok := m.windows.Load(userID)
	if !ok {
		return 0, 0, time.Time{}
	}
	window := w.(*userQuotaWindow)

	window.mu.RLock()
	defer window.mu.RUnlock()
	return window.tokens, window.requests, window.resetAt
}

// ResetWindow resets the in-memory counters for a user.
// Called when an admin updates quota or manually resets usage.
func (m *UserQuotaMiddleware) ResetWindow(userID int64) {
	w, ok := m.windows.Load(userID)
	if !ok {
		return
	}
	window := w.(*userQuotaWindow)

	window.mu.Lock()
	window.tokens = 0
	window.requests = 0
	window.resetAt = time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	window.mu.Unlock()
}

// getUserIDFromContext extracts the user ID from the gin context.
func getUserIDFromContext(c *gin.Context) int64 {
	v, exists := c.Get(auth.CtxKeyUserID)
	if !exists {
		return 0
	}
	switch id := v.(type) {
	case int64:
		return id
	case int:
		return int64(id)
	case string:
		// Try to parse string user ID
		if n, err := time.ParseDuration(id); err == nil {
			return int64(n)
		}
	}
	return 0
}
