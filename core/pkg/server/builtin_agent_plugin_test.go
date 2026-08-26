package server

import (
	"strings"
	"testing"

	"centag/core/internal/agent/skills"
)

func TestLoadBuiltinSkillPlugins(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil, want non-nil registry")
	}
	plugins := reg.ListAll()
	if len(plugins) != 7 {
		t.Fatalf("builtin skill count = %d, want 7 (status-check/config-analysis/error-diagnosis/log-analysis/strategy-recommend/billing-audit/cost-analysis)", len(plugins))
	}
	wantNames := []string{"status-check", "config-analysis", "error-diagnosis", "log-analysis", "strategy-recommend", "billing-audit", "cost-analysis"}
	got := make(map[string]bool)
	for _, p := range plugins {
		if !p.Internal() {
			t.Errorf("skill %q should be internal", p.GetSkillDefinition().Name)
		}
		got[p.GetSkillDefinition().Name] = true
		if p.GetSkillDefinition().SystemPrompt == "" {
			t.Errorf("skill %q has empty system_prompt", p.GetSkillDefinition().Name)
		}
	}
	for _, n := range wantNames {
		if !got[n] {
			t.Errorf("builtin skill %q not loaded", n)
		}
	}
}

func TestSkillFromPluginProducesPrompt(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil")
	}
	p, ok := reg.Get("status-check")
	if !ok {
		t.Fatal("status-check not loaded")
	}
	skill := skills.SkillFromPlugin(p)
	if skill.Name != "status-check" {
		t.Errorf("skill.Name = %q, want status-check", skill.Name)
	}
	prompt := skill.BuildPrompt("请检查状态")
	if !strings.Contains(prompt, "你是一个 centag 运维助手，正在执行 skill: status-check") {
		t.Errorf("prompt missing skill header:\n%s", prompt)
	}
	if !strings.Contains(prompt, "用户请求: 请检查状态") {
		t.Errorf("prompt missing user request:\n%s", prompt)
	}
	if !strings.Contains(prompt, "请按照以下步骤执行") {
		t.Errorf("prompt missing steps section:\n%s", prompt)
	}
	// manifest 权威：prompt 不应回退到 BuildPrompt 默认模板
	if strings.Contains(prompt, "正在执行 skill: 状态检查") {
		t.Errorf("prompt should use manifest prompt, got fallback")
	}
}
