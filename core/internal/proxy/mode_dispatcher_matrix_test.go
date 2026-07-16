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

// stubMatrixEngine supports Execute and ExecuteStream for matrix tests.
type stubMatrixEngine struct {
	output *pipeline.PipelineOutput
	chunks []pipeline.PipelineStreamResult
}

func (s *stubMatrixEngine) Execute(context.Context, string, *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	return s.output, nil
}

func (s *stubMatrixEngine) HasPipeline(string) bool { return true }

func (s *stubMatrixEngine) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *stubMatrixEngine) ExecuteStream(context.Context, string, *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	ch := make(chan pipeline.PipelineStreamResult, len(s.chunks)+1)
	for _, c := range s.chunks {
		ch <- c
	}
	if s.output != nil {
		ch <- pipeline.PipelineStreamResult{Output: s.output}
	}
	close(ch)
	return ch, nil
}

func TestDispatch_WithThinkSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		content          string
		wantVisible      string
		wantReasoning    string
		wantNoReasoning  bool
	}{
		{
			name:            "content with think tag",
			content:         "Hello <think>Let me think about this</think> World",
			wantVisible:     "Hello  World",
			wantReasoning:   "Let me think about this",
		},
		{
			name:            "content without think tag",
			content:         "Plain response without any thinking",
			wantVisible:     "Plain response without any thinking",
			wantNoReasoning: true,
		},
		{
			name:            "only think tag",
			content:         "<think>I am reasoning</think>",
			wantVisible:     "",
			wantReasoning:   "I am reasoning",
		},
		{
			name:            "think tag at start",
			content:         "<think>reasoning</think> Here is my answer",
			wantVisible:     " Here is my answer",
			wantReasoning:   "reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &stubMatrixEngine{
				output: &pipeline.PipelineOutput{
					Content: tt.content,
					ExecutionLog: &pipeline.ExecutionLog{
						TotalTokens: 10,
						Success:     true,
					},
				},
			}
			dispatcher := NewModeDispatcher(engine, nil, nil)
			dispatcher.SetStreamFakeConfig(StreamFakeConfig{Enabled: false})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Request.Header.Set("X-Request-ID", "req-test-"+tt.name)

			req := &plugin.ProxyRequest{
				Model: "gpt-4",
				Messages: []plugin.Message{
					{Role: "user", Content: "hello"},
				},
			}

			if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
				t.Fatalf("Dispatch() error = %v", err)
			}

			body := w.Body.String()
			var resp map[string]interface{}
			if err := json.Unmarshal([]byte(body), &resp); err != nil {
				t.Fatalf("failed to parse response JSON: %v", err)
			}

			choices, ok := resp["choices"].([]interface{})
			if !ok || len(choices) == 0 {
				t.Fatalf("expected choices in response, got: %s", body)
			}
			choice, ok := choices[0].(map[string]interface{})
			if !ok {
				t.Fatalf("choice is not a map: %v", choices[0])
			}
			message, ok := choice["message"].(map[string]interface{})
			if !ok {
				t.Fatalf("message is not a map: %v", choice["message"])
			}

			if got, ok := message["content"].(string); !ok || got != tt.wantVisible {
				t.Errorf("content = %q, want %q", got, tt.wantVisible)
			}

			if tt.wantNoReasoning {
				if _, exists := message["reasoning_content"]; exists {
					t.Errorf("reasoning_content should not exist, but found")
				}
			} else {
				got, ok := message["reasoning_content"].(string)
				if !ok || got != tt.wantReasoning {
					t.Errorf("reasoning_content = %q, want %q", got, tt.wantReasoning)
				}
			}
		})
	}
}

