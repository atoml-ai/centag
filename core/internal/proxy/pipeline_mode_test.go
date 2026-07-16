package proxy

import (
	"testing"

	"centag/core/pkg/pipeline"
)

func TestResolvePipelineID_WithoutRegistry(t *testing.T) {
	if got := ResolvePipelineID("#nonexistent"); got != "" {
		t.Fatalf("ResolvePipelineID without registry = %q, want empty", got)
	}
}

func TestDefaultModeMappings_DiagnosticsOnly(t *testing.T) {
	if len(defaultModeMappings) == 0 {
		t.Fatal("expected diagnostic defaultModeMappings entries")
	}
	foundRaw := false
	for _, mapping := range defaultModeMappings {
		if mapping.PipelineID == "" {
			t.Errorf("diagnostic mapping %s has empty PipelineID", mapping.Mode)
		}
		if mapping.Mode == ModeRawForward {
			foundRaw = true
			if mapping.PipelineID != "raw-forward" {
				t.Errorf("ModeRawForward pipeline = %q, want raw-forward", mapping.PipelineID)
			}
		}
	}
	if !foundRaw {
		t.Fatal("expected ModeRawForward in defaultModeMappings")
	}
}

func TestPipelineResolver_CustomUserPipeline(t *testing.T) {
	engine := &stubPipelineEngine{ids: map[string]bool{"user-analytics": true}}
	reg := pipeline.NewPipelineRegistry()
	_ = reg.Register(&pipeline.AgentPatternPipeline{
		ID:           "user-analytics",
		Name:         "用户分析",
		ShortcutCode: "#ua",
		Nodes:        []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
	})

	r := NewPipelineResolver(engine, reg, nil)
	if got := r.Resolve("#ua", ""); got != "user-analytics" {
		t.Fatalf("custom shortcut resolved to %q, want user-analytics", got)
	}
}