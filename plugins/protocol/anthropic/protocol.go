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

	// [v0.2.8 G1] 先解析原始 map 保留透传能力（与 OpenAI 一致），RawBody 存 map 而非已解析 struct
	var rawBody map[string]interface{}
	if err := json.Unmarshal(body, &rawBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw request: %w", err)
	}

	var anthropicReq MessagesRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	req := &plugin.ProxyRequest{
		Messages:    convertAnthropicMessages(anthropicReq.Messages),
		Model:       anthropicReq.Model,
		Stream:      anthropicReq.Stream,
		Temperature: 0,
		MaxTokens:   anthropicReq.MaxTokens,
		TopP:        anthropicReq.TopP,
		Metadata:    make(map[string]interface{}),
		RawBody:     rawBody, // [G1] map，供后端以原文为基础透传
		Headers:     make(map[string]string),
		System:      anthropicReq.System,
	}

	if anthropicReq.Temperature > 0 {
		req.Temperature = anthropicReq.Temperature
	}

	// [v0.2.8] thinking → ReasoningSpec
	if anthropicReq.Thinking != nil && anthropicReq.Thinking.Type == "enabled" {
		req.Reasoning.Specified = true
		if anthropicReq.Thinking.BudgetTokens > 0 {
			budget := anthropicReq.Thinking.BudgetTokens
			req.Reasoning.BudgetTokens = &budget
		}
	}

	// [v0.2.8] metadata.user_id / stream_options → Metadata（无显式字段承载）
	if anthropicReq.Metadata != nil && anthropicReq.Metadata.UserID != "" {
		req.Metadata["anthropic_user_id"] = anthropicReq.Metadata.UserID
	}
	if anthropicReq.StreamOptions != nil {
		req.Metadata["stream_options"] = anthropicReq.StreamOptions
	}

	// [v0.2.8 L4] Tools / ToolChoice → 显式字段（供 ModeDispatcher 工具感知调度）
	if len(anthropicReq.Tools) > 0 {
		req.Tools = convertAnthropicTools(anthropicReq.Tools)
	}
	if anthropicReq.ToolChoice != nil {
		req.ToolChoice = anthropicReq.ToolChoice
	}

	// [v0.2.9 P2] TopK 显式映射
	if anthropicReq.TopK > 0 {
		req.TopK = anthropicReq.TopK
	}

	// [v0.2.9 P2] Container / OutputConfig → Metadata
	if anthropicReq.Container != nil {
		req.Metadata["container"] = anthropicReq.Container
	}
	if anthropicReq.OutputConfig != nil {
		req.Metadata["output_config"] = anthropicReq.OutputConfig
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
		// [v0.2.8 G2] Anthropic 标准错误结构：error 为对象 {type, message}，而非字符串
		errType := resp.Error.Type
		if errType == "" {
			errType = "invalid_request_error"
		}
		c.JSON(500, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": resp.Error.Message,
			},
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

	// [v0.2.8] stop_sequence：从后端响应提取，无则省略
	if ss, ok := resp.Metadata["stop_sequence"].(string); ok && ss != "" {
		anthropicResp.StopSequence = ss
	}

	// [v0.2.8] cache tokens：从后端响应提取，无则 0（omitempty 省略）
	if ct, ok := resp.Metadata["cache_creation_input_tokens"].(int); ok {
		anthropicResp.Usage.CacheCreationInputTokens = ct
	}
	if ct, ok := resp.Metadata["cache_read_input_tokens"].(int); ok {
		anthropicResp.Usage.CacheReadInputTokens = ct
	}

	// [v0.2.9 P2] container：从后端响应提取，无则省略
	if container, ok := resp.Metadata["container"].(*ContainerInfo); ok && container != nil {
		anthropicResp.Container = container
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

	// [v0.2.8 R04] thinking 事件流（Claude 3.7+）：index=1，与 text 的 index=0 区分。
	// 接口为每 chunk 无状态调用，故每个 thinking chunk 输出完整 start→delta→stop 事件组，
	// 每个事件组自成合法 SSE 块（同一 index 的 start/delta/stop 配对完整）。
	if chunk.ReasoningContent != "" {
		cbStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": 1,
			"content_block": map[string]interface{}{
				"type":     "thinking",
				"thinking": "",
			},
		}
		dataBytes, _ := json.Marshal(cbStart)
		events = append(events, fmt.Sprintf("event: content_block_start\ndata: %s", string(dataBytes)))

		delta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]interface{}{
				"type":     "thinking_delta",
				"thinking": chunk.ReasoningContent,
			},
		}
		dataBytes, _ = json.Marshal(delta)
		events = append(events, fmt.Sprintf("event: content_block_delta\ndata: %s", string(dataBytes)))

		cbStopThinking := map[string]interface{}{
			"type":  "content_block_stop",
			"index": 1,
		}
		dataBytes, _ = json.Marshal(cbStopThinking)
		events = append(events, fmt.Sprintf("event: content_block_stop\ndata: %s", string(dataBytes)))
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

	// [v0.2.8 协议对齐] 新增字段
	TopK          int             `json:"top_k,omitempty"`
	Thinking      *ThinkingConfig `json:"thinking,omitempty"`
	Metadata      *MetadataConfig `json:"metadata,omitempty"`
	StreamOptions *StreamOptions  `json:"stream_options,omitempty"`

	// [v0.2.9 P2] 新增字段
	Container    *ContainerConfig `json:"container,omitempty"`
	OutputConfig *OutputConfig    `json:"output_config,omitempty"`
}

