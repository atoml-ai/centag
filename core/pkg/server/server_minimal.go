package server

import (
	"os"
	"path/filepath"
	"time"

	"centag/core/internal/auth"
	"centag/core/internal/billing"
	"centag/core/internal/cache"
	"centag/core/internal/conversation"
	"centag/core/internal/edition"
	"centag/core/internal/handler"
	"centag/core/internal/proxy"
	"centag/core/internal/session"
	"centag/core/internal/tokenusage"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/hooks"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	pluginregistry "centag/core/pkg/plugin/registry"
	"centag/core/pkg/proxymode"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// NewMinimal creates a server for the minimal edition (file-based config, no database).
// This constructor skips all database-dependent handlers and only includes
// the core proxy functionality and config-generator API.
func NewMinimal(cfg *config.Config) *Server {
	gin.SetMode(cfg.Server.Mode)
	router := gin.New()
	router.Use(recoveryMiddleware())
	router.Use(corsMiddleware())
	router.Use(loggerMiddleware())

	// Create plugin manager
	pluginManager := plugin.NewManager()
	if err := registerPlugins(pluginManager); err != nil {
		logger.Errorf("Failed to register plugins: %v", err)
	}

	// Backend manager (already loaded by entrypoint_minimal.go)
	backendManager := backend.GetManager()

	// Create pipeline engine (in-memory for minimal)
	nodeRegistry := pipeline.NewNodeRegistry()
	if err := pipeline.RegisterBuiltinNodes(nodeRegistry); err != nil {
		logger.Warnf("Failed to register builtin nodes: %v", err)
	} else {
		logger.Infof("Builtin nodes registered: %d plugins", len(nodeRegistry.GetPluginDescriptors()))
	}

	// Security validator
	securityValidator := pipeline.NewPluginSecurityValidator(cfg.PluginSecurity)
	admissionChecker := pipeline.NewAdmissionChecker(cfg.PluginSecurity.AdmissionCheck)
	nodeRegistry.SetSecurityValidator(securityValidator)
	nodeRegistry.SetAdmissionChecker(admissionChecker)

	// Business plugin registry
	bizRegistry := pipeline.NewBusinessPluginRegistry()
	nodeRegistry.SetBusinessRegistry(bizRegistry)

	// Data dir for file persistence (backends / pipelines / proxy-config)
	dataDir := resolveDataDir()
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		logger.Warnf("Failed to create data dir %s: %v", dataDir, err)
	}
	if absDir, err := filepath.Abs(dataDir); err == nil {
		dataDir = absDir
	}

	// Pipeline registry backed by YAML files under data/pipeline-templates
	// (PUT /pipelines/:id must survive restart — not memory-only).
	pipelineTmplDir := filepath.Join(dataDir, "pipeline-templates")
	fileStore, err := pipeline.NewFilePipelineStore(pipelineTmplDir)
	if err != nil {
		logger.Errorf("[Minimal] file pipeline store init failed: %v; falling back to memory-only", err)
		fileStore = nil
	}
	var pipelineRegistry *pipeline.PipelineRegistry
	if fileStore != nil {
		pipelineRegistry = pipeline.NewPipelineRegistryWithStore(fileStore)
		if loadErr := pipelineRegistry.LoadFromStore(); loadErr != nil {
			logger.Warnf("[Minimal] load pipelines from %s: %v", pipelineTmplDir, loadErr)
		} else {
			logger.Infof("[Minimal] Loaded %d pipelines from %s", len(pipelineRegistry.List()), pipelineTmplDir)
		}
	} else {
		pipelineRegistry = pipeline.NewPipelineRegistry()
	}

	// Builtin templates: seed only missing IDs (never overwrite user-saved YAML)
	templates := resolvePipelineTemplatesWithEdition("minimal")
	logger.Infof("Pipeline templates loaded: %d builtin templates (edition: minimal)", len(templates))
	seeded := 0
	for _, tmpl := range templates {
		if pipelineRegistry.Exists(tmpl.ID) {
			continue
		}
		p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
		if err := pipelineRegistry.Register(p); err != nil {
			logger.Warnf("Failed to register builtin pipeline %s: %v", p.ID, err)
		} else {
			seeded++
		}
	}
	logger.Infof("[Minimal] Pipeline registry ready: total=%d seeded_from_builtin=%d dir=%s",
		len(pipelineRegistry.List()), seeded, pipelineTmplDir)

	// Create pipeline engine
	pipelineEngine := pipeline.NewPipelineEngine(
		nodeRegistry,
		pipelineRegistry,
		nil, // CapabilityBroker — injected below
		pipeline.NewPipelineLogger(),
		nil, // storage.Manager
	)

	// Create and inject CapabilityBroker for pipeline engine
	secretsProvider := pipeline.NewEnvSecretsProvider("LLM_PROXY_")
	httpConfig := pipeline.HTTPConfig{
		Timeout: cfg.Proxy.Timeout,
	}
	capabilityBroker := pipeline.NewCapabilityBroker(
		nil, // storageProvider — not available in minimal
		nil, // memoryProvider — not available in minimal
		secretsProvider,
		httpConfig,
	)
	if backendManager != nil {
		llmProvider := pipeline.NewDefaultLLMProvider(backendManager, pluginManager)
		capabilityBroker.SetLLMProvider(llmProvider)
	}
	pipelineEngine.SetCapabilityBroker(capabilityBroker)
	logger.Info("[Minimal] CapabilityBroker injected into PipelineEngine")

	// 智能调度 / 透明转发 hook：与完整版 server.go 对齐，供 scheduler / transparent_forward 节点使用
	appScheduler := buildScheduler(cfg, backendManager)
	wireSchedulerBackend(appScheduler)
	wireSchedulerMetricsFeedback(appScheduler)
	wireTransparentBackend(backendManager)
	if appScheduler != nil {
		logger.Infof("[Minimal] Scheduler initialized and wired to pipeline (intent_recognition=%v, task_strategies=%d)",
			cfg.Scheduler.EnableIntentRecognition, len(cfg.Scheduler.TaskStrategies))
	} else {
		logger.Warn("[Minimal] Scheduler not available (backend manager nil); smart-scheduling pipeline will fail until backends load")
	}

	// Create proxy handler
	proxyService := proxy.New(pluginManager)
	// Use nil for proxyCache — not used by the handler directly
	proxyHandler := proxy.NewHandler(proxyService, pluginManager, nil)
	proxyHandler.SetPipelineEngine(pipelineEngine)
	proxyHandler.SetPipelineRegistry(pipelineRegistry)
	proxyHandler.SetBusinessPluginRegistry(bizRegistry)

	// Create and inject DefaultPipelineResolver (minimal edition: from config, no DB/user quotas)
	defaultPipelineResolver := proxy.NewDefaultPipelineResolver(cfg)
	proxyHandler.SetDefaultPipelineResolver(defaultPipelineResolver)
	logger.Infof("[Minimal] DefaultPipelineResolver initialized, default pipeline: %s", cfg.Proxy.EffectiveDefaultPipeline())

	// Create cache manager (basic, in-memory only)
	cacheConfig := &cache.CacheConfig{
		Enabled:         false, // disabled by default in minimal
		DefaultTTL:      3600,
		MaxSize:         1000,
		CleanupInterval: 5 * time.Minute,
	}
	cacheManager, err := cache.NewManager(cacheConfig)
	if err != nil {
		logger.Warnf("Failed to create cache manager: %v", err)
	}

	// Proxy cache (used by backend handler probe)
	var proxyCache *cache.ProxyCache
	if cacheManager != nil {
		proxyCache = cache.NewProxyCache(cacheManager, false)
	}

	// Create backend handler
	backendHandler := NewBackendHandler(backendManager)

	// Create pipeline handler
	pipelineHandler := NewPipelineHandler(pipelineEngine, nodeRegistry, pipelineRegistry, templates, nil)
	pipelineDefaultsHandler := handler.NewPipelineDefaultsHandler(cfg, pipelineRegistry)

	// Create proxy mode manager + session store（供 ProxyModeMiddleware 解析 pipeline.<id>）
	modeMgr := proxymode.NewManager()
	if synced := modeMgr.SyncFromPipelines(pipelineRegistry.ListAll()); synced > 0 {
		logger.Infof("Synced %d pipeline shortcuts into ModeManager", synced)
	}
	sessionStore := session.NewProxyModeStore()

	// Config handler for minimal (file-based); dataDir already resolved above
	// 确保 API Save 路径与 config-generator 写入路径一致
	backendFile := filepath.Join(dataDir, "initial-backends.yaml")
	backendManager.SetStore(backend.NewFileBackendStore(backendFile))

	// JWT secret for minimal auth (file-based, no DB)
	if err := auth.EnsureFileSecret(filepath.Join(dataDir, "jwt.secret")); err != nil {
		logger.Errorf("[Minimal] failed to init JWT secret: %v", err)
	}
	minimalAuth := NewMinimalAuthHandler(dataDir)

	minimalConfigHandler := NewMinimalConfigHandler(dataDir, nil, pipelineRegistry)
	pipelineDefaultsHandler.SetPersistFn(func(defaultPipelineID string) error {
		cfg := DefaultPipelineConfig{DefaultPipeline: defaultPipelineID}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return err
		}
		pipelineFile := filepath.Join(dataDir, "default-pipeline.yaml")
		if err := os.WriteFile(pipelineFile, data, 0o644); err != nil {
			return err
		}
		logger.Infof("[Minimal] Default pipeline persisted to %s: %s", pipelineFile, defaultPipelineID)
		return nil
	})
	minimalConfigHandler.SetReloadFunc(func() error {
		// 1. 重新加载后端配置，并确保 Save() 写回 data 目录
		backendManager.SetStore(backend.NewFileBackendStore(backendFile))
		if err := backendManager.LoadFromFile(backendFile); err != nil {
			logger.Warnf("Failed to reload backends from %s: %v", backendFile, err)
		} else {
			logger.Infof("Backends reloaded: %d backends from %s", len(backendManager.List()), backendFile)
		}

		// 2. 重新加载流水线模板：优先从 dataDir/pipeline-templates 读取（用户在页面保存的）
		// 然后从全局 config/initdata/pipeline-templates 读取（内置的）
		var templates []pipeline.PatternTemplate

		// 2a. 从 dataDir 加载用户自定义模板（覆盖全局同名模板）
		tmplMap := make(map[string]pipeline.PatternTemplate)
		dataTmplDir := filepath.Join(dataDir, "pipeline-templates")
		if info, err := os.Stat(dataTmplDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(dataTmplDir)
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if len(name) < 5 || name[len(name)-5:] != ".yaml" {
					continue
				}
				fullPath := filepath.Join(dataTmplDir, name)
				fileData, err := os.ReadFile(fullPath)
				if err != nil {
					logger.Warnf("Failed to read template %s: %v", fullPath, err)
					continue
				}
				var tmpl pipeline.PatternTemplate
				if err := yaml.Unmarshal(fileData, &tmpl); err != nil {
					logger.Warnf("Failed to parse template %s: %v", fullPath, err)
					continue
				}
				if tmpl.ID != "" {
					tmplMap[tmpl.ID] = tmpl
				}
			}
		}

		// 2b. 从全局目录加载内置模板（dataDir 的同名模板会覆盖）
		builtinTemplates := resolvePipelineTemplatesWithEdition("minimal")
		for _, t := range builtinTemplates {
			if _, exists := tmplMap[t.ID]; !exists {
				tmplMap[t.ID] = t
			}
		}

		// 2c. 转为列表
		for _, t := range tmplMap {
			templates = append(templates, t)
		}
		logger.Infof("Pipeline templates reloaded: %d templates (dataDir: %s)", len(templates), dataTmplDir)

		// Re-register all templates as pipelines
		for _, tmpl := range templates {
			p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
			if err := pipelineRegistry.Register(p); err != nil {
				logger.Warnf("Failed to register builtin pipeline %s: %v", p.ID, err)
			}
		}
		logger.Infof("Registered %d pipelines from templates", len(pipelineRegistry.List()))

		// Sync pipeline shortcuts to ModeManager
		if synced := modeMgr.SyncFromPipelines(pipelineRegistry.ListAll()); synced > 0 {
			logger.Infof("Synced %d pipeline shortcuts into ModeManager", synced)
		}

		return nil
	})

	// Plugin registry (in-memory for minimal)
	pluginMarketStore := pluginregistry.NewMemoryStore()
	pluginRegistryAPI := pluginregistry.NewHandler(pluginMarketStore)

	// --- Ephemeral hooks: file conversations + in-memory token metering (cleared on restart) ---
	hookManager := hooks.NewManager()
	hooks.SetDefault(hookManager)
	logger.Infof("[hooks] HookManager registered (minimal, fail-open)")

	var tokenUsageHandler *TokenUsageHandler
	var costHandler *CostHandler
	var billingRulesHandler *BillingRulesHandler
	var pricingService billing.PricingService
	if tokenSvc, err := tokenusage.NewEphemeralService(); err != nil {
		logger.Warnf("[hooks] ephemeral token meter init failed: %v", err)
	} else {
		tokenusage.SetDefaultService(tokenSvc)
		hookManager.RegisterTokenHook(newTokenUsageHookAdapter(tokenSvc))
		wireTokenUsagePersistence(tokenSvc, hookManager)
		proxyHandler.SetTokenUsageService(tokenSvc)
		tokenUsageHandler = NewTokenUsageHandler(tokenSvc)
		costHandler = NewCostHandler(tokenSvc)
		logger.Infof("[hooks] TokenHook registered (ephemeral in-memory; cleared on restart)")
	}
	{
		var memStore *billing.MemoryRuleStore
		if path, err := billing.ResolveDefaultPricingPath(); err != nil {
			logger.Warnf("[billing] default pricing YAML not found: %v", err)
			memStore = billing.NewMemoryRuleStore()
		} else if s, err := billing.NewMemoryRuleStoreFromFile(path); err != nil {
			logger.Warnf("[billing] load pricing YAML failed: %v", err)
			memStore = billing.NewMemoryRuleStore()
		} else {
			memStore = s
			logger.Infof("[billing] memory pricing rules loaded from %s", path)
		}
		pricingService = billing.NewPricingService(memStore)
		tokenusage.SetPricingService(pricingService)
		billingRulesHandler = NewBillingRulesHandler(memStore, pricingService)
	}

	var conversationHandler *ConversationHandler
	convRoot := filepath.Join(dataDir, "runtime", "conversations")
	if err := conversation.PrepareEphemeralFileRoot(convRoot); err != nil {
		logger.Warnf("[conversation] prepare ephemeral root failed: %v", err)
	} else if store, err := conversation.NewStore(conversation.Options{
		Edition:  edition.Minimal,
		FileRoot: convRoot,
	}); err != nil {
		logger.Warnf("[conversation] store init failed: %v", err)
	} else {
		conversation.SetDefault(store)
		hookManager.RegisterStorageHook(conversation.NewLoggingHook(store))
		conversationHandler = NewConversationHandler(store, edition.Minimal)
		logger.Infof("[hooks] Conversation LoggingHook registered (file=%s; wiped on restart)", convRoot)
	}

	// Create the server
	srv := &Server{
		router:                  router,
		cfg:                     cfg,
		pluginManager:           pluginManager,
		backendManager:          backendManager,
		proxyHandler:            proxyHandler,
		backendHandler:          backendHandler,
		pipelineHandler:         pipelineHandler,
		pipelineDefaultsHandler: pipelineDefaultsHandler,
		modeManager:             modeMgr,
		sessionStore:            sessionStore,
		cacheManager:            cacheManager,
		proxyCache:              proxyCache,
		tokenUsageHandler:       tokenUsageHandler,
		costHandler:             costHandler,
		billingRulesHandler:     billingRulesHandler,
		pricingService:          pricingService,
		conversationHandler:     conversationHandler,
		edition:                 edition.Minimal,
		startTime:               time.Now(),
	}

	// Register minimal routes
	srv.setupMinimalRoutes(minimalConfigHandler, pluginRegistryAPI, minimalAuth)

	return srv
}

// resolveDataDir finds the data directory for config files.
func resolveDataDir() string {
	// Check environment variable first
	if envDir := os.Getenv("CENTAG_DATA_DIR"); envDir != "" {
		return envDir
	}

	// Try relative to executable
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)

	// Try common locations relative to executable directory
	candidates := []string{
		filepath.Join(execDir, "data"),
		filepath.Join(execDir, "..", "data"),
		filepath.Join(execDir, "..", "bin", "server", "data"),
		"./data",
		"../data",
		"./bin/server/data",
		"../bin/server/data",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			absDir, err := filepath.Abs(dir)
			if err == nil {
				return absDir
			}
			return dir
		}
	}
	return ""
}
