// Package config defines the runtime configuration for centag.
//
// Configuration is loaded in two steps:
//  1. LoadBootstrap() reads startup settings (port, log, DB DSN) from
//     environment variables only – no files are ever read.
//  2. LoadFromDB() reads all remaining settings from the PostgreSQL database
//     and populates the global Config that components use via Get().
//
// The old config.yaml approach has been intentionally removed to avoid the
// inconsistency that arises from having two configuration sources.
package config

import (
	"sync"
)

// ── global state ──────────────────────────────────────────────────────────────

var (
	mu           sync.RWMutex
	globalConfig *Config
)

// BackendManager defines the interface for backend management.
// Used to break circular dependency between config and backend packages.
type BackendManager interface {
	List() []*BackendConfig
	Add(*BackendConfig) error
	Save() error
	Load() error
}

// backendManager is the global backend manager instance.
// Set by main.go during initialization.
var backendManager BackendManager

// SetBackendManager sets the global backend manager instance.
// This must be called before LoadInitialBackends().
func SetBackendManager(m BackendManager) {
	backendManager = m
}

// getBackendManager returns the global backend manager.
func getBackendManager() BackendManager {
	return backendManager
}

// GetBackendManager returns the global backend manager (exported for fallback policy store).
func GetBackendManager() BackendManager {
	return backendManager
}

// Set replaces the global runtime config.  Called by LoadFromDB.
func Set(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	globalConfig = cfg
}

// Get returns the current global runtime config.
// Returns nil before LoadFromDB has been called.
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig
}

// ── top-level structure ───────────────────────────────────────────────────────

// Config is the full runtime configuration.  All fields are JSON-serialisable
// so they can be stored in / loaded from the database as JSON blobs.
type Config struct {
	Server         ServerConfig         `json:"server"`
	Log            LogConfig            `json:"log"`
	Proxy          ProxyConfig          `json:"proxy"`
	Cache          CacheConfig          `json:"cache"`
	Redis          RedisConfig          `json:"redis"`
	Vector         VectorConfig         `json:"vector"`
	Embedding      EmbeddingConfig      `json:"embedding"`
	QASplit        QASplitConfig        `json:"qa_split"`
	QuestionSplit  QuestionSplitConfig  `json:"question_split"`
	Plugins        PluginsConfig        `json:"plugins"`
	PluginSecurity PluginSecurityConfig `json:"plugin_security"` // 插件安全配置
	SystemProxy    SystemProxyConfig    `json:"system_proxy"`
	HostProxy      HostProxyConfig      `json:"host_proxy"`
	Backends       []BackendConfig      `json:"backends"`
	Storages          []StorageConfig      `json:"storages"`
	DefaultStorage    string               `json:"default_storage"`
	DataStores        []DataStoreConfig    `json:"data_stores"`
	DefaultDataStores []string             `json:"default_data_stores"`
	ModelMatching  ModelMatchingConfig  `json:"model_matching"`
	CacheControl   CacheControlConfig   `json:"cache_control"`
	Scheduler      SchedulerConfig      `json:"scheduler"`
}

// ── sub-structures ────────────────────────────────────────────────────────────

// BackendHealthStatus 后端健康状态
type BackendHealthStatus struct {
	Status       string `json:"status,omitempty"`        // healthy, unhealthy, unknown, checking
	LastCheckAt  string `json:"last_check_at,omitempty"` // 最后检查时间
	LastError    string `json:"last_error,omitempty"`    // 最后错误信息
	ResponseTime int64  `json:"response_time,omitempty"` // 响应时间（毫秒）
	ModelsCount  int    `json:"models_count,omitempty"`  // 获取到的模型数量
}

