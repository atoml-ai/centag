package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// ── Minimal 版配置管理（无需认证，供 config-generator 前端调用）──────────────

// SupportedModel 支持的模型配置
type SupportedModel struct {
	RequestedModel     string  `yaml:"requested_model" json:"requested_model"`
	ActualModel        string  `yaml:"actual_model" json:"actual_model"`
	IsExact            bool    `yaml:"is_exact" json:"is_exact"`
	CompatibilityScore float64 `yaml:"compatibility_score" json:"compatibility_score"`
}

// Capabilities 后端能力配置
type Capabilities struct {
	MaxContextTokens int      `yaml:"max_context_tokens" json:"max_context_tokens"`
	Features         []string `yaml:"features" json:"features"`
	SupportsTools    bool     `yaml:"supports_tools" json:"supports_tools"`
}

// MinimalBackendConfig 后端配置结构
type MinimalBackendConfig struct {
	ID              string           `yaml:"id" json:"id"`
	Name            string           `yaml:"name" json:"name"`
	Type            string           `yaml:"type" json:"type"`
	BaseURL         string           `yaml:"base_url" json:"base_url"`
	APIKey          string           `yaml:"api_key" json:"api_key"`
	Enabled         bool             `yaml:"enabled" json:"enabled"`
	Timeout         int              `yaml:"timeout" json:"timeout"`
	MaxRetries      int              `yaml:"max_retries" json:"max_retries"`
	AutoFetchModels bool             `yaml:"auto_fetch_models" json:"auto_fetch_models"`
	Description     string           `yaml:"description" json:"description"`
	SupportedModels []SupportedModel `yaml:"supported_models" json:"supported_models"`
	Capabilities    Capabilities     `yaml:"capabilities" json:"capabilities"`
	Weight          int              `yaml:"weight" json:"weight"`
	Priority        int              `yaml:"priority" json:"priority"`
}

// MinimalBackendsConfig 完整后端配置
type MinimalBackendsConfig struct {
	Version           string                 `yaml:"version" json:"version"`
	Description       string                 `yaml:"description" json:"description"`
	Backends          []MinimalBackendConfig `yaml:"backends" json:"backends"`
	PipelineTemplates map[string]string      `yaml:"-" json:"pipeline_templates"`
}

// MinimalPipelineConfig Pipeline 配置
type MinimalPipelineConfig struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Active   bool   `yaml:"active" json:"active"`
	Filename string `yaml:"filename" json:"filename"`
}

// MinimalConfigHandler 配置管理处理器
type MinimalConfigHandler struct {
	dataDir         string
	mu              sync.RWMutex
	reloadFunc      func() error
	pipelineRegistry *pipeline.PipelineRegistry
}

// NewMinimalConfigHandler 创建配置处理器
func NewMinimalConfigHandler(dataDir string, reloadFunc func() error, pipelineRegistry *pipeline.PipelineRegistry) *MinimalConfigHandler {
	return &MinimalConfigHandler{
		dataDir:         dataDir,
		reloadFunc:      reloadFunc,
		pipelineRegistry: pipelineRegistry,
	}
}

// SetReloadFunc 设置重载函数
func (h *MinimalConfigHandler) SetReloadFunc(fn func() error) {
	h.reloadFunc = fn
}

// RegisterMinimalConfigRoutes 注册 minimal 版配置管理路由（无需认证）
func (h *MinimalConfigHandler) RegisterMinimalConfigRoutes(router *gin.RouterGroup) {
	// 配置管理 API
	config := router.Group("/api/config")
	{
		// 后端配置 CRUD
		config.GET("/backends", h.GetBackends)
		config.PUT("/backends", h.UpdateBackends)
		config.POST("/backends", h.AddBackend)
		config.DELETE("/backends/:id", h.DeleteBackend)

		// Pipeline 配置
		config.GET("/pipelines", h.GetPipelines)
		config.PUT("/pipeline/:id/activate", h.ActivatePipeline)
		config.PUT("/pipeline/:id/deactivate", h.DeactivatePipeline)

		// 默认流水线配置
		config.GET("/default-pipeline", h.GetDefaultPipeline)
		config.PUT("/default-pipeline", h.UpdateDefaultPipeline)

		// 配置重载
		config.POST("/reload", h.ReloadConfig)
	}
}

