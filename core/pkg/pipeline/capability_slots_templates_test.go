package pipeline

import (
	"strings"
	"testing"
)

// TestCapabilitySlotsTemplates_Contract 锁定 v0.2.3 能力槽样板模板结构（coding / router / education）。
func TestCapabilitySlotsTemplates_Contract(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))

	t.Run("coding-agent is multi-stage with capability_slots", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "coding-agent")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p.ShortcutCode != "#code" {
			t.Fatalf("shortcut = %q, want #code", p.ShortcutCode)
		}

		wantNodes := []string{
			"stage_router",
			"architect-generator",
			"implement-generator",
			"review-generator",
			"test-generator",
		}
		byID := map[string]PipelineNodeConfig{}
		for _, n := range p.Nodes {
			byID[n.ID] = n
		}
		for _, id := range wantNodes {
			if _, ok := byID[id]; !ok {
				t.Fatalf("missing node %q", id)
			}
		}
		if byID["stage_router"].Type != NodeTypeRouter {
			t.Fatalf("stage_router type = %v, want router", byID["stage_router"].Type)
		}
		cc := byID["stage_router"].Config.CustomConfig
		if cc == nil {
			t.Fatal("stage_router custom_config is nil")
		}
		if got, _ := cc["default_route"].(string); got != "implement-generator" {
			t.Fatalf("default_route = %q, want implement-generator", got)
		}
		routes, _ := cc["routes"].(map[string]interface{})
		if len(routes) == 0 {
			// yaml may decode as map[string]string depending on loader
			if rs, ok := cc["routes"].(map[string]string); ok {
				routes = make(map[string]interface{}, len(rs))
				for k, v := range rs {
					routes[k] = v
				}
			}
		}
		if len(routes) == 0 {
			t.Fatal("stage_router routes empty")
		}
		// Spot-check keyword → stage mapping
		for kw, want := range map[string]string{
			"架构": "architect-generator",
			"实现": "implement-generator",
			"审查": "review-generator",
			"测试": "test-generator",
		} {
			got, _ := routes[kw].(string)
			if got != want {
				t.Fatalf("routes[%q] = %q, want %q", kw, got, want)
			}
		}

		slots := metadataStringSliceOfMaps(tmpl.Metadata, "capability_slots")
		if len(slots) < 4 {
			t.Fatalf("capability_slots len = %d, want >= 4", len(slots))
		}
		nodeIDs := map[string]bool{}
		for _, s := range slots {
			nid, _ := s["node_id"].(string)
			nodeIDs[nid] = true
		}
		for _, id := range []string{
			"architect-generator", "implement-generator", "review-generator", "test-generator",
		} {
			if !nodeIDs[id] {
				t.Fatalf("capability_slots missing node_id %q", id)
			}
		}
		for _, id := range []string{"architect-generator", "implement-generator"} {
			n := byID[id]
			if n.Backend == "bigmodel" {
				t.Fatalf("%s backend hard-coded to bigmodel", id)
			}
			if !strings.Contains(n.Backend, "system.default") {
				t.Fatalf("%s backend = %q, want system default placeholder", id, n.Backend)
			}
		}
	})

	t.Run("router-mode declares capability_slots", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "router-mode")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		slots := metadataStringSliceOfMaps(tmpl.Metadata, "capability_slots")
		if len(slots) < 4 {
			t.Fatalf("capability_slots len = %d, want >= 4", len(slots))
		}
		targets := metadataStringSliceOfMaps(tmpl.Metadata, "route_model_targets")
		if len(targets) < 4 {
			t.Fatalf("route_model_targets dual-write missing, len=%d", len(targets))
		}
	})

	t.Run("education-scene slots and no bigmodel seed", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "education-scene")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p.ShortcutCode != "#edu" {
			t.Fatalf("shortcut = %q, want #edu", p.ShortcutCode)
		}
		slots := metadataStringSliceOfMaps(tmpl.Metadata, "capability_slots")
		if len(slots) < 6 {
			t.Fatalf("capability_slots len = %d, want >= 6", len(slots))
		}
		for _, n := range p.Nodes {
			if n.Type != NodeTypeGenerator {
				continue
			}
			if n.Backend == "bigmodel" {
				t.Fatalf("node %s backend still hard-codes bigmodel", n.ID)
			}
		}
	})
}

func metadataStringSliceOfMaps(meta map[string]interface{}, key string) []map[string]interface{} {
	if meta == nil {
		return nil
	}
	raw, ok := meta[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return v
	default:
		return nil
	}
}