// BackendConfig describes one LLM backend service.
// Note: Weight and Priority are scheduling parameters managed by scheduler/preset modules.
// Backend management module only handles basic configuration (name, type, URL, API key, etc.).
type BackendConfig struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Type            string            `json:"type"` // openai, ollama, anthropic
	BaseURL         string            `json:"base_url"`
	APIKey          string            `json:"api_key,omitempty"`
	Enabled         bool              `json:"enabled"`
	Timeout         int               `json:"timeout"`     // Request timeout in seconds
	MaxRetries      int               `json:"max_retries"` // Max retry attempts
	Description     string            `json:"description,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	SupportedModels []ModelMapping    `json:"supported_models,omitempty"` // Supported models list
	AutoFetchModels bool              `json:"auto_fetch_models"`          // Auto-fetch models from backend (注意：不要用 omitempty，否则 false 会被省略)
	ProbeModel      string            `json:"probe_model,omitempty"`      // 默认探测模型（用于连通性/可用性探测）
	Capabilities    ModelCapabilities `json:"capabilities,omitempty"`     // Model capabilities
	CreatedAt       string            `json:"created_at,omitempty"`       // Creation timestamp
	UpdatedAt       string            `json:"updated_at,omitempty"`       // Last update timestamp

	// 健康状态（由探测功能更新）
	HealthStatus *BackendHealthStatus `json:"health_status,omitempty"`

	// Scheduling parameters (managed by scheduler/preset modules, not shown in backend management UI)
	Weight   int `json:"weight,omitempty"`   // Load balancing weight / strictness
	Priority int `json:"priority,omitempty"` // Scheduling priority

	// 租户隔离：空=系统共享后端，非空=租户私有
	TenantID string `json:"tenant_id,omitempty"`
}

// DataStoreConfig describes a data store configuration for agent knowledge/vector/kv access.
type DataStoreConfig struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // kv, vector, knowledge
	StorageName string                 `json:"storage_name"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// StorageConfig describes one external storage plugin instance.
type StorageConfig struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // postgresql, redis, elasticsearch, chroma
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Description string                 `json:"description"`
}

// ModelMapping maps a requested model name to an actual model the backend supports.
type ModelMapping struct {
	RequestedModel     string  `json:"requested_model"`
	ActualModel        string  `json:"actual_model"`
	CompatibilityScore float64 `json:"compatibility_score,omitempty"`
	IsExact            bool    `json:"is_exact,omitempty"`
}

// ModelCapabilities describes what a backend model can do.
type ModelCapabilities struct {
	MaxContextTokens int      `json:"max_context_tokens,omitempty"`
	Features         []string `json:"features,omitempty"`
	SupportsImages   bool     `json:"supports_images,omitempty"`
	SupportsTools    bool     `json:"supports_tools,omitempty"`
}

// ServerConfig holds HTTP server settings (sourced from env vars via bootstrap).
type ServerConfig struct {
	Port        int    `json:"port"`
	Host        string `json:"host"`
	Mode        string `json:"mode"`         // debug | release
	ExternalURL string `json:"external_url"` // 对外暴露的访问地址，用于 UI 展示
	Edition     string `json:"edition"`      // personal | team
}

// LogConfig holds logging settings (sourced from env vars via bootstrap).
type LogConfig struct {
	Level  string        `json:"level"`
	Format string        `json:"format"` // json | console
	Output string        `json:"output"` // stdout | file
	File   FileLogConfig `json:"file"`
}

// FileLogConfig holds log-file rotation settings.
type FileLogConfig struct {
	Path       string `json:"path"`
	Filename   string `json:"filename"`
	MaxSize    int    `json:"max_size"`    // MB
	MaxBackups int    `json:"max_backups"` // count
	MaxAge     int    `json:"max_age"`     // days
	Compress   bool   `json:"compress"`
}

