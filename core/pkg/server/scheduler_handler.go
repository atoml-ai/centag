// Package server provides the SchedulerHandler for scheduler decision log APIs.
package server

import (
	"strconv"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/scheduler"

	"github.com/gin-gonic/gin"
)

// SchedulerHandler scheduler decision log handler
type SchedulerHandler struct {
	decisionLogService *scheduler.DecisionLogService
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(decisionLogService *scheduler.DecisionLogService) *SchedulerHandler {
	return &SchedulerHandler{
		decisionLogService: decisionLogService,
	}
}

// ── Response Structures ──────────────────────────────────────────────────────

type schedulerDecisionResponse struct {
	ID        int64   `json:"id"`
	RequestID string  `json:"request_id"`
	UserID    int64   `json:"user_id"`
	TeamID    string  `json:"team_id"`
	Model     string  `json:"model"`
	Backend   string  `json:"backend"`
	Strategy  string  `json:"strategy"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
	CreatedAt string  `json:"created_at"`
}

type schedulerDecisionStatsResponse struct {
	TotalDecisions int     `json:"total_decisions"`
	UniqueModels   int     `json:"unique_models"`
	UniqueBackends int     `json:"unique_backends"`
	AvgScore       float64 `json:"avg_score"`
}

// ── GET /api/v1/admin/scheduler/decisions ────────────────────────────────────

// ListDecisions lists scheduler decision logs
func (h *SchedulerHandler) ListDecisions(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var startTime, endTime time.Time
	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err == nil {
			startTime = t
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err == nil {
			endTime = t
		}
	} else {
		endTime = time.Now()
	}

	decisions, err := h.decisionLogService.QueryByTimeRange(c.Request.Context(), startTime, endTime, limit, offset)
	if err != nil {
		logger.Errorf("list scheduler decisions: %v", err)
		RespondInternalError(c, "failed to list scheduler decisions")
		return
	}

	responses := make([]*schedulerDecisionResponse, 0, len(decisions))
	for _, d := range decisions {
		responses = append(responses, &schedulerDecisionResponse{
			ID:        d.ID,
			RequestID: d.RequestID,
			UserID:    d.UserID,
			TeamID:    d.TenantID,
			Model:     d.Model,
			Backend:   d.Backend,
			Strategy:  d.Strategy,
			Score:     d.Score,
			Reason:    d.Reason,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		})
	}

	RespondSuccess(c, gin.H{
		"decisions": responses,
		"total":     len(responses),
		"limit":     limit,
		"offset":    offset,
	})
}

// ── GET /api/v1/admin/scheduler/decisions/stats ─────────────────────────────

// GetDecisionStats returns scheduler decision statistics
func (h *SchedulerHandler) GetDecisionStats(c *gin.Context) {
	// Parse time range
	startStr := c.DefaultQuery("start_time", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end_time", time.Now().Format(time.RFC3339))

	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	endTime, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		endTime = time.Now()
	}

	stats, err := h.decisionLogService.GetDecisionStats(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Errorf("get scheduler decision stats: %v", err)
		RespondInternalError(c, "failed to get scheduler decision stats")
		return
	}

	RespondSuccess(c, &schedulerDecisionStatsResponse{
		TotalDecisions: stats.TotalDecisions,
		UniqueModels:   stats.UniqueModels,
		UniqueBackends: stats.UniqueBackends,
		AvgScore:       stats.AvgScore,
	})
}

// ── DELETE /api/v1/admin/scheduler/decisions ─────────────────────────────────

// CleanupDecisions cleans up old scheduler decisions
func (h *SchedulerHandler) CleanupDecisions(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}

	deleted, err := h.decisionLogService.DeleteOldDecisions(c.Request.Context(), time.Duration(days)*24*time.Hour)
	if err != nil {
		logger.Errorf("cleanup scheduler decisions: %v", err)
		RespondInternalError(c, "failed to cleanup scheduler decisions")
		return
	}

	RespondSuccessWithMessage(c, "cleaned up "+strconv.FormatInt(deleted, 10)+" decisions older than "+daysStr+" days")
}
