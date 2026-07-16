package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/internal/abeval"
)

// ABEvalHandler exposes A/B comparison history APIs.
type ABEvalHandler struct {
	service *abeval.Service
}

func NewABEvalHandler(service *abeval.Service) *ABEvalHandler {
	return &ABEvalHandler{service: service}
}

// ListResults GET /api/v1/admin/ab-eval/results
func (h *ABEvalHandler) ListResults(c *gin.Context) {
	if h.service == nil {
		RespondError(c, http.StatusServiceUnavailable, "ab eval service unavailable")
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
	limit := 100
	results, err := h.service.ListResults(c.Request.Context(), from, to, limit)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	RespondSuccess(c, map[string]interface{}{
		"results": results,
		"from":    from.Format("2006-01-02"),
		"to":      to.Format("2006-01-02"),
	})
}

// GetSummary GET /api/v1/admin/ab-eval/summary
func (h *ABEvalHandler) GetSummary(c *gin.Context) {
	if h.service == nil {
		RespondError(c, http.StatusServiceUnavailable, "ab eval service unavailable")
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
	summary, err := h.service.GetSummary(c.Request.Context(), from, to)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	RespondSuccess(c, summary)
}