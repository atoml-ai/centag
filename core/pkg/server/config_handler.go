package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"centag/core/pkg/backend"
	"centag/core/internal/cache"
	"centag/core/pkg/config"
	"centag/core/pkg/embedding"
	"centag/core/internal/hostproxy"
	"centag/core/internal/llm"
	"centag/core/pkg/logger"
	"centag/core/pkg/processor"
	"centag/core/pkg/storage"

	"github.com/gin-gonic/gin"
)

// ConfigHandler 统一配置处理器
type ConfigHandler struct {
	storageManager  *storage.Manager
	cacheManager    *cache.Manager
	backendManager  *backend.Manager
	hostProxyServer *hostproxy.Server
	proxyCache      *cache.ProxyCache
	// mitmToggle 在 SystemProxy.Enabled 变化时被调用，由 Server 注册
	mitmToggle func(enabled bool)
	// mitmForceRestart 在 SystemProxy 端口变更时强制重启 MITM，由 Server 注册
	mitmForceRestart func()
}

// NewConfigHandler 创建统一配置处理器
func NewConfigHandler(storageMgr *storage.Manager, cacheMgr *cache.Manager, backendMgr *backend.Manager) *ConfigHandler {
	return &ConfigHandler{
		storageManager: storageMgr,
		cacheManager:   cacheMgr,
		backendManager: backendMgr,
	}
}

// SetHostProxyServer 注册 Host 代理服务器，用于开关热生效
func (h *ConfigHandler) SetHostProxyServer(s *hostproxy.Server) {
	h.hostProxyServer = s
}

// SetProxyCache 注册代理缓存，用于开关热生效
func (h *ConfigHandler) SetProxyCache(pc *cache.ProxyCache) {
	h.proxyCache = pc
}

// SetMitmToggle 注册 MITM 代理开关回调，用于开关热生效
func (h *ConfigHandler) SetMitmToggle(fn func(enabled bool)) {
	h.mitmToggle = fn
}

// SetMitmForceRestart 注册 MITM 强制重启回调，用于端口变更后的热生效
func (h *ConfigHandler) SetMitmForceRestart(fn func()) {
	h.mitmForceRestart = fn
}

// GetAllConfig 获取所有配置（统一接口）
func (h *ConfigHandler) GetAllConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		RespondInternalError(c, "Config not initialized")
		return
	}

	// 构造统一响应
	response := gin.H{
		"server":          cfg.Server,
		"log":             cfg.Log,
		"proxy":           cfg.Proxy,
		"cache":           cfg.Cache,
		"redis":           cfg.Redis,
		"vector":          cfg.Vector,
		"embedding":       cfg.Embedding,
		"qa_split":        cfg.QASplit,
		"question_split":  cfg.QuestionSplit,
		"plugins":         cfg.Plugins,
		"system_proxy":    cfg.SystemProxy,
		"host_proxy":      cfg.HostProxy,
		"backends":        cfg.Backends,
		"storages":        cfg.Storages,
		"default_storage": cfg.DefaultStorage,
		"model_matching":  cfg.ModelMatching, // 添加模型调度配置
		"scheduler":        cfg.Scheduler,      // 智能调度配置
	}

	RespondSuccess(c, response)
}

