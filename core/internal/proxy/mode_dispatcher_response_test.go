package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

type stubResponsePipelineEngine struct {
	output *pipeline.PipelineOutput
}

func (s *stubResponsePipelineEngine) Execute(context.Context, string, *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	return s.output, nil
}

func (s *stubResponsePipelineEngine) HasPipeline(string) bool { return true }

func (s *stubResponsePipelineEngine) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *stubResponsePipelineEngine) ExecuteStream(context.Context, string, *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	ch := make(chan pipeline.PipelineStreamResult)
	close(ch)
	return ch, nil
}

func TestDispatch_NonStream_ExposesBypassHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubResponsePipelineEngine{
		output: &pipeline.PipelineOutput{
			Content: "fallback response",
			Metadata: map[string]interface{}{
				"bypass":        true,
				"bypass_node":   "generate",
				"bypass_reason": "upstream backend timeout",
			},
			ExecutionLog: &pipeline.ExecutionLog{
				PipelineID:  "direct-backend",
				Duration:    12,
				TotalTokens: 0,
				Success:     false,
			},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-test-1")

	req := &plugin.ProxyRequest{
		Model: "pipeline-mode",
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}

	if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got := w.Header().Get("X-Pipeline-Bypass"); got != "true" {
		t.Fatalf("X-Pipeline-Bypass = %q, want true", got)
	}
	if got := w.Header().Get("X-Pipeline-Bypass-Node"); got != "generate" {
		t.Fatalf("X-Pipeline-Bypass-Node = %q, want generate", got)
	}
	if got := w.Header().Get("X-Pipeline-Bypass-Reason"); got != "upstream backend timeout" {
		t.Fatalf("X-Pipeline-Bypass-Reason = %q, want upstream backend timeout", got)
	}
}

type failingLLMClient struct{}

func (f *failingLLMClient) Chat(context.Context, *pipeline.LLMRequest) (*pipeline.LLMResponse, error) {
	return nil, fmt.Errorf("simulated upstream failure")
}

type failingCapabilityBroker struct{}

func (b *failingCapabilityBroker) GetLLMClient(context.Context, []string) (pipeline.LLMClient, error) {
	return &failingLLMClient{}, nil
}
func (b *failingCapabilityBroker) GetLLMStreamClient(context.Context, []string) (pipeline.LLMStreamClient, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetStorage(context.Context, []string) (pipeline.Storage, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetMemory(context.Context, []string) (pipeline.Memory, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetSecretsResolver(context.Context, []string) (pipeline.SecretsResolver, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetHTTPClient(context.Context, []string) (pipeline.HTTPClient, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetCacheStrategy(context.Context, string, []string) (pipeline.CacheStrategyCapability, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetVectorCache(context.Context, []string) (pipeline.VectorCacheCapability, error) {
	return nil, nil
}
func (b *failingCapabilityBroker) GetEmbeddingService(context.Context, []string) (pipeline.EmbeddingCapability, error) {
	return nil, nil
}

func TestDispatch_EndToEnd_NoUsableBypassOutputReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	nodeRegistry := pipeline.NewNodeRegistry()
	if err := pipeline.RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes() error = %v", err)
	}
	registry := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:   "direct-backend",
		Name: "Direct",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:      "generate",
				Type:    pipeline.NodeTypeGenerator,
				Backend: "bigmodel",
				Model:   "glm-4-flash",
			},
		},
		GlobalConfig: pipeline.GlobalPipelineConfig{
			BypassOnError: true,
			ParallelLimit: 1,
		},
	}
	p.Nodes[0].Normalize()
	if err := registry.Register(p); err != nil {
		t.Fatalf("registry.Register() error = %v", err)
	}

	engine := pipeline.NewPipelineEngine(nodeRegistry, registry, &failingCapabilityBroker{}, pipeline.NewPipelineLogger(), nil)
	dispatcher := NewModeDispatcher(engine, registry, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	req := &plugin.ProxyRequest{
		Model: "pipeline-mode",
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}
	err := dispatcher.Dispatch(c, ModeDirectBackend, req)
	if err == nil {
		t.Fatal("expected dispatch error, got nil")
	}
	if !strings.Contains(err.Error(), "no usable fallback output") {
		t.Fatalf("unexpected error = %v", err)
	}
}

// TestDispatch_NonStream_ToolCallsPreserved 验证非流式响应正确输出 tool_calls
// 和 finish_reason=tool_calls，而非丢失或硬编码为 stop。
func TestDispatch_NonStream_ToolCallsPreserved(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubResponsePipelineEngine{
		output: &pipeline.PipelineOutput{
			Content: "",
			ToolCalls: []pipeline.ToolCall{
				{
					ID:   "call_abc",
					Type: "function",
					Function: pipeline.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"Beijing"}`,
					},
				},
			},
			FinishReason: "tool_calls",
			ExecutionLog: &pipeline.ExecutionLog{
				PipelineID:  "direct-backend",
				TotalTokens: 20,
				Success:     true,
			},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-tool-1")

	req := &plugin.ProxyRequest{
		Model: "pipeline-mode",
		Messages: []plugin.Message{
			{Role: "user", Content: "what's the weather?"},
		},
	}

	if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string                   `json:"role"`
				Content   *string                  `json:"content"`
				ToolCalls []map[string]interface{} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nbody: %s", err, w.Body.String())
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	choice := resp.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q, want tool_calls", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool_call, got %d", len(choice.Message.ToolCalls))
	}
	tc := choice.Message.ToolCalls[0]
	if tc["id"] != "call_abc" {
		t.Fatalf("tool_call id = %v, want call_abc", tc["id"])
	}
	fn, _ := tc["function"].(map[string]interface{})
	if fn["name"] != "get_weather" {
		t.Fatalf("function name = %v, want get_weather", fn["name"])
	}
	if fn["arguments"] != `{"location":"Beijing"}` {
		t.Fatalf("function arguments = %v, want {\"location\":\"Beijing\"}", fn["arguments"])
	}
}

// TestWriteStreamResponse_AnthropicToolUse 验证 Anthropic 流式格式化器
// 在 chunk 携带 tool_calls 时输出 tool_use content block。
func TestWriteStreamResponse_AnthropicToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	toolCall := plugin.ToolCall{
		ID:   "toolu_01",
		Type: "function",
		Function: plugin.FunctionCall{
			Name:      "search",
			Arguments: `{"query":"hello"}`,
		},
	}
	engine := &stubStreamPipelineEngineWithToolCall{
		chunk: &plugin.StreamChunk{
			Content:      "",
			ToolCalls:    []plugin.ToolCall{toolCall},
			FinishReason: "tool_calls",
			Done:         true,
		},
		output: &pipeline.PipelineOutput{
			FinishReason: "tool_calls",
			ToolCalls: []pipeline.ToolCall{
				{
					ID:   toolCall.ID,
					Type: toolCall.Type,
					Function: pipeline.FunctionCall{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				},
			},
			ExecutionLog: &pipeline.ExecutionLog{TotalTokens: 8},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set("protocol_plugin", "anthropic-protocol")

	resultCh, err := engine.ExecuteStream(context.Background(), "router-mode", &pipeline.PipelineInput{Content: "search hello"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if err := dispatcher.writeStreamResponse(c, resultCh, ModeRouter, "claude-3-5-sonnet"); err != nil {
		t.Fatalf("writeStreamResponse: %v", err)
	}

	body := w.Body.String()
	// 应包含 tool_use content_block_start
	if !strings.Contains(body, `"type":"tool_use"`) {
		t.Fatalf("expected tool_use content_block_start in SSE body, got: %s", body)
	}
	// 应包含 input_json_delta
	if !strings.Contains(body, `"input_json_delta"`) {
		t.Fatalf("expected input_json_delta in SSE body, got: %s", body)
	}
	// 应包含工具名
	if !strings.Contains(body, `"search"`) {
		t.Fatalf("expected tool name 'search' in SSE body, got: %s", body)
	}
	// stop_reason 应为 tool_use
	if !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected stop_reason tool_use in SSE body, got: %s", body)
	}
}

type stubStreamPipelineEngineWithToolCall struct {
	chunk  *plugin.StreamChunk
	output *pipeline.PipelineOutput
}

func (s *stubStreamPipelineEngineWithToolCall) Execute(context.Context, string, *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	return s.output, nil
}

func (s *stubStreamPipelineEngineWithToolCall) HasPipeline(string) bool { return true }

func (s *stubStreamPipelineEngineWithToolCall) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *stubStreamPipelineEngineWithToolCall) ExecuteStream(context.Context, string, *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	ch := make(chan pipeline.PipelineStreamResult, 2)
	if s.chunk != nil {
		ch <- pipeline.PipelineStreamResult{Chunk: s.chunk}
	}
	ch <- pipeline.PipelineStreamResult{Output: s.output}
	close(ch)
	return ch, nil
}

