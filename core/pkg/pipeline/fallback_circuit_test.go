package pipeline

import (
	"context"
	"strings"
	"testing"

	"centag/core/pkg/config"
)

func TestNodeBackendID_ResolvesVirtualVars(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "default-be",
			FallbackBackendID: "fallback-be",
		},
	})
	defer config.Set(prev)

	got := nodeBackendID(PipelineNodeConfig{Backend: "{{system.default_backend}}"}, NodeConfig{})
	if got != "default-be" {
		t.Fatalf("default backend = %q", got)
	}
	got = nodeBackendID(PipelineNodeConfig{}, NodeConfig{Backend: "{{system.fallback_backend}}"})
	if got != "fallback-be" {
		t.Fatalf("fallback backend = %q", got)
	}
}

func TestExecute_PrimarySuccess_WithFallbackGroup(t *testing.T) {
	// 回归：主节点成功时 executeFallbackGroup 必须返回 groupOK=true，
	// 否则透明代理等带 FallbackGroups 的流水线会误报 all fallback attempts failed。
	mockClient := &testBackendClient{response: "from-primary"}
	mockBroker := &testCapabilityBroker{llmClient: mockClient}
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes: %v", err)
	}
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, mockBroker, NewPipelineLogger(), nil)

	pipeline := &AgentPatternPipeline{
		ID:   "primary-ok",
		Name: "Primary OK",
		Nodes: []PipelineNodeConfig{
			{
				ID:        "forward",
				Type:      NodeTypeGenerator,
				Kind:      "llm.generate",
				Backend:   "ok-backend",
				Model:     "m",
				Config:    NodeConfig{Backend: "ok-backend", Model: "m", PromptTemplate: "{{input}}"},
				NextNodes: []string{"forward_fallback"},
			},
			{
				ID:        "forward_fallback",
				Type:      NodeTypeGenerator,
				Kind:      "llm.generate",
				Backend:   "fb-backend",
				Model:     "m",
				Config:    NodeConfig{Backend: "fb-backend", Model: "m", PromptTemplate: "{{input}}"},
				DependsOn: []string{"forward"},
			},
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			BypassOnError: true,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "forward", FallbackNodes: []string{"forward_fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := pipelineRegistry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	out, err := engine.Execute(context.Background(), "primary-ok", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Content, "from-primary") {
		t.Fatalf("content=%q, want from-primary", out.Content)
	}

	ch, err := engine.ExecuteStream(context.Background(), "primary-ok", &PipelineInput{Content: "hello", Stream: true})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	var streamOut string
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			t.Fatalf("stream error: %v", res.Chunk.Error)
		}
		if res.Output != nil {
			streamOut = res.Output.Content
		}
	}
	if !strings.Contains(streamOut, "from-primary") {
		t.Fatalf("stream content=%q, want from-primary", streamOut)
	}
}

func TestExecuteStream_PrimaryCircuitOpen_RunsFallbackGroup(t *testing.T) {
	IsCircuitOpen = func(backendID string) bool {
		return backendID == "broken-backend"
	}
	defer func() { IsCircuitOpen = nil }()

	mockClient := &testBackendClient{response: "from-fallback"}
	mockBroker := &testCapabilityBroker{llmClient: mockClient}
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes: %v", err)
	}
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, mockBroker, NewPipelineLogger(), nil)

	pipeline := &AgentPatternPipeline{
		ID:   "stream-fallback",
		Name: "Stream Fallback",
		Nodes: []PipelineNodeConfig{
			{
				ID:     "primary",
				Type:   NodeTypeGenerator,
				Kind:   "llm.generate",
				Backend: "broken-backend",
				Model:  "m",
				Config: NodeConfig{Backend: "broken-backend", Model: "m", PromptTemplate: "{{input}}"},
				NextNodes: []string{"fallback"},
			},
			{
				ID:     "fallback",
				Type:   NodeTypeGenerator,
				Kind:   "llm.generate",
				Backend: "healthy-backend",
				Model:  "m",
				Config: NodeConfig{Backend: "healthy-backend", Model: "m", PromptTemplate: "{{input}}"},
				DependsOn: []string{"primary"},
			},
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			BypassOnError: true,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "primary", FallbackNodes: []string{"fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := pipelineRegistry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ch, err := engine.ExecuteStream(context.Background(), "stream-fallback", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	var got string
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			t.Fatalf("stream error: %v", res.Chunk.Error)
		}
		if res.Output != nil {
			got = res.Output.Content
		}
	}
	if !strings.Contains(got, "from-fallback") {
		t.Fatalf("content=%q, want from-fallback", got)
	}
}

func TestFilterFallbackNodesByCircuit(t *testing.T) {
	IsCircuitOpen = func(backendID string) bool {
		return backendID == "broken-backend"
	}
	defer func() {
		IsCircuitOpen = nil
	}()

	engine := NewPipelineEngine(NewNodeRegistry(), NewPipelineRegistry(), nil, NewPipelineLogger(), nil)
	pipeline := &AgentPatternPipeline{
		Nodes: []PipelineNodeConfig{
			{ID: "fb-open", Type: NodeTypeGenerator, Backend: "broken-backend", Model: "m"},
			{ID: "fb-ok", Type: NodeTypeGenerator, Backend: "healthy-backend", Model: "m"},
		},
	}
	graph := NewExecutionGraph(pipeline)

	filtered := engine.filterFallbackNodesByCircuit(context.Background(), graph, []string{"fb-open", "fb-ok"})
	if len(filtered) != 1 || filtered[0] != "fb-ok" {
		t.Fatalf("filtered = %v, want [fb-ok]", filtered)
	}
	if graph.GetNode("fb-open").Status != StatusSkipped {
		t.Fatalf("fb-open status = %v, want skipped", graph.GetNode("fb-open").Status)
	}
}

func TestExecuteNode_SkipsCircuitOpenBackend(t *testing.T) {
	IsCircuitOpen = func(backendID string) bool {
		return backendID == "broken-backend"
	}
	defer func() { IsCircuitOpen = nil }()

	nodeRegistry := NewNodeRegistry()
	RegisterBuiltinNodes(nodeRegistry)
	engine := NewPipelineEngine(nodeRegistry, NewPipelineRegistry(), nil, NewPipelineLogger(), nil)

	_, err := engine.executeNode(context.Background(), PipelineNodeConfig{
		ID:      "primary",
		Type:    NodeTypeGenerator,
		Backend: "broken-backend",
		Model:   "m",
	}, &NodeInput{Content: "hello"})
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if !isCircuitBreakerSkipError(err) {
		t.Fatalf("unexpected error: %v", err)
	}
}

