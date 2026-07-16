package plugin

import (
	"github.com/gin-gonic/gin"
)

// ProtocolPlugin 协议插件接口
// 职责: 解析客户端请求,将不同协议的请求转换为统一的内部格式;
//       将统一响应格式还原为客户端期望的协议格式
type ProtocolPlugin interface {
	Plugin

	// ParseRequest 解析客户端请求为统一的 ProxyRequest
	ParseRequest(c *gin.Context) (*ProxyRequest, error)

	// HandleResponse 处理统一的 ProxyResponse,返回给客户端（非流式）
	HandleResponse(c *gin.Context, resp *ProxyResponse) error

	// FormatStreamChunk 将内部 StreamChunk 格式化为客户端期望的 SSE 数据行
	// 返回值: SSE "data: ..." 行（不含 "data: " 前缀和尾部 \n\n），或空字符串表示跳过
	FormatStreamChunk(model string, chunk *StreamChunk, chunkIndex int) string

	// FormatStreamDone 返回流结束标记（如 OpenAI 的 "[DONE]" 或 Anthropic 的 "[DONE]"）
	FormatStreamDone() string

	// SupportStream 是否支持流式响应
	SupportStream() bool

	// GetModels 获取支持的模型列表
	GetModels() ([]ModelInfo, error)

	// ValidateRequest 验证请求是否有效
	ValidateRequest(req *ProxyRequest) error
}

// ReasoningSpec 定义推理（thinking/reasoning）相关的请求参数。
// 各协议插件把原生 thinking 字段解析后，写入此结构；
// backend 再映射为厂商方言。
type ReasoningSpec struct {
	// Specified 表示客户端是否显式指定了推理参数。
	// 用于区分「未设置」与「显式 none」。
	Specified bool `json:"specified,omitempty"`

	// Disabled 表示客户端是否显式禁用推理。
	Disabled bool `json:"disabled,omitempty"`

	// Effort 表示推理努力级别：none|minimal|low|medium|high|xhigh。
	// 各协议的原生字段会映射到此标准级别。
	Effort string `json:"effort,omitempty"`

	// BudgetTokens 表示推理 token 预算。
	// 与 Effort 互斥，优先使用 Effort；若厂商原生支持 budget 则直接填入。
	BudgetTokens *int `json:"budget_tokens,omitempty"`
}

// ProxyRequest 代理请求统一格式
type ProxyRequest struct {
	// 消息列表
	Messages []Message `json:"messages"`

	// 模型名称
	Model string `json:"model"`

	// 是否流式响应
	Stream bool `json:"stream"`

	// 指定后端 ID（由 handler 在"直接后端"模式下设置，插件应优先使用此 ID 对应的后端配置）
	BackendID string `json:"backend_id,omitempty"`

	// 温度参数 (0-2)
	Temperature float64 `json:"temperature,omitempty"`

	// 最大token数
	MaxTokens int `json:"max_tokens,omitempty"`

	// Top-P采样
	TopP float64 `json:"top_p,omitempty"`

	// 频率惩罚
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// 存在惩罚
	PresencePenalty float64 `json:"presence_penalty,omitempty"`

	// 停止序列
	Stop []string `json:"stop,omitempty"`

	// 系统提示词 (Anthropic 等协议使用)
	System string `json:"system,omitempty"`

	// 推理参数（thinking/reasoning）
	Reasoning ReasoningSpec `json:"reasoning,omitempty"`

	// 元数据
	Metadata map[string]any `json:"metadata,omitempty"`

	// 原始请求体 (用于调试和日志)
	RawBody any `json:"-"`

	// 请求头 (用于认证和追踪)
	Headers map[string]string `json:"-"`
}

// Message 消息
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function FunctionCall     `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ProxyResponse 代理响应统一格式
type ProxyResponse struct {
	// 响应内容
	Content string `json:"content"`

	// 推理内容（DeepSeek thinking 模式等）
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// 使用的token数
	TokensUsed int `json:"tokens_used"`

	// 完成原因
	FinishReason string `json:"finish_reason"`

	// 模型名称
	Model string `json:"model"`

	// 工具调用
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// 元数据
	Metadata map[string]any `json:"metadata,omitempty"`

	// 原始响应体
	RawBody any `json:"-"`

	// 错误信息
	Error *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
