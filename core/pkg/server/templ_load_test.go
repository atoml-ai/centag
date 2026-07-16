package server

import (
	"fmt"
	"centag/core/pkg/pipeline"
	"testing"
)

func TestLoadCacheTemplatesFromInitdata(t *testing.T) {
	templates := resolvePipelineTemplates()
	fmt.Printf("Loaded %d templates\n", len(templates))

	for _, tmpl := range templates {
		fmt.Printf("  - id=%q name=%q\n", tmpl.ID, tmpl.Name)
		if tmpl.ID == "cache-hit" || tmpl.ID == "cache-mode" {
			fmt.Printf("    >>> Found target template: %s\n", tmpl.ID)
			p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
			if p == nil {
				t.Fatalf("CreatePipelineFromTemplate returned nil for %s", tmpl.ID)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("Validation failed for %s: %v", tmpl.ID, err)
			}
			fmt.Printf("    >>> Validation OK for %s\n", tmpl.ID)
		}
	}
}
