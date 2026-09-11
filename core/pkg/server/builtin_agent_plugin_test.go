package server

import (
	"strings"
	"testing"

	agentpkg "centag/core/internal/agent"
	"centag/core/internal/agent/skills"
	"centag/core/internal/agent/tools"
)

func TestLoadBuiltinSkillPlugins(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil, want non-nil registry")
	}
	plugins := reg.ListAll()
	if len(plugins) != 10 {
		t.Fatalf("builtin skill count = %d, want 10 (...self-evolution/evolution-smoke/evolution-dryrun)", len(plugins))
	}
	wantNames := []string{"status-check", "config-analysis", "error-diagnosis", "log-analysis", "strategy-recommend", "billing-audit", "cost-analysis", "self-evolution", "evolution-smoke", "evolution-dryrun"}
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

func TestSelfEvolutionSkillExposesEvolutionTools(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil")
	}
	p, ok := reg.Get("self-evolution")
	if !ok {
		t.Fatal("self-evolution skill not loaded")
	}
	if !p.Enabled() || !p.Internal() {
		t.Fatalf("self-evolution should be enabled and internal")
	}
	skillDef := p.GetSkillDefinition()
	for _, name := range []string{"propose_change", "dryrun_request", "apply_change", "measure_effect", "rollback_change", "record_learning"} {
		if !strings.Contains(skillDef.SystemPrompt, name) {
			t.Errorf("self-evolution prompt missing tool %q", name)
		}
	}
	// 工具集 ∩ 全局白名单后，evolution 工具必须仍可注册（unionSkillTools 语义）
	allowed := 	agentpkg.DefaultAgentConfig().Tools.Allowed
	effective := tools.IntersectAllowedTools(skillDef.Tools, allowed)
	got := make(map[string]bool, len(effective))
	for _, n := range effective {
		got[n] = true
	}
	for _, n := range []string{"propose_change", "dryrun_request", "measure_effect", "record_learning"} {
		if !got[n] {
			t.Errorf("evolution tool %q dropped after whitelist intersection", n)
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