// ProxyConfig controls how the LLM proxy forwards requests.
type ProxyConfig struct {
	Timeout             int    `json:"timeout"` // seconds
	MaxRetries          int    `json:"max_retries"`
	RetryDelay          int    `json:"retry_delay"` // seconds
	Enabled             bool   `json:"enabled"`
	DefaultMode         string `json:"default_mode"` // transparent-proxy | direct-backend | smart-scheduling | …
	AllowHeaderOverride bool   `json:"allow_header_override"`
	DefaultBackendID    string `json:"default_backend_id"`
	DefaultModel        string `json:"default_model"`
	// FallbackBackendID 降级后端 ID：当 DefaultBackendID 不可用时自动使用
	FallbackBackendID string `json:"fallback_backend_id,omitempty"`
	// FallbackModel 降级模型：当 DefaultModel 不可用时自动使用
	FallbackModel string `json:"fallback_model,omitempty"`
	// 模式模板启用开关
	ModeATemplateEnabled bool `json:"mode_a_template_enabled"` // #a 审核模式模板
	ModeOTemplateEnabled bool `json:"mode_o_template_enabled"` // #o 优化模式模板
	ModeDTemplateEnabled bool `json:"mode_d_template_enabled"` // #d 直接后端模板
	ModeTTemplateEnabled bool `json:"mode_t_template_enabled"` // #t 透明代理模板
	ModeFTemplateEnabled bool `json:"mode_f_template_enabled"` // #f 降级模式模板
	ModeMTemplateEnabled bool `json:"mode_m_template_enabled"` // #m 模型匹配模板
	ModeCTemplateEnabled bool `json:"mode_c_template_enabled"` // #c 自定义/意图分类模板
	ModePTemplateEnabled bool `json:"mode_p_template_enabled"` // #p 流水线模板
	// AuditConfig 审核模式配置
	AuditConfig *AuditConfig `json:"audit_config,omitempty"`
	// OptimizeConfig 优化模式配置
	OptimizeConfig *OptimizeConfig `json:"optimize_config,omitempty"`
	// PipelineConfig 流水线模式配置
	PipelineConfig *PipelineConfig `json:"pipeline_config,omitempty"`
	// FallbackConfig 降级模式配置
	FallbackConfig *FallbackConfig `json:"fallback_config,omitempty"`
	// RetryableStatusCodes 哪些 HTTP 状态码应触发重试/降级（热生效）
	RetryableStatusCodes []int `json:"retryable_status_codes,omitempty"`
	// RetryableErrorCodes 提供方返回的错误码列表（热生效）
	RetryableErrorCodes []string `json:"retryable_error_codes,omitempty"`
	// TimeoutRetryable 超时是否触发重试/降级（热生效）
	TimeoutRetryable *bool `json:"timeout_retryable,omitempty"`
	// NetworkRetryable 网络错误是否触发重试/降级（热生效）
	NetworkRetryable *bool `json:"network_retryable,omitempty"`
	// CircuitBreaker 熔断器配置（热生效）
	CircuitBreaker *CircuitBreakerSettings `json:"circuit_breaker,omitempty"`
}

// CircuitBreakerSettings 熔断器可配置参数（热生效）。
type CircuitBreakerSettings struct {
	FailureThreshold int `json:"failure_threshold"` // 窗口内失败次数触发熔断（默认 3）
	SuccessThreshold int `json:"success_threshold"` // 半开状态恢复所需成功次数（默认 2）
	TimeoutSec       int `json:"timeout_sec"`        // 熔断持续秒数（默认 60）
	WindowSec        int `json:"window_sec"`         // 滑动窗口秒数（默认 60）
	RateLimitWeight  int `json:"rate_limit_weight"`  // 429 重试-after 权重（默认 2，即1次429计为2次失败）
}

// PipelineConfig 流水线模式配置
type PipelineConfig struct {
	DefaultPipeline string `json:"default_pipeline"` // 默认流水线ID
	ConfigDir       string `json:"config_dir"`       // 流水线配置文件目录
	// AllowUserOverride 是否允许用户级覆盖默认流水线
	AllowUserOverride bool `json:"allow_user_override"`
}

