//go:build !minimal

package entrypoint

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"centag/core/internal"
	"centag/core/internal/auth"
	"centag/core/internal/billing"
	"centag/core/pkg/backend"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/configsync"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/metrics"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/proxymode"
	"centag/core/pkg/server"
)

// Run starts the Centag server with the given version info.
// This is the full version with database support (team/personal editions).
func Run(version, buildTime string) {
	Version = version
	BuildTime = buildTime

	if HandleVersionCommand(version, buildTime, os.Args) {
		return
	}
	if HandleHelpCommand(os.Args) {
		return
	}
	if HandleWrapCommand(os.Args) {
		return
	}
	if HandleCleanupCommand(os.Args) {
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		fmt.Fprintf(os.Stderr, "plugin subcommand not available in this distribution\n")
		os.Exit(1)
	}

	flag.String("config", "", "Deprecated: config file path (ignored)")
	flag.Parse()

	// Step 1: Bootstrap config from env vars
	boot := config.LoadBootstrap()

	// Step 2: Initialize logger
	loggerCfg := logger.Config{
		Level:  boot.Log.Level,
		Format: boot.Log.Format,
		Output: boot.Log.Output,
		Path:   boot.Log.File.Path,
		File: logger.FileConfig{
			Filename:   boot.Log.File.Filename,
			MaxSize:    boot.Log.File.MaxSize,
			MaxBackups: boot.Log.File.MaxBackups,
			MaxAge:     boot.Log.File.MaxAge,
			Compress:   boot.Log.File.Compress,
		},
	}
	if err := logger.Init(loggerCfg); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	// Step 3: Set build info
	internal.SetBuildInfo(Version, BuildTime)
	metrics.Init()

	logger.Info("Starting Centag Service")
	logger.Infof("Version: %s, Build: %s", Version, BuildTime)
	logger.Infof("DB driver: %s, path: %s", boot.DB.Driver, boot.DB.Path)
	logger.Infof("Product edition: %s", boot.Server.Edition)
	logger.Infof("Registered database plugins: %v", database.ListRegisteredPlugins())

	// Step 4: Open database & run migrations
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer dbCancel()

	dbConfig := map[string]interface{}{
		"path": boot.DB.Path,
		"dsn":  boot.DB.DSN,
	}
	if err := database.Init(dbCtx, boot.DB.Driver, dbConfig); err != nil {
		logger.Fatalf("Failed to initialise database: %v", err)
		os.Exit(1)
	}
	logger.Infof("数据库插件 %q 已就绪", boot.DB.Driver)
	defer func() {
		if err := database.Get().Close(); err != nil {
			logger.Errorf("Database close error: %v", err)
		}
	}()

	// Step 5: API Key 二次查看（默认可揭示；LLM_PROXY_API_KEY_REVEAL_ONCE=true 时关闭）
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := auth.EnsureAPIKeyStorage(storageCtx); err != nil {
		storageCancel()
		logger.Fatalf("API key storage init failed: %v", err)
		os.Exit(1)
	}
	storageCancel()
	if auth.APIKeyRevealOnce() {
		logger.Info("API Key 二次查看已关闭（LLM_PROXY_API_KEY_REVEAL_ONCE）：完整密钥仅在创建响应中返回一次")
	} else {
		logger.Info("API Key 二次查看已启用：完整密钥加密落库，可在 Web 再次查看/复制")
	}

	// Step 6: First-run seed
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer seedCancel()

	if err := bootstrap.Seed(seedCtx); err != nil {
		logger.Fatalf("Bootstrap seed failed: %v", err)
		os.Exit(1)
	}

	// 管理密码不再从环境变量预置；首次启动请在 WebUI 首启向导中设置（仅落库，不打印）
	logger.Infof("Admin account ready — username: %q (set password via WebUI first-run setup; not preseeded from env)", bootstrap.AdminUsername())
	if bootstrap.DefaultAdminAPIKeyString() != "" {
		logger.Info("Default admin API Key is configured (value not logged; manage via Web / env)")
	}

	// Step 7: Load full runtime config from database
	cfgCtx, cfgCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cfgCancel()

	adminName := bootstrap.AdminUsername()
	var adminUserID int64
	adminUser, err := database.Get().UserStore().GetByUsername(cfgCtx, adminName)
	if err == nil {
		adminUserID = adminUser.ID
	}

	cfg, err := config.LoadFromDB(cfgCtx, boot, adminUserID)
	if err != nil {
		logger.Fatalf("Failed to load runtime config from database: %v", err)
		os.Exit(1)
	}
	logger.Infof("Runtime config loaded – server will listen on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Step 6.5: Initialize proxy mode manager
	modeManager := proxymode.NewManager()
	logger.Infof("Proxy mode manager initialized with %d default modes", len(modeManager.ListModes()))

	// Step 6.6: Load initial backends
	loadInitialBackends()

	// Step 7: Start the HTTP server
	srv := server.New(cfg)

	// Step 7b: Start configsync scheduler (if enabled).
	startConfigsync(srv)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Step 8: Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	go func() {
		<-quit
		logger.Info("Force exiting...")
		os.Exit(1)
	}()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()

	if err := srv.Stop(shutCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}

func loadInitialBackends() {
	manager := backend.GetManager()
	manager.SetStore(backend.NewDBBackendStore())
	if err := manager.Load(); err != nil {
		logger.Warnf("后端管理器 Load 失败: %v", err)
	}
	if len(manager.List()) > 0 {
		logger.Infof("已从数据库加载 %d 个后端配置", len(manager.List()))
		return
	}

	logger.Info("数据库中无后端记录，从初始配置补种...")
	cfgs := bootstrap.LoadInitialBackendsFromJSON()
	if len(cfgs) == 0 {
		return
	}

	now := time.Now().Format(time.RFC3339)
	added := 0
	for i := range cfgs {
		c := &cfgs[i]
		if c.CreatedAt == "" {
			c.CreatedAt = now
		}
		if c.UpdatedAt == "" {
			c.UpdatedAt = now
		}
		b := backendConfigFromConfig(c)
		if err := manager.Add(b); err != nil {
			logger.Warnf("添加后端配置失败 [%s]: %v", b.ID, err)
			continue
		}
		added++
	}
	if added > 0 {
		_ = manager.Save()
		logger.Infof("初始化完成：已从文件添加 %d 个后端配置", added)
	}
}

func backendConfigFromConfig(c *config.BackendConfig) *backend.BackendConfig {
	sms := make([]backend.ModelMapping, len(c.SupportedModels))
	for i := range c.SupportedModels {
		sms[i] = backend.ModelMapping{
			RequestedModel:     c.SupportedModels[i].RequestedModel,
			ActualModel:        c.SupportedModels[i].ActualModel,
			CompatibilityScore: c.SupportedModels[i].CompatibilityScore,
			IsExact:            c.SupportedModels[i].IsExact,
		}
	}
	return &backend.BackendConfig{
		ID:              c.ID,
		Name:            c.Name,
		Type:            c.Type,
		BaseURL:         c.BaseURL,
		APIKey:          c.APIKey,
		Enabled:         c.Enabled,
		Timeout:         c.Timeout,
		MaxRetries:      c.MaxRetries,
		Description:     c.Description,
		ProbeModel:      c.ProbeModel,
		Metadata:        c.Metadata,
		SupportedModels: sms,
		Capabilities: backend.ModelCapabilities{
			MaxContextTokens: c.Capabilities.MaxContextTokens,
			Features:         c.Capabilities.Features,
			SupportsImages:   c.Capabilities.SupportsImages,
			SupportsTools:    c.Capabilities.SupportsTools,
		},
		AutoFetchModels: c.AutoFetchModels,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		Weight:          c.Weight,
		Priority:        c.Priority,
	}
}

// startConfigsync optionally starts the configsync scheduler.
// It checks CENTAG_CONFIGSYNC env (default on) and starts a feishu provider
// based on available credentials. Both personal and team editions use feishu.
func startConfigsync(srv *server.Server) {
	if os.Getenv("CENTAG_CONFIGSYNC") == "off" {
		return
	}

	provider := configsync.MustNewFeishuProvider()
	if provider == nil {
		logger.Infof("Configsync not configured (no feishu credentials)")
		return
	}

	stateDir := os.Getenv("CENTAG_CONFIGSYNC_STATE_DIR")
	if stateDir == "" {
		if dataDir := os.Getenv("CENTAG_DATA_DIR"); dataDir != "" {
			stateDir = dataDir
		}
	}

	mapper := buildBackendMapper()
	priceStore := buildPriceStore()
	clashRulesApplier := configsync.NewClashRulesApplier()
	srv.SetClashRulesApplier(clashRulesApplier)

	// 创建流水线模板 applier
	pipelineTemplateStore := buildPipelineTemplateStore()
	pipelineTemplateApplier := configsync.NewPipelineTemplateApplier(pipelineTemplateStore)

	// Create database config store for persisting config data
	dbConfigStore, err := configsync.NewDBConfigStore()
	if err != nil {
		logger.Warnf("failed to create database config store: %v", err)
	}
	dbConfigApplier := configsync.NewDBConfigApplier(dbConfigStore)

	// Create generic applier for config data
	genericApplier := configsync.NewGenericApplier()

	// Create backend applier for backend configs
	backendStore := &backendManagerStore{manager: backend.GetManager()}
	backendApplier := configsync.NewBackendApplier(backendStore)

	// Load config from database if available (skip Feishu sync on subsequent startups)
	if dbConfigStore != nil {
		count, _ := dbConfigStore.Count()
		if count > 0 {
			logger.Infof("configsync: loading %d config entries from database", count)
			dbConfigApplier.LoadFromDB(genericApplier)
		}
	}

	scheduler := configsync.NewScheduler(configsync.SchedulerConfig{
		Provider: provider,
		StateDir: stateDir,
		RunOnce:  true, // only sync once on startup; manual sync via API button
		OnUpdate: func(snap *configsync.Snapshot) {
			if len(snap.Prices) > 0 && mapper != nil && priceStore != nil {
				result, err := configsync.ApplyPrices(
					context.Background(), snap.Prices, mapper, priceStore, true,
				)
				if err != nil {
					logger.Warnf("configsync: apply prices failed: %v", err)
				} else {
					logger.Infof("configsync: prices applied=%d skipped=%d", result.Applied, result.Skipped)
				}
			}
			// Apply clash rules from config and persist to DB
			clashRulesApplier.Apply(snap.Config)
			if rules := clashRulesApplier.GetRules(); rules != "" {
				logger.Infof("configsync: clash rules applied (remote)")
				// Persist clash rules to config_store
				if dbConfigStore != nil {
					if err := dbConfigStore.Upsert("clash.rules", rules); err != nil {
						logger.Warnf("configsync: persist clash rules failed: %v", err)
					}
				}
			}
			// Apply config data to generic applier and save to database
			genericApplier.Apply(snap.Config)
			if dbConfigApplier != nil && len(snap.Config) > 0 {
				dbConfigApplier.ApplyFromRows(snap.Config)
				logger.Infof("configsync: config data saved to database")
			}
			// Apply backend configs
			if len(snap.Config) > 0 {
				backendApplier.Apply(snap.Config)
				logger.Infof("configsync: backend configs applied")
			}
			// Apply pipeline templates from remote
			if len(snap.PipelineTemplates) > 0 {
				pipelineTemplateApplier.ApplyFromTemplates(snap.PipelineTemplates)
				logger.Infof("configsync: pipeline templates applied=%d", len(snap.PipelineTemplates))
				// Reload templates in the server to use the newly synced ones
				srv.ReloadPipelineTemplates(os.Getenv("CENTAG_EDITION"))
			}
			srv.InvalidatePricingCache()
		},
	})
	srv.SetConfigSyncScheduler(scheduler)
	configsync.SetGlobalScheduler(scheduler)
	scheduler.Start(context.Background())
	logger.Infof("Configsync scheduler started (feishu, config_table=%s, price_table=%s, pipeline_table=%s)",
		provider.ConfigTableID(), provider.PriceTableID(), provider.PipelineTableID())
}

// buildBackendMapper returns a BackendMapper that resolves base_url → []backend_id
// by matching against the global backend manager's loaded configs.
func buildBackendMapper() configsync.BackendMapper {
	return func(baseURL string) []string {
		manager := backend.GetManager()
		normalized := configsync.NormalizeBaseURL(baseURL)
		var ids []string
		for _, cfg := range manager.List() {
			if configsync.NormalizeBaseURL(cfg.BaseURL) == normalized {
				ids = append(ids, cfg.ID)
			}
		}
		return ids
	}
}

// buildPriceStore returns a PriceStore backed by the SQL database.
// Returns nil if the database is not available.
func buildPriceStore() configsync.PriceStore {
	db := database.Get().GetDB()
	if db == nil {
		return nil
	}
	return billing.NewSQLRuleStore(db, database.Get().DriverName())
}

// buildPipelineTemplateStore returns a PipelineTemplateStore backed by the database.
func buildPipelineTemplateStore() configsync.PipelineTemplateStore {
	store, err := pipeline.NewDBPipelineTemplateStore()
	if err != nil {
		logger.Warnf("failed to create database pipeline template store, falling back to in-memory: %v", err)
		return &inMemoryPipelineTemplateStore{
			templates: make(map[string]configsync.PipelineTemplate),
		}
	}
	return store
}

// inMemoryPipelineTemplateStore is a simple in-memory implementation of PipelineTemplateStore.
type inMemoryPipelineTemplateStore struct {
	templates map[string]configsync.PipelineTemplate
}

func (s *inMemoryPipelineTemplateStore) Upsert(tmpl configsync.PipelineTemplate) error {
	s.templates[tmpl.ID] = tmpl
	return nil
}

// backendManagerStore adapts backend.Manager to configsync.BackendStore interface.
type backendManagerStore struct {
	manager *backend.Manager
}

func (s *backendManagerStore) List() []configsync.BackendConfig {
	if s.manager == nil {
		return nil
	}
	var result []configsync.BackendConfig
	for _, cfg := range s.manager.List() {
		result = append(result, configsync.BackendConfig{
			ID:          cfg.ID,
			Name:        cfg.Name,
			Type:        cfg.Type,
			BaseURL:     cfg.BaseURL,
			Enabled:     cfg.Enabled,
			Timeout:     cfg.Timeout,
			MaxRetries:  cfg.MaxRetries,
			Description: cfg.Description,
		})
	}
	return result
}

func (s *backendManagerStore) Upsert(cfg configsync.BackendConfig) error {
	if s.manager == nil {
		return nil
	}
	// Check if backend exists
	existing, err := s.manager.Get(cfg.ID)
	if err != nil || existing == nil {
		// New backend - create with minimal config (api_key will be empty)
		newCfg := &backend.BackendConfig{
			ID:          cfg.ID,
			Name:        cfg.Name,
			Type:        cfg.Type,
			BaseURL:     cfg.BaseURL,
			Enabled:     cfg.Enabled,
			Timeout:     cfg.Timeout,
			MaxRetries:  cfg.MaxRetries,
			Description: cfg.Description,
		}
		return s.manager.Add(newCfg)
	}
	// Update existing backend - preserve api_key and other secrets
	existing.Name = cfg.Name
	existing.Type = cfg.Type
	existing.BaseURL = cfg.BaseURL
	existing.Enabled = cfg.Enabled
	existing.Timeout = cfg.Timeout
	existing.MaxRetries = cfg.MaxRetries
	existing.Description = cfg.Description
	return s.manager.Update(existing)
}
