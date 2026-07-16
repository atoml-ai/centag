package pipeline

import (
	"testing"

	"centag/core/internal/agent"
	"centag/core/pkg/bootstrap"
)

// TestAgentProviderManager_WithSpecialization 验证 AgentProviderManager
// 正确集成 SpecializedAgentRegistry
func TestAgentProviderManager_WithSpecialization(t *testing.T) {
	mgr := agent.NewAgentProviderManager()
	reg := mgr.GetSpecializationRegistry()
	if reg == nil {
		t.Fatal("GetSpecializationRegistry returned nil")
	}

	// 验证默认 Seed 已调用
	if reg.Count() != 4 {
		t.Errorf("expected 4 specializations after SeedDefaults, got %d", reg.Count())
	}

	// 验证 TUI Agent 可获取
	if _, ok := reg.GetTUIAgent(agent.AgentCodingTUI); !ok {
		t.Error("coding-tui TUI agent should be registered")
	}
	if _, ok := reg.GetTUIAgent(agent.AgentEducationTUI); !ok {
		t.Error("education-tui TUI agent should be registered")
	}

	// 验证 Web Agent 可获取
	if _, ok := reg.GetWebAgent(agent.AgentCodingWeb); !ok {
		t.Error("coding-web Web agent should be registered")
	}
	if _, ok := reg.GetWebAgent(agent.AgentEducationWeb); !ok {
		t.Error("education-web Web agent should be registered")
	}

	// 验证 TUI Agent 按类别可查询
	tuiSpecs := reg.ListByCategory(agent.AgentCategoryTUI)
	if len(tuiSpecs) != 2 {
		t.Errorf("expected 2 TUI specializations, got %d", len(tuiSpecs))
	}
	webSpecs := reg.ListByCategory(agent.AgentCategoryWeb)
	if len(webSpecs) != 2 {
		t.Errorf("expected 2 Web specializations, got %d", len(webSpecs))
	}

	// 验证能力发现
	codingAgents := reg.DiscoverCapabilities("code_highlight")
	if len(codingAgents) == 0 {
		t.Error("code_highlight capability should discover agents")
	}
	eduAgents := reg.DiscoverCapabilities("quiz_interaction")
	if len(eduAgents) == 0 {
		t.Error("quiz_interaction capability should discover agents")
	}

	// 验证 Get 方法
	spec, ok := reg.Get(agent.AgentCodingTUI)
	if !ok || spec.Type != agent.AgentCodingTUI {
		t.Errorf("Get(coding-tui): ok=%v spec=%v", ok, spec)
	}
}

// TestTemplateRegistry_Integrated 验证 TemplateRegistry 包含所有新增 Agent 类型
func TestTemplateRegistry_Integrated(t *testing.T) {
	reg := agent.NewTemplateRegistry()

	// 验证 CLI Agent 类型仍存在
	cliTypes := []agent.AgentType{
		agent.AgentClaudeCode, agent.AgentClaudeDesktop, agent.AgentCodex,
		agent.AgentGeminiCLI, agent.AgentOpenCode, agent.AgentOpenClaw, agent.AgentHermes,
	}
	for _, at := range cliTypes {
		if _, ok := reg.Get(at); !ok {
			t.Errorf("CLI agent %s should still be registered", at)
		}
	}

	// 验证新 TUI/Web 类型
	newTypes := []agent.AgentType{
		agent.AgentCodingTUI, agent.AgentEducationTUI,
		agent.AgentCodingWeb, agent.AgentEducationWeb,
	}
	for _, at := range newTypes {
		if _, ok := reg.Get(at); !ok {
			t.Errorf("new agent type %s should be registered", at)
		}
	}

	// 验证 List 总数量（7 CLI + 2 TUI + 2 Web = 11）
	allTypes := reg.List()
	if len(allTypes) < 11 {
		t.Errorf("expected >= 11 agent types, got %d: %v", len(allTypes), allTypes)
	}
}

// TestPipelineWithStorageAndAgentConfig 端到端验证流水线加载
// 同时含存储钩子和 Agent 配置
func TestPipelineWithStorageAndAgentConfig(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))

	// 1. 加载教育场景模板
	eduTmpl := mustLoadEducationSceneTemplate(t)
	if eduTmpl.GlobalConfig == nil {
		t.Fatal("education-scene GlobalConfig is nil")
	}

	// 2. 验证存储钩子
	eduPipeline := CreatePipelineFromTemplate(eduTmpl, nil)
	if !eduPipeline.GlobalConfig.HasStorageHook() {
		t.Error("education-scene should have storage hook")
	}

	// 3. 加载编程 Agent 模板
	codingTmpl := mustLoadPipelineTemplate(t, "coding-agent")
	if codingTmpl.GlobalConfig == nil {
		t.Fatal("coding-agent GlobalConfig is nil")
	}

	codingPipeline := CreatePipelineFromTemplate(codingTmpl, nil)
	if !codingPipeline.GlobalConfig.HasStorageHook() {
		t.Error("coding-agent should have storage hook")
	}

	// 4. 验证两类流水线的存储命名空间隔离
	eduNS := eduPipeline.GlobalConfig.StorageNamespace(eduPipeline.ID)
	codingNS := codingPipeline.GlobalConfig.StorageNamespace(codingPipeline.ID)
	if eduNS == codingNS {
		t.Errorf("education and coding namespaces should differ: %q vs %q", eduNS, codingNS)
	}

	// 5. 验证 Agent 专业化注册表与流水线场景的对应关系
	mgr := agent.NewAgentProviderManager()
	reg := mgr.GetSpecializationRegistry()

	// 编程场景 Agent
	if spec, ok := reg.Get(agent.AgentCodingTUI); !ok || spec.Metadata["scene"] != "coding" {
		t.Errorf("coding-tui scene metadata: %v", spec)
	}
	if spec, ok := reg.Get(agent.AgentCodingWeb); !ok || spec.Metadata["scene"] != "coding" {
		t.Errorf("coding-web scene metadata: %v", spec)
	}

	// 教育场景 Agent
	if spec, ok := reg.Get(agent.AgentEducationTUI); !ok || spec.Metadata["scene"] != "education" {
		t.Errorf("education-tui scene metadata: %v", spec)
	}
	if spec, ok := reg.Get(agent.AgentEducationWeb); !ok || spec.Metadata["scene"] != "education" {
		t.Errorf("education-web scene metadata: %v", spec)
	}

	// 6. 验证所有模板加载无错误
	allTemplates := bootstrap.LoadInitialPipelineTemplatesFromFiles()
	for _, raw := range allTemplates {
		tmpl := convertBootstrapTemplate(raw)
		p := CreatePipelineFromTemplate(tmpl, nil)
		if err := p.Validate(); err != nil {
			t.Errorf("template %s validation failed: %v", raw.ID, err)
		}
	}
}
