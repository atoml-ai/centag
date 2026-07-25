package pipeline

import (
	"strings"
	"testing"
)

// TestModeTemplates_TransparentDirectFixedEgressContract 锁定透明/直连/#j 模板语义契约。
func TestModeTemplates_TransparentDirectFixedEgressContract(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))

	// v2.0 语义：transparent-proxy 使用 transparent_forward 节点（不再用 generator 节点），
	// 用户原始请求（model/params/messages）原样转发到系统配置的默认后端 endpoint。
	// 不注入 system_prompt。
	t.Run("transparent-proxy uses transparent_forward and no system_prompt", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "transparent-proxy")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(p.Nodes) == 0 || p.Nodes[0].Type != NodeTypeTransparentForward {
			t.Fatalf("first node type = %v, want transparent_forward", p.Nodes[0].Type)
		}
		if sp := strings.TrimSpace(p.Nodes[0].Config.SystemPrompt); sp != "" {
			t.Fatalf("transparent-proxy must not inject system_prompt, got %q", sp)
		}
		if p.ShortcutCode != "#t" {
			t.Fatalf("shortcut = %q, want #t", p.ShortcutCode)
		}
		strategy, _ := p.Nodes[0].Config.CustomConfig["system_prompt_strategy"].(string)
		if strategy != "passthrough" {
			t.Fatalf("system_prompt_strategy=%q, want passthrough", strategy)
		}
	})

	// direct-backend 使用 transparent_forward + 注入非空 system_prompt（inject_system_prompt=true）。
	t.Run("direct-backend injects non-empty system_prompt", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "direct-backend")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(p.Nodes) == 0 || p.Nodes[0].Type != NodeTypeTransparentForward {
			t.Fatalf("first node type = %v, want transparent_forward", p.Nodes[0].Type)
		}
		sp := strings.TrimSpace(p.Nodes[0].Config.SystemPrompt)
		if sp == "" {
			t.Fatal("direct-backend must have non-empty system_prompt")
		}
		if !strings.Contains(sp, "可靠助手") {
			t.Fatalf("system_prompt should match refined default persona, got %q", sp)
		}
		if p.ShortcutCode != "#d" {
			t.Fatalf("shortcut = %q, want #d", p.ShortcutCode)
		}
		strategy, _ := p.Nodes[0].Config.CustomConfig["system_prompt_strategy"].(string)
		if strategy != "replace" {
			t.Fatalf("system_prompt_strategy=%q, want replace", strategy)
		}
	})

	// fixed-egress（#j）使用 transparent_forward + route_policy=fixed。
	t.Run("fixed-egress uses transparent_forward with fixed route", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "fixed-egress")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(p.Nodes) == 0 || p.Nodes[0].Type != NodeTypeTransparentForward {
			t.Fatalf("first node type = %v, want transparent_forward", p.Nodes[0].Type)
		}
		if p.ShortcutCode != "#j" {
			t.Fatalf("shortcut = %q, want #j", p.ShortcutCode)
		}
		routePolicy, _ := p.Nodes[0].Config.CustomConfig["route_policy"].(string)
		if routePolicy != "fixed" {
			t.Fatalf("route_policy = %q, want fixed", routePolicy)
		}
	})
}
