package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof" // pprof 观测
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"centag/core/internal"
	"centag/core/internal/abeval"
	agentpkg "centag/core/internal/agent"
	"centag/core/internal/auth"
	"centag/core/internal/billing"
	"centag/core/internal/cache"
	evalmanager "centag/core/internal/cache/evaluation/manager"
	evaluationplugins "centag/core/internal/cache/evaluation/plugins"
	cacheexpansion "centag/core/internal/cache/expansion"
	cachestrategy "centag/core/internal/cache/strategy"
	"centag/core/internal/conversation"
	"centag/core/internal/edition"
	"centag/core/internal/handler"
	"centag/core/internal/hostproxy"
	"centag/core/internal/llm"
	"centag/core/internal/middleware"
	"centag/core/internal/mitm"
	"centag/core/internal/pac"
	"centag/core/internal/proxy"
	"centag/core/internal/session"
	"centag/core/internal/strategy"
	"centag/core/internal/tokenusage"
	"centag/core/internal/webhook"
	"centag/core/pkg/abevalapi"
	"centag/core/pkg/agentmemory"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/editionmodule"
	"centag/core/pkg/embedding"
	"centag/core/pkg/extension"
	"centag/core/pkg/hooks"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	pluginapi "centag/core/pkg/plugin"
	pluginregistry "centag/core/pkg/plugin/registry"
	"centag/core/pkg/processor"
	"centag/core/pkg/proxymode"
	"centag/core/pkg/storage"
	"centag/core/pkg/systemupdateapi"
	"centag/core/pkg/tokenusageapi"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server HTTP 服务器
type Server struct {
	router                 *gin.Engine
	server                 *http.Server
	cfg                    *config.Config
	pluginManager          *plugin.Manager
	backendManager         *backend.Manager
	storageManager         *storage.Manager
	cacheManager           *cache.Manager
	proxyCache             *cache.ProxyCache
	proxyHandler           *proxy.Handler
	backendHandler         *BackendHandler
	storageHandler         *StorageHandler
	dataStoreHandler       *DataStoreHandler
	cacheHandler           *CacheHandler
	metricsHandler         *MetricsHandler
	configHandler          *ConfigHandler
	authHandler            *AuthHandler
	userHandler            *UserHandler
	apiKeyHandler          *APIKeyHandler
	strategyHandler        *handler.StrategyHandler
	proxyHandlerExt        *handler.ProxyHandler
	clashHandler           *ClashHandler
	evaluationHandler      *EvaluationHandler
	evaluationManager      *evalmanager.Manager
	logHandler             *LogHandler
	mitmServer             *mitm.Server
	mitmMu                 sync.Mutex // 保护 mitmServer 的并发访问
	hostProxyServer        *hostproxy.Server
	hostProxyHandler       *hostproxy.Handler
	systemUpdate           *internal.SystemUpdateHandler
	tokenUsageHandler      *TokenUsageHandler
	costHandler            *CostHandler
	billingRulesHandler    *BillingRulesHandler
	personalBillingHandler *PersonalBillingHandler
	pricingService         billing.PricingService
	memoryHandler          *MemoryHandler
	modeManager            *proxymode.ModeManager
	proxyModeHandler       *ProxyModeHandler
	pipelineHandler        *PipelineHandler
	webhookHandler         *webhook.Handler
	pluginRegistryAPI      *pluginregistry.Handler
	sessionStore           *session.ProxyModeStore
	userQuotaMiddleware    *middleware.UserQuotaMiddleware // v2.1: User-level quota
	edition                edition.Edition
	agentHandler           *AgentHandler
	agentProviderHandler   *agentpkg.AgentProviderHandler
	builtinAgentHandler    *BuiltinAgentHandler
	mcpProxyHandler        *MCPProxyHandler
	hookManager            *hooks.DefaultHookManager
	conversationHandler    *ConversationHandler
	conversationStore      conversation.Store
	extensionHost          *extension.RuntimeHost

	// 流水线默认模式相关 handler
	pipelineDefaultsHandler *handler.PipelineDefaultsHandler
	// 模型变量配置 handler
	modelConfigHandler *handler.ModelConfigHandler

	startTime time.Time
	mitmWg    sync.WaitGroup

	// onProxyConfigChanged 系统默认后端配置保存成功后的联动钩子：
	// 刷新 CapabilityBroker 默认 LLM 目标并幂等重建 centag-ops-router，
	// 使动态生成的 skill 路由管线始终跟随最新 default_backend/model。
	onProxyConfigChanged func(defaultBackendID, defaultModel string)
}

