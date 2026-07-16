package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// Protocol Anthropic 协议插件
type Protocol struct {
	name   string
	status plugin.PluginStatus
}

// NewProtocol 创建 Anthropic 协议插件
func NewProtocol() (plugin.Plugin, error) {
	return &Protocol{
		name:   "anthropic-protocol",
		status: plugin.StatusStopped,
	}, nil
}

// Name 返回插件名称
func (p *Protocol) Name() string {
	return p.name
}

// Type 返回插件类型
func (p *Protocol) Type() plugin.PluginType {
	return plugin.TypeProtocol
}

// Version 返回插件版本
func (p *Protocol) Version() string {
	return "1.0.0"
}

// Init 初始化插件
func (p *Protocol) Init(config any) error {
	p.status = plugin.StatusStopped
	return nil
}

// Start 启动插件
func (p *Protocol) Start(ctx context.Context) error {
	p.status = plugin.StatusRunning
	return nil
}

// Stop 停止插件
func (p *Protocol) Stop(ctx context.Context) error {
	p.status = plugin.StatusStopped
	return nil
}

// Status 返回插件状态
func (p *Protocol) Status() plugin.PluginStatus {
	return p.status
}

// ParseRequest 解析 Anthropic Messages API 请求
func (p *Protocol) ParseRequest(c *gin.Context) (*plugin.ProxyRequest, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var anthropicReq MessagesRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	req := &plugin.ProxyRequest{
		Messages:         convertAnthropicMessages(anthropicReq.Messages),
		Model:            anthropicReq.Model,
		Stream:           anthropicReq.Stream,
		Temperature:      0,
		MaxTokens:        anthropicReq.MaxTokens,
		TopP:             anthropicReq.TopP,
		Metadata:         make(map[string]interface{}),
		RawBody:          anthropicReq,
		Headers:          make(map[string]string),
		System:           anthropicReq.System,
	}

	if anthropicReq.Temperature > 0 {
		req.Temperature = anthropicReq.Temperature
	}

	for k, v := range c.Request.Header {
		if len(v) > 0 {
			req.Headers[k] = v[0]
		}
	}

	return req, nil
}

// HandleResponse 处理响应并返回给客户端（非流式）
func (p *Protocol) HandleResponse(c *gin.Context, resp *plugin.ProxyResponse) error {
	if resp.Error != nil {
		c.JSON(500, gin.H{
			"type":  "error",
			"error": resp.Error.Message,
		})
		return nil
	}

	// 构建 content blocks（支持 text + tool_use）
	contentBlocks := buildContentBlocks(resp)

	// 映射 finish_reason → stop_reason
	stopReason := mapFinishReason(resp.FinishReason)

	anthropicResp := MessagesResponse{
		ID:         "msg-" + generateID(),
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      resp.Model,
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:  0,
			OutputTokens: resp.TokensUsed,
		},
	}

	if metadata, ok := resp.Metadata["prompt_tokens"].(int); ok {
		anthropicResp.Usage.InputTokens = metadata
	}

	c.JSON(200, anthropicResp)
	return nil
}

// FormatStreamChunk 将内部 StreamChunk 格式化为 Anthropic SSE data 行
// 返回值为完整的 "event: xxx\ndata: xxx" 字符串（多行），或空字符串表示跳过
func (p *Protocol) FormatStreamChunk(model string, chunk *plugin.StreamChunk, chunkIndex int) string {
	if chunk == nil {
		return ""
	}

	var events []string

	// 第一个 chunk: 发送 message_start
	if chunkIndex == 0 {
		msgStart := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":            "msg-" + generateID(),
				"type":          "message",
				"role":          "assistant",
				"content":       []interface{}{},
				"model":         model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]interface{}{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}
		dataBytes, _ := json.Marshal(msgStart)
		events = append(events, fmt.Sprintf("event: message_start\ndata: %s", string(dataBytes)))

		// 发送 content_block_start（text 类型）
		cbStart := map[string]interface{}{
			"type":         "content_block_start",
			"index":        0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		dataBytes, _ = json.Marshal(cbStart)
		events = append(events, fmt.Sprintf("event: content_block_start\ndata: %s", string(dataBytes)))
	}

	// 内容 chunk: 发送 content_block_delta
	if chunk.Content != "" {
		delta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": chunk.Content,
			},
		}
		dataBytes, _ := json.Marshal(delta)
		events = append(events, fmt.Sprintf("event: content_block_delta\ndata: %s", string(dataBytes)))
	}

	// done chunk: 发送 content_block_stop + message_delta + message_stop
	if chunk.Done {
		// content_block_stop
		cbStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}
		dataBytes, _ := json.Marshal(cbStop)
		events = append(events, fmt.Sprintf("event: content_block_stop\ndata: %s", string(dataBytes)))

		// message_delta (含 stop_reason 和 usage)
		stopReason := mapFinishReason(chunk.FinishReason)
		if stopReason == "" {
			stopReason = "end_turn"
		}
		msgDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": chunk.TokensUsed,
			},
		}
		dataBytes, _ = json.Marshal(msgDelta)
		events = append(events, fmt.Sprintf("event: message_delta\ndata: %s", string(dataBytes)))

		// message_stop
		msgStop := map[string]interface{}{
			"type": "message_stop",
		}
		dataBytes, _ = json.Marshal(msgStop)
		events = append(events, fmt.Sprintf("event: message_stop\ndata: %s", string(dataBytes)))
	}

	return strings.Join(events, "\n")
}

