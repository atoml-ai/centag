package server

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"centag/core/internal/billing"
)

// BillingRulesHandler manages pricing rules via admin API.
type BillingRulesHandler struct {
	store   billing.RuleStore
	pricing billing.PricingService
}

// NewBillingRulesHandler creates a rules admin handler.
func NewBillingRulesHandler(store billing.RuleStore, pricing billing.PricingService) *BillingRulesHandler {
	return &BillingRulesHandler{store: store, pricing: pricing}
}

func (h *BillingRulesHandler) invalidate(ctx context.Context) {
	if h.pricing != nil {
		_ = h.pricing.InvalidateCache(ctx)
	}
}

// ListRules GET /api/v1/admin/billing/rules
func (h *BillingRulesHandler) ListRules(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	rules, err := h.store.ListRules(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []*billing.PricingRule{}
	}
	RespondSuccess(c, rules)
}

// CreateRule POST /api/v1/admin/billing/rules
func (h *BillingRulesHandler) CreateRule(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	var rule billing.PricingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.CreateRule(c.Request.Context(), &rule); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.invalidate(c.Request.Context())
	RespondSuccess(c, rule)
}

// UpdateRule PUT /api/v1/admin/billing/rules/:id
func (h *BillingRulesHandler) UpdateRule(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	var rule billing.PricingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.UpdateRule(c.Request.Context(), id, &rule); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.invalidate(c.Request.Context())
	rule.ID = id
	RespondSuccess(c, rule)
}

// DeleteRule DELETE /api/v1/admin/billing/rules/:id
func (h *BillingRulesHandler) DeleteRule(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteRule(c.Request.Context(), id); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.invalidate(c.Request.Context())
	RespondSuccessWithMessage(c, "deleted")
}

// ImportRules POST /api/v1/admin/billing/rules/import
func (h *BillingRulesHandler) ImportRules(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.ImportFromYAML(c.Request.Context(), data); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.invalidate(c.Request.Context())
	n, _ := h.store.CountRules(c.Request.Context())
	RespondSuccess(c, gin.H{"imported": n})
}

// ExportRules GET /api/v1/admin/billing/rules/export
func (h *BillingRulesHandler) ExportRules(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "pricing store unavailable")
		return
	}
	data, err := h.store.ExportToYAML(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Content-Type", "application/x-yaml")
	c.Data(http.StatusOK, "application/x-yaml", data)
}
