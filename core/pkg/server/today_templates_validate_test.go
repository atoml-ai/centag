package server

import (
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/pipeline"
)

func TestTodayTemplatesValidateWithProjectRoot(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))

	// Builtin initdata only; business templates (e.g. security-mode) live outside this repo.
	// centag-ops-router is intentionally absent: it is generated from skill manifests
	// (single source of truth), not seeded from an initdata template.
	targets := []string{"transparent", "router-pipeline", "cache-pipeline"}
	found := make(map[string]bool)

	for _, tmpl := range resolvePipelineTemplates() {
		for _, id := range targets {
			if tmpl.ID != id {
				continue
			}
			found[id] = true
			p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
			if p == nil {
				t.Fatalf("CreatePipelineFromTemplate nil for %s", id)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("validate %s: %v", id, err)
			}
		}
	}

	for _, id := range targets {
		if !found[id] {
			t.Fatalf("template %q not loaded", id)
		}
	}
}

func mustFindProjectRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(root, "config", "initdata", "pipeline-templates")); err == nil {
			return root
		}
		root = filepath.Dir(root)
	}
	t.Fatal("could not find project root")
	return ""
}