// Package hooks 定义钩子系统接口。
//
// 钩子系统允许插件在请求处理的关键节点注入逻辑，
// 如存储、计费、日志、监控等。
//
// 框架在以下时机触发钩子：
//   - 请求到达时（OnRequest）
//   - 响应生成后（OnResponse）
//   - 缓存命中时（OnCacheHit）
//   - Token 使用时（OnTokenUsed）
//   - 配额超限时（OnQuotaExceeded）
package hooks

import (
	"context"

	"centag/core/pkg/types"
)

// HookManager 钩子管理器接口。
//
// 框架提供默认实现，插件通过 RegisterHook 注册钩子。
type HookManager interface {
	// RegisterStorageHook 注册存储钩子。
	RegisterStorageHook(hook StorageHook)

	// RegisterBillingHook 注册计费钩子。
	RegisterBillingHook(hook BillingHook)

	// RegisterTokenHook 注册 Token 计量钩子。
	RegisterTokenHook(hook TokenHook)

	// RegisterLoggingHook 注册日志钩子。
	RegisterLoggingHook(hook LoggingHook)

	// TriggerRequestHooks 触发请求到达钩子。
	TriggerRequestHooks(ctx context.Context, req *types.UnifiedRequest) error

	// TriggerResponseHooks 触发响应生成钩子。
	TriggerResponseHooks(ctx context.Context, resp *types.UnifiedResponse) error

	// TriggerCacheHitHooks 触发缓存命中钩子。
	TriggerCacheHitHooks(ctx context.Context, key string, data []byte) error

	// TriggerTokenUsedHooks 触发 Token 使用钩子。
	TriggerTokenUsedHooks(ctx context.Context, usage *TokenUsage) error

	// TriggerQuotaExceededHooks 触发配额超限钩子。
	TriggerQuotaExceededHooks(ctx context.Context, userID int64) error
}

// StorageHook 存储钩子接口。
type StorageHook interface {
	// OnRequest 请求到达时触发（可做请求预处理/缓存查询）。
	OnRequest(ctx context.Context, req *types.UnifiedRequest) error

	// OnResponse 响应生成后触发（可做响应缓存/存储）。
	OnResponse(ctx context.Context, resp *types.UnifiedResponse) error

	// OnCacheHit 缓存命中时触发。
	OnCacheHit(ctx context.Context, key string, data []byte) error
}

// BillingHook 计费钩子接口。
type BillingHook interface {
	// OnUsage Token 使用时触发（记录用量/扣费）。
	OnUsage(ctx context.Context, usage *TokenUsage) error

	// OnQuotaExceeded 配额超限时触发（通知/限流）。
	OnQuotaExceeded(ctx context.Context, userID int64) error
}

// TokenHook Token 计量钩子接口。
type TokenHook interface {
	// OnTokenUsed Token 使用时触发（Prometheus 指标/日志）。
	OnTokenUsed(ctx context.Context, usage *TokenUsage) error
}

// LoggingHook 日志钩子接口。
type LoggingHook interface {
	// OnRequestLog 请求日志记录时触发。
	OnRequestLog(ctx context.Context, log *RequestLog) error
}

// TokenUsage Token 使用量。
type TokenUsage struct {
	// UserID 用户 ID
	UserID int64 `json:"user_id"`

	// TenantID 租户 ID（team；单用户可为空）
	TenantID string `json:"tenant_id,omitempty"`

	// RequestID 请求 ID
	RequestID string `json:"request_id,omitempty"`

	// SessionID 会话 ID（对话记录关联）
	SessionID string `json:"session_id,omitempty"`

	// Model 模型名称
	Model string `json:"model"`

	// Backend 后端名称
	Backend string `json:"backend"`

	// InputTokens 输入 token 数
	InputTokens int `json:"input_tokens"`

	// OutputTokens 输出 token 数
	OutputTokens int `json:"output_tokens"`

	// TotalTokens 总 token 数
	TotalTokens int `json:"total_tokens"`

	// CostUSD 估算成本（可选）
	CostUSD float64 `json:"cost_usd,omitempty"`

	// Success 是否成功完成
	Success bool `json:"success"`

	// DeptTag 部门标签（可选）
	DeptTag string `json:"dept_tag,omitempty"`

	// AgentType Agent 类型（可选）
	AgentType string `json:"agent_type,omitempty"`
}

// RequestLog 请求日志。
type RequestID string

type RequestLog struct {
	// RequestID 请求 ID
	RequestID RequestID `json:"request_id"`

	// Model 模型名称
	Model string `json:"model"`

	// Backend 后端名称
	Backend string `json:"backend"`

	// LatencyMs 延迟（毫秒）
	LatencyMs int64 `json:"latency_ms"`

	// StatusCode HTTP 状态码
	StatusCode int `json:"status_code"`

	// InputTokens 输入 token 数
	InputTokens int `json:"input_tokens"`

	// OutputTokens 输出 token 数
	OutputTokens int `json:"output_tokens"`
}
