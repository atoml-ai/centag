package config

import (
	"context"
	"encoding/json"
	"errors"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// System-config DB keys.  All values are JSON strings.
const (
	KeyProxyConfig       = "proxy_config"
	KeyCacheConfig       = "cache_config"
	KeyRedisConfig       = "redis_config"
	KeyVectorConfig      = "vector_config"
	KeyEmbeddingConfig   = "embedding_config"
	KeyQASplitConfig     = "qa_split_config"
	KeyQuestionSplitConfig = "question_split_config"
	KeyPluginsConfig     = "plugins_config"
	KeySystemProxyConfig = "system_proxy_config"
	KeyHostProxyConfig   = "host_proxy_config"
	KeyStorages          = "storages"
	KeyDefaultStorage    = "default_storage"
	KeyModelMatching     = "model_matching_config"
	KeyCacheControl      = "cache_control_config"
	// KeyPresetModeConfig stores the full PresetConfig for all modes, ensuring
	// model-matching analyzer config survives restarts.
	KeyPresetModeConfig = "preset_mode_config"
	// KeyBackends is used by the backend manager (admin-level, pre-user-isolation).
	KeyBackends = "admin_backends"
	// KeyJWTSecret is generated on first run and never exposed through the API.
	KeyJWTSecret = "jwt_secret"
	// KeySchedulerConfig stores the scheduler configuration including task strategies.
	KeySchedulerConfig = "scheduler_config"
	// KeyDataStores stores agent data store configurations.
	KeyDataStores = "data_stores"
	// KeyDefaultDataStores stores the default data store names.
	KeyDefaultDataStores = "default_data_stores"
)

// LoadFromDB constructs a runtime Config from values stored in the database,
// using bootstrap for the server/log settings (which live only in env vars).
//
// adminUserID is the ID of the admin user whose user_configs row supplies the
// backends, embedding, and qa_split fields.  Pass 0 to skip user-level config
// loading (useful during the seeder phase before a user exists).
func LoadFromDB(ctx context.Context, bootstrap *BootstrapConfig, adminUserID int64) (*Config, error) {
	db := database.Get()
	scs := db.SystemConfigStore()

	cfg := &Config{}

	// Server and Log come from bootstrap (env vars), not the DB.
	cfg.Server = bootstrap.Server
	cfg.Log = bootstrap.Log

	// ── system-level config ──────────────────────────────────────────────────
	cfg.Proxy = dbLoadOrDefault(ctx, scs, KeyProxyConfig, DefaultProxyConfig())
	cfg.Cache = dbLoadOrDefault(ctx, scs, KeyCacheConfig, DefaultCacheConfig())
	for _, w := range NormalizeCacheConfig(&cfg.Cache) {
		logger.Warnf("cache config: %s", w)
	}
	cfg.Redis = dbLoadOrDefault(ctx, scs, KeyRedisConfig, DefaultRedisConfig())
	cfg.Vector = dbLoadOrDefault(ctx, scs, KeyVectorConfig, DefaultVectorConfig())
	cfg.Plugins = dbLoadOrDefault(ctx, scs, KeyPluginsConfig, DefaultPluginsConfig())
	cfg.SystemProxy = dbLoadOrDefault(ctx, scs, KeySystemProxyConfig, GetDefaultSystemProxyConfig())
	cfg.HostProxy = dbLoadOrDefault(ctx, scs, KeyHostProxyConfig, GetDefaultHostProxyConfig())
	normalizeProxyPathFields(cfg)
	cfg.ModelMatching = dbLoadOrDefault(ctx, scs, KeyModelMatching, DefaultModelMatchingConfig())
	cfg.CacheControl = dbLoadOrDefault(ctx, scs, KeyCacheControl, DefaultCacheControlConfig())
	cfg.QuestionSplit = dbLoadOrDefault(ctx, scs, KeyQuestionSplitConfig, GetDefaultQuestionSplitConfig())
	cfg.Scheduler = dbLoadOrDefault(ctx, scs, KeySchedulerConfig, DefaultSchedulerConfig())

	// 部署级配置（fnOS 等安装包）：从数据目录的 centag.conf 读取，不写入 DB。
	cfg.Deployment = LoadDeploymentConfig()

	// ── data stores ───────────────────────────────────────────────────────────
	if raw, err := scs.Get(ctx, KeyDataStores); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg.DataStores)
	}
	if raw, err := scs.Get(ctx, KeyDefaultDataStores); err == nil && raw != "" {
		var names []string
		if jsonErr := json.Unmarshal([]byte(raw), &names); jsonErr == nil {
			cfg.DefaultDataStores = names
		}
	}

	// Storages list
	if raw, err := scs.Get(ctx, KeyStorages); err == nil {
		logger.Infof("从数据库加载存储配置: %s", raw)
		_ = json.Unmarshal([]byte(raw), &cfg.Storages)
	} else {
		logger.Warnf("从数据库加载存储配置失败: %v，将使用默认配置", err)
	}
	if cfg.Storages == nil {
		logger.Info("存储配置为空，使用默认配置")
		cfg.Storages = DefaultStorages()
	} else {
		logger.Infof("成功加载 %d 个存储配置", len(cfg.Storages))
	}

	// Default storage name – value was stored via json.Marshal(string), so
	// it arrives as a JSON-encoded string (e.g. `"postgresql-main"` with quotes).
	// We must json.Unmarshal it back to a plain Go string, exactly like
	// dbLoadOrDefault does for every other config key.
	if ds, err := scs.Get(ctx, KeyDefaultStorage); err == nil && ds != "" {
		var s string
		if jsonErr := json.Unmarshal([]byte(ds), &s); jsonErr == nil {
			cfg.DefaultStorage = s
		} else {
			// Fallback for any legacy rows that were stored without quotes.
			cfg.DefaultStorage = ds
		}
	}

	// ── backends（system_config admin_backends，与 bootstrap.Seed 写入一致）────────
	// 必须始终从 DB 加载，不能依赖 adminUserID；否则管理员用户未匹配时 cfg.Backends 为空，
	// 后续 loadInitialBackends 会误用错误解析的 JSON 覆盖 seed 中已解析的 api_key / 占位符。
	if raw, err := scs.Get(ctx, KeyBackends); err == nil && raw != "" {
		parsed, perr := ParseAdminBackendsJSON(raw)
		if perr != nil {
			logger.Warnf("解析 admin_backends 失败: %v", perr)
		} else {
			cfg.Backends = parsed
		}
	}

	// ── admin user-level config ───────────────────────────────────────────────
	if adminUserID > 0 {
		// Embedding 和 QASplit：优先从 system_config 读取（SaveConfig 的写入目标），
		// 找不到时再回退到 user_configs（初始 seeder 数据），保证 API 修改重启后不丢失。
		if raw, err := scs.Get(ctx, KeyEmbeddingConfig); err == nil && raw != "" {
			_ = json.Unmarshal([]byte(raw), &cfg.Embedding)
		} else {
			if userCfg, _ := db.UserConfigStore().Get(ctx, adminUserID); userCfg != nil {
				_ = json.Unmarshal([]byte(userCfg.Embedding), &cfg.Embedding)
			}
		}

		if raw, err := scs.Get(ctx, KeyQASplitConfig); err == nil && raw != "" {
			_ = json.Unmarshal([]byte(raw), &cfg.QASplit)
		} else {
			if userCfg, _ := db.UserConfigStore().Get(ctx, adminUserID); userCfg != nil {
				_ = json.Unmarshal([]byte(userCfg.QASplit), &cfg.QASplit)
			}
		}
	}

	// ── apply defaults for any still-zero fields ─────────────────────────────
	if cfg.Embedding.Provider == "" {
		cfg.Embedding = GetDefaultEmbeddingConfig()
	} else if cfg.Embedding.Provider == "ollama" && cfg.Embedding.BackendID == "" {
		// 兼容旧数据库：ollama provider 没有存 backend_id 的情况
		cfg.Embedding.BackendID = "ollama-local"
	}
	if cfg.QASplit.Prompt == "" {
		cfg.QASplit = GetDefaultQASplitConfig()
	}

	globalConfig = cfg
	return cfg, nil
}

