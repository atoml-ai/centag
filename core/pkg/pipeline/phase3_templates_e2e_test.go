package pipeline

import (
	"testing"
)

func TestPhase3Templates_Validate(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	cases := []struct {
		id       string
		shortcut string
		first    string
	}{
		{"security-mode", "#sec", "security_check"},
		{"multilingual-support", "#cs", "cache_read"},
		{"geo-routing-mode", "#geo", "geo_router"},
		{"transparent-proxy", "#t", "forward"},
		{"fixed-egress", "#j", "forward"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			tmpl := mustLoadPipelineTemplate(t, tc.id)
			p := CreatePipelineFromTemplate(tmpl, nil)
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if p.ShortcutCode != tc.shortcut {
				t.Fatalf("shortcut = %q, want %s", p.ShortcutCode, tc.shortcut)
			}
			if len(p.Nodes) == 0 || p.Nodes[0].ID != tc.first {
				t.Fatalf("first node = %v, want %s", p.Nodes[0].ID, tc.first)
			}
		})
	}
}

func TestSecurityMode_ShortCircuitConditions(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadPipelineTemplate(t, "security-mode")
	p := CreatePipelineFromTemplate(tmpl, nil)

	conditions := map[string]string{}
	for _, n := range p.Nodes {
		conditions[n.ID] = n.Condition
	}

	if got := conditions["generator"]; got != "{{.security_check.metadata.passed}} == true" {
		t.Fatalf("generator condition = %q", got)
	}
	if got := conditions["pii_redactor"]; got != "{{.quality_audit.metadata.passed}} == true" {
		t.Fatalf("pii_redactor condition = %q", got)
	}
	if got := conditions["token_usage_record"]; got != "{{.security_check.metadata.passed}} == true" {
		t.Fatalf("token_usage_record condition = %q", got)
	}
}