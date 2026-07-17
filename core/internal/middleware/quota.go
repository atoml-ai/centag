// Package middleware provides the QuotaMiddleware for tenant-level resource limiting.
//
// Design goals:
//   - Zero overhead for single-user mode (tenantID == "" → skip)
//   - In-memory window cache to avoid DB round-trips on every request
//   - Async usage recording so the proxy path stays fast
//   - Graceful degradation: if quota check fails, log and allow (configurable)
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/hooks"
	"centag/core/pkg/logger"
)

// QuotaMiddleware performs tenant-level quota checks before LLM requests.
type QuotaMiddleware struct {
	db *database.Manager

	// windows caches the in-flight counters per tenant.
	// Key: tenantID, Value: *quotaWindow
	windows sync.Map
}

// quotaWindow holds the mutable counters for a single tenant.
type quotaWindow struct {
	mu        sync.RWMutex
	tokens    int64
	requests  int64
	resetAt   time.Time // when the daily window expires
}

// NewQuotaMiddleware creates a new quota checker.
// db may be nil; in that case every request is allowed.
func NewQuotaMiddleware(db *database.Manager) *QuotaMiddleware {
	return &QuotaMiddleware{db: db}
}

// Middleware returns a gin.HandlerFunc that checks quota before allowing
// the request to proceed.  It is intended to be placed *after* auth
// middleware so that GetTenantID can read the authenticated tenant.
func (m *QuotaMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := auth.GetTenantID(c)

		// Single-user mode or unauthenticated → no quota enforcement
		if tenantID == "" {
			c.Next()
			return
		}

		// No DB → cannot check quota, allow through
		if m.db == nil {
			c.Next()
			return
		}

		// Perform the quota check
		allowed, reason := m.check(c, tenantID)
		if !allowed {
			logger.Warnf("Quota exceeded for tenant %s: %s", tenantID, reason)
			if hm := hooks.Default(); hm != nil {
				userID, _ := auth.GetUserID(c)
				_ = hm.TriggerQuotaExceededHooks(c.Request.Context(), userID)
			}
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
		defer m.RecordRequest(tenantID)

		c.Next()
	}
}

// check evaluates both token and request quotas for the tenant.
// It first consults the in-memory window, falling back to the DB
// when the window has expired or is missing.
func (m *QuotaMiddleware) check(c *gin.Context, tenantID string) (bool, string) {
	now := time.Now().UTC()

	// Load or create the in-memory window
	w, _ := m.windows.LoadOrStore(tenantID, &quotaWindow{
		resetAt: now.Truncate(24 * time.Hour).Add(24 * time.Hour),
	})
	window := w.(*quotaWindow)

	// If the window expired, reset counters and refresh from DB
	window.mu.Lock()
	if now.After(window.resetAt) {
		window.tokens = 0
		window.requests = 0
		window.resetAt = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	window.mu.Unlock()

	// Fetch quota limits from DB
	quota, err := m.db.TenantStore().GetTenantQuota(c.Request.Context(), tenantID)
	if err != nil {
		// No quota row → unlimited
		return true, ""
	}

	// Check daily token limit
	if quota.DailyTokenLimit > 0 {
		window.mu.RLock()
		current := window.tokens
		window.mu.RUnlock()

		// If in-memory counter is stale (e.g. server restart), seed from DB
		if current == 0 && quota.UsedTodayTokens > 0 {
			window.mu.Lock()
			window.tokens = quota.UsedTodayTokens
			window.mu.Unlock()
			current = quota.UsedTodayTokens
		}

		if current >= quota.DailyTokenLimit {
			return false, "daily token quota exceeded"
		}
	}

	// Check daily request limit
	if quota.DailyRequestLimit > 0 {
		window.mu.RLock()
		current := window.requests
		window.mu.RUnlock()

		if current == 0 && quota.UsedTodayRequests > 0 {
			window.mu.Lock()
			window.requests = quota.UsedTodayRequests
			window.mu.Unlock()
			current = quota.UsedTodayRequests
		}

		if current >= quota.DailyRequestLimit {
			return false, "daily request quota exceeded"
		}
	}

	// Check monthly token limit
	if quota.MonthlyTokenLimit > 0 {
		if quota.UsedMonthTokens >= quota.MonthlyTokenLimit {
			return false, "monthly token quota exceeded"
		}
	}

	// Check monthly request limit
	if quota.MonthlyRequestLimit > 0 {
		if quota.UsedMonthRequests >= quota.MonthlyRequestLimit {
			return false, "monthly request quota exceeded"
		}
	}

	return true, ""
}

// RecordTokens records token usage for a tenant after a successful request.
// This should be called by the proxy handler after the response is sent.
func (m *QuotaMiddleware) RecordTokens(tenantID string, tokens int64) {
	if tenantID == "" || tokens <= 0 {
		return
	}

	w, ok := m.windows.Load(tenantID)
	if !ok {
		return
	}
	window := w.(*quotaWindow)

	window.mu.Lock()
	window.tokens += tokens
	window.requests++
	window.mu.Unlock()
}

// RecordRequest records a single request for a tenant.
// Useful for endpoints where token count is not yet known.
func (m *QuotaMiddleware) RecordRequest(tenantID string) {
	if tenantID == "" {
		return
	}

	w, ok := m.windows.Load(tenantID)
	if !ok {
		return
	}
	window := w.(*quotaWindow)

	window.mu.Lock()
	window.requests++
	window.mu.Unlock()
}

// GetWindowStats returns the current in-memory counters for a tenant.
// Used by admin dashboards and tests.
func (m *QuotaMiddleware) GetWindowStats(tenantID string) (tokens int64, requests int64, resetAt time.Time) {
	w, ok := m.windows.Load(tenantID)
	if !ok {
		return 0, 0, time.Time{}
	}
	window := w.(*quotaWindow)

	window.mu.RLock()
	defer window.mu.RUnlock()
	return window.tokens, window.requests, window.resetAt
}

// ResetWindow resets the in-memory counters for a tenant.
// Called when an admin updates quota or manually resets usage.
func (m *QuotaMiddleware) ResetWindow(tenantID string) {
	w, ok := m.windows.Load(tenantID)
	if !ok {
		return
	}
	window := w.(*quotaWindow)

	window.mu.Lock()
	window.tokens = 0
	window.requests = 0
	window.resetAt = time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	window.mu.Unlock()
}