// New 创建服务器
func New(cfg *config.Config) *Server {
	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 创建路由(不使用gin.Default(),因为它会自动添加Logger中间件)
	router := gin.New()

	// 添加中间件
	router.Use(recoveryMiddleware())
	router.Use(corsMiddleware())
	router.Use(loggerMiddleware())

	// 创建插件管理器
	pluginManager := plugin.NewManager()

	// 注册插件
	if err := registerPlugins(pluginManager); err != nil {
		logger.Errorf("Failed to register plugins: %v", err)
	}

	// 创建后端管理器
	backendManager := backend.GetManager()
	// 使用统一的配置路径
	if err := backendManager.Load(); err != nil {
		logger.Warnf("Failed to load backend configs: %v", err)
	}

	// 加载全局降级策略（热生效，无需重启）
	fallbackStore := config.GetFallbackPolicyStore()
	if err := fallbackStore.Load(); err != nil {
		logger.Warnf("Failed to load fallback policies: %v", err)
	}

	// 创建存储管理器
	storageManager, err := storage.NewManager("")
	if err != nil {
		logger.Errorf("Failed to create storage manager: %v", err)
	}
	if storageManager != nil {
		if err := storageManager.LoadConfig(); err != nil {
			logger.Warnf("Failed to load storage config: %v", err)
		}
	}

	// 创建缓存管理器
	cacheConfig := &cache.CacheConfig{
		Enabled:         true,
		DefaultTTL:      3600 * time.Second,
		MaxSize:         1000,
		CleanupInterval: 5 * time.Minute,
	}
	cacheManager, err := cache.NewManager(cacheConfig)
	if err != nil {
		logger.Errorf("Failed to create cache manager: %v", err)
	}

	// embedding service（供语义缓存和记忆服务使用）
	var embeddingSvc embedding.EmbeddingService
	var embErr error

	// 注入存储到缓存管理器
	if storageManager != nil {
		// 获取默认KV存储 (可选,不依赖Redis)
		if kvStore, err := storageManager.GetDefaultKVStore(); err == nil {
			cacheManager.SetKVStore(kvStore)
		} else {
			logger.Infof("No default KV store configured: %v (exact cache will use memory only)", err)
		}

		// 注册回调：当默认KV存储变更时，自动更新缓存管理器的KVStore
		storageManager.RegisterKVStoreChangeCallback(func(kvStore storage.KVStore) {
			if kvStore != nil {
				cacheManager.SetKVStore(kvStore)
				logger.Info("Cache manager KV store updated due to default storage change")
			}
		})

		// ── 语义缓存初始化（与向量存储解耦）────────────────────────────────────
		// embedding service + 语义缓存始终尝试创建（只需要 Ollama 可用）。
		// 向量存储（Elasticsearch/ChromaDB）是可选的持久化后端，不可用时降级到纯内存。
		if cfg.Embedding.Enabled {
			cfgEmbeddingConfig := cfg.Embedding

			// 优先从 BackendID 对应的后端配置中获取实际 BaseURL 和 Provider，
			// 避免使用可能已过期的默认值（如 localhost:21434）。
			resolvedBaseURL := cfgEmbeddingConfig.BaseURL
			resolvedProvider := cfgEmbeddingConfig.Provider
			if cfgEmbeddingConfig.BackendID != "" {
				if b, berr := backendManager.Get(cfgEmbeddingConfig.BackendID); berr == nil && b.Enabled {
					resolvedBaseURL = b.BaseURL
					resolvedProvider = b.Type
					logger.Infof("Embedding: resolved backend %s → provider: %s, url: %s",
						cfgEmbeddingConfig.BackendID, resolvedProvider, resolvedBaseURL)
				} else if berr != nil {
					logger.Warnf("Embedding backend %s not found: %v – falling back to configured base_url: %s",
						cfgEmbeddingConfig.BackendID, berr, resolvedBaseURL)
				} else {
					logger.Warnf("Embedding backend %s is disabled – falling back to configured base_url: %s",
						cfgEmbeddingConfig.BackendID, resolvedBaseURL)
				}
			}

			embeddingConfig := &embedding.EmbeddingConfig{
				Provider: resolvedProvider,
				Model:    cfgEmbeddingConfig.Model,
				BaseURL:  resolvedBaseURL,
				Timeout:  cfgEmbeddingConfig.Timeout,
				Enabled:  cfgEmbeddingConfig.Enabled,
			}
			embeddingSvc, embErr = embedding.NewOllamaEmbeddingService(embeddingConfig)
			if embErr != nil {
				logger.Warnf("Failed to create embedding service: %v (semantic cache will use keyword fallback)", embErr)
			} else {
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
				if semanticConfig.Threshold == 0 {
					semanticConfig.Threshold = 0.85
				}
				if semanticConfig.TopK == 0 {
					semanticConfig.TopK = 5
				}
				if semanticConfig.DistanceType == "" {
					semanticConfig.DistanceType = "cosine"
				}
				if !semanticConfig.EnableAutoEmbedding {
					logger.Warn("Semantic cache: enable_auto_embedding=false, embedding vectors will not be generated")
				}

				semanticCache, scErr := cache.NewSemanticCacheWithMetrics(semanticConfig, embeddingSvc, cacheManager.GetMetrics())
				if scErr != nil {
					logger.Warnf("Failed to create semantic cache: %v", scErr)
				} else {
					// 尝试附加向量存储（可选持久化）
					var vectorStore storage.VectorStore
					if vectorStore, err = storageManager.GetVectorStore(""); err == nil {
						semanticCache.SetVectorStore(vectorStore)
						cacheManager.SetVectorStore(vectorStore)
						logger.Info("Semantic cache initialized with persistent vector store")
					} else {
						logger.Infof("No vector store available (%v) - semantic cache using in-memory storage only", err)
					}
					cacheManager.SetSemanticCache(semanticCache)
					cacheManager.SetEmbeddingService(embeddingSvc)
					cacheManager.SetSemanticConfig(semanticConfig)
					logger.Infof("Semantic cache ready - provider: %s, model: %s, auto_embedding: %v",
						cfgEmbeddingConfig.Provider, cfgEmbeddingConfig.Model, semanticConfig.EnableAutoEmbedding)
				}
			}
		} else {
			logger.Info("Embedding disabled - semantic cache not initialized")
		}
	}

	// 初始化问答拆分服务（如果配置了后端ID）
	if cfg.QASplit.BackendID != "" {
		// 从后端管理器获取后端配置
		backend, err := backendManager.Get(cfg.QASplit.BackendID)
		if err != nil {
			logger.Warnf("QA split backend %s not found (QA split disabled)", cfg.QASplit.BackendID)
		} else if !backend.Enabled {
			logger.Warnf("QA split backend %s is disabled (QA split disabled)", cfg.QASplit.BackendID)
		} else {
			// 创建 chat service 配置
			qaSplitConfig := &llm.ChatConfig{
				Provider:    backend.Type,
				Model:       cfg.QASplit.Model,
				BaseURL:     backend.BaseURL,
				APIKey:      backend.APIKey,
				Timeout:     cfg.QASplit.Timeout,
				Temperature: cfg.QASplit.Temperature,
				MaxTokens:   cfg.QASplit.MaxTokens,
				Enabled:     cfg.QASplit.Enabled,
			}

			var chatService llm.ChatService
			var err error

			// 根据后端类型创建对应的 chat service
			switch backend.Type {
			case "ollama":
				chatService, err = llm.NewOllamaChatService(qaSplitConfig)
			case "openai":
				chatService, err = llm.NewOpenAIChatService(qaSplitConfig)
			default:
				logger.Warnf("Unsupported QA split backend type: %s (QA split disabled)", backend.Type)
			}

			if err != nil {
				logger.Warnf("Failed to create QA split chat service: %v (QA split disabled)", err)
			} else if chatService != nil {
				qaSplitter := processor.NewQASplitter(&processor.QASplitterConfig{
					ChatService: chatService,
					Prompt:      cfg.QASplit.Prompt,
					Enabled:     cfg.QASplit.Enabled,
				})
				cacheManager.SetQASplitter(qaSplitter)
				logger.Info("QA splitter initialized",
					zap.String("backend_id", cfg.QASplit.BackendID),
					zap.String("model", cfg.QASplit.Model),
					zap.String("backend_type", backend.Type))
			}
		}
	}

	// 创建代理缓存
	proxyCache := cache.NewProxyCache(cacheManager, true)

	// 设置仅保存模式
	proxyCache.SetSaveOnlyMode(cfg.Cache.SaveOnlyMode)

	// v0.3.3: 接线查询展开（hit_strategies 含 expand 时生效）
	if expander, err := cacheexpansion.NewRuleBasedExpander(nil); err != nil {
		logger.Warnf("[cache] rule-based expander init failed: %v", err)
	} else {
		proxyCache.SetExpander(expander)
		logger.Info("[cache] query expander registered (hit_strategies)")
	}

	// 创建代理服务和处理器
	proxyService := proxy.New(pluginManager)
	proxyHandler := proxy.NewHandler(proxyService, pluginManager, proxyCache)

	// PR-2.1: 实例化智能调度器并接入流水线 hook
	appScheduler := buildScheduler(cfg, backendManager)
	wireSchedulerBackend(appScheduler)
	wireSchedulerMetricsFeedback(appScheduler)
	wireTransparentBackend(backendManager)
	if appScheduler != nil {
		logger.Infof("[Scheduler] Initialized and wired to pipeline (intent_recognition=%v, task_strategies=%d)",
			cfg.Scheduler.EnableIntentRecognition, len(cfg.Scheduler.TaskStrategies))
	}

	// PR-2.3: 熔断器接入 fallback 降级组
	cbManager := buildCircuitBreakerManager()
	wireCircuitBreaker(cbManager)

	// 创建流水线引擎
	nodeRegistry := pipeline.NewNodeRegistry()
	if err := pipeline.RegisterBuiltinNodes(nodeRegistry); err != nil {
		logger.Warnf("Failed to register builtin nodes: %v", err)
	} else {
		logger.Infof("Builtin nodes registered: %d plugins", len(nodeRegistry.GetPluginDescriptors()))
	}

	// 注入插件安全验证器和准入检查器
	securityValidator := pipeline.NewPluginSecurityValidator(cfg.PluginSecurity)
	admissionChecker := pipeline.NewAdmissionChecker(cfg.PluginSecurity.AdmissionCheck)
	nodeRegistry.SetSecurityValidator(securityValidator)
	nodeRegistry.SetAdmissionChecker(admissionChecker)
	logger.Infof("Plugin security validator enabled=%v, admission checker enabled=%v",
		securityValidator.IsEnabled(), admissionChecker.IsEnabled())

	// 注册业务插件（BusinessPlugin）
	// 业务插件通过 core/pkg/plugin 的 BusinessRegistry 注册
	bizRegistry := pipeline.NewBusinessPluginRegistry()
	nodeRegistry.SetBusinessRegistry(bizRegistry)
	for _, name := range pluginapi.ListBusinessPlugins() {
		fn, ok := pluginapi.GetBusinessPlugin(name)
		if !ok {
			continue
		}
		if err := fn(nodeRegistry, bizRegistry); err != nil {
			logger.Warnf("Failed to register business plugin %s: %v", name, err)
		} else {
			logger.Infof("Business plugin %s (%s) registered", name, "business."+name)
		}
	}

	// 注入业务插件注册表到 Handler（支持插件化问答拆分/合成/任务检测）
	proxyHandler.SetBusinessPluginRegistry(bizRegistry)

	// 创建 PipelineStore（如果数据库可用）
	var pipelineStore pipeline.PipelineStore
	if db := database.Get().GetDB(); db != nil {
		var psErr error
		pipelineStore, psErr = pipeline.NewDBPipelineStore()
		if psErr != nil {
			logger.Warnf("Failed to create pipeline store: %v", psErr)
		}
	}

	// 使用带 store 的 registry，启动时加载已有配置
	var pipelineRegistry *pipeline.PipelineRegistry
	storeWasEmpty := true
	if pipelineStore != nil {
		pipelineRegistry = pipeline.NewPipelineRegistryWithStore(pipelineStore)
		if err := pipelineRegistry.LoadFromStore(); err != nil {
			logger.Warnf("Failed to load pipelines from store: %v", err)
		} else {
			n := len(pipelineRegistry.List())
			logger.Infof("Loaded %d pipelines from store", n)
			storeWasEmpty = n == 0
			if !storeWasEmpty {
				updatedPipelines, updatedNodes, migErr := migrateRouterImplementationsToBusinessPlugin(pipelineRegistry, pipelineStore)
				if migErr != nil {
					logger.Warnf("Router plugin migration failed: %v", migErr)
				} else if updatedPipelines > 0 {
					logger.Infof("Router plugin migration completed: pipelines=%d nodes=%d",
						updatedPipelines, updatedNodes)
				}
			}
		}
	} else {
		pipelineRegistry = pipeline.NewPipelineRegistry()
	}

	// 解析内置流水线模板（按 CENTAG_EDITION：personal/minimal 仅 common，team 含 team/）
	templates := resolvePipelineTemplatesWithEdition(cfg.Server.Edition)
	logger.Infof("Pipeline templates loaded: %d builtin templates resolved (edition=%q)", len(templates), cfg.Server.Edition)

	// 仅当数据库中完全没有流水线时（首次启动），才从模板创建默认流水线
	// 用户主动删除后不应重新创建
	if storeWasEmpty {
		registered := 0
		for _, tmpl := range templates {
			p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
			if err := pipelineRegistry.Register(p); err != nil {
				logger.Warnf("Failed to register builtin pipeline %s: %v", p.ID, err)
			} else {
				registered++
			}
		}
		logger.Infof("Pipeline seed: registered %d/%d builtin pipelines to store", registered, len(templates))
	}

	// 为已有流水线回填 RouteConfig（修复 Template 加载阶段的 bug）
	if !storeWasEmpty {
		backfilled := backfillRouteConfigForExistingPipelines(pipelineRegistry, templates)
		if backfilled > 0 {
			logger.Infof("RouteConfig backfill: %d nodes updated across existing pipelines", backfilled)
		}
	}

	pipelineEngine := pipeline.NewPipelineEngine(
		nodeRegistry,
		pipelineRegistry,
		nil, // CapabilityBroker 将在 agentMemSvc 初始化后设置
		pipeline.NewPipelineLogger(),
		storageManager, // 传递 storage.Manager
	)
	if facade := proxyCache.Facade(); facade != nil {
		pipelineEngine.SetCacheFacade(facade)
	}
	proxyHandler.SetPipelineEngine(pipelineEngine)
	proxyHandler.SetPipelineRegistry(pipelineRegistry)
	if pipelineStore != nil {
		proxyHandler.SetPipelineStore(pipelineStore)
	}

	// 创建插件注册表存储（如果数据库可用）
	var pluginRegistryStore pipeline.PluginRegistryStore
	if db := database.Get().GetDB(); db != nil {
		if prs, err := pipeline.NewDBPluginRegistryStore(); err == nil {
			pluginRegistryStore = prs
			logger.Info("Plugin registry store initialized")

			// 将 NodeRegistry 中的内置插件注册到 pluginRegistryStore
			registerBuiltinPluginsToStore(nodeRegistry, pluginRegistryStore)
		} else {
			logger.Warnf("Failed to create plugin registry store: %v", err)
		}
	}

	// 创建流水线处理器
	pipelineHandler := NewPipelineHandler(pipelineEngine, nodeRegistry, pipelineRegistry, templates, pluginRegistryStore)
	pipelineHandler.SetAutoBuildScheduler(appScheduler)
	pipelineHandler.SetAutoBuildBackendManager(backendManager)
	pipelineHandler.StartAutoBuildRebuildLoopFromEnv()
	webhookHandler := webhook.NewHandler(pipelineEngine, "")

	// 注入流水线执行历史存储（用于记录和查询执行历史）
	if pipelineStore != nil {
		pipelineHandler.SetPipelineStore(pipelineStore)
	}

	// 创建流水线默认模式相关 handler
	pipelineDefaultsHandler := handler.NewPipelineDefaultsHandler(cfg, pipelineRegistry)

	// 创建模型变量配置 handler
	modelConfigHandler := handler.NewModelConfigHandler(cfg)

	// 初始化默认流水线解析器
	defaultPipelineResolver := proxy.NewDefaultPipelineResolver(cfg)
	proxyHandler.SetDefaultPipelineResolver(defaultPipelineResolver)
	logger.Info("DefaultPipelineResolver initialized")

	// 初始化问题拆分处理器（QuestionSplit，输入侧）
	// 与 QASplit（输出侧）独立，仅在 enabled=true 且 llm_split_enabled=true 时才初始化 LLM 服务
	if cfg.QuestionSplit.Enabled && cfg.QuestionSplit.LLMSplitEnabled && cfg.QuestionSplit.BackendID != "" {
		if qsBackend, qsErr := backendManager.Get(cfg.QuestionSplit.BackendID); qsErr != nil {
			logger.Warnf("QuestionSplit backend %s not found, LLM split disabled: %v", cfg.QuestionSplit.BackendID, qsErr)
		} else if !qsBackend.Enabled {
			logger.Warnf("QuestionSplit backend %s is disabled, LLM split disabled", cfg.QuestionSplit.BackendID)
		} else {
			llmCfg := &llm.LLMConfig{
				Provider:    qsBackend.Type,
				ModelName:   cfg.QuestionSplit.Model,
				BaseURL:     qsBackend.BaseURL,
				APIKey:      qsBackend.APIKey,
				Timeout:     30,
				Temperature: 0.1,
				MaxTokens:   512,
			}
			var qsLLMSvc processor.LLMService
			var llmErr error
			switch qsBackend.Type {
			case "ollama":
				qsLLMSvc, llmErr = llm.NewOllamaLLMService(llmCfg)
			case "openai":
				qsLLMSvc, llmErr = llm.NewOpenAILLMService(llmCfg)
			default:
				logger.Warnf("QuestionSplit unsupported backend type: %s", qsBackend.Type)
			}
			if llmErr != nil {
				logger.Warnf("QuestionSplit LLM service creation failed: %v", llmErr)
			} else if qsLLMSvc != nil {
				maxSub := cfg.QuestionSplit.MaxSubQuestions
				if maxSub <= 0 {
					maxSub = 5
				}
				processorCfg := &processor.ProcessorConfig{
					Split: processor.SplitConfig{
						Enabled:             true,
						Strategy:            processor.SplitStrategy(cfg.QuestionSplit.SplitStrategy),
						ComplexityThreshold: cfg.QuestionSplit.ComplexityThreshold,
						MaxSplitCount:       maxSub,
						MinSplitLength:      5,
						EnableAutoSplit:     true,
					},
					Synthesis: processor.SynthesisConfig{
						Strategy:      processor.SynthesisStrategy(cfg.QuestionSplit.SynthesisStrategy),
						PreserveOrder: true,
					},
				}
				if qp, qpErr := processor.NewQuestionProcessorWithLLM(processorCfg, qsLLMSvc); qpErr != nil {
					logger.Warnf("QuestionSplit processor creation failed: %v", qpErr)
				} else {
					proxyHandler.SetQuestionProcessor(qp)
					logger.Info("QuestionSplit LLM processor initialized",
						zap.String("backend_id", cfg.QuestionSplit.BackendID),
						zap.String("model", cfg.QuestionSplit.Model),
						zap.String("strategy", cfg.QuestionSplit.SplitStrategy))
				}
			}
		}
	} else if cfg.QuestionSplit.Enabled {
		logger.Info("QuestionSplit enabled with fast (rule-based) splitter only")
	}

	// 创建后端配置处理器
	backendHandler := NewBackendHandler(backendManager)

	// 创建 Agent 快速配置处理器
	agentRegistry := agentpkg.NewTemplateRegistry()
	agentHandler := NewAgentHandler(agentRegistry, backendManager)

	// 创建 Agent 供应商配置处理器
	agentProviderMgr := agentpkg.GetProviderManager()
	agentProviderHandler := agentpkg.NewAgentProviderHandler(agentProviderMgr)

	// 加载 Agent 供应商配置（含默认种子数据）
	if err := agentProviderMgr.Load(); err != nil {
		logger.Warnf("[Server] Failed to load agent provider configs: %v", err)
	}
	agentProviderMgr.SeedDefaults()

	// 创建内置 Agent 处理器
	// 数据库未配置 agent 字段时应用默认值（否则工具/权限为空导致 agent 不可用）
	defAgent := agentpkg.DefaultAgentConfig()
	builtinAgentConfig := &agentpkg.AgentConfig{
		Enabled:     cfg.Agent.Enabled,
		MaxTurns:    firstNonZero(cfg.Agent.MaxTurns, defAgent.MaxTurns),
		MaxTokens:   firstNonZero(cfg.Agent.MaxTokens, defAgent.MaxTokens),
		Timeout:     firstDuration(cfg.Agent.Timeout, defAgent.Timeout),
		ToolTimeout: firstDuration(cfg.Agent.ToolTimeout, defAgent.ToolTimeout),
		Filesystem: agentpkg.FilesystemConfig{
			AllowedDirs: firstStrings(cfg.Agent.Filesystem.AllowedDirs, defAgent.Filesystem.AllowedDirs),
			DeniedDirs:  firstStrings(cfg.Agent.Filesystem.DeniedDirs, defAgent.Filesystem.DeniedDirs),
		},
		Database: agentpkg.DatabaseConfig{
			AllowedTables:  firstStrings(cfg.Agent.Database.AllowedTables, defAgent.Database.AllowedTables),
			ReadOnlyTables: firstStrings(cfg.Agent.Database.ReadOnlyTables, defAgent.Database.ReadOnlyTables),
			DeniedTables:   firstStrings(cfg.Agent.Database.DeniedTables, defAgent.Database.DeniedTables),
		},
		Tools: agentpkg.ToolsConfig{
			Allowed:        firstStrings(cfg.Agent.Tools.Allowed, defAgent.Tools.Allowed),
			Denied:         firstStrings(cfg.Agent.Tools.Denied, defAgent.Tools.Denied),
			RequireConfirm: firstStrings(cfg.Agent.Tools.RequireConfirm, defAgent.Tools.RequireConfirm),
		},
		Skills: agentpkg.SkillsConfig{
			InternalOnly: cfg.Agent.Skills.InternalOnly,
			Enabled:      firstStrings(cfg.Agent.Skills.Enabled, defAgent.Skills.Enabled),
		},
	}
	dataDir := os.Getenv("CENTAG_DATA_DIR")
	if dataDir == "" {
		dataDir = os.Getenv("HOME") + "/.centag"
	}
	builtinAgentProvider := NewAgentDataProvider(backendManager, pipelineRegistry, cfg)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
	dbPath := resolveAgentDBPath(dataDir)

	// skill 插件注册表：加载内置 manifest + 自定义 manifest（data/agent-skills/，
	// 重启后重新载入）。centag-ops-router 仅由 manifest 生成（单一数据源，
	// 见 skill_pipeline.go），启动时幂等重建，与 skill CRUD 重建结果一致；
	// 静态 initdata 模板 agent-skill-router.yaml 已移除，不再作为路由真源。
	skillPluginRegistry := loadSkillPluginRegistry(dataDir)
	registeredSkillRouter := registerSkillRouterWithAdmission(skillPluginRegistry, pipelineRegistry, cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel, admissionChecker)
	if registeredSkillRouter != "" {
		logger.Infof("Skill router pipeline generated from manifests: %s", registeredSkillRouter)
	}

	builtinAgentHandler := NewBuiltinAgentHandler(builtinAgentConfig, dataDir, database.Get().GetDB(), database.Get().DriverName(), builtinAgentProvider, baseURL, dbPath, skillPluginRegistry, pipelineRegistry, cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel)
	builtinAgentHandler.SetAdmissionChecker(admissionChecker) // P1-9：CRUD 与启动共用准入

	// 创建 MCP 代理处理器
	mcpProxyHandler := NewMCPProxyHandler()

	// 创建存储配置处理器
	storageHandler := NewStorageHandler(storageManager)
	dataStoreHandler := NewDataStoreHandler(storageManager)

	// 创建统一配置处理器
	configHandler := NewConfigHandler(storageManager, cacheManager, backendManager)

	// 创建缓存处理器
	cacheHandler := NewCacheHandler(cacheManager, proxyCache, backendManager)
	cacheHandler.SetQASplitConfig(&cfg.QASplit)

	// 创建监控处理器
	metricsHandler := NewMetricsHandler(pluginManager, cacheManager, backendManager, storageManager)

	// 创建日志处理器（路径与 logger.Init 写入的 lumberjack 文件一致）
	logHandler := NewLogHandler(cfg)

	// 创建认证/用户/API Key 处理器
	authHandler := NewAuthHandler()

	// Tenant handler + tenant QuotaMiddleware are registered by centag-pro (E2.3).
	// v2.1: User-level quota middleware (open-core)
	userQuotaMiddleware := middleware.NewUserQuotaMiddleware(database.Get())

	// Profile/self-service only; team admin user CRUD is registered by centag-pro (E2.1).
	userHandler := NewUserHandler()
	apiKeyHandler := NewAPIKeyHandler()

	// 钩子管理器（增值能力统一入口：计量 / 计费 / 对话等）
	hookManager := hooks.NewManager()
	hooks.SetDefault(hookManager)
	logger.Infof("[hooks] HookManager registered (fail-open)")

	// v0.3.3: 缓存命中走统一 Facade，并触发 StorageHook.OnCacheHit
	proxyCache.SetHitNotifier(func(ctx context.Context, key string, data []byte) {
		_ = hookManager.TriggerCacheHitHooks(ctx, key, data)
	})
	if facade := proxyCache.Facade(); facade != nil {
		if cfg.Cache.Backend == config.CacheBackendExternal || cfg.Cache.External.Plugin != "" {
			facade.SetExternalBackend(&cache.UnconfiguredRecallBackend{PluginID: cfg.Cache.External.Plugin})
		}
		if err := facade.EnsureBackendReady(); err != nil {
			logger.Warnf("[cache] facade backend not ready: %v", err)
		} else {
			logger.Infof("[cache] facade ready backend=%s", facade.EffectiveBackend())
		}
	}

	// 创建 Token 计量处理器 + 定价子系统
	var tokenUsageHandler *TokenUsageHandler
	var costHandler *CostHandler
	var billingRulesHandler *BillingRulesHandler
	var personalBillingHandler *PersonalBillingHandler
	var pricingService billing.PricingService
	var abEvalAdmin abevalapi.AdminService
	if db := database.Get().GetDB(); db != nil {
		tokenUsageService := tokenusage.NewService(db, database.Get().DriverName())
		tokenUsageHandler = NewTokenUsageHandler(tokenUsageService)
		costHandler = NewCostHandler(tokenUsageService)
		abEvalSvc := abeval.NewService(db, database.Get().DriverName())
		abEvalAdmin = abevalapi.Wrap(abEvalSvc)
		wireABEvalPersistence(abEvalSvc)
		tokenusage.SetDefaultService(tokenUsageService)
		hookManager.RegisterTokenHook(newTokenUsageHookAdapter(tokenUsageService))
		wireTokenUsagePersistence(tokenUsageService, hookManager)
		proxyHandler.SetTokenUsageService(tokenUsageService)
		// 用户默认流水线写在 users.default_pipeline_id；此前从未注入，导致对话始终走 system-default。
		userQuotaSvc := tokenusage.NewUserQuotaService(db, database.Get().DriverName())
		defaultPipelineResolver.SetUserQuotaService(userQuotaSvc)
		logger.Infof("[hooks] TokenHook adapter registered; user default pipeline resolver wired")

		ruleStore := billing.NewSQLRuleStore(db, database.Get().DriverName())
		if path, err := billing.ResolveDefaultPricingPath(); err != nil {
			logger.Warnf("[billing] default pricing YAML not found: %v", err)
		} else if err := billing.EnsureSeededFromYAML(context.Background(), ruleStore, path); err != nil {
			logger.Warnf("[billing] seed pricing rules failed: %v", err)
		} else {
			logger.Infof("[billing] pricing rules ready (seeded from %s if empty)", path)
		}
		pricingService = billing.NewPricingService(ruleStore)
		tokenusage.SetPricingService(pricingService)
		billingRulesHandler = NewBillingRulesHandler(ruleStore, pricingService)
		personalBillingHandler = NewPersonalBillingHandler(ruleStore)
	}

	ed := edition.Parse(cfg.Server.Edition)
	// BillingHook is wired by centag-pro/internal/teamadmin; open-core does not register it.

	var conversationStore conversation.Store
	var conversationHandler *ConversationHandler
	{
		opts := conversation.Options{Edition: ed, FileRoot: filepath.Join("var", "conversations")}
		if db := database.Get().GetDB(); db != nil {
			opts.DB = db
			opts.Driver = database.Get().DriverName()
		}
		if ed.IsTeam() && opts.Driver != "" && !isPostgresDriverName(opts.Driver) {
			logger.Warnf("[edition] CENTAG_EDITION=team with DB driver %q — personal profile should use CENTAG_EDITION=personal (SQLite conversations without team billing)", opts.Driver)
		}
		if store, err := conversation.NewStore(opts); err != nil {
			logger.Warnf("[conversation] store init failed: %v", err)
		} else {
			conversationStore = store
			conversation.SetDefault(store)
			hookManager.RegisterStorageHook(conversation.NewLoggingHook(store))
			conversationHandler = NewConversationHandler(store, ed)
			logger.Infof("[hooks] Conversation LoggingHook registered (edition=%s store=%s)", ed, conversationStoreKind(ed, opts.Driver))
		}
	}

	// 创建记忆服务处理器
	// 记忆存储目录：从环境变量或配置获取
	memoryStoreRoot := os.Getenv("MEMORY_STORE_ROOT")
	if memoryStoreRoot == "" {
		memoryStoreRoot = "./memory-store"
	}
	// 获取向量存储
	var memoryVectorStore storage.VectorStore
	if storageManager != nil {
		memoryVectorStore, _ = storageManager.GetVectorStore("")
	}
	var agentMemSvc *agentmemory.Service
	if db := database.Get().GetDB(); db != nil {
		agentMemSvc = agentmemory.NewService(db, database.Get().DriverName(), embeddingSvc)
	}
	// rag_retrieval：通过 blank import plugins/business/rag_retrieval 注册（见 dist/*/main.go）
	// agentMemSvc 可供后续真实知识库 S3 后端接线使用
	_ = agentMemSvc
	memoryHandler := NewMemoryHandler(memoryVectorStore, embeddingSvc, memoryStoreRoot, agentMemSvc)

	// 创建 CapabilityBroker（延迟注入，因为 agentMemSvc 刚初始化）
	// 创建 StorageProvider 适配器
	storageAdapter := pipeline.NewStorageManagerAdapter(storageManager)

	// 创建 MemoryProvider 适配器（如果 agentMemSvc 可用）
	var memoryAdapter *pipeline.MemoryServiceAdapter
	if agentMemSvc != nil {
		memoryAdapter = pipeline.NewMemoryServiceAdapter(agentMemSvc)
	}

	// 创建 SecretsProvider（从环境变量读取）
	secretsProvider := pipeline.NewEnvSecretsProvider("LLM_PROXY_")

	// 创建 HTTPConfig（使用默认值，后续可从配置读取）
	httpConfig := pipeline.HTTPConfig{
		Timeout: cfg.Proxy.Timeout,
	}

	capabilityBroker := pipeline.NewCapabilityBroker(
		storageAdapter,
		memoryAdapter,
		secretsProvider,
		httpConfig,
	)

	// 创建并注入 LLM 提供者（用于通过后端管理器调用 LLM）
	if backendManager != nil {
		llmProvider := pipeline.NewDefaultLLMProvider(backendManager, pluginManager)
		capabilityBroker.SetLLMProvider(llmProvider)
		logger.Info("LLM provider injected into CapabilityBroker")
	}
	// 注入默认 LLM 目标：节点权限仅声明纯 "llm.call" 时回退系统默认后端，
	// 避免注册期快照为空（如首次部署先启动后配默认后端）导致 generator 500。
	capabilityBroker.SetDefaultLLMTarget(cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel)

	// ── Phase 4A: 注册缓存策略并注入 CapabilityBroker ─────────────────────────────
	// 注册内置缓存策略（exact/semantic/hybrid）到全局注册表
	strategyReg := cachestrategy.GetRegistry()
	var kvStore storage.KVStore
	var vectorStore storage.VectorStore
	if storageManager != nil {
		kvStore, _ = storageManager.GetDefaultKVStore()
		vectorStore, _ = storageManager.GetVectorStore("")
	}
	if err := strategyReg.RegisterBuiltinStrategies(kvStore, vectorStore, embeddingSvc); err != nil {
		logger.Warnf("Failed to register builtin cache strategies: %v (cache pipeline nodes will use fallback)", err)
	} else {
		logger.Infof("Cache strategies registered: %v", strategyReg.ListAll())
	}

	// 创建策略 Provider 并注入 broker（桥接 strategy.Strategy → CacheStrategyCapability）
	cacheProvider := pipeline.NewCacheStrategyProvider(strategyReg)
	capabilityBroker.SetCacheStrategyProvider(cacheProvider)

	// 设置 CapabilityBroker 到 pipelineEngine
	if pipelineEngine != nil {
		pipelineEngine.SetCapabilityBroker(capabilityBroker)
		logger.Info("CapabilityBroker injected into PipelineEngine")
	}

	// PC Agent 作为独立客户端，无需注入内部依赖
	wireReviewContent(capabilityBroker)

	if agentMemSvc != nil {
		logger.Infof("Agent memory DB persistence enabled (driver=%s); file store remains fallback when handler uses direct-only paths.", database.Get().DriverName())
	}
	if absMem, err := filepath.Abs(memoryStoreRoot); err == nil {
		logger.Infof("Agent memory file storage (MEMORY_STORE_ROOT): %s", absMem)
	} else {
		logger.Infof("Agent memory file storage (MEMORY_STORE_ROOT): %s", memoryStoreRoot)
	}

	// 初始化 JWT secret（首次运行时自动生成并持久化）
	if err := auth.LoadSecret(context.Background()); err != nil {
		logger.Warnf("Failed to load JWT secret: %v – auth endpoints may not work", err)
	}
	// API Key 二次查看存储密钥（entrypoint 已 Ensure；此处兜底，便于测试/嵌入启动）
	if err := auth.EnsureAPIKeyStorage(context.Background()); err != nil {
		logger.Warnf("Failed to ensure API key storage secret: %v – secondary reveal may not work", err)
	}
	// 密钥解密健康自检：统计当前 STORAGE_SECRET 下无法解密的历史密钥（多为部署轮换密钥所致）
	if checked, bad := auth.AuditUndecryptableKeys(context.Background()); bad > 0 {
		logger.Warnf("API key storage audit: %d/%d encrypted key(s) cannot be decrypted with the current %s – secondary reveal and Agent proxy auto-resolve are unavailable for these keys. Restore the original secret or recreate the affected keys.",
			bad, checked, "LLM_PROXY_API_KEY_STORAGE_SECRET")
	}

	// 创建系统更新处理器
	updateConfigPath := "./update_config.yml"
	if _, err := os.Stat(updateConfigPath); os.IsNotExist(err) {
		updateConfigPath = "../update_config.yml"
	}
	systemUpdate := internal.NewSystemUpdateHandler(updateConfigPath)
	systemUpdate.SetEdition(cfg.Server.Edition)

	// 创建代理模式管理器
	modeMgr := proxymode.NewManager()
	logger.Infof("Proxy mode manager initialized with %d default modes", len(modeMgr.ListModes()))

	// 将策略管理中的流水线快捷码同步到 ModeManager（与注册表同源）
	if synced := modeMgr.SyncFromPipelines(pipelineRegistry.ListAll()); synced > 0 {
		logger.Infof("Synced %d pipeline shortcuts into ModeManager (total modes: %d)", synced, len(modeMgr.ListModes()))
	}
	pipelineHandler.SetModeManager(modeMgr)

	// 创建会话存储
	sessionStore := session.NewProxyModeStore()

	// 创建代理模式处理器
	proxyModeHandler := NewProxyModeHandler(modeMgr, sessionStore)

	// 创建策略处理器
	strategyHandler := handler.NewStrategyHandler()

	// 创建评估管理器和处理器
	evaluationManager := evalmanager.NewManager()
	// 注册默认评估插件
	_ = evaluationManager.Register(evaluationplugins.NewFollowUpDetectorPlugin())
	_ = evaluationManager.Register(evaluationplugins.NewLengthEvaluatorPlugin())
	_ = evaluationManager.Register(evaluationplugins.NewWeightedAggregatorPlugin())
	// 获取缓存配置中的精确匹配设置
	exactMatchEnabled := cfg.Cache.Enabled // 默认使用缓存启用状态
	evaluationHandler := NewEvaluationHandler(evaluationManager, exactMatchEnabled)

	// 将评估管理器传递给缓存管理器（使用缓存内部适配器）
	cacheManager.SetEvaluationManager(cache.NewEvaluationManagerWrapper(evaluationManager))

	// 创建 Clash 订阅处理器
	clashHandler := NewClashHandler()

	// 注册自定义策略解析函数，让 proxy 层能查找自定义策略
	proxy.SetCustomStrategyResolver(func(id string) *proxy.CustomStrategyWeights {
		custom, ok := strategy.GetStore().Get(id)
		if !ok {
			return nil
		}
		return &proxy.CustomStrategyWeights{
			NameSimilarity: custom.Weights.NameSimilarity,
			CapacityMatch:  custom.Weights.CapacityMatch,
			FamilyMatch:    custom.Weights.FamilyMatch,
			Strictness:     custom.Strictness,
			Tolerance:      custom.Tolerance,
		}
	})

	// 创建MITM代理服务器和代理处理器
	var mitmServer *mitm.Server
	var proxyHandlerExt *handler.ProxyHandler

	// Normalize/validate system proxy before PAC/MITM wiring
	if err := config.ValidateSystemProxyConfig(&cfg.SystemProxy); err != nil {
		logger.Warnf("system_proxy config invalid, falling back to local defaults: %v", err)
		cfg.SystemProxy.AllowLANClients = false
		config.NormalizeSystemProxyConfig(&cfg.SystemProxy)
	}

	// 始终创建PAC配置和ProxyHandler,以便在未启用时也能管理域名
	pacConfig := buildSystemProxyPACConfig(cfg)

	// 创建扩展代理处理器(即使未启用也要创建,用于管理PAC域名)
	proxyHandlerExt = handler.NewProxyHandler(nil, pacConfig, cfg)

	if cfg.SystemProxy.Enabled {
		logger.Info("Initializing system proxy...")

		// Auto-bind egress API key at startup (no env restart required for daily ops)
		if config.ResolveSystemProxyEgressAPIKey(&cfg.SystemProxy) == "" {
			if changed, err := EnsureSystemProxyEgressAPIKey(context.Background(), cfg, 0); err != nil {
				logger.Warnf("system_proxy: ensure egress API key at startup: %v", err)
			} else if changed {
				if err := config.SaveConfig(cfg); err != nil {
					logger.Warnf("system_proxy: save egress API key at startup: %v", err)
				} else {
					logger.Info("system_proxy: egress API key auto-bound at startup")
				}
			}
		}

		// 创建MITM服务器
		// 注意: BackendAddr不能用0.0.0.0,需要改为127.0.0.1
		backendHost := cfg.Server.Host
		if backendHost == "0.0.0.0" || backendHost == "" {
			backendHost = "127.0.0.1"
		}

		mitmConfig := buildMITMConfig(cfg, backendHost)

		var err error
		mitmServer, err = mitm.NewServer(mitmConfig)
		if err != nil {
			logger.Errorf("Failed to create MITM server: %v", err)
		} else {
			// 更新ProxyHandler的mitmServer引用
			proxyHandlerExt = handler.NewProxyHandler(mitmServer, pacConfig, cfg)

			logger.Info("System proxy initialized successfully",
				zap.Int("mitm_port", cfg.SystemProxy.ListenPort),
				zap.Int("domains", len(cfg.SystemProxy.Domains)),
				zap.Bool("egress_api_key_configured", mitmConfig.BackendAuthToken != ""))
			if mitmConfig.BackendAuthToken == "" {
				logger.Warn("System proxy MITM has no egress API key; open Web → 本机代理出口 → 一键绑定出口 Key（热更新，无需停服）")
			}
		}
	}

	// 初始化Host代理 (总是创建Handler以支持API管理,但根据配置决定是否启动Server)
	var hostProxyServer *hostproxy.Server
	var hostProxyHandler *hostproxy.Handler

	hostProxyConfig := &hostproxy.Config{
		HTTPPort:      cfg.HostProxy.HTTPPort,
		HTTPSPort:     cfg.HostProxy.HTTPSPort,
		BackendAddr:   cfg.HostProxy.BackendAddr,
		CACertPath:    cfg.HostProxy.CACertPath,
		CAKeyPath:     cfg.HostProxy.CAKeyPath,
		CertDir:       cfg.HostProxy.CertDir,
		CertValidDays: cfg.HostProxy.CertValidDays,
		DomainMapping: cfg.HostProxy.DomainMapping,
		PathPatterns:  cfg.HostProxy.PathPatterns,
	}

	hostProxyServer, err = hostproxy.NewServer(hostProxyConfig)
	if err != nil {
		logger.Errorf("Failed to create Host proxy server: %v", err)
	} else {
		// 根据配置设置初始启用状态
		hostProxyServer.SetEnabled(cfg.HostProxy.Enabled)
		hostProxyHandler = hostproxy.NewHandler(hostProxyServer)
		logger.Info("Host proxy initialized",
			zap.Int("http_port", cfg.HostProxy.HTTPPort),
			zap.Int("https_port", cfg.HostProxy.HTTPSPort),
			zap.Int("domains", len(cfg.HostProxy.DomainMapping)),
			zap.Bool("enabled", cfg.HostProxy.Enabled))
	}

	var pluginMarketStore pluginregistry.Store = pluginregistry.NewMemoryStore()
	if db := database.Get().GetDB(); db != nil {
		pluginMarketStore = pluginregistry.NewDBStore(db)
		logger.Info("Plugin marketplace registry API initialized with database store")
	} else {
		logger.Info("Plugin marketplace registry API initialized with in-memory store")
	}
	pluginRegistryAPI := pluginregistry.NewHandler(pluginMarketStore)

	// 创建服务器实例
	srv := &Server{
		router:                 router,
		cfg:                    cfg,
		pluginManager:          pluginManager,
		backendManager:         backendManager,
		storageManager:         storageManager,
		cacheManager:           cacheManager,
		proxyCache:             proxyCache,
		proxyHandler:           proxyHandler,
		backendHandler:         backendHandler,
		storageHandler:         storageHandler,
		dataStoreHandler:       dataStoreHandler,
		cacheHandler:           cacheHandler,
		metricsHandler:         metricsHandler,
		configHandler:          configHandler,
		authHandler:            authHandler,
		userHandler:            userHandler,
		apiKeyHandler:          apiKeyHandler,
		tokenUsageHandler:      tokenUsageHandler,
		costHandler:            costHandler,
		billingRulesHandler:    billingRulesHandler,
		personalBillingHandler: personalBillingHandler,
		pricingService:         pricingService,
		memoryHandler:          memoryHandler,
		strategyHandler:        strategyHandler,
		proxyHandlerExt:        proxyHandlerExt,
		clashHandler:           clashHandler,
		evaluationHandler:      evaluationHandler,
		evaluationManager:      evaluationManager,
		logHandler:             logHandler,
		mitmServer:             mitmServer,
		hostProxyServer:        hostProxyServer,
		hostProxyHandler:       hostProxyHandler,
		systemUpdate:           systemUpdate,
		modeManager:            modeMgr,
		proxyModeHandler:       proxyModeHandler,
		pipelineHandler:        pipelineHandler,
		webhookHandler:         webhookHandler,
		pluginRegistryAPI:      pluginRegistryAPI,
		sessionStore:           sessionStore,
		userQuotaMiddleware:    userQuotaMiddleware, // v2.1: User-level quota
		edition:                edition.Parse(cfg.Server.Edition),
		agentHandler:           agentHandler,
		agentProviderHandler:   agentProviderHandler,
		builtinAgentHandler:    builtinAgentHandler,
		mcpProxyHandler:        mcpProxyHandler,
		hookManager:            hookManager,
		conversationStore:      conversationStore,
		conversationHandler:    conversationHandler,
		// 流水线默认模式相关 handler
		pipelineDefaultsHandler: pipelineDefaultsHandler,
		// 模型变量配置 handler
		modelConfigHandler:      modelConfigHandler,
		startTime:               time.Now(),
	}
	backendHandler.SetEdition(srv.edition)
	pipelineHandler.SetEdition(srv.edition)

	// 系统默认后端配置变更联动：刷新 broker 默认 LLM 目标 + 幂等重建 skill 路由管线。
	// 修复：centag-ops-router 注册时快照 default_backend/model，此后修改默认后端
	// 不生效（节点仍指向旧值），首启未配置时更是空快照导致 generator 500。
	srv.onProxyConfigChanged = func(defaultBackendID, defaultModel string) {
		capabilityBroker.SetDefaultLLMTarget(defaultBackendID, defaultModel)
		if id := registerSkillRouterWithAdmission(skillPluginRegistry, pipelineRegistry, defaultBackendID, defaultModel, admissionChecker); id != "" {
			logger.Infof("Skill router pipeline rebuilt after proxy config change: %s (default=%s/%s)", id, defaultBackendID, defaultModel)
		}
	}

	switch {
	case srv.edition.IsPersonal():
		logger.Infof("Product edition: personal (single-user; multi-tenant admin APIs disabled; no BillingHook)")
	case srv.edition.IsMinimal():
		logger.Infof("Product edition: minimal")
	default:
		logger.Infof("Product edition: team")
	}

	// 将运行时组件注册到 configHandler，使配置保存时能热更新
	configHandler.SetHostProxyServer(hostProxyServer)
	configHandler.SetProxyCache(proxyCache)
	configHandler.SetMitmToggle(srv.toggleMITM)
	configHandler.SetMitmForceRestart(srv.forceRestartMITM)
	configHandler.SetMitmSyncEgress(srv.syncMITMEgressAuth)
	configHandler.SetMitmSyncClientProxyAuth(srv.syncMITMClientProxyAuth)
	configHandler.SetProxyHandlerRefresh(srv.refreshProxyHandlerPAC)

	// Commercial plugins (centag-pro) blank-import Register themselves.
	// Init runs after Server deps are ready (R13), before setupRoutes so
	// editionmodule mounts and queued Host registrars see a complete Host.
	srv.extensionHost = extension.NewRuntimeHost(srv.edition.String(), extension.Deps{
		HookManager:       hookManager,
		BackendManager:    backendManager,
		PipelineRegistry:  pipelineRegistry,
		Database:          database.Get(),
		TokenUsageService: tokenusageapi.Default(),
		SystemUpdate:      systemupdateapi.Wrap(systemUpdate),
		ABEvalHandler:     abEvalAdmin,
		ModeManager:       modeMgr,
	})
	if err := extension.InitAll(srv.extensionHost); err != nil {
		logger.Warnf("extension plugin init failed: %v", err)
	}
	srv.extensionHost.FlushBillingHooks(hookManager)

	// 注册路由
	srv.setupRoutes()

	return srv
}