// ThinkingConfig Anthropic 扩展思考配置
type ThinkingConfig struct {
	Type         string `json:"type"` // "enabled"
	BudgetTokens int    `json:"budget_tokens"`
}

// MetadataConfig Anthropic 请求元数据
type MetadataConfig struct {
	UserID string `json:"user_id,omitempty"`
}

// StreamOptions Anthropic 流式选项
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ContainerConfig Anthropic 容器配置（P2）
// 用于代码执行工具，指定容器环境
type ContainerConfig struct {
	ID        string `json:"id,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// OutputConfig Anthropic 输出配置（P2）
// 配置模型输出格式
type OutputConfig struct {
	Effort string          `json:"effort,omitempty"` // low|medium|high|xhigh|max
	Format *JSONOutputFormat `json:"format,omitempty"`
}

// JSONOutputFormat JSON 输出格式配置
type JSONOutputFormat struct {
	Type   string      `json:"type"` // json_schema
	Schema interface{} `json:"schema,omitempty"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// MessagesResponse Anthropic Messages API 响应
type MessagesResponse struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Role         string           `json:"role"`
	Content      []ContentBlock   `json:"content"`
	Model        string           `json:"model"`
	StopReason   string           `json:"stop_reason"`
	StopSequence string           `json:"stop_sequence,omitempty"` // [v0.2.8]
	Usage        Usage            `json:"usage"`
	Container    *ContainerInfo   `json:"container,omitempty"` // [v0.2.9 P2]
}

// ContainerInfo 容器信息（P2）
// 响应中返回容器使用信息
type ContainerInfo struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expires_at"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"` // [v0.2.8 G3] tool_result 块引用 tool_use 的 id
	Citations []Citation  `json:"citations,omitempty"`   // [v0.2.9 P2] 引用信息
}

// Citation 引用信息（P2）
type Citation struct {
	Type          string `json:"type"`                     // char_location | page_location | content_block_location
	CitedText     string `json:"cited_text,omitempty"`
	DocumentIndex int    `json:"document_index,omitempty"`
	DocumentTitle string `json:"document_title,omitempty"`
	// char_location 字段
	StartCharIndex int `json:"start_char_index,omitempty"`
	EndCharIndex   int `json:"end_char_index,omitempty"`
	// page_location 字段
	StartPageNumber int `json:"start_page_number,omitempty"`
	EndPageNumber   int `json:"end_page_number,omitempty"`
	// content_block_location 字段
	StartBlockIndex int `json:"start_block_index,omitempty"`
	EndBlockIndex   int `json:"end_block_index,omitempty"`
}

// Message 消息
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// Usage 使用量
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"` // [v0.2.8]
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`     // [v0.2.8]
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
// [v0.2.8 G3] 修复：tool_result 的引用 id 在顶层 tool_use_id 字段，而非 input
func extractToolCallID(msg Message) string {
	for _, block := range msg.Content {
		if block.Type == "tool_result" && block.ToolUseID != "" {
			return block.ToolUseID
		}
	}
	return ""
}

// convertAnthropicTools 将 Anthropic 工具定义转换为内部统一 ToolDefinition
// Anthropic 形态：{name, description, input_schema} → 内部 OpenAI 形态：{type:"function", function:{...}}
func convertAnthropicTools(tools []ToolDefinition) []plugin.ToolDefinition {
	result := make([]plugin.ToolDefinition, len(tools))
	for i, t := range tools {
		result[i] = plugin.ToolDefinition{
			Type: "function",
			Function: plugin.FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return result
}

// buildContentBlocks 从 ProxyResponse 构建 Anthropic content blocks
func buildContentBlocks(resp *plugin.ProxyResponse) []ContentBlock {
	var blocks []ContentBlock

	// 文本内容
	if resp.Content != "" {
		textBlock := ContentBlock{
			Type: "text",
			Text: resp.Content,
		}
		// [v0.2.9 P2] citations：从后端响应提取，无则省略
		if citations, ok := resp.Metadata["citations"].([]Citation); ok && len(citations) > 0 {
			textBlock.Citations = citations
		}
		blocks = append(blocks, textBlock)
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
