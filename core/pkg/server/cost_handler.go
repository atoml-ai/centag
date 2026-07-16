package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/internal/tokenusage"
)

// CostHandler 成本聚合 API。
type CostHandler struct {
	service *tokenusage.Service
}

// NewCostHandler creates a cost summary handler.
func NewCostHandler(service *tokenusage.Service) *CostHandler {
	return &CostHandler{service: service}
}

// GetSummary GET /api/v1/admin/cost/summary
func (h *CostHandler) GetSummary(c *gin.Context) {
	if h.service == nil {
		RespondError(c, http.StatusServiceUnavailable, "token usage service unavailable")
		return
	}

	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}

	summary, err := h.service.GetCostSummary(c.Request.Context(), tokenusage.CostSummaryQuery{
		GroupBy:  c.DefaultQuery("group_by", "model"),
		From:     from,
		To:       to,
		TenantID: c.Query("tenant_id"),
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(c, summary)
}