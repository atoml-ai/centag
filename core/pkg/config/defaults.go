package config

import (
	"fmt"
	"strconv"
)

// This file collects all default configuration values as plain Go functions.
// They are used by:
//   - the first-run seeder (internal/bootstrap/seeder.go) to initialise the DB
//   - db_loader.go as fallbacks when a key is missing from the DB

// DefaultProxyConfig returns sensible proxy defaults.
// Routing fields can be overridden via environment variables
// (config/secrets/.env 由 scripts/generate-secrets.sh 生成时会写入「四 B」节)：
//
//	LLM_PROXY_DEFAULT_MODE              (default: transparent-proxy；各发行版首轮初始化统一透明模式)
//	LLM_PROXY_DEFAULT_BACKEND_ID        (default: ollama-local)
//	LLM_PROXY_DEFAULT_MODEL             (default: qwen2.5:1.5b，Ollama 常见对话模型)
//	LLM_PROXY_ALLOW_HEADER_OVERRIDE     (default: true，允许 X-Proxy-Mode 头覆盖流水线路由)
//
// 嵌入模型默认值见 GetDefaultEmbeddingConfig / LLM_PROXY_DEFAULT_EMBEDDING_MODEL。
func DefaultProxyConfig() ProxyConfig {
	defaultMode := envStr("LLM_PROXY_DEFAULT_MODE", DefaultSystemPipelineID)
	return ProxyConfig{
		Timeout:             30,
		MaxRetries:          3,
		RetryDelay:          1,
		Enabled:             true,
		DefaultMode:         defaultMode,
		AllowHeaderOverride: envBool("LLM_PROXY_ALLOW_HEADER_OVERRIDE", true),
		DefaultBackendID:    envStr("LLM_PROXY_DEFAULT_BACKEND_ID", "ollama-local"),
		DefaultModel:        envStr("LLM_PROXY_DEFAULT_MODEL", "qwen2.5:1.5b"),
		PipelineConfig: &PipelineConfig{
			DefaultPipeline:   defaultMode,
			AllowUserOverride: true,
		},
		// 模式模板启用开关 (通过环境变量 LLM_PROXY_MODE_A_TEMPLATE_ENABLED 等覆盖)
		ModeATemplateEnabled: envBool("LLM_PROXY_MODE_A_TEMPLATE_ENABLED", false),
		ModeOTemplateEnabled: envBool("LLM_PROXY_MODE_O_TEMPLATE_ENABLED", false),
		ModeDTemplateEnabled: envBool("LLM_PROXY_MODE_D_TEMPLATE_ENABLED", false),
		ModeTTemplateEnabled: envBool("LLM_PROXY_MODE_T_TEMPLATE_ENABLED", false),
		ModeFTemplateEnabled: envBool("LLM_PROXY_MODE_F_TEMPLATE_ENABLED", false),
		ModeMTemplateEnabled: envBool("LLM_PROXY_MODE_M_TEMPLATE_ENABLED", false),
		ModeCTemplateEnabled: envBool("LLM_PROXY_MODE_C_TEMPLATE_ENABLED", false),
		ModePTemplateEnabled: envBool("LLM_PROXY_MODE_P_TEMPLATE_ENABLED", false),
	}
}

// DefaultCacheConfig returns sensible cache defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:          true,
		EnableCacheRead:  true,  // 默认启用缓存命中流程
		EnableCacheWrite: true,  // 默认启用缓存写入
		SaveOnlyMode:     false, // 默认不启用仅保存模式
		DefaultTTL:       3600,
		MaxCacheSize:     10000,
		Strategy:         "semantic",
		CleanupInterval:  300,
		Semantic: SemanticCacheConfig{
			Threshold:           0.8,
			TopK:                5,
			DistanceType:        "cosine",
			EnableAutoEmbedding: true,
		},
	}
}

// DefaultPluginsConfig returns the default plugin configuration.
func DefaultPluginsConfig() PluginsConfig {
	return PluginsConfig{
		Dir: "./plugins",
		Enabled: []string{
			"protocol/openai",
			"backend/openai",
			"backend/ollama",
		},
	}
}

// DefaultModelMatchingConfig returns the default model-matching strategy.
func DefaultModelMatchingConfig() ModelMatchingConfig {
	return ModelMatchingConfig{
		Strategy: "hybrid",
		HybridWeights: HybridWeights{
			NameSimilarity: 0.6,
			CapacityMatch:  0.4,
		},
		CapacityTolerance: 0.2,
		DefaultStrictness: 80,
	}
}

// DefaultCacheControlConfig returns the default cache-control settings.
func DefaultCacheControlConfig() CacheControlConfig {
	return CacheControlConfig{
		Enabled:        true,
		DefaultRead:    true,
		DefaultWrite:   true,
		DefaultQASplit: true,
	}
}

// DefaultRedisConfig returns a disabled Redis configuration.
func DefaultRedisConfig() RedisConfig {
	hostIP := envStr("HOST_IP", "192.168.1.5")
	redisAddr := "redis.atoml.net:26379"
	if envStr("USE_HOST_IP", "false") == "true" {
		redisAddr = fmt.Sprintf("%s:26379", hostIP)
	}
	return RedisConfig{
		Enabled:  false,
		Addr:     redisAddr,
		Password: envStr("REDIS_PASSWORD", DefaultInitMiddlewarePassword),
		DB:       0,
		PoolSize: 10,
	}
}

