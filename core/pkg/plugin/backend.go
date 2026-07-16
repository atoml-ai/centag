package plugin

import (
	"context"
)

// BackendPlugin 后端插件接口
// 职责: 将内部格式的请求转发到实际的大模型服务
type BackendPlugin interface {
	Plugin

	// CallModel 调用模型 (非流式)
	CallModel(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)

	// CallModelStream 流式调用模型
	CallModelStream(ctx context.Context, req *ProxyRequest) (<-chan StreamChunk, error)

	// Authenticate 使用配置进行认证
	Authenticate(config any) error

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// GetAvailableModels 获取可用的模型列表
	GetAvailableModels() ([]ModelInfo, error)
}

// StreamChunk 流式响应块
type StreamChunk struct {
	// 响应内容
	Content string `json:"content"`

	// 推理内容增量（DeepSeek thinking 模式等）
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// 是否完成
	Done bool `json:"done"`

	// 使用的token数
	TokensUsed int `json:"tokens_used,omitempty"`

	// 完成原因
	FinishReason string `json:"finish_reason,omitempty"`

	// 工具调用（function calling，非流式适配流式时携带）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// 错误
	Error error `json:"-"`

	// 原始 SSE data 字节（后端返回的 JSON chunk 原文），非 nil 时 handler 优先透传，
	// 避免重新构造 SSE 导致 tool_calls 等非 content 字段丢失。
	RawData []byte `json:"-"`

	// ContentIsNonStandard 为 true 时表示内容来自非标准字段（如 reasoning_content），
	// handler 应重构 SSE 把内容放回标准 delta.content，而非直接透传原始数据。
	ContentIsNonStandard bool `json:"-"`

	// UsagePromptTokens / UsageCompletionTokens 流式分项用量（由后端插件从流事件中解析）。
	// 仅用于代理层 Token 统计，不转发给下游客户端。
	UsagePromptTokens     int `json:"-"`
	UsageCompletionTokens int `json:"-"`
}

// BackendConfig 后端配置
type BackendConfig struct {
	// API密钥
	APIKey string `json:"api_key" mapstructure:"api_key"`

	// 基础URL
	BaseURL string `json:"base_url" mapstructure:"base_url"`

	// 超时时间 (秒)
	Timeout int `json:"timeout" mapstructure:"timeout"`

	// 最大重试次数
	MaxRetries int `json:"max_retries" mapstructure:"max_retries"`

	// 重试延迟 (秒)
	RetryDelay int `json:"retry_delay" mapstructure:"retry_delay"`

	// 是否自动获取模型列表（默认 true）
	AutoFetchModels bool `json:"auto_fetch_models" mapstructure:"auto_fetch_models"`

	// 自定义配置
	Custom map[string]any `json:"custom,omitempty" mapstructure:"custom"`
}
