package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/internal/tokenusage"

	"github.com/gin-gonic/gin"
)

func requireUserID(c *gin.Context) (int64, bool) {
	userID, err := auth.GetUserID(c)
	if err != nil || userID == 0 {
		RespondError(c, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return userID, true
}

// TokenUsageHandler serves user self-service token-usage APIs.
// Team admin usage/quotas live in centag-pro/internal/teamadmin (E2R).
type TokenUsageHandler struct {
	service *tokenusage.Service
}

// NewTokenUsageHandler 创建 Handler
func NewTokenUsageHandler(service *tokenusage.Service) *TokenUsageHandler {
	return &TokenUsageHandler{service: service}
}

// parseUsageRange parses from/to query params (YYYY-MM-DD); defaults to last 30 days.
func parseUsageRange(c *gin.Context) (time.Time, time.Time) {
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	return from, to
}

// GetUserUsage 获取用户使用情况
func (h *TokenUsageHandler) GetUserUsage(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	from, to := parseUsageRange(c)

	stats, err := h.service.GetUserUsage(c.Request.Context(), userID, from, to)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"stats": stats,
		"from":  from.Format("2006-01-02"),
		"to":    to.Format("2006-01-02"),
	})
}

// GetUsageBreakdown GET /api/v1/user/usage — user-scoped detailed metering/billing
// grouped per (backend_id, model), including unit prices and costs.
func (h *TokenUsageHandler) GetUsageBreakdown(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	from, to := parseUsageRange(c)

	breakdown, err := h.service.GetUsageBreakdown(c.Request.Context(), userID, from, to)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, breakdown)
}

// GetSessionsUsage GET /api/v1/user/usage/sessions — per-session metering/billing
// summaries (keyed by session_id) for the current user's sessions.
func (h *TokenUsageHandler) GetSessionsUsage(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	var ids []string
	for _, p := range strings.Split(c.Query("ids"), ",") {
		if id := strings.TrimSpace(p); id != "" {
			ids = append(ids, id)
		}
	}

	summaries, err := h.service.GetSessionsUsageBreakdown(c.Request.Context(), userID, ids)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{"sessions": summaries})
}

// GetSelfLimit GET /api/v1/user/usage/self-limit — user self-limits.
// User-level daily/monthly limits were removed; enforcement lives on effective
// plans, so self-limit reports "not set" (enabled=false).
func (h *TokenUsageHandler) GetSelfLimit(c *gin.Context) {
	if _, ok := requireUserID(c); !ok {
		return
	}
	RespondSuccess(c, gin.H{
		"enabled":              false,
		"daily_token_limit":    nil,
		"monthly_budget_limit": nil,
	})
}

// GetAggregateUsage GET process-wide usage (minimal / single-user editions).
func (h *TokenUsageHandler) GetAggregateUsage(c *gin.Context) {
	if _, ok := requireUserID(c); !ok {
		return
	}
	fromStr := c.Query("from")
	toStr := c.Query("to")
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}
	stats, err := h.service.GetAggregateUsage(c.Request.Context(), from, to)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, gin.H{
		"stats": stats,
		"from":  from.Format("2006-01-02"),
		"to":    to.Format("2006-01-02"),
		"scope": "all",
	})
}

// GetDailyUsage 获取每日使用情况
func (h *TokenUsageHandler) GetDailyUsage(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	days := c.DefaultQuery("days", "30")

	daysInt, _ := strconv.Atoi(days)
	if daysInt <= 0 {
		daysInt = 30
	}

	dailyStats, err := h.service.GetDailyUsage(c.Request.Context(), userID, daysInt)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"daily_stats": dailyStats,
		"days":        daysInt,
	})
}

// GetModelStats 获取模型使用统计
func (h *TokenUsageHandler) GetModelStats(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	days := c.DefaultQuery("days", "30")

	daysInt, _ := strconv.Atoi(days)
	if daysInt <= 0 {
		daysInt = 30
	}

	modelStats, err := h.service.GetModelStats(c.Request.Context(), userID, daysInt)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"model_stats": modelStats,
		"days":        daysInt,
	})
}

// GetBackendStats 获取后端使用统计
func (h *TokenUsageHandler) GetBackendStats(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	days := c.DefaultQuery("days", "30")

	daysInt, _ := strconv.Atoi(days)
	if daysInt <= 0 {
		daysInt = 30
	}

	backendStats, err := h.service.GetBackendStats(c.Request.Context(), userID, daysInt)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"backend_stats": backendStats,
		"days":          daysInt,
	})
}
