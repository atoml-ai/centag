package pipeline

import (
	"testing"

	"centag/core/pkg/bootstrap"
)

func TestSmartSchedulingTemplate_UsesSchedulerNode(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadPipelineTemplate(t, "smart-scheduling")
	p := CreatePipelineFromTemplate(tmpl, nil)
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.ShortcutCode != "#s" {
		t.Fatalf("shortcut = %q, want #s", p.ShortcutCode)
	}

	schedulerNode, generatorNode := false, false
	for _, n := range p.Nodes {
		switch n.ID {
		case "scheduler":
			if n.Type != NodeTypeScheduler {
				t.Fatalf("scheduler type = %v", n.Type)
			}
			schedulerNode = true
		case "generator":
			if len(n.DependsOn) != 1 || n.DependsOn[0] != "scheduler" {
				t.Fatalf("generator depends_on = %v", n.DependsOn)
			}
			generatorNode = true
		}
	}
	if !schedulerNode || !generatorNode {
		t.Fatalf("topology incomplete: scheduler=%v generator=%v", schedulerNode, generatorNode)
	}
}

func mustLoadPipelineTemplate(t *testing.T, id string) PatternTemplate {
	t.Helper()
	for _, raw := range bootstrap.LoadInitialPipelineTemplatesFromFiles() {
		if raw.ID == id {
			return convertBootstrapTemplate(raw)
		}
	}
	t.Fatalf("template %q not loaded from initdata", id)
	return PatternTemplate{}
}