func migrateRouterImplementationsToBusinessPlugin(registry *pipeline.PipelineRegistry, store pipeline.PipelineStore) (int, int, error) {
	if registry == nil || store == nil {
		return 0, 0, nil
	}
	if !routerPluginMigrationEnabled() {
		return 0, 0, nil
	}

	pipelinesUpdated := 0
	nodesUpdated := 0
	for _, p := range registry.ListAll() {
		if p == nil {
			continue
		}
		changed, nodeCount := migrateRouterImplementationInPipeline(p)
		if !changed {
			continue
		}
		if err := store.Update(p); err != nil {
			return pipelinesUpdated, nodesUpdated, fmt.Errorf("update pipeline %s for router migration: %w", p.ID, err)
		}
		pipelinesUpdated++
		nodesUpdated += nodeCount
	}

	return pipelinesUpdated, nodesUpdated, nil
}

func migrateRouterImplementationInPipeline(p *pipeline.AgentPatternPipeline) (bool, int) {
	if p == nil {
		return false, 0
	}

	changed := false
	updatedNodes := 0
	builtinRouterImpl := pipeline.BuiltinImplementationForType(pipeline.NodeTypeRouter)

	for i := range p.Nodes {
		node := &p.Nodes[i]
		if node.Type != pipeline.NodeTypeRouter {
			continue
		}
		impl := strings.TrimSpace(node.Implementation)
		if impl == "" || strings.EqualFold(impl, builtinRouterImpl) {
			node.Implementation = "business.router"
			changed = true
			updatedNodes++
		}
	}

	return changed, updatedNodes
}

func routerPluginMigrationEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("LLM_PROXY_ROUTER_PLUGIN_MIGRATION_ENABLED"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "off", "no", "disabled":
		return false
	default:
		return true
	}
}

// registerPlugins 从插件注册表获取已注册的插件并初始化。
// 插件通过 dist/*/main.go 中的 _ import 触发 init() 注册。
func registerPlugins(manager *plugin.Manager) error {
	// 从注册表获取并注册所有已注册的协议插件
	for _, name := range pluginapi.ListProtocols() {
		factory, ok := pluginapi.GetProtocol(name)
		if !ok {
			continue
		}
		p, err := factory(nil)
		if err != nil {
			return fmt.Errorf("failed to create protocol plugin %q: %w", name, err)
		}
		pluginImpl, ok := p.(plugin.Plugin)
		if !ok {
			return fmt.Errorf("protocol plugin %q does not implement plugin.Plugin", name)
		}
		if err := manager.Register(pluginImpl); err != nil {
			return fmt.Errorf("failed to register protocol plugin %q: %w", name, err)
		}
	}

	// 从注册表获取并注册所有已注册的后端插件
	for _, name := range pluginapi.ListBackends() {
		factory, ok := pluginapi.GetBackend(name)
		if !ok {
			continue
		}
		p, err := factory(nil)
		if err != nil {
			return fmt.Errorf("failed to create backend plugin %q: %w", name, err)
		}
		pluginImpl, ok := p.(plugin.Plugin)
		if !ok {
			return fmt.Errorf("backend plugin %q does not implement plugin.Plugin", name)
		}
		if err := manager.Register(pluginImpl); err != nil {
			return fmt.Errorf("failed to register backend plugin %q: %w", name, err)
		}
	}

	// 初始化插件
	if err := manager.Init(); err != nil {
		return fmt.Errorf("failed to init plugins: %w", err)
	}

	// 启动插件
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start plugins: %w", err)
	}

	return nil
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// STATIC_PATH 环境变量允许显式指定静态文件目录（桌面应用等场景），
	// 否则使用 CWD 相对路径的回退链。
	staticDir := os.Getenv("STATIC_PATH")
	if staticDir == "" {
		staticDir = "./static"
	}
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		if _, err2 := os.Stat("./bin/server/static"); err2 == nil {
			staticDir = "./bin/server/static"
		} else if _, err2 := os.Stat("./bin/static"); err2 == nil {
			staticDir = "./bin/static"
		} else if home, err2 := os.UserHomeDir(); err2 == nil {
			for _, candidate := range []string{
				filepath.Join(home, ".centag", "lib", "personal", "static"),
				filepath.Join(home, ".centag", "lib", "minimal", "static"),
			} {
				if _, err3 := os.Stat(candidate); err3 == nil {
					staticDir = candidate
					break
				}
			}
		}
	}

	// 从/static/路径提供静态文件服务
	// Vite构建的base是/static/，所以所有资源都是/static/assets/开头
	s.router.Static("/static", staticDir)

	// index.html 从根路径也提供（兼容旧链接）；注入 data-edition 供 WebUI 首屏识别
	s.router.GET("/", func(c *gin.Context) {
		s.serveSPAIndex(c, staticDir)
	})

	// SPA路由支持 - 将所有非API和非静态文件的请求重定向到index.html
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 排除API路径
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") {
			c.Next()
			return
		}

		// 对于 /static/ 开头的路径，检查静态文件是否存在
		if strings.HasPrefix(path, "/static/") {
			// 尝试检查静态文件是否存在
			staticFile := path[len("/static/"):] // 去掉 /static/ 前缀
			if staticFile == "" {
				s.serveSPAIndex(c, staticDir)
				return
			}

			// 检查文件是否存在
			if _, err := os.Stat(staticDir + "/" + staticFile); err == nil {
				// 静态文件存在，让 Gin 提供
				c.Next()
				return
			}

			// 静态文件不存在，返回 SPA 的 index.html
			s.serveSPAIndex(c, staticDir)
			return
		}

		// Minimal 版：处理 /config/ 路径（config-generator UI）
		if s.edition.IsMinimal() && strings.HasPrefix(path, "/config/") {
			configStaticDir := "./static"
			if _, err := os.Stat(configStaticDir); os.IsNotExist(err) {
				configStaticDir = "./config-generator"
			}
			if _, err := os.Stat(configStaticDir); err == nil {
				// 去掉 /config/ 前缀，获取实际文件路径
				filePath := path[len("/config/"):]
				if filePath == "" {
					filePath = "index.html"
				}

				// 检查文件是否存在
				if _, err := os.Stat(configStaticDir + "/" + filePath); err == nil {
					c.File(configStaticDir + "/" + filePath)
					return
				}

				// 文件不存在，返回 index.html（SPA 路由）
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
				c.File(configStaticDir + "/index.html")
				return
			}
		}

		// 尝试为请求路径提供静态文件（支持 config-generator 等裸路径资源）
		filePath := strings.TrimPrefix(path, "/")
		if filePath != "" {
			fullPath := staticDir + "/" + filePath
			if _, err := os.Stat(fullPath); err == nil {
				c.File(fullPath)
				return
			}
		}

		// 其他路径返回 SPA 的 index.html
		s.serveSPAIndex(c, staticDir)
	})

	// 健康检查
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/health/ready", s.healthReady)
	s.router.GET("/ping", s.ping)

	// ── 认证路由（公开，无需 JWT）──────────────────────────────────────────────
	apiAuth := s.router.Group("/api/auth")
	{
		apiAuth.POST("/login", s.authHandler.Login)
		apiAuth.POST("/refresh", s.authHandler.Refresh)
		apiAuth.POST("/logout", s.authHandler.Logout)
		apiAuth.GET("/me", auth.JWTMiddleware(), s.authHandler.Me)
	}

	// ── 初始化路由（公开，无需 JWT）────────────────────────────────────────────
	v1Auth := s.router.Group("/api/v1/auth")
	{
		v1Auth.GET("/bootstrap-status", s.authHandler.BootstrapStatus)
		v1Auth.POST("/setup", s.authHandler.Setup)
	}

	// ── 普通用户路由（需要 JWT）────────────────────────────────────────────────
	userAPI := s.router.Group("/api/v1/user", auth.JWTMiddleware())
	{
		userAPI.GET("/profile", s.userHandler.GetProfile)
		userAPI.PUT("/profile", s.userHandler.UpdateProfile)
		userAPI.PUT("/password", s.userHandler.ChangePassword)
		// API key management
		userAPI.GET("/apikeys", s.apiKeyHandler.ListAPIKeys)
		userAPI.GET("/apikeys/:id", s.apiKeyHandler.GetAPIKey)
		userAPI.POST("/apikeys", s.apiKeyHandler.CreateAPIKey)
		userAPI.PUT("/apikeys/:id", s.apiKeyHandler.UpdateAPIKey)
		userAPI.DELETE("/apikeys/:id", s.apiKeyHandler.DeleteAPIKey)
		// Clash 默认规则内容（编辑器预填用）
		userAPI.GET("/clash/default-rule", s.clashHandler.GetDefaultRule)

		// Clash 订阅规则管理（多规则 CRUD）
		clashRules := userAPI.Group("/clash/rules")
		{
			clashRules.GET("", s.clashHandler.ListRules)
			clashRules.POST("", s.clashHandler.CreateRule)
			clashRules.GET("/:id", s.clashHandler.GetRule)
			clashRules.PUT("/:id", s.clashHandler.UpdateRule)
			clashRules.DELETE("/:id", s.clashHandler.DeleteRule)
			clashRules.POST("/:id/reset", s.clashHandler.ResetRuleContent)
			clashRules.POST("/:id/token", s.clashHandler.RegenerateToken)
		}

		// Token 使用统计（用户自己的使用情况）
		if s.tokenUsageHandler != nil {
			userAPI.GET("/token-usage", s.tokenUsageHandler.GetUserUsage)
			userAPI.GET("/token-usage/daily", s.tokenUsageHandler.GetDailyUsage)
			userAPI.GET("/token-usage/models", s.tokenUsageHandler.GetModelStats)
			userAPI.GET("/token-usage/backends", s.tokenUsageHandler.GetBackendStats)
			userAPI.GET("/usage", s.tokenUsageHandler.GetUsageBreakdown)
			userAPI.GET("/usage/sessions", s.tokenUsageHandler.GetSessionsUsage)
			userAPI.GET("/usage/self-limit", s.tokenUsageHandler.GetSelfLimit)
		}

		// Personal 计费配置只读 API
		if s.personalBillingHandler != nil {
			s.personalBillingHandler.RegisterPersonalBillingRoutes(userAPI)
		}

		// 对话记录浏览
		if s.conversationHandler != nil {
			convs := userAPI.Group("/conversations")
			{
				convs.GET("/sessions", s.conversationHandler.ListSessions)
				convs.GET("/sessions/:id", s.conversationHandler.GetSession)
				convs.GET("/sessions/:id/messages", s.conversationHandler.ListMessages)
				convs.GET("/categories", s.conversationHandler.ListCategories)
				convs.DELETE("/sessions/:id", s.conversationHandler.DeleteSession)
				convs.POST("/sessions/delete", s.conversationHandler.DeleteSessions)
				convs.POST("/sessions/:id/messages/delete", s.conversationHandler.DeleteMessages)
			}
		}

		// /user/tenant* registered by centag-pro via extension.Host (E2.3).
		if s.extensionHost != nil {
			s.extensionHost.ApplyUserAPI(userAPI)
		}
	}

	// 管理面对话浏览（与 user 路径一致，便于非 JWT user 组的代理鉴权客户端）
	if s.conversationHandler != nil {
		// also under /api/v1/conversations with JWT
		convsJWT := s.router.Group("/api/v1/conversations", auth.JWTMiddleware())
		{
			convsJWT.GET("/sessions", s.conversationHandler.ListSessions)
			convsJWT.GET("/sessions/:id", s.conversationHandler.GetSession)
			convsJWT.GET("/sessions/:id/messages", s.conversationHandler.ListMessages)
			convsJWT.GET("/categories", s.conversationHandler.ListCategories)
			convsJWT.DELETE("/sessions/:id", s.conversationHandler.DeleteSession)
			convsJWT.POST("/sessions/delete", s.conversationHandler.DeleteSessions)
			convsJWT.POST("/sessions/:id/messages/delete", s.conversationHandler.DeleteMessages)
		}
	}

	// Clash 订阅下载（无需鉴权，通过令牌区分用户和规则）
	s.router.GET("/clash/subscribe/:token", s.clashHandler.ServeSubscription)

	// ── 管理员路由（需要 JWT + admin 角色）────────────────────────────────────
	adminAPI := s.router.Group("/api/v1/admin", auth.JWTMiddleware(), auth.AdminOnlyMiddleware())
	teamAdmin := adminAPI.Group("", s.teamEditionOnly())
	{
		// Admin /users* and /api-keys* are registered by centag-pro via extension.Host (E2.1/E2.2).

		// Token 使用统计（管理员查看所有用户）
		// Admin token-usage/all|ranking and /quotas* registered by centag-pro (E2.4).
		// cost/summary / billing/rules 挂在 adminAPI（非 team-only）：
		// personal 也要看成本与定价规则；BillingHook 扣费仅 team+pro。
		if s.costHandler != nil {
			adminAPI.GET("/cost/summary", s.costHandler.GetSummary)
		}
		if s.billingRulesHandler != nil {
			billingRules := adminAPI.Group("/billing/rules")
			{
				billingRules.GET("", s.billingRulesHandler.ListRules)
				billingRules.POST("", s.billingRulesHandler.CreateRule)
				billingRules.PUT("/:id", s.billingRulesHandler.UpdateRule)
				billingRules.DELETE("/:id", s.billingRulesHandler.DeleteRule)
				billingRules.POST("/import", s.billingRulesHandler.ImportRules)
				billingRules.GET("/export", s.billingRulesHandler.ExportRules)
			}
		}
		// /admin/ab-eval* and /admin/tenants* registered by centag-pro (E2.3/E2.5).
		// Paths stay under /api/v1/admin (not /admin/pro).
		if s.extensionHost != nil {
			s.extensionHost.ApplyTeamAdmin(teamAdmin)
		}
	}

	// Commercial edition modules (centag-pro etc.) register via blank import in private assemble builds.
	proAdmin := adminAPI.Group("/pro")
	if err := editionmodule.MountAdmin(proAdmin, editionmodule.AdminDeps{}); err != nil {
		logger.Warnf("edition module mount failed: %v", err)
	}

	// 代理类 API 共用：JWT 或 API Key（与 OpenAI 兼容路由一致）
	dbPlugin := database.Get().Plugin()
	// 仅单机 personal 版（SQLite）跳过预算/模型/限速检查；team/minimal 即便 SQLite 也须强制
	isDesktop := dbPlugin.Name() == "sqlite" && s.edition.IsPersonal()
	proxyAuth := auth.ProxyAuthMiddleware(&auth.AuthConfig{
		RateLimiter: auth.NewRateLimiter(s.cfg.Redis),
		IsDesktop:   isDesktop,
	})

	// API v1
	v1 := s.router.Group("/api/v1")
	v1Protected := v1.Group("")
	v1Protected.Use(proxyAuth)

	// Tenant QuotaMiddleware is registered by centag-pro (E2.3) via Host.
	if s.extensionHost != nil {
		for _, mw := range s.extensionHost.ProtectedMiddlewares() {
			v1Protected.Use(mw)
		}
	}

	{
		// 配置管理
		config := v1Protected.Group("/config")
		{
			config.GET("", s.configHandler.GetAllConfig)
			config.PUT("", s.configHandler.SaveAllConfig)
			config.GET("/proxy", s.handleGetProxyConfig)
			// Team 普通用户写入自己的 user_config.proxy_settings；管理员写系统默认。
			config.PUT("/proxy", s.handleSaveProxyConfig)
			// 模型变量配置
			config.GET("/model-variables", s.modelConfigHandler.GetModelVariables)
			config.PUT("/model-variables", s.modelConfigHandler.UpdateModelVariables)
			config.DELETE("/model-variables/:name", s.modelConfigHandler.DeleteUserVariable)
		}

		// 监控统计
		monitor := v1Protected.Group("/monitor")
		{
			monitor.GET("/stats", s.getStats)
			monitor.GET("/cache", s.getCacheStats)
			monitor.GET("/dashboard", s.metricsHandler.GetDashboardStats)
			monitor.GET("/request", s.metricsHandler.GetRequestStats)
			monitor.GET("/route-backend", s.metricsHandler.GetRouteBackendStats)
			monitor.GET("/cache-legacy", s.metricsHandler.GetCacheStats)
			monitor.GET("/plugins", s.metricsHandler.GetPluginStatus)
			monitor.GET("/config", s.metricsHandler.GetConfigInfo)
			monitor.POST("/reset", s.metricsHandler.ResetStats)
		}

		// 记忆服务 (Agent Memory API)
		memory := v1Protected.Group("/memory")
		{
			memory.POST("/index", s.memoryHandler.BuildIndex)
			memory.POST("/search", s.memoryHandler.Search)
			memory.GET("/get", s.memoryHandler.Get)
			memory.GET("/stats", s.memoryHandler.GetStats)
			memory.PUT("/put", s.memoryHandler.Put)
			memory.POST("/append", s.memoryHandler.Append)
			memory.DELETE("/doc", s.memoryHandler.Delete)
			memory.POST("/sync", s.memoryHandler.Sync)
			memory.POST("/pull", s.memoryHandler.Pull)
			// 版本管理
			memory.GET("/versions", s.memoryHandler.ListVersions)
			memory.GET("/version", s.memoryHandler.GetVersion)
			memory.POST("/restore", s.memoryHandler.RestoreVersion)
		}

		// 插件管理
		plugins := v1Protected.Group("/plugins")
		{
			plugins.GET("", s.listPlugins)
			plugins.GET("/:name", s.getPlugin)
			plugins.PUT("/:name", s.updatePlugin)
		}

		// 插件市场注册中心（插件包/分发元数据）
		if s.pluginRegistryAPI != nil {
			s.pluginRegistryAPI.RegisterRoutes(v1Protected)
		}

		// 后端配置管理（team：租户内 CRUD 全员可写；export/import 等敏感操作仅 admin）
		backends := v1Protected.Group("/backends")
		{
			adminSensitive := s.teamAdminWriteOnly()
			backends.GET("", s.backendHandler.ListBackends)
			backends.GET("/types", s.backendHandler.ListBackendTypes)
			backends.GET("/export", adminSensitive, s.backendHandler.ExportBackends)
			backends.POST("/import", adminSensitive, s.backendHandler.ImportBackends)
			backends.POST("/fetch-models", s.backendHandler.FetchModels)
			backends.POST("", s.backendHandler.CreateBackend)
			backends.POST("/test", s.backendHandler.TestConnection)
			backends.POST("/probe-all", adminSensitive, s.backendHandler.ProbeAllBackends)
			backends.POST("/probe-all-sse", adminSensitive, s.backendHandler.ProbeAllBackendsSSE)
			backends.GET("/circuit-breaker", s.backendHandler.GetCircuitBreakerStatus)
			backends.POST("/circuit-breaker/:id/reset", adminSensitive, s.backendHandler.ResetCircuitBreaker)
			backends.GET("/:id", s.backendHandler.GetBackend)
			backends.GET("/:id/models", s.backendHandler.GetModels)
			backends.PUT("/:id", s.backendHandler.UpdateBackend)
			backends.DELETE("/:id", s.backendHandler.DeleteBackend)
			backends.POST("/:id/probe", s.backendHandler.ProbeBackend)

			// 账户池 CRUD
			backends.GET("/:id/accounts", s.backendHandler.ListBackendAccounts)
			backends.PUT("/:id/account-pool", s.backendHandler.UpdateAccountPool)
			backends.GET("/:id/accounts/stats", s.backendHandler.GetAccountPoolStats)
			backends.GET("/:id/accounts/:accountId", s.backendHandler.GetBackendAccount)
			backends.POST("/:id/accounts", s.backendHandler.CreateBackendAccount)
			backends.PUT("/:id/accounts/:accountId", s.backendHandler.UpdateBackendAccount)
			backends.DELETE("/:id/accounts/:accountId", s.backendHandler.DeleteBackendAccount)
			backends.POST("/:id/accounts/:accountId/reset-breaker", s.backendHandler.ResetAccountBreaker)
		}

		// 配置归档导入（一键还原：应用「配置导出」生成的 centag-initdata.zip）
		v1Protected.POST("/config/import", s.teamAdminWriteOnly(), s.importConfigArchive)

		// 全局降级策略管理
		fallbackPolicies := v1Protected.Group("/fallback-policies")
		{
			adminWrite := s.teamAdminWriteOnly()
			fallbackHandler := NewFallbackPolicyHandler()
			fallbackPolicies.GET("", fallbackHandler.ListPolicies)
			fallbackPolicies.GET("/:id", fallbackHandler.GetPolicy)
			fallbackPolicies.POST("", adminWrite, fallbackHandler.CreatePolicy)
			fallbackPolicies.PUT("/:id", adminWrite, fallbackHandler.UpdatePolicy)
			fallbackPolicies.DELETE("/:id", adminWrite, fallbackHandler.DeletePolicy)
			fallbackPolicies.POST("/:id/test", fallbackHandler.TestPolicy)
		}

		// 存储配置管理（team：写操作仅超管；personal/minimal 保持登录可写）
		storages := v1Protected.Group("/storage")
		{
			adminWrite := s.teamAdminWriteOnly()
			storages.GET("", s.storageHandler.ListStorages)
			storages.GET("/get", s.storageHandler.GetStorage)
			storages.GET("/default-config", s.storageHandler.GetDefaultConfig)
			storages.GET("/status", s.storageHandler.GetStorageStatus)
			storages.GET("/kv/keys", s.storageHandler.ListKVKeys)
			storages.GET("/kv/get", s.storageHandler.GetKVValue)
			storages.POST("/add", adminWrite, s.storageHandler.AddStorage)
			storages.POST("/update", adminWrite, s.storageHandler.UpdateStorage)
			storages.DELETE("", adminWrite, s.storageHandler.DeleteStorage)
			storages.DELETE("/delete", adminWrite, s.storageHandler.DeleteStorage)
			storages.POST("/delete", adminWrite, s.storageHandler.DeleteStorage)
			storages.POST("/toggle", adminWrite, s.storageHandler.ToggleStorage)
			storages.POST("/test", adminWrite, s.storageHandler.TestConnection)
			storages.POST("/connect", adminWrite, s.storageHandler.ConnectStorage)
			storages.POST("/disconnect", adminWrite, s.storageHandler.DisconnectStorage)
			storages.POST("/set-default", adminWrite, s.storageHandler.SetDefaultStorage)
			storages.POST("/kv/delete", adminWrite, s.storageHandler.DeleteKVKey)
		}

		// 数据存储配置管理（team：写操作仅超管）
		dataStores := v1Protected.Group("/data-store")
		{
			adminWrite := s.teamAdminWriteOnly()
			dataStores.GET("", s.dataStoreHandler.ListDataStores)
			dataStores.GET("/get", s.dataStoreHandler.GetDataStore)
			dataStores.GET("/status", s.dataStoreHandler.GetStatus)
			dataStores.POST("/add", adminWrite, s.dataStoreHandler.AddDataStore)
			dataStores.POST("/update", adminWrite, s.dataStoreHandler.UpdateDataStore)
			dataStores.DELETE("", adminWrite, s.dataStoreHandler.DeleteDataStore)
			dataStores.DELETE("/delete", adminWrite, s.dataStoreHandler.DeleteDataStore)
			dataStores.POST("/delete", adminWrite, s.dataStoreHandler.DeleteDataStore)
			dataStores.POST("/toggle", adminWrite, s.dataStoreHandler.ToggleDataStore)
			dataStores.POST("/test", adminWrite, s.dataStoreHandler.TestConnection)
			dataStores.POST("/set-default", adminWrite, s.dataStoreHandler.SetDefault)
			dataStores.POST("/remove-default", adminWrite, s.dataStoreHandler.RemoveDefault)
		}

		// 缓存管理
		cache := v1Protected.Group("/cache")
		{
			cache.GET("/stats", s.cacheHandler.GetStats)
			cache.POST("/clear", s.cacheHandler.ClearCache)
			cache.DELETE("/clear", s.cacheHandler.ClearCache)
			cache.DELETE("/invalidate/:key", s.cacheHandler.InvalidateCache)
			cache.POST("/enable", s.cacheHandler.SetCacheEnabled)
			cache.GET("/enabled", s.cacheHandler.GetCacheEnabled)
			cache.POST("/ttl", s.cacheHandler.SetCacheTTL)
			cache.POST("/check", s.cacheHandler.CheckCache)
			cache.POST("/info", s.cacheHandler.GetCacheInfo)
			cache.DELETE("/entry", s.cacheHandler.DeleteCacheEntry)
			cache.GET("/entry", s.cacheHandler.GetCacheEntry)
			cache.POST("/warmup", s.cacheHandler.WarmupCache)
			cache.GET("/list", s.cacheHandler.ListCacheEntries)
			cache.GET("/entries", s.cacheHandler.ListCacheEntries) // alias for management console
			cache.GET("/semantic/threshold", s.cacheHandler.GetSemanticThreshold)
			cache.POST("/semantic/threshold", s.cacheHandler.SetSemanticThreshold)
			cache.POST("/semantic/search", s.cacheHandler.SemanticSearch)
			cache.POST("/generate-key", s.cacheHandler.GenerateCacheKey)

			// 问答拆分相关
			cache.GET("/qa-split/status", s.cacheHandler.GetQASplitStatus)
			cache.POST("/qa-split/enable", s.cacheHandler.SetQASplitEnabled)
			cache.POST("/qa-split/test", s.cacheHandler.TestQASplit)
			cache.GET("/qa-split/config", s.cacheHandler.GetQASplitConfig)
			cache.POST("/qa-split/config", s.cacheHandler.UpdateQASplitConfig)
		}

		// /system/update* registered by centag-pro via extension.Host (E2.5).
		// 个人/极简版为单管理员，同样支持 OTA；团队版仅管理员可操作。
		system := v1Protected.Group("/system", s.teamAdminWriteOnly())
		if s.extensionHost != nil {
			s.extensionHost.ApplySystemAPI(system)
		}
		// 个人/极简版不加载 centag-pro 插件，OTA 路由由 open-core 直接注册；
		// 团队版由插件注册（避免重复路由）。
		if !s.edition.IsTeam() {
			s.registerSystemUpdateRoutes(system)
		}

		// 匹配策略管理（内置 + 自定义）
		strategies := v1Protected.Group("/strategies")
		{
			strategies.GET("", s.strategyHandler.ListStrategies)
			strategies.POST("", s.strategyHandler.CreateStrategy)
			strategies.PUT("/:id", s.strategyHandler.UpdateStrategy)
			strategies.DELETE("/:id", s.strategyHandler.DeleteStrategy)
		}

		// 缓存评估管理
		evaluation := v1Protected.Group("/evaluation")
		{
			evaluation.GET("/plugins", s.evaluationHandler.ListPlugins)
			evaluation.POST("/plugins/:name/enable", s.evaluationHandler.EnablePlugin)
			evaluation.POST("/plugins/:name/disable", s.evaluationHandler.DisablePlugin)
			evaluation.PUT("/plugins/order", s.evaluationHandler.UpdatePluginOrder)
			evaluation.GET("/plugins/:name/config", s.evaluationHandler.GetPluginConfig)
			evaluation.PUT("/plugins/:name/config", s.evaluationHandler.UpdatePluginConfig)
			evaluation.GET("/plugins/:name/schema", s.evaluationHandler.GetPluginSchema)
			evaluation.POST("/test", s.evaluationHandler.TestEvaluation)
			evaluation.GET("/stats", s.evaluationHandler.GetEvaluationStats)
		}

		// Host代理管理
		if s.hostProxyHandler != nil {
			s.hostProxyHandler.RegisterRoutes(v1Protected)
		}
	}

	// 状态
	v1.GET("/status", s.handleStatus)

	// 日志管理
	logs := v1Protected.Group("/logs")
	{
		logs.GET("", s.logHandler.GetLogs)
		logs.GET("/stats", s.logHandler.GetLogStats)
		logs.POST("/export", s.logHandler.ExportLogs)
		logs.GET("/stream", s.logHandler.StreamLogs)
		logs.GET("/tail", s.logHandler.TailLogs)
		logs.POST("/clear", s.logHandler.ClearLogs)
	}

	traces := v1Protected.Group("/traces")
	{
		traces.GET("/:request_id", s.logHandler.GetTrace)
	}

	// 代理模式管理
	if s.proxyModeHandler != nil {
		proxyModes := v1Protected.Group("/proxy-modes")
		{
			proxyModes.GET("", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.ListModes)))
			proxyModes.POST("", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.CreateMode)))
			proxyModes.PUT("/:key", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.UpdateMode)))
			proxyModes.DELETE("/:key", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.DeleteMode)))
			proxyModes.POST("/:key/enable", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.EnableMode)))
		}

		// 会话代理模式
		sessionMode := v1Protected.Group("/session/proxy-mode")
		{
			sessionMode.POST("", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.SetProxyMode)))
			sessionMode.GET("", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.GetProxyMode)))
			sessionMode.DELETE("", gin.WrapH(http.HandlerFunc(s.proxyModeHandler.DeleteProxyMode)))
		}
	}

	// 流水线管理
	if s.pipelineHandler != nil {
		s.pipelineHandler.RegisterPipelineRoutes(v1Protected)
	}
	if s.webhookHandler != nil {
		v1Protected.POST("/webhooks/pipeline/:id", s.webhookHandler.TriggerPipeline)
	}

	// 流水线默认：管理员改系统默认；Team 普通用户改自己的 default_pipeline_id
	if s.pipelineDefaultsHandler != nil {
		pipelineDefaults := v1Protected.Group("/pipeline/defaults")
		{
			pipelineDefaults.GET("", s.getPipelineDefaultsForUser)
			pipelineDefaults.PUT("", s.updatePipelineDefaultsForUser)
		}
	}

	// 系统代理管理API(始终可用,不受Enable状态限制)
	if s.proxyHandlerExt != nil {
		// PAC 与 CA 证书下载需要在无鉴权环境下可访问（系统代理配置/安装证书场景）
		proxyPublic := v1.Group("/proxy")
		{
			proxyPublic.GET("/pac", s.proxyHandlerExt.ServePAC)
			proxyPublic.GET("/ca.crt", s.proxyHandlerExt.GetCACert)
		}

		// 其余代理管理接口要求鉴权
		proxy := v1Protected.Group("/proxy")
		{
			proxy.GET("/status", s.proxyHandlerExt.GetProxyStatus)
			proxy.GET("/setup/status", s.proxyHandlerExt.GetSetupStatus)
			proxy.POST("/egress-key/ensure", s.configHandler.EnsureSystemProxyEgress)
			proxy.POST("/egress-key/bind", s.configHandler.BindSystemProxyEgress)
			proxy.GET("/domains", s.proxyHandlerExt.GetPACDomains)
			proxy.POST("/domains/add", s.proxyHandlerExt.AddDomain)
			proxy.POST("/domains/remove", s.proxyHandlerExt.RemoveDomain)
			proxy.POST("/domains/ensure-defaults", s.proxyHandlerExt.EnsureDefaultPACRules)
			proxy.POST("/patterns/add", s.proxyHandlerExt.AddPathPattern)
			proxy.POST("/patterns/remove", s.proxyHandlerExt.RemovePathPattern)
		}
	}

	// Agent 快速配置（需要 JWT 认证）
	if s.agentHandler != nil {
		agentCfg := v1Protected.Group("/agent")
		{
			agentCfg.GET("/types", s.agentHandler.ListAgentTypes)
			agentCfg.POST("/configs/generate", s.agentHandler.GenerateConfig)
			agentCfg.POST("/configs/write", s.agentHandler.WriteConfig)
			agentCfg.POST("/configs/restore", s.agentHandler.RestoreConfig)
			agentCfg.GET("/configs/preview", s.agentHandler.GetConfigPreview)
			agentCfg.POST("/configs/script", s.agentHandler.GenerateScript)
		}
	}

	// 内置 Agent（需要 JWT 认证；team 发行版整体收紧为管理员，/health 保留给已认证用户）
	if s.builtinAgentHandler != nil {
		builtinAgent := v1Protected.Group("/builtin-agent")
		agentAdmin := s.teamAdminWriteOnly()
		{
			builtinAgent.GET("/health", s.builtinAgentHandler.Health)
			builtinAgent.POST("/sessions", agentAdmin, s.builtinAgentHandler.CreateSession)
			builtinAgent.GET("/sessions", agentAdmin, s.builtinAgentHandler.ListSessions)
			builtinAgent.GET("/sessions/:id", agentAdmin, s.builtinAgentHandler.GetSession)
			builtinAgent.DELETE("/sessions/:id", agentAdmin, s.builtinAgentHandler.DeleteSession)
			builtinAgent.POST("/sessions/:id/messages", agentAdmin, s.builtinAgentHandler.SendMessage)
			builtinAgent.GET("/sessions/:id/messages", agentAdmin, s.builtinAgentHandler.ListMessages)
			builtinAgent.GET("/skills", agentAdmin, s.builtinAgentHandler.ListSkills)
			builtinAgent.POST("/skills", agentAdmin, s.builtinAgentHandler.CreateSkill)
			builtinAgent.PUT("/skills/:name", agentAdmin, s.builtinAgentHandler.UpdateSkill)
			builtinAgent.DELETE("/skills/:name", agentAdmin, s.builtinAgentHandler.DeleteSkill)
			builtinAgent.POST("/skills/:name/clone", agentAdmin, s.builtinAgentHandler.CloneSkill)
			builtinAgent.POST("/sessions/:id/confirm", agentAdmin, s.builtinAgentHandler.ConfirmTool)
			builtinAgent.POST("/sessions/:id/cancel", agentAdmin, s.builtinAgentHandler.CancelExecution)
		}
	}

	// wrap run：本机终端启动第三方 Agent（personal/minimal 或 loopback）
	wrapAPI := v1Protected.Group("/wrap")
	{
		wrapAPI.GET("/presets", s.ListWrapPresets)
		wrapAPI.POST("/run", s.RunWrapAgent)
	}

	// Agent 供应商配置管理（需要 JWT 认证）
	if s.agentProviderHandler != nil {
		agentProv := v1Protected.Group("/agent-providers")
		{
			agentProv.GET("", s.agentProviderHandler.List)
			agentProv.GET("/by-type", s.agentProviderHandler.GetByAgentType)
			agentProv.GET("/:id", s.agentProviderHandler.Get)
		}

		agentProvAdmin := v1.Group("", auth.JWTMiddleware(), auth.AdminOnlyMiddleware()).Group("/agent-providers")
		{
			agentProvAdmin.POST("/:id/hot-swap", s.agentProviderHandler.HotSwap)
		}
	}

	// 对外 LLM 代理路由：支持 API Key 或 JWT 认证
	llmProxyHandler := middleware.NewLLMProxyHandler(s.backendManager, s.proxyCache, &s.cfg.Proxy, &s.cfg.Cache)

	// 注册代理模式解析中间件（支持关键字前缀、会话模式、请求头/体指定）
	proxyModeMw := middleware.ProxyModeMiddlewareGin(s.modeManager, s.sessionStore)
	agentDetectMw := middleware.AgentClientDetectMiddlewareGin()

	// v2.1: User-level quota (open-core). Tenant QuotaMiddleware comes from centag-pro (E2.3).
	userQuotaMw := middleware.NewUserQuotaMiddleware(database.Get()).Middleware()
	resourceGuard := s.teamResourceModelGuard()
	var tenantQuotaMWs []gin.HandlerFunc
	if s.extensionHost != nil {
		tenantQuotaMWs = s.extensionHost.ProtectedMiddlewares()
	}
	llmChain := func(extra ...gin.HandlerFunc) []gin.HandlerFunc {
		h := []gin.HandlerFunc{proxyAuth, userQuotaMw}
		h = append(h, tenantQuotaMWs...)
		h = append(h, extra...)
		return h
	}

	// 代理模式路由：支持关键字解析（user quota → optional tenant quota from pro）
	s.router.POST("/v1/chat/completions", llmChain(resourceGuard, agentDetectMw, proxyModeMw, s.proxyHandler.HandleChatCompletions)...)
	s.router.GET("/v1/models", llmChain(agentDetectMw, proxyModeMw, s.proxyHandler.ListModels)...)
	s.router.GET("/v1/backends", llmChain(agentDetectMw, proxyModeMw, s.proxyHandler.ListBackends)...)
	s.router.POST("/v1/messages", llmChain(resourceGuard, agentDetectMw, proxyModeMw, s.proxyHandler.HandleChatCompletions)...)
	// Anthropic 兼容前缀（DeepSeek/Kimi 等通行做法：base_url 带 /anthropic 后缀，SDK 自动拼接 /v1/messages）
	s.router.POST("/anthropic/v1/messages", llmChain(resourceGuard, agentDetectMw, proxyModeMw, s.proxyHandler.HandleChatCompletions)...)
	s.router.POST("/v1/responses", llmChain(resourceGuard, agentDetectMw, proxyModeMw, s.proxyHandler.HandleChatCompletions)...)
	s.router.POST("/v1beta/models/*action", llmChain(resourceGuard, agentDetectMw, proxyModeMw, s.proxyHandler.HandleChatCompletions)...)
	s.router.POST("/v1/completions", llmChain(resourceGuard, agentDetectMw, proxyModeMw, llmProxyHandler.HandleOpenAIRequest)...)
	s.router.POST("/v1/embeddings", llmChain(resourceGuard, agentDetectMw, proxyModeMw, llmProxyHandler.HandleOpenAIRequest)...)
	s.router.POST("/api/v1/openai/chat/completions", llmChain(resourceGuard, agentDetectMw, proxyModeMw, llmProxyHandler.HandleOpenAIRequest)...)
	s.router.POST("/api/v1/openai/embeddings", llmChain(resourceGuard, agentDetectMw, proxyModeMw, llmProxyHandler.HandleOpenAIRequest)...)
	s.router.GET("/api/v1/openai/models", llmChain(agentDetectMw, proxyModeMw, llmProxyHandler.HandleOpenAIRequest)...)

	// xAI / Grok Build 私有端点 mock（wrap 模式兼容，避免 404 触发登录）
	s.router.GET("/v1/settings", s.proxyHandler.HandleXAISettings)
	s.router.GET("/v1/user", s.proxyHandler.HandleXAIUser)
	s.router.GET("/v1/billing", s.proxyHandler.HandleXAIBilling)
	s.router.GET("/v1/mcp/configs", s.proxyHandler.HandleXAIMCPConfigs)
	s.router.GET("/v1/login-config", s.proxyHandler.HandleXAILoginConfig)
	s.router.GET("/v1/bundle/archive", s.proxyHandler.HandleXAIBundleArchive)

	// MCP (Model Context Protocol) 代理路由
	if s.mcpProxyHandler != nil {
		mcpGroup := s.router.Group("/v1/mcp", proxyAuth)
		s.mcpProxyHandler.RegisterMCPRoutes(mcpGroup)
	}

	logger.Info("Proxy mode middleware registered for LLM proxy routes")
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)

	// pprof 观测（仅 loopback）。开启：CENTAG_PPROF=true 或 LLM_PROXY_PPROF_ENABLED=true
	if s.cfg.Server.PprofEnabled {
		go func() {
			logger.Info("pprof listening on 127.0.0.1:6060 (CENTAG_PPROF)")
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				logger.Warn("pprof server stopped", zap.Error(err))
			}
		}()
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.cfg.Proxy.Timeout) * time.Second,
		WriteTimeout: 0,                 // 禁用WriteTimeout,避免流式响应被截断
		IdleTimeout:  300 * time.Second, // 5分钟空闲超时
	}

	logger.Infof("Starting server on %s", addr)

	// 启动Host代理服务器(如果启用)
	if s.hostProxyServer != nil {
		s.mitmWg.Add(1)
		go func() {
			defer s.mitmWg.Done()
			if err := s.hostProxyServer.Start(); err != nil {
				logger.Errorf("Host proxy server error: %v", err)
			}
		}()
		logger.Infof("Host proxy server started on HTTP port %d, HTTPS port %d",
			s.cfg.HostProxy.HTTPPort, s.cfg.HostProxy.HTTPSPort)
	}

	// 启动MITM代理服务器(如果启用)
	if s.mitmServer != nil {
		s.mitmWg.Add(1)
		go func() {
			defer s.mitmWg.Done()
			if err := s.mitmServer.Start(); err != nil {
				logger.Errorf("MITM server error: %v", err)
			}
		}()
		logger.Infof("MITM proxy server started on port %d", s.cfg.SystemProxy.ListenPort)
	}

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("Server ListenAndServe failed: %v", err)
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Stopping server...")

	if s.extensionHost != nil {
		s.extensionHost.Close()
	}

	// 停止远程插件健康检查协程，避免后台 goroutine 泄漏。
	if s.pipelineHandler != nil {
		s.pipelineHandler.Stop()
	}

	// 停止Host代理服务器
	if s.hostProxyServer != nil {
		logger.Info("Stopping Host proxy server...")
		if err := s.hostProxyServer.Stop(ctx); err != nil {
			logger.Errorf("Failed to stop Host proxy server: %v", err)
		} else {
			logger.Info("Host proxy server stopped")
		}
	}

	// 停止MITM代理服务器(在Wait之前调用,确保goroutine能退出)
	if s.mitmServer != nil {
		logger.Info("Stopping MITM proxy server...")
		if err := s.mitmServer.Stop(ctx); err != nil {
			logger.Errorf("Failed to stop MITM server: %v", err)
		} else {
			logger.Info("MITM proxy server stopped")
		}
	}

	// 等待Host Proxy和MITM的goroutine退出
	logger.Info("Waiting for goroutines to exit...")
	s.mitmWg.Wait()
	logger.Info("All goroutines exited")

	// 关闭存储管理器
	if s.storageManager != nil {
		logger.Info("Closing storage manager...")
		if err := s.storageManager.Close(); err != nil {
			logger.Errorf("Failed to close storage manager: %v", err)
		}
	}

	// 停止插件
	logger.Info("Stopping plugins...")
	if err := s.pluginManager.Stop(); err != nil {
		logger.Errorf("Failed to stop plugins: %v", err)
	}

	// 停止主服务器
	if s.server != nil {
		logger.Info("Shutting down main server...")
		// 先禁用 keep-alive，让空闲连接尽快关闭，避免 Shutdown 阻塞在长连接上
		s.server.SetKeepAlivesEnabled(false)
		if err := s.server.Shutdown(ctx); err != nil {
			logger.Errorf("Main server shutdown error: %v", err)
			// Shutdown 超时/取消时强制关闭，避免进程卡死
			if cerr := s.server.Close(); cerr != nil {
				return cerr
			}
			return err
		}
	}

	return nil
}

