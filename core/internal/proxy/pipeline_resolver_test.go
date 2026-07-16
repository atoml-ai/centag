package proxy

import (
	"context"
	"testing"

	"centag/core/pkg/pipeline"
)

type stubPipelineEngine struct {
	ids map[string]bool
}

func (s *stubPipelineEngine) Execute(context.Context, string, *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	return nil, nil
}

func (s *stubPipelineEngine) HasPipeline(id string) bool {
	return s.ids[id]
}

func (s *stubPipelineEngine) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *stubPipelineEngine) ExecuteStream(context.Context, string, *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	return nil, nil
}

func TestPipelineResolver_ByPipelineID(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"my-custom-flow": true}}
	r := NewPipelineResolver(engine, nil, nil)

	if got := r.Resolve("my-custom-flow", ""); got != "my-custom-flow" {
		t.Fatalf("Resolve() = %q, want my-custom-flow", got)
	}
}

func TestPipelineResolver_ByRegistryShortcut(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"cache-hit": true}}
	reg := pipeline.NewPipelineRegistry()
	_ = reg.Register(&pipeline.AgentPatternPipeline{
		ID:           "cache-hit",
		Name:         "Cache",
		ShortcutCode: "#ch",
		Nodes:        []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
	})

	r := NewPipelineResolver(engine, reg, nil)
	if got := r.Resolve("#ch", ""); got != "cache-hit" {
		t.Fatalf("Resolve(#ch) = %q, want cache-hit", got)
	}
	if got := r.Resolve(ModeCacheHit, ""); got != "cache-hit" {
		t.Fatalf("Resolve(cache-hit) = %q, want cache-hit", got)
	}
}

func TestPipelineResolver_RAGMode(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"rag-mode": true}}
	reg := pipeline.NewPipelineRegistry()
	_ = reg.Register(&pipeline.AgentPatternPipeline{
		ID:           "rag-mode",
		Name:         "RAG",
		ShortcutCode: "#rag",
		Nodes:        []pipeline.PipelineNodeConfig{{ID: "cache_read", Type: pipeline.NodeTypeCache}},
	})

	r := NewPipelineResolver(engine, reg, nil)
	for _, mode := range []string{"#rag", "rag-mode"} {
		if got := r.Resolve(ProxyMode(mode), ""); got != "rag-mode" {
			t.Fatalf("Resolve(%q) = %q, want rag-mode", mode, got)
		}
	}
}

func TestPipelineResolver_ModelMatchingShortcut(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"model-matching": true}}
	reg := pipeline.NewPipelineRegistry()
	_ = reg.Register(&pipeline.AgentPatternPipeline{
		ID:           "model-matching",
		Name:         "Model Matching",
		ShortcutCode: "#m",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "classifier", Type: pipeline.NodeTypeRouter},
		},
	})

	r := NewPipelineResolver(engine, reg, nil)
	for _, mode := range []ProxyMode{"#m", ModeModelMatching, "model-matching"} {
		if got := r.Resolve(mode, ""); got != "model-matching" {
			t.Fatalf("Resolve(%q) = %q, want model-matching", mode, got)
		}
	}
}

func TestPipelineResolver_IntentClassificationAlias(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"router-mode": true}}
	r := NewPipelineResolver(engine, nil, nil)

	for _, mode := range []string{"#c", "intent-classification"} {
		if got := r.Resolve(ProxyMode(mode), ""); got != "router-mode" {
			t.Fatalf("Resolve(%q) = %q, want router-mode", mode, got)
		}
	}
}

func TestPipelineResolver_HeaderPipelineID(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"tenant-flow": true}}
	r := NewPipelineResolver(engine, nil, nil)
	if got := r.Resolve(ModeDefault, "tenant-flow"); got != "tenant-flow" {
		t.Fatalf("Resolve(header) = %q, want tenant-flow", got)
	}
}