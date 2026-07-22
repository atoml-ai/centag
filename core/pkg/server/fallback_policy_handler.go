package server

import (
	"net/http"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

type FallbackPolicyHandler struct{}

func NewFallbackPolicyHandler() *FallbackPolicyHandler {
	return &FallbackPolicyHandler{}
}

// ListPolicies GET /api/v1/fallback-policies
func (h *FallbackPolicyHandler) ListPolicies(c *gin.Context) {
	store := config.GetFallbackPolicyStore()
	policies := store.List()
	RespondSuccess(c, policies)
}

// GetPolicy GET /api/v1/fallback-policies/:id
func (h *FallbackPolicyHandler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	store := config.GetFallbackPolicyStore()
	policy := store.Get(id)
	if policy == nil {
		RespondError(c, http.StatusNotFound, "fallback policy not found")
		return
	}
	RespondSuccess(c, policy)
}

// CreatePolicy POST /api/v1/fallback-policies
func (h *FallbackPolicyHandler) CreatePolicy(c *gin.Context) {
	var policy config.GlobalFallbackPolicy
	if !BindJSON(c, &policy) {
		return
	}
	store := config.GetFallbackPolicyStore()
	if err := store.Create(&policy); err != nil {
		logger.Warnf("[FallbackPolicy] Create failed: %v", err)
		RespondBadRequest(c, err.Error())
		return
	}
	logger.Infof("[FallbackPolicy] Created: id=%s name=%s strategy=%s", policy.ID, policy.Name, policy.Strategy)
	RespondSuccess(c, policy)
}

// UpdatePolicy PUT /api/v1/fallback-policies/:id
func (h *FallbackPolicyHandler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy config.GlobalFallbackPolicy
	if !BindJSON(c, &policy) {
		return
	}
	policy.ID = id
	store := config.GetFallbackPolicyStore()
	if err := store.Update(&policy); err != nil {
		logger.Warnf("[FallbackPolicy] Update failed: %v", err)
		RespondBadRequest(c, err.Error())
		return
	}
	logger.Infof("[FallbackPolicy] Updated: id=%s", id)
	RespondSuccess(c, policy)
}

// DeletePolicy DELETE /api/v1/fallback-policies/:id
func (h *FallbackPolicyHandler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	store := config.GetFallbackPolicyStore()
	if err := store.Delete(id); err != nil {
		RespondError(c, http.StatusNotFound, "fallback policy not found")
		return
	}
	logger.Infof("[FallbackPolicy] Deleted: id=%s", id)
	RespondSuccessWithMessage(c, "policy deleted")
}

// TestPolicy POST /api/v1/fallback-policies/:id/test
// 模拟错误触发，展示降级链执行路径（不实际调用后端）。
func (h *FallbackPolicyHandler) TestPolicy(c *gin.Context) {
	id := c.Param("id")
	store := config.GetFallbackPolicyStore()
	policy := store.Get(id)
	if policy == nil {
		RespondError(c, http.StatusNotFound, "fallback policy not found")
		return
	}

	// 构建降级路径预览
	type rulePreview struct {
		Priority   int    `json:"priority"`
		BackendID  string `json:"backend_id"`
		Model      string `json:"model"`
		TimeoutSec int    `json:"timeout_sec,omitempty"`
	}
	path := make([]rulePreview, 0, len(policy.SortedRules()))
	for _, r := range policy.SortedRules() {
		path = append(path, rulePreview{
			Priority:   r.Priority,
			BackendID:  r.BackendID,
			Model:      r.Model,
			TimeoutSec: r.TimeoutSec,
		})
	}

	RespondSuccess(c, gin.H{
		"policy_id": policy.ID,
		"strategy":  policy.Strategy,
		"rules":     path,
		"note":      "This is a preview. Actual execution depends on circuit breaker status.",
	})
}
