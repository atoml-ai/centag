//go:build protocol_openairesponses

package openairesponses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// Protocol OpenAI Responses 协议插件
type Protocol struct {
	name   string
	status plugin.PluginStatus
}

// NewProtocol 创建 OpenAI Responses 协议插件
func NewProtocol() (plugin.Plugin, error) {
	return &Protocol{
		name:   "responses-protocol",
		status: plugin.StatusStopped,
	}, nil
}

func (p *Protocol) Name() string                          { return p.name }
func (p *Protocol) Type() plugin.PluginType                { return plugin.TypeProtocol }
func (p *Protocol) Version() string                        { return "1.0.0" }
func (p *Protocol) Status() plugin.PluginStatus            { return p.status }
func (p *Protocol) SupportStream() bool                    { return true }

func (p *Protocol) Init(config any) error {
	log.Printf("[Responses Protocol] Plugin initialized")
	return nil
}

func (p *Protocol) Start(ctx context.Context) error {
	p.status = plugin.StatusRunning
	log.Printf("[Responses Protocol] Plugin started")
	return nil
}

func (p *Protocol) Stop(ctx context.Context) error {
	p.status = plugin.StatusStopped
	log.Printf("[Responses Protocol] Plugin stopped")
	return nil
}

// ParseRequest 解析 OpenAI Responses 请求为统一的 ProxyRequest
func (p *Protocol) ParseRequest(c *gin.Context) (*plugin.ProxyRequest, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var rawBody map[string]interface{}
	if err := json.Unmarshal(body, &rawBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw request: %w", err)
	}

	var respReq responsesRequest
	if err := json.Unmarshal(body, &respReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal responses request: %w", err)
	}

	req := &plugin.ProxyRequest{
		Model:    respReq.Model,
		Stream:   respReq.Stream,
		RawBody:  body,
		Headers:  make(map[string]string),
		Metadata: map[string]any{"protocol": "responses"},
	}

	if respReq.Instructions != "" {
		req.System = respReq.Instructions
	}
	if respReq.Temperature != nil {
		req.Temperature = *respReq.Temperature
	}
	if respReq.MaxTokens != nil {
		req.MaxTokens = *respReq.MaxTokens
	}
	if respReq.TopP != nil {
		req.TopP = *respReq.TopP
	}

	// 转换 input 到 messages
	if respReq.Input.StringVal != "" {
		req.Messages = []plugin.Message{
			{Role: "user", Content: respReq.Input.StringVal},
		}
	} else {
		for _, item := range respReq.Input.Items {
			role := item.Role
			if role == "developer" {
				role = "system"
			}
			var text string
			for _, cp := range item.Content {
				if cp.Type == "input_text" {
					text += cp.Text
				}
			}
			req.Messages = append(req.Messages, plugin.Message{
				Role:    role,
				Content: text,
			})
		}
	}

	for k, v := range c.Request.Header {
		if len(v) > 0 {
			req.Headers[k] = v[0]
		}
	}

	return req, nil
}

// HandleResponse 处理非流式响应
func (p *Protocol) HandleResponse(c *gin.Context, resp *plugin.ProxyResponse) error {
	if resp.Error != nil {
		c.JSON(http.StatusInternalServerError, responsesResponse{
			Object: "error",
			Error: &responseError{
				Code:    resp.Error.Code,
				Message: resp.Error.Message,
			},
		})
		return nil
	}

	var output []outputItem
	if resp.Content != "" {
		output = []outputItem{
			{
				Type:    "message",
				ID:      "msg-" + generateID(),
				Role:    "assistant",
				Status:  "completed",
				Content: []contentPart{{Type: "output_text", Text: resp.Content}},
			},
		}
	}

	usage := &usageInfo{
		InputTokens:  0,
		OutputTokens: resp.TokensUsed,
		TotalTokens:  resp.TokensUsed,
	}

	c.JSON(http.StatusOK, responsesResponse{
		ID:     "resp-" + generateID(),
		Object: "response",
		Status: "completed",
		Output: output,
		Usage:  usage,
	})
	return nil
}

// FormatStreamChunk 将内部 StreamChunk 格式化为 Responses API SSE event
func (p *Protocol) FormatStreamChunk(model string, chunk *plugin.StreamChunk, chunkIndex int) string {
	if chunk == nil {
		return ""
	}
	if chunkIndex == 0 {
		evt := sseEvent{Type: "response.created"}
		data, _ := json.Marshal(evt)
		return fmt.Sprintf("event: %s\ndata: %s", evt.Type, string(data))
	}
	if chunk.Content != "" || chunk.ReasoningContent != "" {
		deltaText := chunk.Content
		if deltaText == "" {
			deltaText = chunk.ReasoningContent
		}
		evt := sseEvent{
			Type:  "response.output_text.delta",
			Delta: deltaText,
		}
		data, _ := json.Marshal(evt)
		return fmt.Sprintf("event: %s\ndata: %s", evt.Type, string(data))
	}
	return ""
}

// FormatStreamDone 返回 Responses 流结束事件
func (p *Protocol) FormatStreamDone() string {
	evt := sseEvent{Type: "response.completed"}
	data, _ := json.Marshal(evt)
	return fmt.Sprintf("event: %s\ndata: %s", evt.Type, string(data))
}

func (p *Protocol) GetModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{}, nil
}

func (p *Protocol) ValidateRequest(req *plugin.ProxyRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

func generateID() string {
	return "proxy-" + randomString(16)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
