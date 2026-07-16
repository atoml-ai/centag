package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CacheV2Config 缓存V2配置
type CacheV2Config struct {
	Enabled           bool                     `json:"enabled"`
	ExactMatchEnabled bool                     `json:"exact_match_enabled"`
	SemanticEnabled   bool                     `json:"semantic_enabled"`
	Expansion         ExpansionConfig          `json:"expansion"`
	Evaluation        EvaluationPipelineConfig `json:"evaluation"`
}

// ExpansionConfig 展开配置（用于JSON序列化，避免循环导入）
type ExpansionConfig struct {
	Mode      string          `json:"mode"`
	RuleBased RuleBasedConfig `json:"rule_based,omitempty"`
}

// EvaluationPipelineConfig 评估流水线配置
type EvaluationPipelineConfig struct {
	Enabled bool                   `json:"enabled"`
	Plugins []PluginPipelineConfig `json:"plugins"`
}

// PluginPipelineConfig 插件流水线配置
type PluginPipelineConfig struct {
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Order   int                    `json:"order"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// CacheV2Handler 缓存V2处理器
type CacheV2Handler struct{}

// NewCacheV2Handler 创建缓存V2处理器
func NewCacheV2Handler() *CacheV2Handler {
	return &CacheV2Handler{}
}

// GetCacheV2Config 获取缓存V2配置
// @Summary 获取缓存V2配置
// @Description 获取缓存V2优化功能的完整配置
// @Tags cache-v2
// @Accept json
// @Produce json
// @Success 200 {object} CacheV2Config
// @Router /api/cache/v2/config [get]
func (h *CacheV2Handler) GetCacheV2Config(c *gin.Context) {
	// 返回默认配置
	config := CacheV2Config{
		Enabled:           true,
		ExactMatchEnabled: false, // 默认关闭精确匹配
		SemanticEnabled:   true,
		Expansion: ExpansionConfig{
			Mode: "rule",
			RuleBased: RuleBasedConfig{
				Enabled:          true,
				MaxHistoryRounds: 3,
			},
		},
		Evaluation: EvaluationPipelineConfig{
			Enabled: true,
			Plugins: []PluginPipelineConfig{
				{Name: "follow_up_detector", Enabled: true, Order: 1},
				{Name: "length_evaluator", Enabled: true, Order: 2},
				{Name: "weighted_aggregator", Enabled: true, Order: 3},
			},
		},
	}

	c.JSON(http.StatusOK, config)
}

// UpdateCacheV2Config 更新缓存V2配置
// @Summary 更新缓存V2配置
// @Description 更新缓存V2优化功能的配置参数
// @Tags cache-v2
// @Accept json
// @Produce json
// @Param config body CacheV2Config true "缓存V2配置"
// @Success 200 {object} gin.H{message=string}
// @Failure 400 {object} gin.H{error=string}
// @Router /api/cache/v2/config [put]
func (h *CacheV2Handler) UpdateCacheV2Config(c *gin.Context) {
	var config CacheV2Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// TODO: 实际保存配置到配置文件或数据库

	c.JSON(http.StatusOK, gin.H{"message": "Cache V2 config updated successfully"})
}

// GetCacheV2Status 获取缓存V2状态
// @Summary 获取缓存V2状态
// @Description 获取缓存V2优化功能的运行状态和统计信息
// @Tags cache-v2
// @Accept json
// @Produce json
// @Success 200 {object} gin.H{status=map[string]interface{}}
// @Router /api/cache/v2/status [get]
func (h *CacheV2Handler) GetCacheV2Status(c *gin.Context) {
	status := map[string]interface{}{
		"enabled":             true,
		"expansion_enabled":   true,
		"evaluation_enabled":  true,
		"plugins_registered":  3,
		"total_evaluations":   0,
		"cache_hit_rate":      0.0,
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

// ResetCacheV2Stats 重置缓存V2统计
// @Summary 重置缓存V2统计
// @Description 重置缓存V2优化功能的统计数据
// @Tags cache-v2
// @Accept json
// @Produce json
// @Success 200 {object} gin.H{message=string}
// @Router /api/cache/v2/stats/reset [post]
func (h *CacheV2Handler) ResetCacheV2Stats(c *gin.Context) {
	// TODO: 实际重置统计

	c.JSON(http.StatusOK, gin.H{"message": "Cache V2 stats reset successfully"})
}

// RegisterRoutes 注册路由
func (h *CacheV2Handler) RegisterRoutes(r *gin.RouterGroup) {
	v2 := r.Group("/cache/v2")
	{
		v2.GET("/config", h.GetCacheV2Config)
		v2.PUT("/config", h.UpdateCacheV2Config)
		v2.GET("/status", h.GetCacheV2Status)
		v2.POST("/stats/reset", h.ResetCacheV2Stats)
	}
}

// RuleBasedConfig 规则展开配置（用于JSON序列化）
type RuleBasedConfig struct {
	Enabled          bool     `json:"enabled"`
	MaxHistoryRounds int      `json:"max_history_rounds"`
	Pronouns         []string `json:"pronouns,omitempty"`
}