// SaveSystemConfigToDB persists one system-config value as JSON.
// It is used by handlers that update system-level settings.
// If database is not initialized, it returns nil (no-op) to allow tests to run without DB.
func SaveSystemConfigToDB(ctx context.Context, key string, value interface{}) error {
	// Skip if database is not initialized (e.g., in tests)
	if !database.IsInitialized() {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return database.Get().SystemConfigStore().Set(ctx, key, string(data))
}

// LoadSystemConfigFromDB reads and JSON-decodes a system-config value.
// Returns database.ErrNotFound (unwrapped) when the key doesn't exist.
func LoadSystemConfigFromDB(ctx context.Context, key string, dest interface{}) error {
	raw, err := database.Get().SystemConfigStore().Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), dest)
}

// SaveConfig persists the current runtime Config back to the database so that
// the next LoadFromDB will reproduce the same state.  Server and Log fields
// are intentionally skipped (they live in env vars).
func SaveConfig(cfg *Config) error {
	ctx := context.Background()

	// 关键修复：在保存前，从数据库重新加载最新的 Storages 和 Backends，
	// 避免其他模块（preset/backend/cache等）调用 SaveConfig 时用旧值覆盖
	db := database.Get()
	if db == nil {
		// 数据库未初始化（例如在测试环境中），跳过持久化
		logger.Debug("SaveConfig: database not initialized, skipping persistence")
		return nil
	}
	scs := db.SystemConfigStore()
	if raw, err := scs.Get(ctx, KeyStorages); err == nil && raw != "" {
		var dbStorages []StorageConfig
		if err := json.Unmarshal([]byte(raw), &dbStorages); err == nil {
			// 只有当传入的cfg.Storages为空时，才使用数据库中的值
			if len(cfg.Storages) == 0 {
				cfg.Storages = dbStorages
				logger.Info("SaveConfig: 使用数据库中的存储配置（传入值为空）")
			}
		}
	}

	type kv struct {
		key string
		val interface{}
	}
	_ = NormalizeCacheConfig(&cfg.Cache)

	pairs := []kv{
		{KeyProxyConfig, cfg.Proxy},
		{KeyCacheConfig, cfg.Cache},
		{KeyRedisConfig, cfg.Redis},
		{KeyVectorConfig, cfg.Vector},
		{KeyEmbeddingConfig, cfg.Embedding},
		{KeyQASplitConfig, cfg.QASplit},
		{KeyPluginsConfig, cfg.Plugins},
		{KeySystemProxyConfig, cfg.SystemProxy},
		{KeyHostProxyConfig, cfg.HostProxy},
		{KeyModelMatching, cfg.ModelMatching},
		{KeyCacheControl, cfg.CacheControl},
		{KeyQuestionSplitConfig, cfg.QuestionSplit},
		{KeyDefaultStorage, cfg.DefaultStorage},
		{KeySchedulerConfig, cfg.Scheduler},
		{KeyDataStores, cfg.DataStores},
		{KeyDefaultDataStores, cfg.DefaultDataStores},
	}

	for _, p := range pairs {
		if err := SaveSystemConfigToDB(ctx, p.key, p.val); err != nil {
			return err
		}
	}

	if cfg.Storages != nil {
		if err := SaveSystemConfigToDB(ctx, KeyStorages, cfg.Storages); err != nil {
			return err
		}
	}
	if cfg.Backends != nil {
		if err := SaveSystemConfigToDB(ctx, KeyBackends, cfg.Backends); err != nil {
			return err
		}
	}

	// 更新全局配置缓存，确保后续读取返回最新值
	mu.Lock()
	if globalConfig != nil {
		globalConfig.DefaultStorage = cfg.DefaultStorage
		globalConfig.Storages = cfg.Storages
		globalConfig.Proxy = cfg.Proxy
		globalConfig.Cache = cfg.Cache
		globalConfig.Redis = cfg.Redis
		globalConfig.Vector = cfg.Vector
		globalConfig.Embedding = cfg.Embedding
		globalConfig.QASplit = cfg.QASplit
		globalConfig.Plugins = cfg.Plugins
		globalConfig.SystemProxy = cfg.SystemProxy
		globalConfig.HostProxy = cfg.HostProxy
		globalConfig.ModelMatching = cfg.ModelMatching
		globalConfig.CacheControl = cfg.CacheControl
		globalConfig.QuestionSplit = cfg.QuestionSplit
		globalConfig.Backends = cfg.Backends
		globalConfig.DataStores = cfg.DataStores
		globalConfig.DefaultDataStores = cfg.DefaultDataStores
	}
	mu.Unlock()

	return nil
}