// DefaultSystemPipelineID 是各发行版 / 运行方式在初始化时的统一系统默认流水线（透明模式，不注入 system prompt）。
const DefaultSystemPipelineID = "transparent-proxy"

// EffectiveDefaultPipeline 返回无头请求应使用的默认流水线 ID。
// 优先级：pipeline_config.default_pipeline → proxy.default_mode → DefaultSystemPipelineID。
func (p ProxyConfig) EffectiveDefaultPipeline() string {
	if p.PipelineConfig != nil && p.PipelineConfig.DefaultPipeline != "" {
		return p.PipelineConfig.DefaultPipeline
	}
	if p.DefaultMode != "" {
		return p.DefaultMode
	}
	return DefaultSystemPipelineID
}

// AuditConfig 审核模式配置
type AuditConfig struct {
	ExecutorBackendID string `json:"executor_backend"`  // 执行后端 ID
	ExecutorModel     string `json:"executor_model"`    // 执行模型名称
	AuditorBackendID  string `json:"auditor_backend"`   // 审核后端 ID
	AuditorModel      string `json:"auditor_model"`     // 审核模型名称
	AuditPrompt       string `json:"audit_prompt"`      // 审核 Prompt 模板
	AutoRetry         bool   `json:"auto_retry"`        // 审核不通过自动重试
	MaxRetries        int    `json:"max_retries"`       // 最大重试次数
	BypassOnTimeout   bool   `json:"bypass_on_timeout"` // 审核超时是否绕过
	AuditTimeoutSec   int    `json:"audit_timeout_sec"` // 审核超时时间（秒）
}

// OptimizeConfig 优化模式配置
type OptimizeConfig struct {
	ExecutorBackendID  string `json:"executor_backend"`     // 执行后端 ID
	ExecutorModel      string `json:"executor_model"`       // 执行模型名称
	OptimizerBackend   string `json:"optimizer_backend"`    // 优化后端 ID
	OptimizerModel     string `json:"optimizer_model"`      // 优化模型名称
	OptimizePrompt     string `json:"optimize_prompt"`      // 优化 Prompt 模板
	AutoRetry          bool   `json:"auto_retry"`           // 优化失败自动重试
	MaxRetries         int    `json:"max_retries"`          // 最大重试次数
	BypassOnTimeout    bool   `json:"bypass_on_timeout"`    // 优化超时是否降级返回原始答案
	OptimizeTimeoutSec int    `json:"optimize_timeout_sec"` // 优化超时时间（秒）
}

// FallbackConfig 降级模式配置
type FallbackConfig struct {
	PrimaryBackendID   string `json:"primary_backend"`      // 主后端 ID
	PrimaryModel       string `json:"primary_model"`        // 主模型名称
	FallbackBackendID  string `json:"fallback_backend"`     // 降级后端 ID
	FallbackModel      string `json:"fallback_model"`       // 降级模型名称
	MaxRetries         int    `json:"max_retries"`          // 主后端失败重试次数
	BypassOnTimeout    bool   `json:"bypass_on_timeout"`    // 主后端超时是否降级
	FallbackTimeoutSec int    `json:"fallback_timeout_sec"` // 降级超时时间（秒）
}

// CacheControlConfig sets the per-request cache behaviour defaults.
//
// DefaultRead/DefaultWrite are kept for DB/API backward compatibility; effective
// read/write defaults come from Cache.EnableCacheRead / EnableCacheWrite (see proxy.DetectCacheControl).
type CacheControlConfig struct {
	Enabled        bool `json:"enabled"`
	DefaultRead    bool `json:"default_read"`
	DefaultWrite   bool `json:"default_write"`
	DefaultQASplit bool `json:"default_qa_split"`
}

