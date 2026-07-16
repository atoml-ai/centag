package database

import "time"

// UserRole defines user permission levels
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleNormal UserRole = "normal"
)

// User represents a system user
type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Password    string    `json:"-"` // never serialized
	Role        UserRole  `json:"role"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Enabled     bool      `json:"enabled"`
	TenantID    *string   `json:"tenant_id,omitempty"` // 一用户一租户，NULL = 单用户模式

	// v2.1: User quota fields
	DefaultPipelineID string     `json:"default_pipeline_id,omitempty"` // 默认流水线 ID
	DailyTokenLimit   int64      `json:"daily_token_limit"`            // 每日 Token 限额，0=不限
	MonthlyTokenLimit int64      `json:"monthly_token_limit"`          // 每月 Token 限额，0=不限
	DailyTokenUsed   int64      `json:"daily_token_used"`             // 当日已用 Token
	MonthlyTokenUsed int64      `json:"monthly_token_used"`           // 当月已用 Token
	QuotaResetDate   *time.Time `json:"quota_reset_date,omitempty"`   // 额度重置日期

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey represents an API access key
type APIKey struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	TenantID       *string    `json:"tenant_id,omitempty"` // 所属租户，NULL = 单用户模式
	Name           string     `json:"name"`
	KeyHash        string     `json:"-"` // SHA256 hash, never serialized
	KeySecretEnc   string     `json:"-"` // AES-GCM ciphertext (base64), optional; enables UI reveal
	KeyPrefix      string     `json:"key_prefix"`
	ExpiresAt      *time.Time `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	Enabled        bool       `json:"enabled"`
	BudgetUSD      float64    `json:"budget_usd"`      // 预算上限，0=无限制
	UsedUSD        float64    `json:"used_usd"`         // 已用费用
	RateLimitRPM   int        `json:"rate_limit_rpm"`   // 每分钟请求数上限，0=无限制
	RateLimitTPM   int        `json:"rate_limit_tpm"`   // 每分钟 Token 数上限，0=无限制
	ModelWhitelist string     `json:"model_whitelist"`  // JSON array 或 "*"
	CreatedAt      time.Time  `json:"created_at"`
}

// UserConfig holds per-user configuration, stored as JSON columns for flexibility
type UserConfig struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Backends      string    `json:"backends"`       // JSON []BackendConfig
	ProxySettings string    `json:"proxy_settings"` // JSON ProxyConfig
	CacheSettings string    `json:"cache_settings"` // JSON CacheConfig
	Embedding     string    `json:"embedding"`      // JSON EmbeddingConfig
	QASplit       string    `json:"qa_split"`       // JSON QASplitConfig
	PresetModes   string    `json:"preset_modes"`   // JSON []PresetMode
	Scheduling    string    `json:"scheduling"`     // JSON ModelMatchingConfig
	CacheControl  string    `json:"cache_control"`  // JSON CacheControlConfig
	AuthSettings  string    `json:"auth_settings"`  // JSON AuthSettings
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AuthSettings controls API authentication behavior for a user
type AuthSettings struct {
	// RequireAPIKey: when true, LLM proxy endpoints require a valid API key
	RequireAPIKey bool `json:"require_api_key"`
	// AllowNoAuth: when true, requests without auth headers are allowed through
	// (only meaningful when RequireAPIKey is false)
	AllowNoAuth bool `json:"allow_no_auth"`
}

// RefreshToken represents a stored refresh token record used for JWT renewal.
type RefreshToken struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	TokenHash string    `json:"-"` // SHA-256 of the raw token; never serialized
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

// ClashRule stores a single Clash subscription rule entry for a user.
// A user can have multiple rules, each with its own name, YAML content and
// subscribe token.  The subscribe token is embedded in a public URL so that
// Clash clients can pull the file without Bearer auth.
// When RuleContent is empty the system default rule file is served instead.
type ClashRule struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`            // human-readable label
	RuleContent    string    `json:"rule_content"`    // custom YAML, empty = use default
	SubscribeToken string    `json:"subscribe_token"` // random URL token, globally unique
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TeamQuota represents team-level token quota limits
type TeamQuota struct {
	ID               int64     `json:"id"`
	TenantID         string    `json:"tenant_id"`           // 租户 ID，单用户模式为空
	DailyTokenLimit   int64     `json:"daily_token_limit"`   // 团队每日 Token 总限额，0=不限
	MonthlyTokenLimit int64     `json:"monthly_token_limit"` // 团队每月 Token 总限额，0=不限
	DailyTokenUsed   int64     `json:"daily_token_used"`    // 团队当日已用
	MonthlyTokenUsed int64     `json:"monthly_token_used"`  // 团队当月已用
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UserRequestLog represents a single user request log entry
type UserRequestLog struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	TenantID     string    `json:"tenant_id"`      // 租户 ID，单用户模式为空
	RequestID    string    `json:"request_id"`
	Model        string    `json:"model"`
	Backend      string    `json:"backend"`
	Pipeline     string    `json:"pipeline"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	LatencyMs    int64     `json:"latency_ms"`
	StatusCode   int       `json:"status_code"`
	RequestBody  string    `json:"request_body"`  // 请求摘要（截断存储）
	ResponseBody string    `json:"response_body"` // 响应摘要（截断存储）
	CreatedAt    time.Time `json:"created_at"`
}

// SchedulerDecision represents a scheduler decision log entry
type SchedulerDecision struct {
	ID        int64     `json:"id"`
	RequestID string    `json:"request_id"`
	UserID    int64     `json:"user_id"`
	TenantID  string    `json:"tenant_id"`  // 租户 ID，单用户模式为空
	Model     string    `json:"model"`
	Backend   string    `json:"backend"`
	Strategy  string    `json:"strategy"`
	Score     float64   `json:"score"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
