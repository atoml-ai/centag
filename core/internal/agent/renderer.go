package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AgentRenderer Agent渲染器接口
// 统一TUI和Web两种表现形式的渲染逻辑
type AgentRenderer interface {
	// RenderMessage 渲染消息
	RenderMessage(msg Message) string

	// RenderToolCall 渲染工具调用
	RenderToolCall(call ToolCallInfo) string

	// RenderToolResult 渲染工具结果
	RenderToolResult(result ToolResult) string

	// RenderError 渲染错误
	RenderError(err error) string

	// RenderProgress 渲染进度
	RenderProgress(progress AgentProgressInfo) string

	// PromptUserChoice 用户选择（仅TUI需要，Web返回默认值）
	PromptUserChoice(choices []AgentUserChoice) (int, error)
}

// AgentUserChoice Agent用户选择项
type AgentUserChoice struct {
	// Label 显示标签
	Label string `json:"label"`

	// Description 描述信息
	Description string `json:"description,omitempty"`

	// IsDefault 是否默认选项
	IsDefault bool `json:"is_default,omitempty"`
}

// TUIRenderer 终端渲染器
type TUIRenderer struct {
	// theme 主题名称
	theme string

	// enableColor 是否启用颜色
	enableColor bool
}

// NewTUIRenderer 创建终端渲染器
func NewTUIRenderer(theme string, enableColor bool) *TUIRenderer {
	return &TUIRenderer{
		theme:       theme,
		enableColor: enableColor,
	}
}

// RenderMessage 渲染消息
func (r *TUIRenderer) RenderMessage(msg Message) string {
	var sb strings.Builder

	switch msg.Role {
	case "user":
		sb.WriteString("👤 User: ")
		sb.WriteString(msg.Content)
	case "assistant":
		sb.WriteString("🤖 Assistant: ")
		if msg.Content != "" {
			sb.WriteString(msg.Content)
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				sb.WriteString("\n")
				sb.WriteString(r.RenderToolCall(tc))
			}
		}
	case "system":
		sb.WriteString("⚙️ System: ")
		sb.WriteString(msg.Content)
	case "tool":
		sb.WriteString("🔧 Tool Result: ")
		sb.WriteString(msg.Content)
	default:
		sb.WriteString(msg.Content)
	}

	return sb.String()
}

// RenderToolCall 渲染工具调用
func (r *TUIRenderer) RenderToolCall(call ToolCallInfo) string {
	return fmt.Sprintf("🔧 [Tool Call] %s(%s)", call.Function.Name, call.Function.Arguments)
}

// RenderToolResult 渲染工具结果
func (r *TUIRenderer) RenderToolResult(result ToolResult) string {
	if result.IsError {
		return fmt.Sprintf("❌ [Tool Error] %s", result.Content)
	}
	return fmt.Sprintf("✅ [Tool Result] %s", result.Content)
}

// RenderError 渲染错误
func (r *TUIRenderer) RenderError(err error) string {
	return fmt.Sprintf("❌ Error: %s", err.Error())
}

// RenderProgress 渲染进度
func (r *TUIRenderer) RenderProgress(progress AgentProgressInfo) string {
	return fmt.Sprintf("⏳ Progress: Turn %d/%d - %s", progress.CurrentTurn, progress.MaxTurns, progress.Status)
}

// PromptUserChoice 用户选择（TUI交互式）
func (r *TUIRenderer) PromptUserChoice(choices []AgentUserChoice) (int, error) {
	// TUI渲染器需要实际的终端交互
	// 这里返回默认选项，实际实现需要集成bubbletea等TUI框架
	if len(choices) == 0 {
		return 0, fmt.Errorf("no choices provided")
	}

	// 查找默认选项
	for i, choice := range choices {
		if choice.IsDefault {
			return i, nil
		}
	}

	// 返回第一个选项
	return 0, nil
}

// WebRenderer Web渲染器
type WebRenderer struct {
	// format 输出格式（json/text）
	format string
}

// NewWebRenderer 创建Web渲染器
func NewWebRenderer(format string) *WebRenderer {
	if format == "" {
		format = "json"
	}
	return &WebRenderer{
		format: format,
	}
}

// RenderMessage 渲染消息
func (r *WebRenderer) RenderMessage(msg Message) string {
	if r.format == "json" {
		return r.renderJSON("message", msg)
	}
	return r.renderText(msg)
}

// RenderToolCall 渲染工具调用
func (r *WebRenderer) RenderToolCall(call ToolCallInfo) string {
	if r.format == "json" {
		return r.renderJSON("tool_call", call)
	}
	return fmt.Sprintf("[Tool Call] %s(%s)", call.Function.Name, call.Function.Arguments)
}

// RenderToolResult 渲染工具结果
func (r *WebRenderer) RenderToolResult(result ToolResult) string {
	if r.format == "json" {
		return r.renderJSON("tool_result", result)
	}
	if result.IsError {
		return fmt.Sprintf("[Tool Error] %s", result.Content)
	}
	return fmt.Sprintf("[Tool Result] %s", result.Content)
}

// RenderError 渲染错误
func (r *WebRenderer) RenderError(err error) string {
	if r.format == "json" {
		return r.renderJSON("error", AgentError{
			Code:    "UNKNOWN",
			Message: err.Error(),
		})
	}
	return fmt.Sprintf("[Error] %s", err.Error())
}

// RenderProgress 渲染进度
func (r *WebRenderer) RenderProgress(progress AgentProgressInfo) string {
	if r.format == "json" {
		return r.renderJSON("progress", progress)
	}
	return fmt.Sprintf("[Progress] Turn %d/%d - %s", progress.CurrentTurn, progress.MaxTurns, progress.Status)
}

// PromptUserChoice 用户选择（Web返回默认值）
func (r *WebRenderer) PromptUserChoice(choices []AgentUserChoice) (int, error) {
	if len(choices) == 0 {
		return 0, fmt.Errorf("no choices provided")
	}

	// 查找默认选项
	for i, choice := range choices {
		if choice.IsDefault {
			return i, nil
		}
	}

	// 返回第一个选项
	return 0, nil
}

// renderJSON 渲染JSON格式
func (r *WebRenderer) renderJSON(eventType string, data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"type":"%s","data":%v,"error":"json marshal error"}`, eventType, data)
	}
	return fmt.Sprintf(`{"type":"%s","data":%s}`, eventType, string(b))
}

// renderText 渲染文本格式
func (r *WebRenderer) renderText(msg Message) string {
	return msg.Content
}