func TestDispatch_WithStreamFake(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubMatrixEngine{
		chunks: []pipeline.PipelineStreamResult{
			{Chunk: &plugin.StreamChunk{Content: "Hello"}},
			{Chunk: &plugin.StreamChunk{Content: " World"}},
		},
		output: &pipeline.PipelineOutput{
			ExecutionLog: &pipeline.ExecutionLog{
				TotalTokens: 10,
				Success:     true,
			},
			FinishReason: "stop",
		},
	}

	dispatcher := NewModeDispatcher(engine, nil, nil)
	dispatcher.SetStreamFakeConfig(DefaultStreamFakeConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-stream-fake")

	req := &plugin.ProxyRequest{
		Model:  "gpt-4",
		Stream: false,
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}

	if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	body := w.Body.String()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected choices in response, got: %s", body)
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choice is not a map: %v", choices[0])
	}
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("message is not a map: %v", choice["message"])
	}

	content, ok := message["content"].(string)
	if !ok || content != "Hello World" {
		t.Errorf("content = %q, want %q", content, "Hello World")
	}
}

func TestDispatch_StreamFakeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubMatrixEngine{
		output: &pipeline.PipelineOutput{
			Content: "Direct non-stream response",
			ExecutionLog: &pipeline.ExecutionLog{
				TotalTokens: 5,
				Success:     true,
			},
		},
	}

	dispatcher := NewModeDispatcher(engine, nil, nil)
	dispatcher.SetStreamFakeConfig(StreamFakeConfig{Enabled: false})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-no-fake")

	req := &plugin.ProxyRequest{
		Model:  "gpt-4",
		Stream: false,
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}

	if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	body := w.Body.String()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected choices, got: %s", body)
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choice is not a map")
	}
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("message is not a map")
	}

	content, ok := message["content"].(string)
	if !ok || content != "Direct non-stream response" {
		t.Errorf("content = %q, want %q", content, "Direct non-stream response")
	}
}

func TestWriteStreamResponse_WithThinkSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		chunks           []pipeline.PipelineStreamResult
		wantContentContains   []string
		wantContentExcludes   []string
		wantReasoningContains []string
		wantReasoningExcludes []string
		wantNoThinkTags       bool
	}{
		{
			name: "chunks with think tag",
			chunks: []pipeline.PipelineStreamResult{
				{Chunk: &plugin.StreamChunk{Content: "respond "}},
				{Chunk: &plugin.StreamChunk{Content: "<think>reasoning text</think> answer"}},
			},
			wantContentContains:   []string{`"content":" answer"`},
			wantContentExcludes:   []string{},
			wantReasoningContains: []string{`"reasoning_content":"reasoning text"`},
			wantNoThinkTags:       true,
		},
		{
			name: "chunks with only thinking content",
			chunks: []pipeline.PipelineStreamResult{
				{Chunk: &plugin.StreamChunk{Content: "<think>I'm reasoning</think>"}},
			},
			wantContentContains:   []string{},
			wantContentExcludes:   []string{},
			wantReasoningContains: []string{`"reasoning_content":"I'm reasoning"`},
			wantNoThinkTags:       true,
		},
		{
			name: "chunks without think tags",
			chunks: []pipeline.PipelineStreamResult{
				{Chunk: &plugin.StreamChunk{Content: "Hello"}},
				{Chunk: &plugin.StreamChunk{Content: " World"}},
			},
			wantContentContains:   []string{`"content":"Hello"`, `"content":" World"`},
			wantNoThinkTags:       true,
		},
		{
			name: "chunk with think tag and visible content",
			chunks: []pipeline.PipelineStreamResult{
				{Chunk: &plugin.StreamChunk{
					Content: fmt.Sprintf("<think>reasoning: %s</think> %s", "why X", "final answer"),
				}},
			},
			wantContentContains:   []string{`"content":" final answer"`},
			wantReasoningContains: []string{`"reasoning_content":"reasoning: why X"`},
			wantNoThinkTags:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &stubMatrixEngine{
				output: &pipeline.PipelineOutput{
					Content: "",
					ExecutionLog: &pipeline.ExecutionLog{
						TotalTokens: 10,
						Success:     true,
					},
				},
				chunks: tt.chunks,
			}

			dispatcher := NewModeDispatcher(engine, nil, nil)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Request.Header.Set("X-Pipeline-ID", "matrix-test")
			c.Set("protocol_plugin", "openai-protocol")

			resultCh, err := engine.ExecuteStream(context.Background(), "matrix-test", &pipeline.PipelineInput{Content: "hi"})
			if err != nil {
				t.Fatalf("ExecuteStream: %v", err)
			}
			if err := dispatcher.writeStreamResponse(c, resultCh, ModeDirectBackend, "test"); err != nil {
				t.Fatalf("writeStreamResponse: %v", err)
			}

			body := w.Body.String()

			for _, s := range tt.wantContentContains {
				if !strings.Contains(body, s) {
					t.Errorf("body should contain %q in content, got: %s", s, body)
				}
			}
			for _, s := range tt.wantContentExcludes {
				if strings.Contains(body, s) {
					t.Errorf("body should NOT contain %q in content, got: %s", s, body)
				}
			}
			for _, s := range tt.wantReasoningContains {
				if !strings.Contains(body, s) {
					t.Errorf("body should contain %q in reasoning_content, got: %s", s, body)
				}
			}
			for _, s := range tt.wantReasoningExcludes {
				if strings.Contains(body, s) {
					t.Errorf("body should NOT contain %q in reasoning_content, got: %s", s, body)
				}
			}
			if tt.wantNoThinkTags {
				if strings.Contains(body, "<think>") || strings.Contains(body, "</think>") {
					t.Errorf("body should NOT contain think tags, got: %s", body)
				}
			}
		})
	}
}

