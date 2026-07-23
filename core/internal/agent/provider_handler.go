package agent

import (
	"net/http"
	"strings"

	"centag/core/internal/auth"

	"github.com/gin-gonic/gin"
)

// AgentProviderHandler Agent 供应商配置 HTTP 处理器
type AgentProviderHandler struct {
	manager *AgentProviderManager
}

// NewAgentProviderHandler 创建处理器
func NewAgentProviderHandler(manager *AgentProviderManager) *AgentProviderHandler {
	return &AgentProviderHandler{manager: manager}
}

// List 列出所有 Agent 供应商配置
func (h *AgentProviderHandler) List(c *gin.Context) {
	tenantID, ok := resolveProviderReadTenantScope(c)
	if !ok {
		return
	}

	var configs []*AgentProviderConfig
	if tenantID != "" {
		configs = h.manager.ListByTenant(tenantID)
	} else {
		configs = h.manager.List()
	}
	c.JSON(http.StatusOK, gin.H{
		"agent_providers": configs,
		"count":           len(configs),
	})
}

// Get 获取单个配置
func (h *AgentProviderHandler) Get(c *gin.Context) {
	tenantID, ok := resolveProviderReadTenantScope(c)
	if !ok {
		return
	}

	id := c.Param("id")
	cfg, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent provider not found: " + id})
		return
	}
	if tenantID != "" && cfg.TenantID != "" && cfg.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent provider not found: " + id})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// GetByAgentType 按 Agent 类型查找配置
func (h *AgentProviderHandler) GetByAgentType(c *gin.Context) {
	tenantID, ok := resolveProviderReadTenantScope(c)
	if !ok {
		return
	}

	agentType := c.Query("agent_type")
	if agentType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_type query parameter is required"})
		return
	}

	cfg, ok := h.manager.GetByAgentTypeAndTenant(agentType, tenantID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no enabled provider for agent type: " + agentType})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// HotSwap 热切换 Agent 使用的后端（无需重启 Agent）
func (h *AgentProviderHandler) HotSwap(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		BackendID string `json:"backend_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent provider not found: " + id})
		return
	}

	oldBackend := existing.BackendID
	existing.BackendID = req.BackendID

	if err := h.manager.Update(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "backend switched",
		"id":          id,
		"agent_type":  existing.AgentType,
		"old_backend": oldBackend,
		"new_backend": req.BackendID,
	})
}

func resolveProviderReadTenantScope(c *gin.Context) (string, bool) {
	queryTenant := strings.TrimSpace(c.Query("tenant_id"))
	scope := auth.GetScopedAccess(c)

	switch scope {
	case auth.AccessGlobal:
		return queryTenant, true
	case auth.AccessTenant:
		tenantID := strings.TrimSpace(auth.GetTenantID(c))
		if tenantID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant scope required"})
			return "", false
		}
		if queryTenant != "" && queryTenant != tenantID {
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant scope mismatch"})
			return "", false
		}
		return tenantID, true
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return "", false
	}
}
