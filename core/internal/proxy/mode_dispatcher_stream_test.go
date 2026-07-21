package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

type stubStreamPipelineEngine struct {
	output *pipeline.PipelineOutput
}

func (s *stubStreamPipelineEngine) Execute(context.Context, string, *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	return s.output, nil
}

func (s *stubStreamPipelineEngine) HasPipeline(string) bool { return true }

func (s *stubStreamPipelineEngine) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *stubStreamPipelineEngine) ExecuteStream(context.Context, string, *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	ch := make(chan pipeline.PipelineStreamResult, 2)
	ch <- pipeline.PipelineStreamResult{
		Chunk: &plugin.StreamChunk{Content: "cached answer"},
	}
	ch <- pipeline.PipelineStreamResult{
		Output: s.output,
	}
	close(ch)
	return ch, nil
}

func TestWriteStreamResponse_IncludesFinishReason(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubStreamPipelineEngine{
		output: &pipeline.PipelineOutput{
			Content:      "cached answer",
			ExecutionLog: &pipeline.ExecutionLog{TotalTokens: 10},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Pipeline-ID", "cache-hit")

	resultCh, err := engine.ExecuteStream(context.Background(), "cache-hit", &pipeline.PipelineInput{Content: "hi"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if err := dispatcher.writeStreamResponse(c, resultCh, ModeCacheHit, "test"); err != nil {
		t.Fatalf("writeStreamResponse: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("expected finish_reason stop in SSE body, got: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected data: [DONE] in SSE body, got: %s", body)
	}
}

func TestWriteStreamResponse_RawPassthroughSSENotDoubleWrapped(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamSSE := "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"glm-4-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"我\"}}]}\n\n" +
		"data: [DONE]\n\n"

	ch := make(chan pipeline.PipelineStreamResult, 1)
	ch <- pipeline.PipelineStreamResult{
		Output: &pipeline.PipelineOutput{
			Content: upstreamSSE,
			Metadata: map[string]interface{}{
				"raw_passthrough": true,
				"content_type":    "text/event-stream",
				"status_code":     200,
				"backend_id":      "openai-bigmodel-ai",
				"model":           "glm-4-flash",
			},
			ExecutionLog: &pipeline.ExecutionLog{Success: true},
		},
	}
	close(ch)

	dispatcher := NewModeDispatcher(&stubStreamPipelineEngine{}, nil, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Pipeline-ID", "transparent-proxy")

	if err := dispatcher.writeStreamResponse(c, ch, ModeTransparentProxy, "hy3-free"); err != nil {
		t.Fatalf("writeStreamResponse: %v", err)
	}

	body := w.Body.String()
	if body != upstreamSSE {
		t.Fatalf("expected raw upstream SSE passthrough,\n got: %q\n want: %q", body, upstreamSSE)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type=%q, want text/event-stream", ct)
	}
	// 禁止把 data: 再包进 delta.content
	if strings.Contains(body, `"delta":{"content":"data:`) || strings.Count(body, "data: [DONE]") != 1 {
		t.Fatalf("SSE was double-wrapped or [DONE] duplicated: %s", body)
	}
}

func TestWriteStreamResponse_AnthropicNoDoneMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubStreamPipelineEngine{
		output: &pipeline.PipelineOutput{
			Content: "anthropic answer",
			ExecutionLog: &pipeline.ExecutionLog{
				TotalTokens: 12,
			},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("X-Pipeline-ID", "agent-flow")
	c.Set("protocol_plugin", "anthropic-protocol")

	resultCh, err := engine.ExecuteStream(context.Background(), "agent-flow", &pipeline.PipelineInput{Content: "hi"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if err := dispatcher.writeStreamResponse(c, resultCh, ModeCacheHit, "claude-3-5-sonnet-20241022"); err != nil {
		t.Fatalf("writeStreamResponse: %v", err)
	}

	body := w.Body.String()
	if strings.Contains(body, "data: [DONE]") {
		t.Fatalf("anthropic stream should not contain [DONE], got: %s", body)
	}
	if !strings.Contains(body, "event: message_start") {
		t.Fatalf("anthropic stream should contain message_start, got: %s", body)
	}
}

func TestWriteStreamResponse_ResponsesProtocolKeepsSingleChunkText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubStreamPipelineEngine{
		output: &pipeline.PipelineOutput{
			Content:      "你好，我是助手",
			ExecutionLog: &pipeline.ExecutionLog{TotalTokens: 12},
		},
	}
	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("protocol_plugin", "responses-protocol")

	resultCh, err := engine.ExecuteStream(context.Background(), "direct-backend", &pipeline.PipelineInput{Content: "hi"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if err := dispatcher.writeStreamResponse(c, resultCh, ModeDirectBackend, "gpt-5.6-terra"); err != nil {
		t.Fatalf("writeStreamResponse: %v", err)
	}

	body := w.Body.String()
	for _, want := range []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		"event: response.output_text.delta",
		"cached answer",
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in responses SSE, got: %s", want, body)
		}
	}
	if strings.Contains(body, "data: [DONE]") {
		t.Fatalf("responses stream should not contain [DONE], got: %s", body)
	}
}

func TestResponsesStreamFormatter_FirstChunkIncludesDelta(t *testing.T) {
	f := &responsesStreamFormatter{}
	out := f.FormatChunk("gpt-5.6-terra", &plugin.StreamChunk{Content: "hello", Done: true}, 0, "resp-1", 123)
	for _, want := range []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		"event: response.output_text.delta",
		"hello",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	done := f.FormatDone("gpt-5.6-terra", map[string]interface{}{"total_tokens": 3}, "stop")
	for _, want := range []string{
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
	} {
		if !strings.Contains(done, want) {
			t.Fatalf("FormatDone missing %q: %s", want, done)
		}
	}
}

func TestResponsesStreamFormatter_ToolCalls(t *testing.T) {
	f := &responsesStreamFormatter{}
	out := f.FormatChunk("gpt-5.6-terra", &plugin.StreamChunk{
		ToolCalls: []plugin.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: plugin.FunctionCall{
				Name:      "bash",
				Arguments: `{"command":"ls"}`,
			},
		}},
		FinishReason: "tool_calls",
		Done:         true,
	}, 0, "resp-tools", 123)
	for _, want := range []string{
		"event: response.created",
		`"type":"function_call"`,
		`"call_id":"call_1"`,
		`"name":"bash"`,
		"event: response.function_call_arguments.delta",
		"event: response.function_call_arguments.done",
		`\"command\":\"ls\"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	// Tool-only: must not invent an empty assistant message item before completed.
	if strings.Contains(out, `"type":"message"`) {
		t.Fatalf("tool-only FormatChunk should not open message item: %s", out)
	}
	done := f.FormatDone("gpt-5.6-terra", nil, "tool_calls")
	if strings.Contains(done, "response.output_text.done") {
		t.Fatalf("tool-only FormatDone should skip text lifecycle: %s", done)
	}
	if !strings.Contains(done, "event: response.completed") || !strings.Contains(done, `"call_id":"call_1"`) {
		t.Fatalf("FormatDone missing completed function_call: %s", done)
	}
}
