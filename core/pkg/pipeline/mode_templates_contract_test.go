package pipeline

import (
	"strings"
	"testing"
)

// TestModeTemplates_TransparentDirectRawContract 锁定透明/直连/#raw 模板语义契约。
func TestModeTemplates_TransparentDirectRawContract(t *testing.T) {
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
	})

	// direct-backend 使用 generator 节点并注入非空 system_prompt。
	t.Run("direct-backend injects non-empty system_prompt", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "direct-backend")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(p.Nodes) == 0 || p.Nodes[0].Type != NodeTypeGenerator {
			t.Fatalf("first node type = %v, want generator", p.Nodes[0].Type)
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
	})

	// raw-forward 始终使用 transparent_forward 节点（高级转发模式）。
	t.Run("raw-forward uses transparent_forward node", func(t *testing.T) {
		tmpl := mustLoadPipelineTemplate(t, "raw-forward")
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(p.Nodes) == 0 || p.Nodes[0].Type != NodeTypeTransparentForward {
			t.Fatalf("first node type = %v, want transparent_forward", p.Nodes[0].Type)
		}
		if p.ShortcutCode != "#raw" {
			t.Fatalf("shortcut = %q, want #raw", p.ShortcutCode)
		}
	})
}