// Package types 定义框架统一的请求/响应类型。
//
// 这些类型是协议插件和后端插件之间的桥梁：
//   - 协议插件将客户端请求 Decode 为 UnifiedRequest
//   - 后端插件接收 UnifiedRequest 并调用 LLM
//   - 后端插件返回 UnifiedResponse
//   - 协议插件将 UnifiedResponse Encode 为客户端响应
package types

// UnifiedRequest 统一请求格式。
//
// 由协议插件的 DecodeRequest 生成，传递给后端插件的 Chat/ChatStream。
type UnifiedRequest struct {
	// Model 请求的模型名称
	Model string `json:"model"`

	// Messages 消息列表
	Messages []Message `json:"messages"`

	// Stream 是否流式响应
	Stream bool `json:"stream"`

	// BackendID 指定后端 ID（直接后端模式下由 handler 设置）
	BackendID string `json:"backend_id,omitempty"`

	// Temperature 温度参数 (0-2)
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens 最大 token 数
	MaxTokens int `json:"max_tokens,omitempty"`

	// TopP Top-P 采样
	TopP float64 `json:"top_p,omitempty"`

	// FrequencyPenalty 频率惩罚
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// PresencePenalty 存在惩罚
	PresencePenalty float64 `json:"presence_penalty,omitempty"`

	// Stop 停止序列
	Stop []string `json:"stop,omitempty"`

	// Tools 工具定义（function calling）
	Tools []ToolDefinition `json:"tools,omitempty"`

	// ToolChoice 工具选择策略
	ToolChoice interface{} `json:"tool_choice,omitempty"`

	// Metadata 附加元数据（协议插件可传递额外信息）
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// [v0.2.8 协议对齐] 与 plugin.ProxyRequest 显式字段对齐

	// ResponseFormat 响应格式（JSON Mode / JSON Schema）
	ResponseFormat *ResponseFormatSpec `json:"response_format,omitempty"`

	// Seed 随机种子
	Seed *int `json:"seed,omitempty"`

	// N 生成 choice 数量
	N *int `json:"n,omitempty"`

	// User 终端用户标识
	User string `json:"user,omitempty"`

	// ParallelToolCalls 是否并行工具调用
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`

	// Reasoning 推理参数（thinking/reasoning）
	Reasoning ReasoningSpec `json:"reasoning,omitempty"`
}

// ReasoningSpec 推理参数（与 plugin.ReasoningSpec 对齐）
type ReasoningSpec struct {
	Specified    bool   `json:"specified,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
	Effort       string `json:"effort,omitempty"`
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
}

// ResponseFormatSpec 响应格式规范
type ResponseFormatSpec struct {
	Type       string      `json:"type"`
	JSONSchema interface{} `json:"json_schema,omitempty"`
}

// UnifiedResponse 统一响应格式。
//
// 由后端插件的 Chat 返回，传递给协议插件的 EncodeResponse。
type UnifiedResponse struct {
	// Content 响应内容
	Content string `json:"content"`

	// ReasoningContent 推理内容（DeepSeek thinking 模式等）
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// TokensUsed 使用的 token 数
	TokensUsed int `json:"tokens_used"`

	// FinishReason 完成原因
	FinishReason string `json:"finish_reason"`

	// Model 实际使用的模型名称
	Model string `json:"model"`

	// ToolCalls 工具调用结果
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Metadata 附加元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UnifiedChunk 统一流式响应块。
//
// 由后端插件的 ChatStream 返回，传递给协议插件的 EncodeStreamResponse。
type UnifiedChunk struct {
	// Content 响应内容增量
	Content string `json:"content"`

	// ReasoningContent 推理内容增量
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// Done 是否完成
	Done bool `json:"done"`

	// TokensUsed 使用的 token 数（仅最后一块）
	TokensUsed int `json:"tokens_used,omitempty"`

	// FinishReason 完成原因（仅最后一块）
	FinishReason string `json:"finish_reason,omitempty"`

	// ToolCalls 工具调用（非流式适配流式时携带）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Error 错误（流式过程中出错时设置）
	Error error `json:"-"`
}

// Message 聊天消息。
type Message struct {
	// Role 角色：system, user, assistant, tool
	Role string `json:"role"`

	// Content 消息内容
	Content string `json:"content"`

	// ToolCalls 工具调用（assistant 角色）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID 工具调用 ID（tool 角色）
	ToolCallID string `json:"tool_call_id,omitempty"`

	// ReasoningContent 推理内容
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// Name 发送者名称
	Name string `json:"name,omitempty"`
}

// ToolDefinition 工具定义。
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义。
type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall 工具调用。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ModelInfo 模型信息。
type ModelInfo struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	OwnedBy  string            `json:"owned_by,omitempty"`
	Capabilities ModelCapabilities `json:"capabilities,omitempty"`
}

// ModelCapabilities 模型能力。
type ModelCapabilities struct {
	MaxContextTokens int  `json:"max_context_tokens,omitempty"`
	SupportsImages   bool `json:"supports_images,omitempty"`
	SupportsTools    bool `json:"supports_tools,omitempty"`
}
