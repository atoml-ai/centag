//go:build minimal

package entrypoint

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"centag/core/internal"
	"centag/core/pkg/backend"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/metrics"
	"centag/core/pkg/server"

	"gopkg.in/yaml.v3"
)

// Run starts the Centag minimal server with the given version info.
// This edition uses file-based configuration only — no database required.
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

	logger.Info("Starting Centag Minimal Service (file-based, no database)")
	logger.Infof("Version: %s, Build: %s", Version, BuildTime)
	logger.Infof("Product edition: minimal")

	// Minimal edition: default INITDATA_PATH to the edition-specific initdata
	// directory so that local/dev runs load the correct pipeline templates
	// (e.g. transparent-proxy using generator/openai instead of the global
	// transparent_forward template that requires a configured default backend).
	if os.Getenv("INITDATA_PATH") == "" {
		if root := bootstrap.ProjectRoot(); root != "" {
			profileInitdata := filepath.Join(root, "config", "profiles", "minimal", "initdata")
			if info, err := os.Stat(profileInitdata); err == nil && info.IsDir() {
				os.Setenv("INITDATA_PATH", profileInitdata)
				logger.Infof("Minimal edition default INITDATA_PATH set to: %s", profileInitdata)
			}
		}
	}

	// Step 4: Load config from files (no database)
	cfg := loadMinimalConfig(boot)
	dataDir := config.ResolveDataDir()
	if dataDir != "" {
		if loadedProxy, err := config.LoadProxyConfigFromFile(dataDir, cfg.Proxy); err == nil {
			cfg.Proxy = loadedProxy
			logger.Infof("Loaded proxy config from %s", filepath.Join(dataDir, "proxy-config.yaml"))
		} else {
			logger.Warnf("Failed to load proxy config: %v", err)
		}
	}
	logger.Infof("Minimal config loaded – server will listen on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Step 5: Load backends from YAML file
	loadMinimalBackends()

	// Step 5b: Load default pipeline config from file
	loadDefaultPipelineConfig(cfg)

	// Step 6: Start the HTTP server
	srv := server.NewMinimal(cfg)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Step 7: Wait for shutdown signal
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

// loadMinimalConfig builds a runtime Config from env vars and file-based defaults.
// No database is used — all settings come from environment or hardcoded defaults.
func loadMinimalConfig(boot *config.BootstrapConfig) *config.Config {
	cfg := &config.Config{
		Server: boot.Server,
		Log:    boot.Log,
		Proxy:  config.DefaultProxyConfig(),
		Cache:  config.DefaultCacheConfig(),
		Redis:  config.DefaultRedisConfig(),
		Vector: config.DefaultVectorConfig(),
		Embedding:   config.GetDefaultEmbeddingConfig(),
		QASplit:     config.GetDefaultQASplitConfig(),
		QuestionSplit: config.GetDefaultQuestionSplitConfig(),
		Plugins:     config.DefaultPluginsConfig(),
		PluginSecurity: config.DefaultPluginSecurityConfig(),
		SystemProxy: config.GetDefaultSystemProxyConfig(),
		HostProxy:   config.GetDefaultHostProxyConfig(),
		Storages:    config.DefaultStorages(),
		ModelMatching: config.DefaultModelMatchingConfig(),
		CacheControl:  config.DefaultCacheControlConfig(),
		Scheduler:     config.DefaultSchedulerConfig(),
	}

	// Force minimal edition
	cfg.Server.Edition = "minimal"

	// Set as global config so backend manager can access it
	config.Set(cfg)

	return cfg
}

// loadDefaultPipelineConfig loads the default pipeline setting from
// data/default-pipeline.yaml (or initdata/default-pipeline.yaml) and applies it
// to the runtime config. Missing files fall back to DefaultProxyConfig (transparent-proxy).
func loadDefaultPipelineConfig(cfg *config.Config) {
	candidates := make([]string, 0, 2)
	if dataDir := resolveDataDir(); dataDir != "" {
		candidates = append(candidates, filepath.Join(dataDir, "default-pipeline.yaml"))
	}
	if initdataRoot := bootstrap.InitdataRoot(); initdataRoot != "" {
		candidates = append(candidates, filepath.Join(initdataRoot, "default-pipeline.yaml"))
	}
	if len(candidates) == 0 {
		logger.Info("No data/initdata directory found, using built-in default pipeline")
		return
	}

	var data []byte
	var pipelineFile string
	for _, path := range candidates {
		b, err := os.ReadFile(path)
		if err == nil {
			data = b
			pipelineFile = path
			break
		}
		if !os.IsNotExist(err) {
			logger.Warnf("Failed to read default pipeline config %s: %v", path, err)
			return
		}
	}
	if data == nil {
		logger.Info("No default pipeline config file found, using built-in defaults (transparent-proxy)")
		return
	}

	var pipelineCfg struct {
		DefaultPipeline string `yaml:"default_pipeline" json:"default_pipeline"`
	}
	if err := yaml.Unmarshal(data, &pipelineCfg); err != nil {
		logger.Warnf("Failed to parse default pipeline config %s: %v", pipelineFile, err)
		return
	}

	if pipelineCfg.DefaultPipeline != "" {
		if cfg.Proxy.PipelineConfig == nil {
			cfg.Proxy.PipelineConfig = &config.PipelineConfig{}
		}
		cfg.Proxy.PipelineConfig.DefaultPipeline = pipelineCfg.DefaultPipeline
		cfg.Proxy.DefaultMode = pipelineCfg.DefaultPipeline
		logger.Infof("Default pipeline loaded from file %s: %s", pipelineFile, pipelineCfg.DefaultPipeline)
	} else {
		logger.Info("Default pipeline config file is empty, using built-in defaults")
	}
}

func loadMinimalBackends() {
	manager := backend.GetManager()

	// 持久化目标始终指向 data 目录，避免空配置或 initdata 只读场景下
	// BackendHandler.Save() 回退到数据库路径失败。
	persistFile := ensureMinimalBackendPersistFile()
	manager.SetStore(backend.NewFileBackendStore(persistFile))

	loaded := false
	if err := manager.LoadFromFile(persistFile); err != nil {
		logger.Warnf("Failed to load backends from %s: %v", persistFile, err)
	} else if len(manager.List()) > 0 {
		loaded = true
		logger.Infof("Loaded %d backends from data dir: %s", len(manager.List()), persistFile)
	}

	// fallback: 从 config/initdata 加载到内存（仍持久化回 data 目录）
	if !loaded {
		initdataRoot := bootstrap.InitdataRoot()
		if initdataRoot != "" {
			backendFile := filepath.Join(initdataRoot, "initial-backends.yaml")
			if err := manager.LoadFromFile(backendFile); err != nil {
				logger.Warnf("Failed to load backends from initdata: %v", err)
			} else if len(manager.List()) > 0 {
				loaded = true
				logger.Infof("Loaded %d backends from initdata: %s (persist to %s)", len(manager.List()), backendFile, persistFile)
			}
		}
	}

	if !loaded {
		logger.Infof("No backends found; file store ready at %s", persistFile)
	}
}

// ensureMinimalBackendPersistFile 确保 data 目录存在，并返回 initial-backends.yaml 路径。
func ensureMinimalBackendPersistFile() string {
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
	return filepath.Join(dataDir, "initial-backends.yaml")
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
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".centag", "lib", "personal", "data"),
			filepath.Join(home, ".centag", "lib", "minimal", "data"),
		)
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
