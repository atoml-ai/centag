package server

import (
	"net/http"

	"centag/core/internal/cache/evaluation/manager"
	"centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EvaluationHandler 评估处理器
type EvaluationHandler struct {
	pluginManager       *manager.Manager
	exactMatchEnabled  bool // 精确匹配缓存是否启用
}

// NewEvaluationHandler 创建评估处理器
func NewEvaluationHandler(pm *manager.Manager, exactMatchEnabled bool) *EvaluationHandler {
	return &EvaluationHandler{
		pluginManager:      pm,
		exactMatchEnabled: exactMatchEnabled,
	}
}

// SetExactMatchEnabled 设置精确匹配缓存开关
func (h *EvaluationHandler) SetExactMatchEnabled(enabled bool) {
	h.exactMatchEnabled = enabled
}

// ListPlugins 列出所有插件
// @Summary 列出所有评估插件
// @Description 获取所有已注册的缓存评估插件列表
// @Tags evaluation
// @Accept json
// @Produce json
// @Success 200 {object} gin.H{plugins=[]PluginInfo}
// @Router /api/evaluation/plugins [get]
func (h *EvaluationHandler) ListPlugins(c *gin.Context) {
	plugins := h.pluginManager.ListPlugins()

	infos := make([]map[string]interface{}, 0, len(plugins))
	for _, p := range plugins {
		info := map[string]interface{}{
			"name":        p.Name(),
			"version":     p.Version(),
			"type":        p.Type(),
			"description": p.Description(),
			"enabled":     h.pluginManager.IsEnabled(p.Name()),
		}
		infos = append(infos, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": infos,
	})
}

// EnablePlugin 启用插件
// @Summary 启用指定插件
// @Description 启用指定的缓存评估插件
// @Tags evaluation
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/{name}/enable [post]
func (h *EvaluationHandler) EnablePlugin(c *gin.Context) {
	name := c.Param("name")

	if err := h.pluginManager.Enable(name); err != nil {
		logger.Error("Failed to enable plugin", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Plugin enabled", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "Plugin enabled successfully"})
}

// DisablePlugin 禁用插件
// @Summary 禁用指定插件
// @Description 禁用指定的缓存评估插件
// @Tags evaluation
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/{name}/disable [post]
func (h *EvaluationHandler) DisablePlugin(c *gin.Context) {
	name := c.Param("name")

	if err := h.pluginManager.Disable(name); err != nil {
		logger.Error("Failed to disable plugin", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Plugin disabled", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "Plugin disabled successfully"})
}

// UpdatePluginOrder 更新插件执行顺序
// @Summary 更新插件执行顺序
// @Description 更新插件在流水线中的执行顺序
// @Tags evaluation
// @Accept json
// @Produce json
// @Param order body []string true "插件名称数组，按执行顺序排列"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/order [put]
func (h *EvaluationHandler) UpdatePluginOrder(c *gin.Context) {
	var order []string
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.pluginManager.UpdateOrder(order); err != nil {
		logger.Error("Failed to update plugin order", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Plugin order updated", zap.Strings("order", order))
	c.JSON(http.StatusOK, gin.H{"message": "Plugin order updated successfully"})
}

// GetPluginConfig 获取插件配置
// @Summary 获取插件配置
// @Description 获取指定插件的当前配置
// @Tags evaluation
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Success 200 {object} gin.H{config=map[string]interface{}}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/{name}/config [get]
func (h *EvaluationHandler) GetPluginConfig(c *gin.Context) {
	name := c.Param("name")

	p, err := h.pluginManager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	config := p.GetConfig()
	c.JSON(http.StatusOK, gin.H{"config": config})
}

// UpdatePluginConfig 更新插件配置
// @Summary 更新插件配置
// @Description 更新指定插件的配置参数
// @Tags evaluation
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Param config body map[string]interface{} true "插件配置"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/{name}/config [put]
func (h *EvaluationHandler) UpdatePluginConfig(c *gin.Context) {
	name := c.Param("name")

	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.pluginManager.SetConfig(name, config); err != nil {
		logger.Error("Failed to update plugin config",
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Plugin config updated", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "Plugin config updated successfully"})
}

// GetPluginSchema 获取插件配置Schema
// @Summary 获取插件配置Schema
// @Description 获取指定插件的配置结构定义，用于动态生成配置表单
// @Tags evaluation
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Success 200 {object} gin.H{schema=plugin.ConfigSchema}
// @Failure 404 {object} gin.H{error=string}
// @Router /api/evaluation/plugins/{name}/schema [get]
func (h *EvaluationHandler) GetPluginSchema(c *gin.Context) {
	name := c.Param("name")

	p, err := h.pluginManager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	schema := p.GetConfigSchema()
	c.JSON(http.StatusOK, gin.H{"schema": schema})
}

// TestEvaluationRequest 测试评估请求
type TestEvaluationRequest struct {
	Question       string                `json:"question" binding:"required"`
	Answer         string                `json:"answer" binding:"required"`
	HistoryMessages []plugin.Message     `json:"history_messages,omitempty"`
	IsExpanded     bool                  `json:"is_expanded,omitempty"`
}

// TestEvaluationResponse 测试评估响应
type TestEvaluationResponse struct {
	ShouldCache bool                   `json:"should_cache"`
	FinalScore  float64                `json:"final_score"`
	Results     map[string]interface{} `json:"results"`
}

// TestEvaluation 测试评估流水线
// @Summary 测试评估流水线
// @Description 使用提供的问答数据测试评估流水线，查看各插件评分结果
// @Tags evaluation
// @Accept json
// @Produce json
// @Param request body TestEvaluationRequest true "测试请求"
// @Success 200 {object} TestEvaluationResponse
// @Router /api/evaluation/test [post]
func (h *EvaluationHandler) TestEvaluation(c *gin.Context) {
	var req TestEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	input := &plugin.EvalInput{
		Question:         req.Question,
		OriginalQuestion: req.Question,
		Answer:           req.Answer,
		HistoryMessages:  req.HistoryMessages,
		IsExpanded:       req.IsExpanded,
	}

	result, err := h.pluginManager.ExecutePipeline(c.Request.Context(), input)
	if err != nil {
		logger.Error("Pipeline execution failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 构建详细结果
	detailedResults := make(map[string]interface{})
	for name, output := range result.Results {
		detailedResults[name] = map[string]interface{}{
			"score":   output.Score,
			"passed":  output.Passed,
			"labels":  output.Labels,
			"details": output.Details,
		}
	}

	response := TestEvaluationResponse{
		ShouldCache: result.FinalOutput.Passed,
		FinalScore:  result.FinalOutput.Score,
		Results:     detailedResults,
	}

	c.JSON(http.StatusOK, response)
}

// GetEvaluationStats 获取评估统计信息
// @Summary 获取评估统计
// @Description 获取评估流水线的执行统计信息
// @Tags evaluation
// @Accept json
// @Produce json
// @Success 200 {object} gin.H{stats=map[string]interface{}}
// @Router /api/evaluation/stats [get]
func (h *EvaluationHandler) GetEvaluationStats(c *gin.Context) {
	stats := h.pluginManager.GetStats()

	// 添加 enabled 字段（基于是否有启用的插件）
	statsWithEnabled := map[string]interface{}{
		"enabled":           h.pluginManager.HasEnabledPlugins(),
		"total_executions": stats.TotalExecutions,
		"enabled_plugins":  stats.EnabledPlugins,
		"plugin_exec_times": stats.PluginExecTimes,
		"last_execution_time": stats.LastExecutionTime,
		"exact_match_enabled": h.exactMatchEnabled,
	}

	c.JSON(http.StatusOK, gin.H{"stats": statsWithEnabled})
}

// SetExactMatchEnabledAPI 设置精确匹配缓存开关
// @Summary 设置精确匹配缓存开关
// @Description 启用或禁用精确匹配缓存
// @Tags evaluation
// @Accept json
// @Produce json
// @Param request body struct{enabled bool} true "开关状态"
// @Success 200 {object} gin.H{message=string}
// @Router /api/v1/evaluation/config/exact-match [put]
func (h *EvaluationHandler) SetExactMatchEnabledAPI(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.exactMatchEnabled = req.Enabled
	logger.Info("Exact match cache enabled set", zap.Bool("enabled", req.Enabled))
	c.JSON(http.StatusOK, gin.H{"message": "Exact match cache config updated", "enabled": req.Enabled})
}

// RegisterRoutes 注册评估路由
func (h *EvaluationHandler) RegisterRoutes(r *gin.RouterGroup) {
	eval := r.Group("/evaluation")
	{
		// 插件管理
		eval.GET("/plugins", h.ListPlugins)
		eval.POST("/plugins/:name/enable", h.EnablePlugin)
		eval.POST("/plugins/:name/disable", h.DisablePlugin)
		eval.PUT("/plugins/order", h.UpdatePluginOrder)

		// 插件配置
		eval.GET("/plugins/:name/config", h.GetPluginConfig)
		eval.PUT("/plugins/:name/config", h.UpdatePluginConfig)
		eval.GET("/plugins/:name/schema", h.GetPluginSchema)

		// 流水线测试
		eval.POST("/test", h.TestEvaluation)

		// 统计信息
		eval.GET("/stats", h.GetEvaluationStats)

		// 精确匹配配置
		eval.PUT("/config/exact-match", h.SetExactMatchEnabledAPI)
	}
}