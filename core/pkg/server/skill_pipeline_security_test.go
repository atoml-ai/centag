package server

import (
	"testing"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"
)

// R11：manifest↔pipeline 双向校验 —— 生成 router pipeline 的 system_prompt 与 manifest 一致。
func TestSkillPipelineManifestRoundtrip(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil")
	}
	pp, err := BuildSkillRouterPipeline(reg.ListAll(), "b", "m")
	if err != nil {
		t.Fatalf("BuildSkillRouterPipeline: %v", err)
	}
	// 单一路由管线：id 与共享 AgentSkillRouterPipelineID 一致
	if pp.ID != skills.AgentSkillRouterPipelineID {
		t.Errorf("pipeline id %q != AgentSkillRouterPipelineID %q", pp.ID, skills.AgentSkillRouterPipelineID)
	}
	for _, p := range reg.ListAll() {
		def := p.GetSkillDefinition()
		genID := skillBranchNodeID(def.Name)
		var found bool
		for _, n := range pp.Nodes {
			if n.ID == genID {
				found = true
				// 分支 system_prompt 与 manifest system_prompt 一致
				if n.Config.SystemPrompt != def.SystemPrompt {
					t.Errorf("skill %s: branch prompt mismatch", def.Name)
				}
				// 分支 route_value 指向自身节点 id
				if n.RouteConfig == nil || n.RouteConfig.RouteValue != genID {
					t.Errorf("skill %s: branch route_config missing/incorrect", def.Name)
				}
			}
		}
		if !found {
			t.Errorf("skill %s: branch %s not in router pipeline", def.Name, genID)
		}
	}
	// 注册后可从 registry 取回
	pr := pipeline.NewPipelineRegistry()
	if err := pr.Register(pp); err != nil {
		t.Errorf("register %s: %v", pp.ID, err)
	}
	if pr.Get(pp.ID) == nil {
		t.Errorf("pipeline %s not retrievable", pp.ID)
	}
}

// R14：恶意权限的 skill 应被准入校验拒绝，不进入 router 分支（registerSkillRouterWithAdmission）。
func TestAdmissionRejectsSkill(t *testing.T) {
	evil, err := skills.ParseSkillPluginManifest([]byte(`api_version: centag.agent-skill/v1alpha1
implementation: builtin.agent-skill-evil
kind: agent.skill
name: Evil
permissions:
  - "*"
  - sudo
skill:
  name: evil
  enabled: true
  internal: true
  system_prompt: x
`))
	if err != nil {
		t.Fatalf("parse evil manifest: %v", err)
	}

	reg := skills.NewSkillPluginRegistry()
	if err := reg.Register(evil); err != nil {
		t.Fatalf("register evil: %v", err)
	}
	pr := pipeline.NewPipelineRegistry()
	admission := pipeline.NewAdmissionChecker(config.PluginAdmissionConfig{Enabled: true, CheckPermissions: true})
	id := registerSkillRouterWithAdmission(reg, pr, "b", "m", admission)
	if id != "" {
		t.Errorf("evil skill should be rejected by admission, got registered router %q", id)
	}
	if pr.Get(agentSkillRouterPipelineID) != nil {
		t.Errorf("evil skill should not produce a router pipeline, but %s exists", agentSkillRouterPipelineID)
	}
}
