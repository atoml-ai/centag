package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"centag/core/internal/billing"
)

// PersonalBillingHandler handles Personal billing config read-only APIs.
type PersonalBillingHandler struct {
	store billing.RuleStore
}

// NewPersonalBillingHandler creates a Personal billing handler.
func NewPersonalBillingHandler(store billing.RuleStore) *PersonalBillingHandler {
	return &PersonalBillingHandler{store: store}
}

// GetBillingConfig GET /api/v1/billing/config - 返回配置文件内容
func (h *PersonalBillingHandler) GetBillingConfig(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "billing store unavailable")
		return
	}

	// 获取规则列表
	rules, err := h.store.ListRules(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 导出为 YAML
	data, err := h.store.ExportToYAML(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"rules": rules,
		"yaml":  string(data),
	})
}

// ListBillingRules GET /api/v1/billing/rules - 返回规则列表
func (h *PersonalBillingHandler) ListBillingRules(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "billing store unavailable")
		return
	}

	rules, err := h.store.ListRules(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, rules)
}

// HandleCreateRule POST /api/v1/billing/rules - 返回 405 (Personal 只读)
func (h *PersonalBillingHandler) HandleCreateRule(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success": false,
		"message": "Personal 模式不支持通过 API 创建规则，请编辑本地 YAML 配置文件",
	})
}

// HandleUpdateRule PUT /api/v1/billing/rules/:id - 返回 405 (Personal 只读)
func (h *PersonalBillingHandler) HandleUpdateRule(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success": false,
		"message": "Personal 模式不支持通过 API 更新规则，请编辑本地 YAML 配置文件",
	})
}

// HandleDeleteRule DELETE /api/v1/billing/rules/:id - 返回 405 (Personal 只读)
func (h *PersonalBillingHandler) HandleDeleteRule(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success": false,
		"message": "Personal 模式不支持通过 API 删除规则，请编辑本地 YAML 配置文件",
	})
}

// GetYAMLConfig GET /api/v1/billing/config/edit - 读取 YAML 配置
func (h *PersonalBillingHandler) GetYAMLConfig(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "billing store unavailable")
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

// SaveYAMLConfig POST /api/v1/billing/config/edit - 保存 YAML 配置
func (h *PersonalBillingHandler) SaveYAMLConfig(c *gin.Context) {
	if h.store == nil {
		RespondError(c, http.StatusServiceUnavailable, "billing store unavailable")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.ImportFromYAML(c.Request.Context(), data); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(c, gin.H{
		"message": "配置已保存并重新加载",
	})
}

// RegisterPersonalBillingRoutes registers routes for Personal billing API.
func (h *PersonalBillingHandler) RegisterPersonalBillingRoutes(rg *gin.RouterGroup) {
	billing := rg.Group("/billing")
	{
		billing.GET("/config", h.GetBillingConfig)
		billing.GET("/rules", h.ListBillingRules)
		billing.POST("/rules", h.HandleCreateRule)
		billing.PUT("/rules/:id", h.HandleUpdateRule)
		billing.DELETE("/rules/:id", h.HandleDeleteRule)
		billing.GET("/config/edit", h.GetYAMLConfig)
		billing.POST("/config/edit", h.SaveYAMLConfig)
	}
}
