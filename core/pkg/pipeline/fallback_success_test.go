package pipeline

import (
	"context"
	"strings"
	"testing"
)

func TestFallbackRecovery_ExecutionLogSuccess(t *testing.T) {
	// 主节点失败、降级组恢复时，整体 ExecutionLog.Success 应为 true（此前被主节点失败日志拉成 false）
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
		ID:   "recovery-success",
		Name: "Recovery Success",
		Nodes: []PipelineNodeConfig{
			{
				ID:       "primary",
				Type:     NodeTypeGenerator,
				Kind:     "llm.generate",
				Backend:  "broken-backend",
				Model:    "m",
				Config:   NodeConfig{Backend: "broken-backend", Model: "m", PromptTemplate: "{{input}}"},
				NextNodes: []string{"fallback"},
			},
			{
				ID:        "fallback",
				Type:      NodeTypeGenerator,
				Kind:      "llm.generate",
				Backend:   "healthy-backend",
				Model:     "m",
				Config:    NodeConfig{Backend: "healthy-backend", Model: "m", PromptTemplate: "{{input}}"},
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

	out, err := engine.Execute(context.Background(), "recovery-success", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out == nil || out.ExecutionLog == nil {
		t.Fatal("missing execution log")
	}
	if !strings.Contains(out.Content, "from-fallback") {
		t.Fatalf("content=%q, want from-fallback", out.Content)
	}
	if !out.ExecutionLog.Success {
		t.Fatalf("ExecutionLog.Success = false, want true after fallback recovery")
	}
	if out.ExecutionLog.ErrorMessage != "" {
		t.Fatalf("ExecutionLog.ErrorMessage = %q, want empty after fallback recovery", out.ExecutionLog.ErrorMessage)
	}
}

func TestValidate_FallbackGroup(t *testing.T) {
	mk := func(primary string, fallbacks []string) *AgentPatternPipeline {
		return &AgentPatternPipeline{
			ID:   "fb-validate",
			Name: "FB Validate",
			Nodes: []PipelineNodeConfig{
				{ID: "primary", Type: NodeTypeGenerator, Kind: "llm.generate", Backend: "b", Model: "m"},
				{ID: "fallback", Type: NodeTypeGenerator, Kind: "llm.generate", Backend: "b2", Model: "m"},
			},
			GlobalConfig: GlobalPipelineConfig{
				FallbackGroups: []FallbackGroup{{PrimaryNodeID: primary, FallbackNodes: fallbacks}},
			},
		}
	}

	cases := []struct {
		name    string
		p       *AgentPatternPipeline
		wantErr string
	}{
		{"valid", mk("primary", []string{"fallback"}), ""},
		{"missing-primary", mk("ghost", []string{"fallback"}), "primary node ghost not found"},
		{"missing-fallback", mk("primary", []string{"ghost"}), "fallback node ghost not found"},
		{"self-fallback", mk("primary", []string{"primary"}), "cannot be the primary"},
		{"empty-fallback", mk("primary", nil), "fallback_nodes is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.p.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Validate() = %v, want err containing %q", err, c.wantErr)
			}
		})
	}
}