// GetBackends 获取后端配置
func (h *MinimalConfigHandler) GetBackends(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 优先从 dataDir 读取
	filePath := filepath.Join(h.dataDir, "initial-backends.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// fallback: 从 config/initdata 读取（Docker initdata archive）
	if data == nil {
		initdataRoot := bootstrap.InitdataRoot()
		if initdataRoot != "" {
			fallbackPath := filepath.Join(initdataRoot, "initial-backends.yaml")
			if fbData, fbErr := os.ReadFile(fallbackPath); fbErr == nil {
				data = fbData
			}
		}
	}

	if data == nil {
		c.JSON(http.StatusOK, MinimalBackendsConfig{Version: "2.0", Backends: []MinimalBackendConfig{}})
		return
	}

	var config MinimalBackendsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateBackends 更新后端配置
func (h *MinimalConfigHandler) UpdateBackends(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var config MinimalBackendsConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if config.Version == "" {
		config.Version = "2.0"
	}
	if config.Description == "" {
		config.Description = "Generated by Config Generator"
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 确保 dataDir 存在
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create data dir: " + err.Error()})
		return
	}

	filePath := filepath.Join(h.dataDir, "initial-backends.yaml")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 写入流水线模板文件
	if len(config.PipelineTemplates) > 0 {
		tmplDir := filepath.Join(h.dataDir, "pipeline-templates")
		if err := os.MkdirAll(tmplDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create pipeline-templates dir: " + err.Error()})
			return
		}
		for id, yamlContent := range config.PipelineTemplates {
			tmplPath := filepath.Join(tmplDir, id+".yaml")
			if err := os.WriteFile(tmplPath, []byte(yamlContent), 0644); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write template " + id + ": " + err.Error()})
				return
			}
		}
		logger.Infof("Written %d pipeline templates to %s", len(config.PipelineTemplates), tmplDir)
	}

	// 触发热重载
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated and reloaded"})
}

// AddBackend 添加后端
func (h *MinimalConfigHandler) AddBackend(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var backend MinimalBackendConfig
	if err := c.ShouldBindJSON(&backend); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 确保 dataDir 存在
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create data dir: " + err.Error()})
		return
	}

	filePath := filepath.Join(h.dataDir, "initial-backends.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var config MinimalBackendsConfig
	if data != nil {
		if err := yaml.Unmarshal(data, &config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse existing config: " + err.Error()})
			return
		}
	}

	config.Backends = append(config.Backends, backend)

	outData, err := yaml.Marshal(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := os.WriteFile(filePath, outData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.reloadFunc != nil {
		h.reloadFunc()
	}

	c.JSON(http.StatusOK, gin.H{"message": "backend added"})
}

// DeleteBackend 删除后端
func (h *MinimalConfigHandler) DeleteBackend(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := c.Param("id")

	filePath := filepath.Join(h.dataDir, "initial-backends.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"message": "backend deleted"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var config MinimalBackendsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 过滤掉指定 ID 的后端
	filtered := make([]MinimalBackendConfig, 0)
	for _, b := range config.Backends {
		if b.ID != id {
			filtered = append(filtered, b)
		}
	}
	config.Backends = filtered

	outData, err := yaml.Marshal(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := os.WriteFile(filePath, outData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.reloadFunc != nil {
		h.reloadFunc()
	}

	c.JSON(http.StatusOK, gin.H{"message": "backend deleted"})
}

// GetPipelines 获取 Pipeline 列表
func (h *MinimalConfigHandler) GetPipelines(c *gin.Context) {
	// 读取激活状态
	activeState := h.loadPipelineActivationState()
	
	// 如果没有激活状态文件，默认所有流水线都是激活的
	allActive := len(activeState.ActivePipelines) == 0

	if h.pipelineRegistry != nil {
		// 从内存注册表获取流水线
		pipelines := h.pipelineRegistry.List()
		result := make([]MinimalPipelineConfig, 0, len(pipelines))
		
		for _, p := range pipelines {
		 isActive := allActive
		 if !allActive {
		   for _, activeID := range activeState.ActivePipelines {
		     if activeID == p.ID {
		       isActive = true
		       break
		     }
		   }
		 }
		 
		 result = append(result, MinimalPipelineConfig{
		   ID:       p.ID,
		   Name:     p.Name,
		   Active:   isActive,
		   Filename: p.ID + ".yaml",
		 })
		}
		
		c.JSON(http.StatusOK, result)
		return
	}
	
	// 回退到文件系统读取
	pipelinesDir := filepath.Join(h.dataDir, "pipeline-templates")

	entries, err := os.ReadDir(pipelinesDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, []MinimalPipelineConfig{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var pipelines []MinimalPipelineConfig
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		filePath := filepath.Join(pipelinesDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var meta struct {
			ID   string `yaml:"id"`
			Name string `yaml:"name"`
		}
		if err := yaml.Unmarshal(data, &meta); err != nil {
			// Skip files that cannot be parsed
			logger.Warnf("[MinimalConfig] Failed to parse pipeline template %s: %v", entry.Name(), err)
			continue
		}

		isActive := allActive
		if !allActive {
			for _, activeID := range activeState.ActivePipelines {
				if activeID == meta.ID {
					isActive = true
					break
				}
			}
		}

		pipelines = append(pipelines, MinimalPipelineConfig{
			ID:       meta.ID,
			Name:     meta.Name,
			Active:   isActive,
			Filename: entry.Name(),
		})
	}

	c.JSON(http.StatusOK, pipelines)
}

// loadPipelineActivationState 从文件加载流水线激活状态
func (h *MinimalConfigHandler) loadPipelineActivationState() PipelineActivationState {
	stateFile := filepath.Join(h.dataDir, "pipeline-activation.yaml")
	state := PipelineActivationState{
		ActivePipelines: []string{},
	}
	
	if data, err := os.ReadFile(stateFile); err == nil {
		_ = yaml.Unmarshal(data, &state)
	}
	
	return state
}

// ActivatePipeline 激活 Pipeline
func (h *MinimalConfigHandler) ActivatePipeline(c *gin.Context) {
	id := c.Param("id")
	
	if h.pipelineRegistry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pipeline registry not available"})
		return
	}

	// 检查流水线是否存在
	p := h.pipelineRegistry.Get(id)
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pipeline not found", "id": id})
		return
	}

	// 保存激活状态到文件
	if err := h.savePipelineActivationState(id, true); err != nil {
		logger.Warnf("[MinimalConfig] Failed to save activation state for pipeline %s: %v", id, err)
	}

	// 触发热重载
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			logger.Warnf("[MinimalConfig] Failed to reload config after activating pipeline %s: %v", id, err)
		}
	}

	logger.Infof("[MinimalConfig] Pipeline activated: %s (%s)", id, p.Name)
	c.JSON(http.StatusOK, gin.H{
		"message":  "pipeline activated",
		"id":       id,
		"name":     p.Name,
		"active":   true,
	})
}

// DeactivatePipeline 停用 Pipeline
func (h *MinimalConfigHandler) DeactivatePipeline(c *gin.Context) {
	id := c.Param("id")
	
	if h.pipelineRegistry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pipeline registry not available"})
		return
	}

	// 检查流水线是否存在
	p := h.pipelineRegistry.Get(id)
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pipeline not found", "id": id})
		return
	}

	// 保存停用状态到文件
	if err := h.savePipelineActivationState(id, false); err != nil {
		logger.Warnf("[MinimalConfig] Failed to save deactivation state for pipeline %s: %v", id, err)
	}

	// 触发热重载
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			logger.Warnf("[MinimalConfig] Failed to reload config after deactivating pipeline %s: %v", id, err)
		}
	}

	logger.Infof("[MinimalConfig] Pipeline deactivated: %s (%s)", id, p.Name)
	c.JSON(http.StatusOK, gin.H{
		"message":  "pipeline deactivated",
		"id":       id,
		"name":     p.Name,
		"active":   false,
	})
}

