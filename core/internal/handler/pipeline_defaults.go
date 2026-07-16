package handler

import (
	"net/http"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

// PipelineDefaultsHandler 默认流水线配置处理器
type PipelineDefaultsHandler struct {
	cfg              *config.Config
	pipelineRegistry *pipeline.PipelineRegistry
	// PersistFn 若设置则替代 SaveSystemConfigToDB（minimal 写文件等场景）
	PersistFn func(defaultPipelineID string) error
}

// NewPipelineDefaultsHandler 创建默认流水线配置处理器
func NewPipelineDefaultsHandler(cfg *config.Config, registry *pipeline.PipelineRegistry) *PipelineDefaultsHandler {
	return &PipelineDefaultsHandler{cfg: cfg, pipelineRegistry: registry}
}

// SetPersistFn 设置自定义持久化（如 minimal 写 default-pipeline.yaml）
func (h *PipelineDefaultsHandler) SetPersistFn(fn func(defaultPipelineID string) error) {
	h.PersistFn = fn
}

// getPipelineConfig 获取 PipelineConfig，支持 nil 安全
func (h *PipelineDefaultsHandler) getPipelineConfig() *config.PipelineConfig {
	if h.cfg != nil && h.cfg.Proxy.PipelineConfig != nil {
		return h.cfg.Proxy.PipelineConfig
	}
	return nil
}

// GetDefaults 获取默认流水线配置
// GET /api/v1/pipeline/defaults
func (h *PipelineDefaultsHandler) GetDefaults(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"default_pipeline_id":  h.getDefaultPipelineID(),
		"allow_user_override":  h.isAllowUserOverride(),
	})
}

// UpdateDefaults 更新默认流水线配置
// PUT /api/v1/pipeline/defaults
func (h *PipelineDefaultsHandler) UpdateDefaults(c *gin.Context) {
	var req struct {
		DefaultPipelineID string `json:"default_pipeline_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验流水线 ID 是否有效
	if !h.isValidPipelineID(req.DefaultPipelineID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid pipeline_id: " + req.DefaultPipelineID,
		})
		return
	}

	// 校验是否允许作为系统默认
	if !h.isAllowedAsDefault(req.DefaultPipelineID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "pipeline '" + req.DefaultPipelineID + "' cannot be used as system default (requires additional headers)",
		})
		return
	}

	// 更新内存配置（与 DefaultPipelineResolver 共用同一 cfg 指针）
	pipelineConfig := h.getPipelineConfig()
	if pipelineConfig != nil {
		pipelineConfig.DefaultPipeline = req.DefaultPipelineID
	} else {
		h.cfg.Proxy.PipelineConfig = &config.PipelineConfig{
			DefaultPipeline:   req.DefaultPipelineID,
			AllowUserOverride: true,
		}
	}
	h.cfg.Proxy.DefaultMode = req.DefaultPipelineID

	if h.PersistFn != nil {
		if err := h.PersistFn(req.DefaultPipelineID); err != nil {
			logger.Errorf("[PipelineDefaults] Failed to persist default pipeline: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist default pipeline"})
			return
		}
	} else if err := config.SaveSystemConfigToDB(c.Request.Context(), config.KeyProxyConfig, h.cfg.Proxy); err != nil {
		logger.Errorf("[PipelineDefaults] Failed to persist default pipeline: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist default pipeline"})
		return
	}

	logger.Infof("[PipelineDefaults] Updated default pipeline: %s", req.DefaultPipelineID)

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"default_pipeline_id":  req.DefaultPipelineID,
	})
}

// getDefaultPipelineID 获取默认流水线 ID
func (h *PipelineDefaultsHandler) getDefaultPipelineID() string {
	if h.cfg == nil {
		return config.DefaultSystemPipelineID
	}
	return h.cfg.Proxy.EffectiveDefaultPipeline()
}

// isAllowUserOverride 是否允许用户覆盖默认流水线
func (h *PipelineDefaultsHandler) isAllowUserOverride() bool {
	pipelineConfig := h.getPipelineConfig()
	if pipelineConfig != nil {
		return pipelineConfig.AllowUserOverride
	}
	return true
}

// isValidPipelineID 校验流水线 ID 是否已在注册表中存在（含用户自定义流水线）
func (h *PipelineDefaultsHandler) isValidPipelineID(pipelineID string) bool {
	if pipelineID == "" {
		return false
	}
	if h.pipelineRegistry == nil {
		return false
	}
	for _, p := range h.pipelineRegistry.ListAll() {
		if p != nil && p.ID == pipelineID {
			return true
		}
	}
	return false
}

// isAllowedAsDefault 检查流水线是否允许作为系统默认
// 依赖额外请求头或仅缓存读写的模式，不适合作为无头请求的系统默认
func (h *PipelineDefaultsHandler) isAllowedAsDefault(pipelineID string) bool {
	disallowedAsDefault := map[string]bool{
		"raw-forward": true, // 需要 X-Target-URL / hostproxy
		"cache-hit":   true, // 仅缓存读取
		"cache-mode":  true, // 仅缓存写入
	}

	return !disallowedAsDefault[pipelineID]
}
