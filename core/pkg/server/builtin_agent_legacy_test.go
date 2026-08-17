package server

import (
	"testing"

	"centag/core/internal/agent"
	"centag/core/internal/agent/skills"
	"centag/core/internal/agent/tools"
)

// R05/R09：旧客户端（不带 skill 字段）兼容性 —— 空 skill 走自动路由（agent-skill-router）。
func TestLegacyClientCompatibility(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	skillReg := skills.NewSkillRegistry()
	if reg != nil {
		for _, p := range reg.ListAll() {
			skillReg.RegisterSkill(skills.SkillFromPlugin(p))
		}
	}
	if len(skillReg.ListSkills()) == 0 {
		skills.LoadBuiltinSkills(skillReg)
	}
	h := &BuiltinAgentHandler{
		skillRegistry:       skillReg,
		skillPluginRegistry: reg,
		toolRegistry:        tools.NewToolRegistry(t.TempDir(), nil, nil),
		engine:              agent.NewRuntimeEngine(agent.DefaultAgentConfig(), t.TempDir(), nil),
	}

	if reg != nil {
		// 空 skill：自动路由，统一走 centag-ops-router
		if got := h.resolveSkillPipelineID(""); got != skills.CentagOpsRouterPipelineID {
			t.Errorf("empty skill pipeline = %q, want %q", got, skills.CentagOpsRouterPipelineID)
		}
	} else {
		if got := h.resolveSkillPipelineID(""); got != "" {
			t.Errorf("empty skill pipeline (no registry) = %q, want empty", got)
		}
	}
	if got := h.buildSystemPrompt(""); got == "" {
		t.Error("empty skill system prompt empty")
	}
	// 空 skill 时工具集 = 全部启用 skill 工具并集（会话级）
	if reg != nil {
		if got := h.skillTools(""); len(got) == 0 {
			t.Error("empty skill tools should be union of all skills, got empty")
		}
	}

	// 未知 skill：安全回退（pipeline 空、默认 prompt）
	if got := h.resolveSkillPipelineID("nonexistent-skill"); got != "" {
		t.Errorf("unknown skill pipeline = %q, want empty", got)
	}
	if got := h.buildSystemPrompt("nonexistent-skill"); got == "" {
		t.Error("unknown skill system prompt empty")
	}

	// 已知内置 skill：显式强制路由 + 工具集正常
	if reg != nil {
		if got := h.resolveSkillPipelineID("status-check"); got != skills.ForcedRoutePipelineID("status-check") {
			t.Errorf("status-check pipeline = %q, want %q", got, skills.ForcedRoutePipelineID("status-check"))
		}
		if got := h.skillTools("status-check"); len(got) == 0 {
			t.Error("status-check tools empty")
		}
	}
}
