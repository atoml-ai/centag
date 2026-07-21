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
	"centag/core/pkg/backend"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/metrics"
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

	logger.Infof("Admin login — username: %q, password: %q", bootstrap.AdminUsername(), bootstrap.AdminPassword())
	if apiKey := bootstrap.DefaultAdminAPIKeyString(); apiKey != "" {
		logger.Infof("Admin API Key: %s", apiKey)
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
