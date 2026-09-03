package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// verdictE2EUpstream 构造返回固定状态码与 body 的 mock 上游。
func verdictE2EUpstream(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// newVerdictE2EEngine 构建带 httptest 上游解析的引擎（透明转发 + 固定出站）。
func newVerdictE2EEngine(t *testing.T, backendURLs map[string]string) (*PipelineEngine, *PipelineRegistry) {
	t.Helper()
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes: %v", err)
	}
	broker := &mockCapabilityBroker{httpClient: &http.Client{Timeout: 5 * time.Second}}
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, broker, NewPipelineLogger(), nil)

	prev := ResolveBackendEndpoint
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		u, ok := backendURLs[backendID]
		if !ok {
			return nil, nil
		}
		return &BackendEndpoint{BaseURL: u, APIKey: "sk-test"}, nil
	}
	t.Cleanup(func() { ResolveBackendEndpoint = prev })
	return engine, pipelineRegistry
}

func verdictE2EInput() *PipelineInput {
	return &PipelineInput{
		Content: "hi",
		Stream:  true,
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"deepseek-v4-flash-free","messages":[{"role":"user","content":"hi"}]}`,
		},
	}
}

func transparentForwardNodeConfig(id, backendID string, next []string, depends []string) PipelineNodeConfig {
	return PipelineNodeConfig{
		ID:             id,
		Type:           NodeTypeTransparentForward,
		Backend:        backendID,
		Implementation: "builtin.transparent_forward",
		Config: NodeConfig{
			Backend: backendID,
			CustomConfig: map[string]interface{}{
				"fixed_egress": true,
			},
		},
		NextNodes: next,
		DependsOn: depends,
	}
}

// TestUpstreamVerdictEndToFallbackRescue 覆盖 TC-IT-001（R02）：
// 实案复现——主上游 400+"Model is unavailable"，备用上游正常，客户端拿到备用回答。
func TestUpstreamVerdictEndToFallbackRescue(t *testing.T) {
	primarySrv := verdictE2EUpstream(t, 400,
		`{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`)
	defer primarySrv.Close()
	fbSrv := verdictE2EUpstream(t, 200,
		`{"id":"fb","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"fallback-ok"},"finish_reason":"stop"}]}`)
	defer fbSrv.Close()

	engine, registry := newVerdictE2EEngine(t, map[string]string{
		"be-primary": primarySrv.URL,
		"be-fb":      fbSrv.URL,
	})

	pipeline := &AgentPatternPipeline{
		ID:   "verdict-e2e-rescue",
		Name: "Verdict E2E Rescue",
		Nodes: []PipelineNodeConfig{
			transparentForwardNodeConfig("forward", "be-primary", []string{"forward_fallback"}, nil),
			transparentForwardNodeConfig("forward_fallback", "be-fb", nil, []string{"forward"}),
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "forward", FallbackNodes: []string{"forward_fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ch, err := engine.ExecuteStream(context.Background(), "verdict-e2e-rescue", verdictE2EInput())
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	var content string
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			t.Fatalf("stream error: %v", res.Chunk.Error)
		}
		if res.Chunk != nil && res.Chunk.Content != "" {
			content += res.Chunk.Content
		}
		if res.Output != nil && res.Output.Content != "" {
			content = res.Output.Content
		}
	}
	if !strings.Contains(content, "fallback-ok") {
		t.Fatalf("content=%q, want fallback upstream answer (fallback-ok)", content)
	}
}

// TestUpstreamVerdictEndToFallbackExhausted 覆盖 TC-IT-002（R02）：
// 主备上游均返回显式错误结构 → 降级耗尽，管线失败（网关错误，而非假成功错误体）。
func TestUpstreamVerdictEndToFallbackExhausted(t *testing.T) {
	errBody := `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`
	primarySrv := verdictE2EUpstream(t, 400, errBody)
	defer primarySrv.Close()
	fbSrv := verdictE2EUpstream(t, 503, errBody)
	defer fbSrv.Close()

	engine, registry := newVerdictE2EEngine(t, map[string]string{
		"be-primary": primarySrv.URL,
		"be-fb":      fbSrv.URL,
	})

	pipeline := &AgentPatternPipeline{
		ID:   "verdict-e2e-exhausted",
		Name: "Verdict E2E Exhausted",
		Nodes: []PipelineNodeConfig{
			transparentForwardNodeConfig("forward", "be-primary", []string{"forward_fallback"}, nil),
			transparentForwardNodeConfig("forward_fallback", "be-fb", nil, []string{"forward"}),
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "forward", FallbackNodes: []string{"forward_fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ch, err := engine.ExecuteStream(context.Background(), "verdict-e2e-exhausted", verdictE2EInput())
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	sawErr := false
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("pipeline must fail when both primary and fallback upstream return explicit error structures")
	}
}

// TestUpstreamVerdictEndToTrueClient400Passthrough 覆盖 TC-IT-003（R01 锚点）：
// 真客户端 400（无错误结构，如 context 超长）保持原样透传，零误报。
func TestUpstreamVerdictEndToTrueClient400Passthrough(t *testing.T) {
	srv := verdictE2EUpstream(t, 400, `{"detail":"context too long"}`)
	defer srv.Close()

	engine, registry := newVerdictE2EEngine(t, map[string]string{"be-primary": srv.URL})

	pipeline := &AgentPatternPipeline{
		ID:   "verdict-e2e-passthrough",
		Name: "Verdict E2E Passthrough",
		Nodes: []PipelineNodeConfig{
			transparentForwardNodeConfig("forward", "be-primary", nil, nil),
		},
		GlobalConfig: GlobalPipelineConfig{ParallelLimit: 1},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	input := verdictE2EInput()
	input.Stream = false
	out, err := engine.Execute(context.Background(), "verdict-e2e-passthrough", input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Content, "context too long") {
		t.Fatalf("content=%q, want raw upstream body passthrough (true client 400)", out.Content)
	}
}