// CacheConfig holds all caching settings.
type CacheConfig struct {
	Enabled          bool                `json:"enabled"`
	EnableCacheRead  bool                `json:"enable_cache_read"`  // 是否启用缓存命中流程，关闭后完全不走缓存命中，直接转发
	EnableCacheWrite bool                `json:"enable_cache_write"` // 是否启用缓存写入流程
	SaveOnlyMode     bool                `json:"save_only_mode"`     // 仅保存模式：不进行拆分和向量化，只保存问答数据用于浏览
	DefaultTTL       int                 `json:"default_ttl"`        // seconds
	MaxCacheSize     int                 `json:"max_cache_size"`
	Strategy         string              `json:"strategy"`         // exact | semantic | hybrid
	CleanupInterval  int                 `json:"cleanup_interval"` // seconds
	Semantic         SemanticCacheConfig `json:"semantic"`
}

// SemanticCacheConfig holds semantic-cache specific options.
type SemanticCacheConfig struct {
	Threshold           float32 `json:"threshold"`
	TopK                int     `json:"top_k"`
	DistanceType        string  `json:"distance_type"`
	EnableAutoEmbedding bool    `json:"enable_auto_embedding"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Enabled  bool   `json:"enabled"`
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

// VectorConfig selects and configures a vector database.
type VectorConfig struct {
	Enabled bool         `json:"enabled"`
	Type    string       `json:"type"` // milvus | chroma
	Milvus  MilvusConfig `json:"milvus"`
	Chroma  ChromaConfig `json:"chroma"`
}

// MilvusConfig holds Milvus connection settings.
type MilvusConfig struct {
	Addr       string `json:"addr"`
	Collection string `json:"collection"`
}

// ChromaConfig holds ChromaDB connection settings.
type ChromaConfig struct {
	Addr       string `json:"addr"`
	Collection string `json:"collection"`
	Token      string `json:"token,omitempty"`
}

// EmbeddingConfig describes the embedding (vectorisation) service.
type EmbeddingConfig struct {
	Provider  string `json:"provider"`   // ollama | openai | kimi
	BackendID string `json:"backend_id"` // resolved backend service ID
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"` // API key for cloud providers (kimi, openai)
	Timeout   int    `json:"timeout"`
	Enabled   bool   `json:"enabled"`
}

// GetDefaultEmbeddingConfig returns the default embedding config.
// Embedding is enabled by default using the local Ollama instance and a
// common embedding tag (bge-m3:latest), not a chat model.
//
// Override with LLM_PROXY_DEFAULT_EMBEDDING_MODEL.
func GetDefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		Provider:  "ollama",
		BackendID: "ollama-local",
		Model:     envStr("LLM_PROXY_DEFAULT_EMBEDDING_MODEL", "bge-m3:latest"),
		// 优先使用 OLLAMA_HOST，保持向后兼容（回退到 LLM_PROXY_INIT_BACKEND_URL）
		BaseURL: envStr("OLLAMA_HOST", envStr("LLM_PROXY_INIT_BACKEND_URL", "http://localhost:21434")),
		Timeout: 180,
		Enabled: true,
	}
}

// QuestionSplitConfig controls the question-splitting + partial-cache-hit feature.
// This is distinct from QASplitConfig (which splits LLM *output* into QA pairs for caching).
// QuestionSplitConfig splits the *user question* into sub-questions, checks the cache for each,
// calls the LLM only for cache-misses, then merges all answers before returning to the client.
type QuestionSplitConfig struct {
	Enabled             bool    `json:"enabled"`              // 总开关（默认 false）
	FastSplitEnabled    bool    `json:"fast_split_enabled"`   // 快速规则拆分（默认 true）
	LLMSplitEnabled     bool    `json:"llm_split_enabled"`    // 模型辅助拆分（默认 false）
	SplitStrategy       string  `json:"split_strategy"`       // rule | llm | hybrid
	BackendID           string  `json:"backend_id"`           // LLM 拆分后端 ID
	Model               string  `json:"model"`                // LLM 拆分模型
	SynthesisStrategy   string  `json:"synthesis_strategy"`   // concat | llm | template
	SynthesisBackendID  string  `json:"synthesis_backend_id"` // 合成后端 ID（仅 llm 策略）
	SynthesisModel      string  `json:"synthesis_model"`      // 合成模型（仅 llm 策略）
	MaxSubQuestions     int     `json:"max_sub_questions"`    // 最大子问题数量（默认 5）
	Timeout             int     `json:"timeout"`              // 超时秒数，超时后降级为全量 LLM（默认 30）
	ComplexityThreshold float32 `json:"complexity_threshold"` // 触发拆分的复杂度阈值（默认 0.5）
}