// DefaultVectorConfig returns a disabled vector-store configuration.
func DefaultVectorConfig() VectorConfig {
	// 获取宿主机IP，用于容器内解析域名到宿主机
	hostIP := envStr("HOST_IP", "192.168.1.5")
	chromaAddr := envStr("CHROMADB_ADDR", "chromadb.atoml.net:20008")
	if envStr("USE_HOST_IP", "false") == "true" {
		if chromaAddr == "chromadb.atoml.net:20008" {
			chromaAddr = fmt.Sprintf("%s:20008", hostIP)
		}
	}

	return VectorConfig{
		Enabled: false,
		Type:    "chroma",
		Milvus: MilvusConfig{
			Addr:       "localhost:19530",
			Collection: "llm_cache",
		},
		Chroma: ChromaConfig{
			Addr:       chromaAddr,
			Collection: "llm_cache",
			Token:      envStr("CHROMADB_TOKEN", DefaultInitMiddlewarePassword),
		},
	}
}

// DefaultStorages 返回默认存储配置。
// PostgreSQL、Redis、Elasticsearch 和 ChromaDB 默认禁用，用户可通过 UI 显式启用。
func DefaultStorages() []StorageConfig {
	// 获取宿主机IP，用于容器内解析域名到宿主机
	hostIP := envStr("HOST_IP", "192.168.1.5")

	esAddr := envStr("ELASTICSEARCH_ADDR", "http://es.atoml.net:29200")
	if envStr("USE_HOST_IP", "false") == "true" {
		if esAddr == "http://es.atoml.net:29200" {
			esAddr = fmt.Sprintf("http://%s:29200", hostIP)
		}
	}

	redisAddr := envStr("REDIS_ADDR", "redis.atoml.net:26379")
	if envStr("USE_HOST_IP", "false") == "true" {
		if redisAddr == "redis.atoml.net:26379" {
			redisAddr = fmt.Sprintf("%s:26379", hostIP)
		}
	}

	chromaAddr := envStr("CHROMADB_ADDR", "chromadb.atoml.net:20008")
	if envStr("USE_HOST_IP", "false") == "true" {
		if chromaAddr == "chromadb.atoml.net:20008" {
			chromaAddr = fmt.Sprintf("%s:20008", hostIP)
		}
	}

	return []StorageConfig{
		{
			Name:    "postgresql",
			Type:    "postgresql",
			Enabled: false,
			Config: map[string]interface{}{
				"host":            envStr("PG_HOST", "localhost"),
				"port":            pgPort(),
				"user":            envStr("PG_USER", "postgres"),
				"password":        envStr("PG_PASSWORD", ""),
				"database":        envStr("PG_DATABASE", "centag"),
				"ssl_mode":        envStr("PG_SSL_MODE", "disable"),
				"max_conns":       5,
				"min_conns":       1,
				"kv_table":        "kv_cache",
				"vector_table":    "vector_cache",
				"vector_dimension": 1024,
				"index_type":      "hnsw",
			},
			Description: "PostgreSQL 存储（复用应用数据库，支持 KV + 向量）",
		},
		{
			Name:    "elasticsearch-main",
			Type:    "elasticsearch",
			Enabled: false,
			Config: map[string]interface{}{
				"addresses":        []string{esAddr},
				"username":         envStr("ELASTICSEARCH_USERNAME", "elastic"),
				"password":         envStr("ELASTICSEARCH_PASSWORD", DefaultInitMiddlewarePassword),
				"exact_index":      "cache_exact_index",
				"semantic_index":   "cache_semantic_index",
				"vector_dimension": 1024,
				"request_timeout":  30,
				"enable_tls":       false,
			},
			Description: "Elasticsearch 存储",
		},
		{
			Name:    "redis",
			Type:    "redis",
			Enabled: false,
			Config: map[string]interface{}{
				"addr":      redisAddr,
				"password":  envStr("REDIS_PASSWORD", DefaultInitMiddlewarePassword),
				"db":        0,
				"pool_size": 10,
			},
			Description: "Redis 存储",
		},
		{
			Name:    "chromadb-main",
			Type:    "chroma",
			Enabled: false,
			Config: map[string]interface{}{
				"addr":       chromaAddr,
				"collection": "llm_cache",
				"timeout":    30,
				"token":      envStr("CHROMADB_TOKEN", DefaultInitMiddlewarePassword),
			},
			Description: "ChromaDB 存储",
		},
	}
}

// pgPort 从环境变量读取 PostgreSQL 端口号
func pgPort() int {
	if p := envStr("PG_PORT", ""); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			return v
		}
	}
	return 5432
}

// DefaultBackends 返回空列表。
//
// 后端初始化数据已迁移至 config/initdata/initial-backends.json，由 cmd/centag/main.go 的
// loadInitialBackends() 在首次启动时加载。此函数仅保留以兼容旧引用，不再硬编码任何后端配置。
func DefaultBackends() []BackendConfig {
	return nil
}

// DefaultPluginSecurityConfig 返回默认的插件安全配置
func DefaultPluginSecurityConfig() PluginSecurityConfig {
	return PluginSecurityConfig{
		AllowlistEnabled: false,
		AllowedSources:  []string{},
		AllowedHosts:    []string{},
		RequireSignature: false,
		RequireHashLock:  false,
		NetworkPolicy: PluginNetworkPolicy{
			DefaultDeny:       true,
			AllowedEndpoints: []string{},
			BlockedEndpoints: []string{},
			AllowedPorts:     []int{},
			BlockedPorts:     []int{},
		},
		AdmissionCheck: PluginAdmissionConfig{
			Enabled:           true,
			CheckPermissions:  true,
			CheckTimeout:      true,
			CheckErrorHandling: true,
			CheckObservability: true,
			MaxTimeoutSeconds: 300,
			MinTimeoutSeconds: 5,
		},
	}
}
