package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/internal/tokenusage"
)

func requireUserID(c *gin.Context) (int64, bool) {
	userID, err := auth.GetUserID(c)
	if err != nil || userID == 0 {
		RespondError(c, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return userID, true
}

// TokenUsageHandler Token 使用 API Handler
type TokenUsageHandler struct {
	service *tokenusage.Service
}

// NewTokenUsageHandler 创建 Handler
func NewTokenUsageHandler(service *tokenusage.Service) *TokenUsageHandler {
	return &TokenUsageHandler{service: service}
}

// GetUserUsage 获取用户使用情况
func (h *TokenUsageHandler) GetUserUsage(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	// 解析查询参数
	fromStr := c.Query("from")
	toStr := c.Query("to")

	// 默认最近 30 天
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

// GetAllUsersUsage 管理员获取所有用户使用情况
func (h *TokenUsageHandler) GetAllUsersUsage(c *gin.Context) {
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

	stats, err := h.service.GetAllUsersUsage(c.Request.Context(), from, to)
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

// GetUserRanking 获取用户排行
func (h *TokenUsageHandler) GetUserRanking(c *gin.Context) {
	limit := c.DefaultQuery("limit", "10")
	days := c.DefaultQuery("days", "30")

	limitInt, _ := strconv.Atoi(limit)
	daysInt, _ := strconv.Atoi(days)

	if limitInt <= 0 {
		limitInt = 10
	}
	if daysInt <= 0 {
		daysInt = 30
	}

	ranking, err := h.service.GetUserRanking(c.Request.Context(), limitInt, daysInt)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"ranking": ranking,
		"limit":   limitInt,
		"days":    daysInt,
	})
}

// SetQuota 设置用户配额（管理员）
func (h *TokenUsageHandler) SetQuota(c *gin.Context) {
	var req struct {
		UserID       int64 `json:"user_id"`
		DailyLimit   int   `json:"daily_limit"`
		MonthlyLimit int   `json:"monthly_limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.UserID == 0 {
		RespondError(c, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := h.service.SetQuota(c.Request.Context(), req.UserID, req.DailyLimit, req.MonthlyLimit); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"message":       "Quota updated",
		"user_id":       req.UserID,
		"daily_limit":   req.DailyLimit,
		"monthly_limit": req.MonthlyLimit,
	})
}

// GetUserQuota 获取用户配额（管理员）
func (h *TokenUsageHandler) GetUserQuota(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid user_id")
		return
	}

	q, err := h.service.GetUserQuota(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, gin.H{
		"user_id":         userID,
		"daily_limit":     q.DailyLimit,
		"monthly_limit":   q.MonthlyLimit,
		"used_today":      q.UsedToday,
		"used_this_month": q.UsedThisMonth,
		"has_quota":       q.HasQuota,
	})
}

// ResetQuota 重置用户配额（管理员）
func (h *TokenUsageHandler) ResetQuota(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid user_id")
		return
	}

	if err := h.service.ResetQuota(c.Request.Context(), userID); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"message": "Quota reset successfully",
		"user_id": userID,
	})
}