// SaveAllConfig 保存所有配置（统一接口）
func (h *ConfigHandler) SaveAllConfig(c *gin.Context) {
	// 1. 先读取并缓存原始请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		RespondBadRequest(c, "Failed to read request body: "+err.Error())
		return
	}

	// 2. 解析原始 JSON 到 map（用于 scheduler 手动解析，避免 Gin BindJSON 嵌套结构问题）
	var rawData map[string]json.RawMessage
	if json.Unmarshal(bodyBytes, &rawData) != nil {
		logger.Warn("[Config] Failed to parse raw JSON for manual parsing")
		rawData = nil
	}

	// 3. 恢复请求体供 BindJSON 使用
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 4. 使用 BindJSON 解析其他标准字段
	var req struct {
		Server         *config.ServerConfig          `json:"server"`
		Log            *config.LogConfig             `json:"log"`
		Proxy          *config.ProxyConfig           `json:"proxy"`
		Cache          *config.CacheConfig           `json:"cache"`
		Redis          *config.RedisConfig           `json:"redis"`
		Vector         *config.VectorConfig          `json:"vector"`
		Embedding      *config.EmbeddingConfig       `json:"embedding"`
		QASplit        *config.QASplitConfig         `json:"qa_split"`
		QuestionSplit  *config.QuestionSplitConfig   `json:"question_split"`
		Plugins        *config.PluginsConfig         `json:"plugins"`
		SystemProxy    *config.SystemProxyConfig     `json:"system_proxy"`
		HostProxy      *config.HostProxyConfig       `json:"host_proxy"`
		Backends       []config.BackendConfig        `json:"backends"`
		Storages       []config.StorageConfig        `json:"storages"`
		DefaultStorage string                        `json:"default_storage"`
		ModelMatching  *config.ModelMatchingConfig   `json:"model_matching"`
		Scheduler      config.SchedulerConfig        `json:"scheduler"` // 值类型，非指针
	}

	if !BindJSON(c, &req) {
		return
	}

	// 获取当前配置
	cfg := config.Get()
	if cfg == nil {
		RespondInternalError(c, "Config not initialized")
		return
	}

	// 提前记录旧状态，用于热更新判断
	oldCacheEnabled := cfg.Cache.Enabled
	oldSystemProxyEnabled := cfg.SystemProxy.Enabled
	oldHostProxyEnabled := cfg.HostProxy.Enabled
	oldSystemProxyPort := cfg.SystemProxy.ListenPort
	oldHostProxyHTTPPort := cfg.HostProxy.HTTPPort
	oldHostProxyHTTPSPort := cfg.HostProxy.HTTPSPort

	// 更新配置
	if req.Server != nil {
		cfg.Server = *req.Server
	}
	if req.Log != nil {
		cfg.Log = *req.Log
	}
	if req.Proxy != nil {
		cfg.Proxy = *req.Proxy
	}
	if req.Cache != nil {
		cfg.Cache = *req.Cache
	}
	if req.Redis != nil {
		cfg.Redis = *req.Redis
	}
	if req.Vector != nil {
		cfg.Vector = *req.Vector
	}
	if req.Embedding != nil {
		oldEnabled := cfg.Embedding.Enabled
		oldProvider := cfg.Embedding.Provider
		oldModel := cfg.Embedding.Model
		oldBackendID := cfg.Embedding.BackendID
		oldBaseURL := cfg.Embedding.BaseURL

		cfg.Embedding = *req.Embedding

		// 检查是否需要重新创建 EmbeddingService
		needRecreate := oldEnabled != req.Embedding.Enabled ||
			oldProvider != req.Embedding.Provider ||
			oldModel != req.Embedding.Model ||
			oldBackendID != req.Embedding.BackendID ||
			oldBaseURL != req.Embedding.BaseURL

		if needRecreate && h.cacheManager != nil && h.backendManager != nil {
			logger.Infof("Embedding config changed, recreating embedding service - enabled: %v, provider: %s, model: %s, backend_id: %s",
				req.Embedding.Enabled, req.Embedding.Provider, req.Embedding.Model, req.Embedding.BackendID)

			if req.Embedding.Enabled && req.Embedding.BackendID != "" {
				// 从后端管理器获取后端配置
				backend, err := h.backendManager.Get(req.Embedding.BackendID)
				if err != nil {
					logger.Warnf("Embedding backend %s not found: %v", req.Embedding.BackendID, err)
				} else if !backend.Enabled {
					logger.Warnf("Embedding backend %s is disabled", req.Embedding.BackendID)
				} else {
					// 创建嵌入服务配置：BaseURL 始终从后端配置中取，不使用请求中可能过时的值
					embeddingConfig := &embedding.EmbeddingConfig{
						Provider: backend.Type,
						Model:    req.Embedding.Model,
						BaseURL:  backend.BaseURL,
						Timeout:  req.Embedding.Timeout,
						Enabled:  req.Embedding.Enabled,
					}
					// 同步更新配置中的 BaseURL，确保持久化的值与后端实际地址一致
					cfg.Embedding.BaseURL = backend.BaseURL
					cfg.Embedding.Provider = backend.Type

					var embeddingSvc embedding.EmbeddingService

					// 根据后端类型创建对应的嵌入服务
					switch backend.Type {
					case "ollama":
						embeddingSvc, err = embedding.NewOllamaEmbeddingService(embeddingConfig)
					case "openai":
						embeddingSvc, err = embedding.NewOpenAIEmbeddingService(embeddingConfig)
					default:
						logger.Warnf("Unsupported embedding backend type: %s", backend.Type)
					}

					if err != nil {
						logger.Warnf("Failed to create embedding service: %v", err)
					} else if embeddingSvc != nil {
						// 获取现有的语义缓存
						semanticCache := h.cacheManager.GetSemanticCache()
						vectorStore := h.cacheManager.GetVectorStore()
						metrics := h.cacheManager.GetMetrics()

						if semanticCache != nil {
							// 更新现有语义缓存的嵌入服务
							semanticCache.SetEmbeddingService(embeddingSvc)
							h.cacheManager.SetEmbeddingService(embeddingSvc)
							logger.Infof("Embedding service updated in existing semantic cache")
						} else if vectorStore != nil {
							// 创建新的语义缓存
							semanticConfig := &cache.SemanticCacheConfig{
								CacheConfig: cache.CacheConfig{
									Enabled:         cfg.Cache.Enabled,
									DefaultTTL:      time.Duration(cfg.Cache.DefaultTTL) * time.Second,
									MaxSize:         int64(cfg.Cache.MaxCacheSize),
									CleanupInterval: time.Duration(cfg.Cache.CleanupInterval) * time.Second,
								},
								Threshold:           cfg.Cache.Semantic.Threshold,
								TopK:                cfg.Cache.Semantic.TopK,
								DistanceType:        cfg.Cache.Semantic.DistanceType,
								EnableAutoEmbedding: cfg.Cache.Semantic.EnableAutoEmbedding,
							}

							// 设置默认值
							if semanticConfig.Threshold == 0 {
								semanticConfig.Threshold = 0.85
							}
							if semanticConfig.TopK == 0 {
								semanticConfig.TopK = 5
							}
							if semanticConfig.DistanceType == "" {
								semanticConfig.DistanceType = "cosine"
							}

							newSemanticCache, err := cache.NewSemanticCacheWithMetrics(semanticConfig, embeddingSvc, metrics)
							if err != nil {
								logger.Warnf("Failed to create semantic cache: %v", err)
							} else {
								newSemanticCache.SetVectorStore(vectorStore)
								h.cacheManager.SetSemanticCache(newSemanticCache)
								h.cacheManager.SetEmbeddingService(embeddingSvc)
								h.cacheManager.SetSemanticConfig(semanticConfig)
								logger.Infof("New semantic cache created - provider: %s, model: %s, backend_type: %s",
									req.Embedding.Provider, req.Embedding.Model, backend.Type)
							}
						} else {
							// 只设置嵌入服务，不创建语义缓存
							h.cacheManager.SetEmbeddingService(embeddingSvc)
							logger.Infof("Embedding service set without semantic cache (no vector store)")
						}
					}
				}
			} else if !req.Embedding.Enabled {
				// 如果禁用了嵌入服务，移除语义缓存
				semanticCache := h.cacheManager.GetSemanticCache()
				if semanticCache != nil {
					h.cacheManager.SetSemanticCache(nil)
					h.cacheManager.SetEmbeddingService(nil)
					logger.Infof("Embedding service and semantic cache disabled")
				}
			}
		}
	}
	if req.QASplit != nil {
		oldBackendID := cfg.QASplit.BackendID
		oldModel := cfg.QASplit.Model
		oldEnabled := cfg.QASplit.Enabled

		cfg.QASplit = *req.QASplit

		// 检查是否需要重新创建 QASplitter
		// 当后端 ID、模型、或启用状态发生变化时需要重新创建
		needRecreate := oldBackendID != req.QASplit.BackendID ||
			oldModel != req.QASplit.Model ||
			oldEnabled != req.QASplit.Enabled

		if needRecreate && h.cacheManager != nil && h.backendManager != nil {
			logger.Infof("QA split config changed, recreating QA splitter - enabled: %v, backend_id: %s, model: %s",
				req.QASplit.Enabled, req.QASplit.BackendID, req.QASplit.Model)

			// 如果启用了问答拆分且配置了后端，创建新的 QASplitter
			if req.QASplit.Enabled && req.QASplit.BackendID != "" {
				// 从后端管理器获取后端配置
				backend, err := h.backendManager.Get(req.QASplit.BackendID)
				if err != nil {
					logger.Warnf("QA split backend %s not found: %v", req.QASplit.BackendID, err)
				} else if !backend.Enabled {
					logger.Warnf("QA split backend %s is disabled", req.QASplit.BackendID)
				} else {
					// 创建 chat service 配置
					qaSplitConfig := &llm.ChatConfig{
						Provider:    backend.Type,
						Model:       req.QASplit.Model,
						BaseURL:     backend.BaseURL,
						APIKey:      backend.APIKey,
						Timeout:     req.QASplit.Timeout,
						Temperature: req.QASplit.Temperature,
						MaxTokens:   req.QASplit.MaxTokens,
						Enabled:     req.QASplit.Enabled,
					}

					var chatService llm.ChatService

					// 根据后端类型创建对应的 chat service
					switch backend.Type {
					case "ollama":
						chatService, err = llm.NewOllamaChatService(qaSplitConfig)
					case "openai":
						chatService, err = llm.NewOpenAIChatService(qaSplitConfig)
					default:
						logger.Warnf("Unsupported QA split backend type: %s", backend.Type)
					}

					if err != nil {
						logger.Warnf("Failed to create QA split chat service: %v", err)
					} else if chatService != nil {
						// 创建新的 QASplitter
						qaSplitter := processor.NewQASplitter(&processor.QASplitterConfig{
							ChatService: chatService,
							Prompt:      req.QASplit.Prompt,
							Enabled:     req.QASplit.Enabled,
						})
						h.cacheManager.SetQASplitter(qaSplitter)
						logger.Infof("QA splitter recreated - backend_id: %s, model: %s, backend_type: %s",
							req.QASplit.BackendID, req.QASplit.Model, backend.Type)
					}
				}
			} else if !req.QASplit.Enabled {
				// 如果禁用了问答拆分，移除 QASplitter
				qaSplitter := h.cacheManager.GetQASplitter()
				if qaSplitter != nil {
					qaSplitter.SetEnabled(false)
					logger.Infof("QA splitter disabled")
				}
			}
		}
	}
	if req.Plugins != nil {
		cfg.Plugins = *req.Plugins
	}
	if req.SystemProxy != nil {
		// 只更新提供的字段,保留其他字段不变
		cfg.SystemProxy.Enabled = req.SystemProxy.Enabled
		if req.SystemProxy.ListenPort != 0 {
			cfg.SystemProxy.ListenPort = req.SystemProxy.ListenPort
		}
		// PACEnabled 字段需要特殊处理,因为它的默认值可能与false不同
		cfg.SystemProxy.PACEnabled = req.SystemProxy.PACEnabled
		// 不更新证书路径等敏感配置字段
		// 不更新domains和path_patterns,这些应该通过单独的API管理
	}
	if req.HostProxy != nil {
		// 合并更新：只覆盖显式提供的字段，保留 DomainMapping/证书路径等运行时数据
		if req.HostProxy.HTTPPort > 0 {
			cfg.HostProxy.HTTPPort = req.HostProxy.HTTPPort
		}
		if req.HostProxy.HTTPSPort > 0 {
			cfg.HostProxy.HTTPSPort = req.HostProxy.HTTPSPort
		}
		cfg.HostProxy.Enabled = req.HostProxy.Enabled
	}
	// 只有在 Backends 非空时才覆盖，避免前端误传空数组导致后端数据丢失
	if len(req.Backends) > 0 {
		// 前端因 omitempty 常不带 api_key；合并保留库中已有密钥，避免一次「保存配置」误清空
		cfg.Backends = backend.MergeBackendsPreserveAPIKeys(cfg.Backends, req.Backends)
	}
	if req.Storages != nil {
		cfg.Storages = req.Storages
	}
	if req.DefaultStorage != "" {
		cfg.DefaultStorage = req.DefaultStorage
	}
	// 添加模型调度配置更新
	if req.ModelMatching != nil {
		cfg.ModelMatching = *req.ModelMatching
	}
	// 问题拆分配置更新（仅更新配置，QuestionProcessor 由 server 在启动时初始化）
	if req.QuestionSplit != nil {
		cfg.QuestionSplit = *req.QuestionSplit
	}
	// 智能调度配置更新 - 使用原始 JSON 手动解析（避免 BindJSON 嵌套结构问题）
	if schedulerData, exists := rawData["scheduler"]; exists {
		var schedulerCfg config.SchedulerConfig
		if err := json.Unmarshal(schedulerData, &schedulerCfg); err != nil {
			logger.Errorf("Failed to unmarshal scheduler config: %v", err)
		} else {
			logger.Infof("Saving scheduler config: %d task strategies", len(schedulerCfg.TaskStrategies))
			// 诊断日志：打印每个策略的配置
			for i, strategy := range schedulerCfg.TaskStrategies {
				logger.Infof("Scheduler Strategy[%d]: TaskType=%s, Backend=%s, Model=%s",
					i, strategy.TaskType, strategy.RecommendedBackend, strategy.RecommendedModel)
			}
			cfg.Scheduler = schedulerCfg
		}
	} else {
		logger.Warnf("No scheduler config in request body")
	}

	// 保存配置到数据库
	if err := config.SaveConfig(cfg); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}

	logger.Info("All configurations saved to database successfully")

	// ---- 热更新：将配置变化传播到正在运行的组件 ----

	// 1. Host 代理启用状态热切换（不需要重启）
	if req.HostProxy != nil && cfg.HostProxy.Enabled != oldHostProxyEnabled && h.hostProxyServer != nil {
		h.hostProxyServer.SetEnabled(cfg.HostProxy.Enabled)
		logger.Infof("Host proxy enabled status hot-updated: %v", cfg.HostProxy.Enabled)
	}

	// 2. 缓存启用状态热切换
	if req.Cache != nil && cfg.Cache.Enabled != oldCacheEnabled && h.proxyCache != nil {
		h.proxyCache.SetEnabled(cfg.Cache.Enabled)
		logger.Infof("Proxy cache enabled status hot-updated: %v", cfg.Cache.Enabled)
	}

	// 2b. 仅保存模式 / 语义写入：manager.saveOnlyMode 仅在启动时设置，保存配置后必须同步，否则语义缓存永远不写入向量
	if req.Cache != nil && h.proxyCache != nil {
		h.proxyCache.SetSaveOnlyMode(cfg.Cache.SaveOnlyMode)
		logger.Infof("Cache save_only_mode hot-updated: %v (semantic embedding write follows this flag)",
			cfg.Cache.SaveOnlyMode)
	}

	// 3. 系统代理 (MITM) 启用状态热切换
	if req.SystemProxy != nil && cfg.SystemProxy.Enabled != oldSystemProxyEnabled && h.mitmToggle != nil {
		h.mitmToggle(cfg.SystemProxy.Enabled)
		logger.Infof("System proxy (MITM) enabled status hot-updated: %v", cfg.SystemProxy.Enabled)
	}

	// 3b. 系统代理端口变更且当前处于启用状态 → 强制重启 MITM，使新端口生效
	if req.SystemProxy != nil && cfg.SystemProxy.Enabled &&
		cfg.SystemProxy.ListenPort != oldSystemProxyPort && h.mitmForceRestart != nil {
		h.mitmForceRestart()
		logger.Infof("System proxy (MITM) force-restarted on new port %d", cfg.SystemProxy.ListenPort)
	}

	// 3c. Host 代理端口变更 → 热重启 HTTP/HTTPS 监听器
	if req.HostProxy != nil && h.hostProxyServer != nil &&
		(cfg.HostProxy.HTTPPort != oldHostProxyHTTPPort || cfg.HostProxy.HTTPSPort != oldHostProxyHTTPSPort) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.hostProxyServer.Restart(ctx, cfg.HostProxy.HTTPPort, cfg.HostProxy.HTTPSPort); err != nil {
			logger.Errorf("Failed to restart host proxy after port change: %v", err)
		} else {
			logger.Infof("Host proxy restarted: HTTP:%d, HTTPS:%d",
				cfg.HostProxy.HTTPPort, cfg.HostProxy.HTTPSPort)
		}
	}

	// 4. Backends 变化时同步 backendManager（只在实际更新了 backends 时才重载）
	if len(req.Backends) > 0 && h.backendManager != nil {
		if err := h.backendManager.Load(); err != nil {
			logger.Warnf("Failed to reload backend manager after config save: %v", err)
		} else {
			logger.Info("Backend manager reloaded after config save")
		}
	}

	RespondSuccessWithMessage(c, "Configuration saved successfully")
}