// SaveBackendsToDB persists only admin_backends. Use this for backend CRUD instead
// of SaveConfig to avoid rewriting every system_config row (slow on SQLite + Docker volumes).
func SaveBackendsToDB(backends []BackendConfig) error {
	ctx := context.Background()
	if err := SaveSystemConfigToDB(ctx, KeyBackends, backends); err != nil {
		return err
	}
	mu.Lock()
	if globalConfig != nil {
		globalConfig.Backends = backends
	}
	mu.Unlock()
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// dbLoadOrDefault fetches a JSON value from the system_config store, decodes
// it into T, and returns def on any error.
func dbLoadOrDefault[T any](ctx context.Context, scs database.SystemConfigStore, key string, def T) T {
	raw, err := scs.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			// log but don't fail – use default
			_ = err
		}
		return def
	}
	var result T
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return def
	}
	return result
}

func normalizeProxyPathFields(cfg *Config) {
	if cfg == nil {
		return
	}

	cfg.SystemProxy.CACertPath = resolvePathRelativeToExecutable(cfg.SystemProxy.CACertPath)
	cfg.SystemProxy.CAKeyPath = resolvePathRelativeToExecutable(cfg.SystemProxy.CAKeyPath)
	cfg.SystemProxy.CertDir = resolvePathRelativeToExecutable(cfg.SystemProxy.CertDir)

	cfg.HostProxy.CACertPath = resolvePathRelativeToExecutable(cfg.HostProxy.CACertPath)
	cfg.HostProxy.CAKeyPath = resolvePathRelativeToExecutable(cfg.HostProxy.CAKeyPath)
	cfg.HostProxy.CertDir = resolvePathRelativeToExecutable(cfg.HostProxy.CertDir)
}
