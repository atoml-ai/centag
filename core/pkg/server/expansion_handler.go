package server

import (
	"net/http"

	"centag/core/internal/cache/expansion"
	"centag/core/pkg/logger"
	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ExpansionHandler 查询展开处理器
type ExpansionHandler struct {
	expander expansion.Expander
}

// NewExpansionHandler 创建查询展开处理器
func NewExpansionHandler(expander expansion.Expander) *ExpansionHandler {
	return &ExpansionHandler{
		expander: expander,
	}
}

// ExpansionConfigResponse 查询展开配置响应
type ExpansionConfigResponse struct {
	Mode      string          `json:"mode"`
	RuleBased RuleBasedConfig `json:"rule_based,omitempty"`
}

// GetExpansionConfig 获取查询展开配置
// @Summary 获取查询展开配置
// @Description 获取当前查询展开模块的配置信息
// @Tags expansion
// @Accept json
// @Produce json
// @Success 200 {object} ExpansionConfigResponse
// @Router /api/expansion/config [get]
func (h *ExpansionHandler) GetExpansionConfig(c *gin.Context) {
	// 返回默认配置结构
	config := ExpansionConfigResponse{
		Mode: "rule",
		RuleBased: RuleBasedConfig{
			Enabled:          true,
			MaxHistoryRounds: 3,
			Pronouns:         defaultPronouns(),
		},
	}

	c.JSON(http.StatusOK, config)
}

// UpdateExpansionConfig 更新查询展开配置
// @Summary 更新查询展开配置
// @Description 更新查询展开模块的配置参数
// @Tags expansion
// @Accept json
// @Produce json
// @Param config body ExpansionConfigResponse true "展开配置"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/expansion/config [put]
func (h *ExpansionHandler) UpdateExpansionConfig(c *gin.Context) {
	var config ExpansionConfigResponse
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	logger.Info("Expansion config updated",
		zap.String("mode", config.Mode),
		zap.Bool("enabled", config.RuleBased.Enabled))

	c.JSON(http.StatusOK, gin.H{"message": "Expansion config updated successfully"})
}

// ExpansionTestRequest 展开测试请求
type ExpansionTestRequest struct {
	Current string          `json:"current" binding:"required"`
	History []plugin.Message `json:"history,omitempty"`
}

// ExpansionTestResponse 展开测试响应
type ExpansionTestResponse struct {
	Expanded     string `json:"expanded"`
	IsExpanded   bool   `json:"is_expanded"`
	Original     string `json:"original"`
}

// TestExpansion 测试查询展开
// @Summary 测试查询展开
// @Description 使用提供的对话历史测试查询展开功能
// @Tags expansion
// @Accept json
// @Produce json
// @Param request body ExpansionTestRequest true "测试请求"
// @Success 200 {object} ExpansionTestResponse
// @Router /api/expansion/test [post]
func (h *ExpansionHandler) TestExpansion(c *gin.Context) {
	var req ExpansionTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 如果没有配置展开器，返回原始查询
	if h.expander == nil {
		c.JSON(http.StatusOK, ExpansionTestResponse{
			Expanded:   req.Current,
			IsExpanded: false,
			Original:   req.Current,
		})
		return
	}

	// 转换消息格式
	history := make([]plugin.Message, len(req.History))
	for i, msg := range req.History {
		history[i] = plugin.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 执行展开
	expanded, isExpanded, err := h.expander.Expand(c.Request.Context(), req.Current, history)
	if err != nil {
		logger.Error("Expansion failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := ExpansionTestResponse{
		Expanded:   expanded,
		IsExpanded: isExpanded,
		Original:   req.Current,
	}

	c.JSON(http.StatusOK, response)
}

// GetSupportedPronouns 获取支持的指代词列表
// @Summary 获取支持的指代词
// @Description 获取查询展开模块支持的所有指代词列表
// @Tags expansion
// @Accept json
// @Produce json
// @Success 200 {object} gin.H{pronouns=[]string}
// @Router /api/expansion/pronouns [get]
func (h *ExpansionHandler) GetSupportedPronouns(c *gin.Context) {
	pronouns := defaultPronouns()
	c.JSON(http.StatusOK, gin.H{"pronouns": pronouns})
}

// defaultPronouns 返回默认指代词列表
func defaultPronouns() []string {
	return []string{
		// 中文 - 强指代
		"它", "这", "那", "这个", "那个",
		"前者", "后者", "上述", "该", "此",
		// 中文 - 人称
		"他", "她", "他们", "她们", "它们",
		// 中文 - 地点
		"这里", "那里", "这边", "那边",
		// 英文 - 强指代
		"it", "this", "that", "these", "those",
		// 英文 - 人称
		"he", "she", "they", "them", "him", "her",
		// 英文 - 地点
		"here", "there",
	}
}

// RegisterRoutes 注册路由
func (h *ExpansionHandler) RegisterRoutes(r *gin.RouterGroup) {
	exp := r.Group("/expansion")
	{
		exp.GET("/config", h.GetExpansionConfig)
		exp.PUT("/config", h.UpdateExpansionConfig)
		exp.POST("/test", h.TestExpansion)
		exp.GET("/pronouns", h.GetSupportedPronouns)
	}
}
