package pipeline

import (
	"context"
	"testing"
)

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

	filtered := engine.filterFallbackNodesByCircuit(graph, []string{"fb-open", "fb-ok"})
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

