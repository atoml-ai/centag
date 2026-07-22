package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/internal/cache"
	"centag/core/internal/hostproxy"
	"centag/core/internal/llm"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/embedding"
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
	// mitmSyncEgress 将当前出口 API Key 热同步到运行中的 MITM
	mitmSyncEgress func()
	// proxyHandlerRefresh 刷新 PAC 生成器（advertise/listen 变更）
	proxyHandlerRefresh func()
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

// SetMitmSyncEgress 注册 MITM 出口 Key 热同步回调
func (h *ConfigHandler) SetMitmSyncEgress(fn func()) {
	h.mitmSyncEgress = fn
}

// SetProxyHandlerRefresh 注册 PAC 刷新回调
func (h *ConfigHandler) SetProxyHandlerRefresh(fn func()) {
	h.proxyHandlerRefresh = fn
}

// GetAllConfig 获取所有配置（统一接口）
func (h *ConfigHandler) GetAllConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		RespondInternalError(c, "Config not initialized")
		return
	}

	// 构造统一响应（出口 Key 不回传明文）
	spView := cfg.SystemProxy
	if strings.TrimSpace(spView.EgressAPIKey) != "" {
		spView.EgressAPIKey = "***"
	}

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
		"system_proxy":    spView,
		"host_proxy":      cfg.HostProxy,
		"backends":        cfg.Backends,
		"storages":        cfg.Storages,
		"default_storage": cfg.DefaultStorage,
		"model_matching":  cfg.ModelMatching, // 添加模型调度配置
		"scheduler":       cfg.Scheduler,     // 智能调度配置
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
	oldSystemProxyListenAddr := cfg.SystemProxy.ListenAddr
	oldSystemProxyAllowLAN := cfg.SystemProxy.AllowLANClients
	oldSystemProxyAdvertise := cfg.SystemProxy.AdvertiseHost
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
		cfg.SystemProxy.AllowLANClients = req.SystemProxy.AllowLANClients
		if req.SystemProxy.ListenAddr != "" {
			cfg.SystemProxy.ListenAddr = req.SystemProxy.ListenAddr
		}
		// Allow clearing advertise only when LAN disabled; otherwise take provided value
		if req.SystemProxy.AllowLANClients || req.SystemProxy.AdvertiseHost != "" {
			cfg.SystemProxy.AdvertiseHost = req.SystemProxy.AdvertiseHost
		}
		// 出口 Key：非空且非掩码时更新（Agent 零改场景由 MITM 注入）
		if k := strings.TrimSpace(req.SystemProxy.EgressAPIKey); k != "" && k != "***" {
			cfg.SystemProxy.EgressAPIKey = k
		}
		if err := config.ValidateSystemProxyConfig(&cfg.SystemProxy); err != nil {
			RespondBadRequest(c, "Invalid system_proxy config: "+err.Error())
			return
		}
		// 不更新证书路径等敏感配置字段
		// 不更新domains和path_patterns,这些应该通过单独的API管理
	}

	// MITM 开启时自动绑定出口 Key（热生效，无需改环境变量停服）
	if cfg.SystemProxy.Enabled {
		uid, _ := auth.GetUserID(c)
		if changed, err := EnsureSystemProxyEgressAPIKey(c.Request.Context(), cfg, uid); err != nil {
			logger.Warnf("system_proxy: ensure egress API key: %v", err)
		} else if changed {
			logger.Info("system_proxy: egress API key ensured before config save")
		}
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

	// 2c. 熔断器配置热更新（无需重启）
	if req.Proxy != nil && req.Proxy.CircuitBreaker != nil {
		hotReloadCircuitBreaker()
	}

	// 3. 系统代理 (MITM) 启用状态热切换
	if req.SystemProxy != nil && cfg.SystemProxy.Enabled != oldSystemProxyEnabled && h.mitmToggle != nil {
		h.mitmToggle(cfg.SystemProxy.Enabled)
		logger.Infof("System proxy (MITM) enabled status hot-updated: %v", cfg.SystemProxy.Enabled)
	}

	// 3b. 系统代理监听/LAN 参数变更且当前启用 → 强制重启 MITM
	if req.SystemProxy != nil && cfg.SystemProxy.Enabled && h.mitmForceRestart != nil &&
		(cfg.SystemProxy.ListenPort != oldSystemProxyPort ||
			cfg.SystemProxy.ListenAddr != oldSystemProxyListenAddr ||
			cfg.SystemProxy.AllowLANClients != oldSystemProxyAllowLAN ||
			cfg.SystemProxy.AdvertiseHost != oldSystemProxyAdvertise) {
		h.mitmForceRestart()
		logger.Infof("System proxy (MITM) force-restarted on %s (lan=%v advertise=%s)",
			cfg.SystemProxy.MITMListenAddr(), cfg.SystemProxy.AllowLANClients, cfg.SystemProxy.AdvertiseHost)
	}

	// 3b2. PAC advertise/host 变更时刷新 PAC 生成器（即使 MITM 未启）
	if req.SystemProxy != nil && h.proxyHandlerRefresh != nil {
		h.proxyHandlerRefresh()
	}

	// 3b3. 出口 API Key 热同步到 MITM（Agent 零改：MITM 注入 Centag Key）
	if req.SystemProxy != nil && cfg.SystemProxy.Enabled && h.mitmSyncEgress != nil {
		h.mitmSyncEgress()
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

// EnsureSystemProxyEgress handles POST /api/v1/proxy/egress-key/ensure
// Creates or reuses system-proxy-egress key and binds it (hot sync, no restart).
func (h *ConfigHandler) EnsureSystemProxyEgress(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		RespondInternalError(c, "Config not initialized")
		return
	}
	uid, _ := auth.GetUserID(c)
	changed, err := EnsureSystemProxyEgressAPIKey(c.Request.Context(), cfg, uid)
	if err != nil {
		RespondInternalError(c, "ensure egress API key: "+err.Error())
		return
	}
	if changed {
		if err := config.SaveConfig(cfg); err != nil {
			RespondInternalError(c, "Failed to save config: "+err.Error())
			return
		}
	}
	if cfg.SystemProxy.Enabled && h.mitmSyncEgress != nil {
		h.mitmSyncEgress()
	}
	RespondSuccess(c, gin.H{
		"configured": config.ResolveSystemProxyEgressAPIKey(&cfg.SystemProxy) != "",
		"changed":    changed,
		"key_name":   SystemProxyEgressKeyName,
	})
}

type bindEgressRequest struct {
	APIKeyID int64 `json:"api_key_id" binding:"required"`
}

// BindSystemProxyEgress handles POST /api/v1/proxy/egress-key/bind
func (h *ConfigHandler) BindSystemProxyEgress(c *gin.Context) {
	var req bindEgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	cfg := config.Get()
	if cfg == nil {
		RespondInternalError(c, "Config not initialized")
		return
	}
	if err := BindSystemProxyEgressAPIKeyByID(c.Request.Context(), cfg, req.APIKeyID); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if err := config.SaveConfig(cfg); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}
	if cfg.SystemProxy.Enabled && h.mitmSyncEgress != nil {
		h.mitmSyncEgress()
	}
	RespondSuccess(c, gin.H{
		"configured": true,
		"api_key_id": req.APIKeyID,
	})
}