// PipelineActivationState 流水线激活状态
type PipelineActivationState struct {
	ActivePipelines []string `yaml:"active_pipelines" json:"active_pipelines"`
}

// savePipelineActivationState 保存流水线激活状态到文件
func (h *MinimalConfigHandler) savePipelineActivationState(pipelineID string, active bool) error {
	stateFile := filepath.Join(h.dataDir, "pipeline-activation.yaml")
	
	// 读取现有状态
	state := PipelineActivationState{
		ActivePipelines: []string{},
	}
	
	if data, err := os.ReadFile(stateFile); err == nil {
		_ = yaml.Unmarshal(data, &state)
	}
	
	// 更新状态
	if active {
		// 添加到激活列表（如果不存在）
		found := false
		for _, id := range state.ActivePipelines {
			if id == pipelineID {
				found = true
				break
			}
		}
		if !found {
			state.ActivePipelines = append(state.ActivePipelines, pipelineID)
		}
	} else {
		// 从激活列表中移除
		filtered := make([]string, 0, len(state.ActivePipelines))
		for _, id := range state.ActivePipelines {
			if id != pipelineID {
				filtered = append(filtered, id)
			}
		}
		state.ActivePipelines = filtered
	}
	
	// 确保数据目录存在
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}
	
	// 写入文件
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	
	return os.WriteFile(stateFile, data, 0644)
}

