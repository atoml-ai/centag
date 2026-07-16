package agent

import (
	"context"
	"time"
)

// AgentEngine Agent执行引擎接口
// 统一TUI和Web两种表现形式的底层引擎
type AgentEngine interface {
	// Run 执行Agent循环
	// 返回事件通道，客户端通过监听事件获取执行进度
	Run(ctx context.Context, req *AgentRequest) (<-chan AgentEvent, error)

	// Cancel 取消执行
	Cancel(requestID string) error

	// GetTools 获取可用工具列表
	GetTools() []ToolDefinition

	// RegisterTool 动态注册工具
	RegisterTool(tool ToolDefinition) error

	// UnregisterTool 注销工具
	UnregisterTool(toolName string) error
}

// AgentRequest Agent请求
type AgentRequest struct {
	// RequestID 请求ID（唯一标识）
	RequestID string `json:"request_id"`

	// Messages 对话历史
	Messages []Message `json:"messages"`

	// Model 模型名称（可选，覆盖默认配置）
	Model string `json:"model,omitempty"`

	// Tools 可用工具列表（可选，覆盖默认工具集）
	Tools []ToolDefinition `json:"tools,omitempty"`

	// ToolChoice 工具选择策略（auto/none/required）
	ToolChoice string `json:"tool_choice,omitempty"`

	// PipelineID 指定Pipeline ID（可选）
	PipelineID string `json:"pipeline_id,omitempty"`

	// Scene 场景标识（可选，用于教育/编程等场景路由）
	Scene string `json:"scene,omitempty"`

	// UserID 用户ID
	UserID string `json:"user_id,omitempty"`

	// SessionID 会话ID
	SessionID string `json:"session_id,omitempty"`

	// MaxTurns 最大对话轮数（防止无限循环）
	MaxTurns int `json:"max_turns,omitempty"`

	// Timeout 请求超时时间（秒）
	Timeout int `json:"timeout,omitempty"`
}

// Message 对话消息
type Message struct {
	// Role 角色（user/assistant/system/tool）
	Role string `json:"role"`

	// Content 消息内容
	Content string `json:"content,omitempty"`

	// ToolCalls 工具调用列表（assistant消息）
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"`

	// ToolCallID 工具调用ID（tool消息）
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Name 工具名称（tool消息）
	Name string `json:"name,omitempty"`
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	// ID 工具调用ID
	ID string `json:"id"`

	// Type 工具类型（固定 "function"）
	Type string `json:"type"`

	// Function 函数调用信息
	Function FunctionCallInfo `json:"function"`
}

// FunctionCallInfo 函数调用信息
type FunctionCallInfo struct {
	// Name 函数名称
	Name string `json:"name"`

	// Arguments 参数（JSON字符串）
	Arguments string `json:"arguments"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	// Name 工具名称
	Name string `json:"name"`

	// Description 工具描述
	Description string `json:"description"`

	// Parameters 参数schema（JSON Schema格式）
	Parameters map[string]interface{} `json:"parameters"`

	// IsReadOnly 是否只读工具（不需要权限确认）
	IsReadOnly bool `json:"is_read_only,omitempty"`
}

// AgentEvent Agent事件
type AgentEvent struct {
	// Type 事件类型
	Type EventType `json:"type"`

	// RequestID 请求ID
	RequestID string `json:"request_id"`

	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`

	// Data 事件数据（根据Type不同而不同）
	Data interface{} `json:"data"`
}

// EventType 事件类型
type EventType string

const (
	// EventAgentStart Agent开始执行
	EventAgentStart EventType = "agent_start"

	// EventAgentEnd Agent执行结束
	EventAgentEnd EventType = "agent_end"

	// EventMessageUpdate 消息更新（流式输出）
	EventMessageUpdate EventType = "message_update"

	// EventToolStart 工具开始执行
	EventToolStart EventType = "tool_start"

	// EventToolEnd 工具执行结束
	EventToolEnd EventType = "tool_end"

	// EventToolPermissionRequest 工具权限请求
	EventToolPermissionRequest EventType = "tool_permission_request"

	// EventToolPermissionResponse 工具权限响应
	EventToolPermissionResponse EventType = "tool_permission_response"

	// EventError 错误事件
	EventError EventType = "error"

	// EventProgress 进度事件
	EventProgress EventType = "progress"
)

// AgentEventData 事件数据基类
type AgentEventData struct {
	// Message 消息内容（用于EventMessageUpdate）
	Message *Message `json:"message,omitempty"`

	// ToolCall 工具调用信息（用于EventToolStart/EventToolEnd）
	ToolCall *ToolCallInfo `json:"tool_call,omitempty"`

	// ToolResult 工具执行结果（用于EventToolEnd）
	ToolResult *ToolResult `json:"tool_result,omitempty"`

	// Error 错误信息（用于EventError）
	Error *AgentError `json:"error,omitempty"`

	// Progress 进度信息（用于EventProgress）
	Progress *AgentProgressInfo `json:"progress,omitempty"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	// ToolCallID 工具调用ID
	ToolCallID string `json:"tool_call_id"`

	// Content 执行结果内容
	Content string `json:"content"`

	// IsError 是否执行错误
	IsError bool `json:"is_error,omitempty"`

	// DurationMs 执行耗时（毫秒）
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// AgentError Agent错误
type AgentError struct {
	// Code 错误码
	Code string `json:"code"`

	// Message 错误消息
	Message string `json:"message"`

	// Details 错误详情
	Details interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *AgentError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// AgentProgressInfo Agent进度信息
type AgentProgressInfo struct {
	// CurrentTurn 当前轮数
	CurrentTurn int `json:"current_turn"`

	// MaxTurns 最大轮数
	MaxTurns int `json:"max_turns"`

	// Status 状态描述
	Status string `json:"status"`
}

// AgentToolExecutor 工具执行器接口
// 由外部注入，DefaultAgentEngine通过此接口执行工具调用
type AgentToolExecutor interface {
	// Execute 执行工具调用
	// toolCallID: 工具调用唯一ID
	// toolName: 工具名称
	// arguments: 参数JSON字符串
	// 返回: content(执行结果), isError(是否出错), error(系统级错误)
	Execute(ctx context.Context, toolCallID, toolName string, arguments string) (content string, isError bool, err error)
}