func TestDispatchStream_WithThinkSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubMatrixEngine{
		chunks: []pipeline.PipelineStreamResult{
			{Chunk: &plugin.StreamChunk{Content: "prefix "}},
			{Chunk: &plugin.StreamChunk{Content: "<think>inner monologue</think> suffix"}},
		},
		output: &pipeline.PipelineOutput{
			Content: "",
			ExecutionLog: &pipeline.ExecutionLog{
				TotalTokens:  10,
				Success:      true,
			},
			FinishReason: "stop",
		},
	}

	dispatcher := NewModeDispatcher(engine, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-stream-thinksplit")
	c.Set("protocol_plugin", "openai-protocol")

	req := &plugin.ProxyRequest{
		Model:  "gpt-4",
		Stream: true,
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}

	if err := dispatcher.DispatchStream(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("DispatchStream() error = %v", err)
	}

	body := w.Body.String()

	if !strings.Contains(body, `"content":"prefix "`) {
		t.Errorf("body should contain 'prefix ' in first chunk, got: %s", body)
	}
	if !strings.Contains(body, `"content":" suffix"`) {
		t.Errorf("body should contain ' suffix' in second chunk, got: %s", body)
	}

	if strings.Contains(body, "<think>") || strings.Contains(body, "</think>") {
		t.Errorf("body should NOT contain think tags, got: %s", body)
	}

	if !strings.Contains(body, `"reasoning_content":"inner monologue"`) {
		t.Errorf("body should contain reasoning_content for inner monologue, got: %s", body)
	}
}

func TestDispatch_StreamFakePreservesFinishReason(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &stubMatrixEngine{
		chunks: []pipeline.PipelineStreamResult{
			{Chunk: &plugin.StreamChunk{Content: "Tool response", FinishReason: "tool_calls"}},
		},
		output: &pipeline.PipelineOutput{
			ExecutionLog: &pipeline.ExecutionLog{
				TotalTokens: 10,
				Success:     true,
			},
			FinishReason: "tool_calls",
		},
	}

	dispatcher := NewModeDispatcher(engine, nil, nil)
	dispatcher.SetStreamFakeConfig(DefaultStreamFakeConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Request-ID", "req-finish")

	req := &plugin.ProxyRequest{
		Model:  "gpt-4",
		Stream: false,
		Messages: []plugin.Message{
			{Role: "user", Content: "hello"},
		},
	}

	if err := dispatcher.Dispatch(c, ModeDirectBackend, req); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) {
		t.Errorf("expected finish_reason=tool_calls in response, got: %s", body)
	}
}