// GetRouter 获取路由实例
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// SetVersionProvider injects a remote version provider into the system update
// handler. When set, /update/check queries this provider first, falling back
// to the GitHub OTA client on error.
func (s *Server) SetVersionProvider(vp internal.VersionProvider) {
	if s.systemUpdate != nil {
		s.systemUpdate.SetVersionProvider(vp)
	}
}

// forceRestartMITM 强制重启 MITM 代理服务器，用于端口变更后的热生效。
// 先停止再启动，toggleMITM(true) 会从 s.cfg 读取最新端口，因此调用前
// 确保 cfg.SystemProxy.ListenPort 已更新。
func (s *Server) forceRestartMITM() {
	s.toggleMITM(false)
	s.toggleMITM(true)
}

// toggleMITM 动态启停 MITM 代理服务器，无需重启主服务
// 由 ConfigHandler 在 SystemProxy.Enabled 发生变化时调用
func (s *Server) toggleMITM(enabled bool) {
	s.mitmMu.Lock()
	defer s.mitmMu.Unlock()

	if enabled {
		if s.mitmServer != nil {
			logger.Info("MITM proxy server is already running, skip start")
			return
		}
		// 构造 MITM 配置
		cfg := s.cfg
		backendHost := cfg.Server.Host
		if backendHost == "0.0.0.0" || backendHost == "" {
			backendHost = "127.0.0.1"
		}
		if err := config.ValidateSystemProxyConfig(&cfg.SystemProxy); err != nil {
			logger.Errorf("Invalid system_proxy config on hot-enable: %v", err)
			return
		}
		mitmConfig := buildMITMConfig(cfg, backendHost)
		srv, err := mitm.NewServer(mitmConfig)
		if err != nil {
			logger.Errorf("Failed to create MITM server on hot-enable: %v", err)
			return
		}
		s.mitmServer = srv
		// Gin 路由绑定的是注册时的 handler 实例，只能就地更新引用，不能整体替换
		if s.proxyHandlerExt != nil {
			s.proxyHandlerExt.SetMitmServer(s.mitmServer)
		}
		s.mitmWg.Add(1)
		go func() {
			defer s.mitmWg.Done()
			if err := srv.Start(); err != nil {
				logger.Errorf("MITM server error: %v", err)
			}
		}()
		logger.Infof("MITM proxy server hot-started on port %d", cfg.SystemProxy.ListenPort)
	} else {
		if s.mitmServer == nil {
			logger.Info("MITM proxy server is not running, skip stop")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.mitmServer.Stop(ctx); err != nil {
			logger.Errorf("Failed to stop MITM server on hot-disable: %v", err)
		} else {
			logger.Info("MITM proxy server hot-stopped")
		}
		s.mitmServer = nil
		if s.proxyHandlerExt != nil {
			s.proxyHandlerExt.SetMitmServer(nil)
		}
	}
}

func buildSystemProxyPACConfig(cfg *config.Config) *pac.Config {
	sp := cfg.SystemProxy
	config.NormalizeSystemProxyConfig(&sp)
	return &pac.Config{
		ProxyHost:    sp.PACProxyHost(),
		ProxyPort:    sp.ListenPort,
		Domains:      sp.Domains,
		PathPatterns: sp.PathPatterns,
	}
}

func buildMITMConfig(cfg *config.Config, backendHost string) *mitm.Config {
	// Use RequireClientProxyAuth config; fall back to AllowLANClients for backward compatibility
	requireProxyAuth := cfg.SystemProxy.RequireClientProxyAuth
	var validator mitm.ClientTokenValidator
	if requireProxyAuth {
		validator = func(token string) error {
			return auth.ValidateMITMProxyToken(context.Background(), token)
		}
	}
	return &mitm.Config{
		Addr:                   cfg.SystemProxy.MITMListenAddr(),
		BackendAddr:            fmt.Sprintf("%s:%d", backendHost, cfg.Server.Port),
		CACertPath:             cfg.SystemProxy.CACertPath,
		CAKeyPath:              cfg.SystemProxy.CAKeyPath,
		CertDir:                cfg.SystemProxy.CertDir,
		CertValidDays:          cfg.SystemProxy.CertValidDays,
		Domains:                cfg.SystemProxy.Domains,
		PathPatterns:           cfg.SystemProxy.PathPatterns,
		BackendAuthToken:       config.ResolveSystemProxyEgressAPIKey(&cfg.SystemProxy),
		RequireClientProxyAuth: requireProxyAuth,
		ClientTokenValidator:   validator,
	}
}

func (s *Server) syncMITMEgressAuth() {
	s.mitmMu.Lock()
	defer s.mitmMu.Unlock()
	if s.mitmServer == nil || s.cfg == nil {
		return
	}
	token := config.ResolveSystemProxyEgressAPIKey(&s.cfg.SystemProxy)
	s.mitmServer.SetBackendAuthToken(token)
	logger.Infof("MITM egress API key synced (configured=%v)", token != "")
}

func (s *Server) syncMITMClientProxyAuth() {
	s.mitmMu.Lock()
	defer s.mitmMu.Unlock()
	if s.mitmServer == nil || s.cfg == nil {
		return
	}
	required := s.cfg.SystemProxy.RequireClientProxyAuth
	var validator mitm.ClientTokenValidator
	if required {
		validator = func(token string) error {
			return auth.ValidateMITMProxyToken(context.Background(), token)
		}
	}
	s.mitmServer.SetClientProxyAuth(required, validator)
	logger.Infof("MITM client proxy auth synced: required=%v", required)
}

func (s *Server) refreshProxyHandlerPAC() {
	if s == nil || s.proxyHandlerExt == nil || s.cfg == nil {
		return
	}
	s.proxyHandlerExt.RefreshPACConfig(buildSystemProxyPACConfig(s.cfg))
}

// registerBuiltinPluginsToStore 将 NodeRegistry 中的内置插件注册到 PluginRegistryStore
func registerBuiltinPluginsToStore(nodeRegistry *pipeline.NodeRegistry, pluginRegistryStore pipeline.PluginRegistryStore) {
	if nodeRegistry == nil || pluginRegistryStore == nil {
		return
	}

	descriptors := nodeRegistry.GetPluginDescriptors()
	if len(descriptors) == 0 {
		logger.Info("No builtin plugins to register")
		return
	}

	for _, desc := range descriptors {
		// 检查是否已存在
		_, err := pluginRegistryStore.Get(desc.Implementation)
		if err == nil {
			// 已存在，跳过
			continue
		}

		// 序列化描述符为 JSON
		descriptorJSON, err := json.Marshal(desc)
		if err != nil {
			logger.Warnf("Failed to marshal plugin descriptor for %s: %v", desc.Implementation, err)
			continue
		}

		// 创建 PluginRegistration
		reg := &pipeline.PluginRegistration{
			Implementation:  desc.Implementation,
			Kind:            desc.Kind,
			Version:         desc.Version,
			DescriptorJSON:  string(descriptorJSON),
			Source:          "builtin",
			Enabled:         true,
			SignatureStatus: "none",
			HealthStatus:    "healthy",
		}

		if err := pluginRegistryStore.Register(reg); err != nil {
			logger.Warnf("Failed to register builtin plugin %s to store: %v", desc.Implementation, err)
		} else {
			logger.Infof("Registered builtin plugin %s to registry store", desc.Implementation)
		}
	}
}

// firstNonZero 返回第一个非零值
func firstNonZero(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// firstDuration 返回第一个非零时长
func firstDuration(v time.Duration, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}

// firstStrings 返回第一个非空切片
func firstStrings(v []string, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

// resolveAgentDBPath 解析数据库文件路径（供 centag_info 工具展示）
func resolveAgentDBPath(dataDir string) string {
	// 优先环境变量
	if p := os.Getenv("SQLITE_PATH"); p != "" {
		return p
	}
	// 探测数据目录下常见位置
	candidates := []string{
		filepath.Join(dataDir, "storage", "centag.db"),
		filepath.Join(dataDir, "var", "centag.db"),
		filepath.Join(dataDir, "lib", "personal", "storage", "centag.db"),
		filepath.Join(dataDir, "lib", "minimal", "storage", "centag.db"),
		filepath.Join(dataDir, "lib", "team", "storage", "centag.db"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	// 递归探测
	var found string
	_ = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == "centag.db" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