// GetDefaultQuestionSplitConfig returns sensible defaults for question-split feature.
// Feature is disabled by default to avoid unexpected latency for existing users.
func GetDefaultQuestionSplitConfig() QuestionSplitConfig {
	dm := envStr("LLM_PROXY_DEFAULT_MODEL", "qwen2.5:1.5b")
	return QuestionSplitConfig{
		Enabled:             false,
		FastSplitEnabled:    true,
		LLMSplitEnabled:     false,
		SplitStrategy:       "rule",
		BackendID:           "ollama-local",
		Model:               dm,
		SynthesisStrategy:   "concat",
		SynthesisBackendID:  "ollama-local",
		SynthesisModel:      dm,
		MaxSubQuestions:     5,
		Timeout:             30,
		ComplexityThreshold: 0.2, // 对短复合问题友好（0.5 过于严格）
	}
}

// QASplitConfig controls the question-answer splitting feature.
type QASplitConfig struct {
	Enabled     bool    `json:"enabled"`
	BackendID   string  `json:"backend_id"`
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Timeout     int     `json:"timeout"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// GetDefaultQASplitConfig returns the default QA-split configuration.
// QA-split is enabled by default on ollama-local; model follows LLM_PROXY_DEFAULT_MODEL.
func GetDefaultQASplitConfig() QASplitConfig {
	return QASplitConfig{
		Enabled:     true,
		BackendID:   "ollama-local",
		Model:       envStr("LLM_PROXY_DEFAULT_MODEL", "qwen2.5:1.5b"),
		Prompt:      defaultQASplitPrompt,
		Timeout:     120,
		Temperature: 0.3,
		MaxTokens:   8192,
	}
}

const defaultQASplitPrompt = `你是一个专业的问答拆分专家。请分析以下问答对，判断是否需要拆分。

如果需要拆分，请将问答对拆分为多个独立的、语义完整的问答对。
如果不需要拆分，返回原始问答对。

要求：
1. 拆分后的每个问答对应该是一个完整的问题和对应的答案
2. 问题应该具体、明确，不依赖上下文
3. 答案应该直接回答对应的问题
4. 不要过度拆分，保持问题的语义完整性

请以JSON格式返回，格式如下：
{
  "split": true/false,
  "qa_pairs": [
    {
      "question": "拆分后的问题1",
      "answer": "拆分后的答案1"
    },
    ...
  ]
}

原始问答对：
问题：{{question}}
答案：{{answer}}`

// PluginsConfig describes the plugin directory and enabled plugins.
type PluginsConfig struct {
	Dir     string   `json:"dir"`
	Enabled []string `json:"enabled"`
}

// SystemProxyConfig configures the MITM proxy.
type SystemProxyConfig struct {
	Enabled         bool     `json:"enabled"`
	ListenPort      int      `json:"listen_port"`
	ListenAddr      string   `json:"listen_addr"`      // host part only; default 127.0.0.1
	AdvertiseHost   string   `json:"advertise_host"`   // PAC PROXY host when LAN enabled
	AllowLANClients bool     `json:"allow_lan_clients"`
	PACEnabled      bool     `json:"pac_enabled"`
	CACertPath      string   `json:"ca_cert_path"`
	CAKeyPath       string   `json:"ca_key_path"`
	CertDir         string   `json:"cert_dir"`
	CertValidDays   int      `json:"cert_valid_days"`
	Domains         []string `json:"domains"`
	PathPatterns    []string `json:"path_patterns"`
	// EgressAPIKey is the Centag llmproxy_* key MITM injects when forwarding to :20060.
	// Agents keep their upstream tokens; they never need to know about this key.
	// Empty → resolve from LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY / LLM_PROXY_DEFAULT_ADMIN_API_KEY.
	EgressAPIKey string `json:"egress_api_key,omitempty"`
}

// GetDefaultSystemProxyConfig returns the default MITM proxy config.
func GetDefaultSystemProxyConfig() SystemProxyConfig {
	return SystemProxyConfig{
		Enabled:         false,
		ListenPort:      8081,
		ListenAddr:      "127.0.0.1",
		AdvertiseHost:   "",
		AllowLANClients: false,
		PACEnabled:      true,
		CACertPath:      "./certs/ca.crt",
		CAKeyPath:       "./certs/ca.key",
		CertDir:         "./certs/domains",
		CertValidDays:   90,
		// Domains/PathPatterns: see mitm_default_domains.go (catalog + global providers).
		Domains:      append([]string(nil), DefaultMITMDomains()...),
		PathPatterns: append([]string(nil), DefaultMITMPathPatterns()...),
	}
}

// HostProxyConfig configures the host-hijacking proxy.
type HostProxyConfig struct {
	Enabled       bool              `json:"enabled"`
	HTTPPort      int               `json:"http_port"`
	HTTPSPort     int               `json:"https_port"`
	BackendAddr   string            `json:"backend_addr"`
	CACertPath    string            `json:"ca_cert_path"`
	CAKeyPath     string            `json:"ca_key_path"`
	CertDir       string            `json:"cert_dir"`
	CertValidDays int               `json:"cert_valid_days"`
	DomainMapping map[string]string `json:"domain_mapping"`
	PathPatterns  []string          `json:"path_patterns"`
}

// GetDefaultHostProxyConfig returns the default host-hijacking proxy config.
func GetDefaultHostProxyConfig() HostProxyConfig {
	backend := "http://127.0.0.1:20060"
	return HostProxyConfig{
		Enabled:       false,
		HTTPPort:      8080,
		HTTPSPort:     8443,
		BackendAddr:   backend,
		CACertPath:    "./certs/ca.crt",
		CAKeyPath:     "./certs/ca.key",
		CertDir:       "./certs/domains",
		CertValidDays: 90,
		DomainMapping: DefaultHostProxyDomainMapping(backend),
		PathPatterns:  append([]string(nil), DefaultMITMPathPatterns()...),
	}
}

// ModelMatchingConfig controls how the proxy matches requested models to
// backend capabilities.
type ModelMatchingConfig struct {
	Strategy          string        `json:"strategy"`
	HybridWeights     HybridWeights `json:"hybrid_weights"`
	CapacityTolerance float64       `json:"capacity_tolerance"`
	DefaultStrictness int           `json:"default_strictness"`
}

// HybridWeights are the blending coefficients for the hybrid matching strategy.
type HybridWeights struct {
	NameSimilarity float64 `json:"name_similarity"`
	CapacityMatch  float64 `json:"capacity_match"`
	FamilyMatch    float64 `json:"family_match"`
}

// TaskStrategyConfig 配置单个任务类型的调度策略
type TaskStrategyConfig struct {
	TaskType           string `json:"task_type"`           // 任务类型标识
	Strategy           string `json:"strategy"`            // 调度策略: cost/quality/latency/balance
	RecommendedBackend string `json:"recommended_backend"` // 推荐后端 ID
	RecommendedModel   string `json:"recommended_model"`   // 推荐模型名称
	Priority           int    `json:"priority"`            // 优先级 (1-5)
}

// SchedulerConfig 智能调度配置
type SchedulerConfig struct {
	DefaultStrategy         string               `json:"default_strategy"`          // 默认策略
	EnableIntentRecognition bool                 `json:"enable_intent_recognition"` // 启用意图识别
	AnalyzerBackendID       string               `json:"analyzer_backend_id"`       // 意图分析后端 ID
	AnalyzerModel           string               `json:"analyzer_model"`            // 意图分析模型
	Weights                 map[string]int       `json:"weights"`                   // 权重配置
	TaskStrategies          []TaskStrategyConfig `json:"task_strategies"`           // 任务类型策略列表
}

// DefaultSchedulerConfig 返回默认的调度配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		DefaultStrategy:         "balance",
		EnableIntentRecognition: true,
		AnalyzerBackendID:       "",
		AnalyzerModel:           "",
		Weights: map[string]int{
			"price":       20,
			"performance": 20,
			"quality":     25,
			"latency":     15,
			"privacy":     10,
			"match":       10,
		},
	}
}

// Validate checks that the server port is in a valid range and log level is
// one of the known values.
func (c *Config) Validate() error { return nil }

// PluginSecurityConfig 插件安全配置，用于控制远程插件的加载和执行
type PluginSecurityConfig struct {
	// AllowlistEnabled 是否启用 allowlist 模式，默认 false（白名单模式）
	AllowlistEnabled bool `json:"allowlist_enabled"`
	// AllowedSources 允许的插件来源列表（域名或组织）
	AllowedSources []string `json:"allowed_sources"`
	// AllowedHosts 允许的主机列表（IP 或域名）
	AllowedHosts []string `json:"allowed_hosts"`
	// RequireSignature 是否要求签名验证，默认 false
	RequireSignature bool `json:"require_signature"`
	// RequireHashLock 是否要求哈希锁定，默认 false
	RequireHashLock bool `json:"require_hash_lock"`
	// TrustedPublicKeys 信任的 Ed25519 公钥列表（base64 编码），用于验证远程插件签名
	TrustedPublicKeys []string `json:"trusted_public_keys,omitempty"`
	// NetworkPolicy 网络策略配置
	NetworkPolicy PluginNetworkPolicy `json:"network_policy"`
	// AdmissionCheck 准入检查配置
	AdmissionCheck PluginAdmissionConfig `json:"admission_check"`
}

// PluginNetworkPolicy 插件网络策略配置
type PluginNetworkPolicy struct {
	// DefaultDeny 默认拒绝策略，默认 true（默认拒绝所有出站请求）
	DefaultDeny bool `json:"default_deny"`
	// AllowedEndpoints 允许的端点列表（域名或 CIDR）
	AllowedEndpoints []string `json:"allowed_endpoints"`
	// BlockedEndpoints 禁止的端点列表
	BlockedEndpoints []string `json:"blocked_endpoints"`
	// AllowedPorts 允许的端口列表
	AllowedPorts []int `json:"allowed_ports"`
	// BlockedPorts 禁止的端口列表
	BlockedPorts []int `json:"blocked_ports"`
}

// PluginAdmissionConfig 插件准入检查配置
type PluginAdmissionConfig struct {
	// Enabled 是否启用准入检查，默认 true
	Enabled bool `json:"enabled"`
	// CheckPermissions 检查权限最小化，默认 true
	CheckPermissions bool `json:"check_permissions"`
	// CheckTimeout 检查超时配置，默认 true
	CheckTimeout bool `json:"check_timeout"`
	// CheckErrorHandling 检查错误处理，默认 true
	CheckErrorHandling bool `json:"check_error_handling"`
	// CheckObservability 检查可观测性，默认 true
	CheckObservability bool `json:"check_observability"`
	// MaxTimeoutSeconds 最大超时秒数，默认 300
	MaxTimeoutSeconds int `json:"max_timeout_seconds"`
	// MinTimeoutSeconds 最小超时秒数，默认 5
	MinTimeoutSeconds int `json:"min_timeout_seconds"`
}