// FormatStreamDone 返回 Anthropic 流结束标记
func (p *Protocol) FormatStreamDone() string {
	// Anthropic 不使用 [DONE] 标记，而是通过 message_stop 事件结束
	// 但为了一致性，返回空字符串让调用方跳过
	return ""
}

// SupportStream 是否支持流式响应
func (p *Protocol) SupportStream() bool {
	return true
}

// GetModels 获取支持的模型列表
func (p *Protocol) GetModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{
			ID:      "claude-3-5-sonnet-20241022",
			Name:    "Claude 3.5 Sonnet",
			Enabled: true,
		},
		{
			ID:      "claude-3-opus-20240229",
			Name:    "Claude 3 Opus",
			Enabled: true,
		},
		{
			ID:      "claude-3-sonnet-20240229",
			Name:    "Claude 3 Sonnet",
			Enabled: true,
		},
		{
			ID:      "claude-3-haiku-20240307",
			Name:    "Claude 3 Haiku",
			Enabled: true,
		},
	}, nil
}

// ValidateRequest 验证请求
func (p *Protocol) ValidateRequest(req *plugin.ProxyRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("messages is required")
	}

	return nil
}

// --- Anthropic Messages API 数据模型 ---

// MessagesRequest Anthropic Messages API 请求
type MessagesRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	Messages    []Message        `json:"messages"`
	Stream      bool             `json:"stream,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	System      string           `json:"system,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// MessagesResponse Anthropic Messages API 响应
type MessagesResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

// Message 消息
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// Usage 使用量
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// --- 转换函数 ---

// convertAnthropicMessages 转换 Anthropic 消息格式为统一格式
func convertAnthropicMessages(messages []Message) []plugin.Message {
	result := make([]plugin.Message, len(messages))
	for i, msg := range messages {
		var content strings.Builder
		var toolCalls []plugin.ToolCall

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				content.WriteString(block.Text)
			case "tool_use":
				// 将 Anthropic tool_use 转为内部 ToolCall 格式
				argsJSON, _ := json.Marshal(block.Input)
				toolCalls = append(toolCalls, plugin.ToolCall{
					ID:   block.ID,
					Type: "function",
					Function: plugin.FunctionCall{
						Name:      block.Name,
						Arguments: string(argsJSON),
					},
				})
			case "tool_result":
				// tool_result 是用户消息中的内容，合并到 content
				content.WriteString(fmt.Sprintf("[tool result: %s]", block.Text))
			}
		}

		result[i] = plugin.Message{
			Role:       msg.Role,
			Content:    content.String(),
			ToolCalls:  toolCalls,
			ToolCallID: extractToolCallID(msg),
		}
	}
	return result
}

// extractToolCallID 从消息中提取 tool_call_id（Anthropic 的 tool_result 消息）
func extractToolCallID(msg Message) string {
	for _, block := range msg.Content {
		if block.Type == "tool_result" {
			// Anthropic 的 tool_result 使用 tool_use_id 字段
			if id, ok := block.Input.(string); ok {
				return id
			}
		}
	}
	return ""
}

// buildContentBlocks 从 ProxyResponse 构建 Anthropic content blocks
func buildContentBlocks(resp *plugin.ProxyResponse) []ContentBlock {
	var blocks []ContentBlock

	// 文本内容
	if resp.Content != "" {
		blocks = append(blocks, ContentBlock{
			Type: "text",
			Text: resp.Content,
		})
	}

	// 工具调用
	for _, tc := range resp.ToolCalls {
		var input interface{}
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		blocks = append(blocks, ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	// 至少返回一个空文本块
	if len(blocks) == 0 {
		blocks = append(blocks, ContentBlock{Type: "text", Text: ""})
	}

	return blocks
}

// mapFinishReason 将 OpenAI finish_reason 映射为 Anthropic stop_reason
func mapFinishReason(reason string) string {
	switch reason {
	case "stop", "":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

// generateID 生成简单的ID
func generateID() string {
	return "proxy-" + randomString(16)
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