// ReloadConfig 重载配置
func (h *MinimalConfigHandler) ReloadConfig(c *gin.Context) {
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "config reloaded"})
}

// ── 默认流水线配置 API ─────────────────────────────────────────────────────

// DefaultPipelineConfig 默认流水线配置文件结构
type DefaultPipelineConfig struct {
	DefaultPipeline string `yaml:"default_pipeline" json:"default_pipeline"`
}

// GetDefaultPipeline 获取默认流水线配置
// GET /api/config/default-pipeline
func (h *MinimalConfigHandler) GetDefaultPipeline(c *gin.Context) {
	cfg := DefaultPipelineConfig{}

	pipelineFile := filepath.Join(h.dataDir, "default-pipeline.yaml")
	data, err := os.ReadFile(pipelineFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No file yet — return current runtime default
			c.JSON(http.StatusOK, gin.H{
				"default_pipeline": h.getRuntimeDefaultPipeline(),
				"source":          "fallback",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"default_pipeline": cfg.DefaultPipeline,
		"source":          "file",
	})
}

// UpdateDefaultPipeline 更新默认流水线配置
// PUT /api/config/default-pipeline
func (h *MinimalConfigHandler) UpdateDefaultPipeline(c *gin.Context) {
	var req struct {
		DefaultPipeline string `json:"default_pipeline" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := DefaultPipelineConfig{
		DefaultPipeline: req.DefaultPipeline,
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure data directory exists
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pipelineFile := filepath.Join(h.dataDir, "default-pipeline.yaml")
	if err := os.WriteFile(pipelineFile, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update runtime config in-memory (DefaultPipelineResolver shares the same cfg pointer)
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
			return
		}
	}

	logger.Infof("[MinimalConfig] Default pipeline updated to: %s", req.DefaultPipeline)
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"default_pipeline": req.DefaultPipeline,
	})
}

// getRuntimeDefaultPipeline reads the current default pipeline from the global config.
func (h *MinimalConfigHandler) getRuntimeDefaultPipeline() string {
	cfg := config.Get()
	if cfg != nil {
		return cfg.Proxy.EffectiveDefaultPipeline()
	}
	return "smart-scheduling"
}
