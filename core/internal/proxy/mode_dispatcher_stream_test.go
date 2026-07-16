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
