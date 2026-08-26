package server

import (
	"path/filepath"
	"strings"
	"testing"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/pipeline"
)

func TestBuildSkillRouterPipeline(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	if reg == nil {
		t.Fatal("loadBuiltinSkillPlugins() = nil")
	}

	pp, err := BuildSkillRouterPipeline(reg.ListAll(), "default-backend", "default-model")
	if err != nil {
		t.Fatalf("BuildSkillRouterPipeline failed: %v", err)
	}
	if pp.ID != centagOpsRouterPipelineID {
		t.Errorf("pipeline id = %q, want %q", pp.ID, centagOpsRouterPipelineID)
	}
	// classifier + 7 skill 分支 + chat-gen
	wantNodes := 9
	if len(pp.Nodes) != wantNodes {
		t.Fatalf("nodes = %d, want %d", len(pp.Nodes), wantNodes)
	}

	classifier := pp.Nodes[0]
	if classifier.Type != pipeline.NodeTypeRouter || classifier.Implementation != "builtin.router" {
		t.Errorf("classifier = %+v", classifier)
	}
	if s, _ := classifier.Config.CustomConfig["routing_strategy"].(string); s != "llm_classify" {
		t.Errorf("classifier routing_strategy = %q, want llm_classify", s)
	}
	if d, _ := classifier.Config.CustomConfig["default_route"].(string); d != agentSkillRouterChatID {
		t.Errorf("classifier default_route = %q, want %q", d, agentSkillRouterChatID)
	}
	routes, _ := classifier.Config.CustomConfig["routes"].(map[string]interface{})
	if len(routes) != 7 {
		t.Errorf("classifier routes = %d, want 7", len(routes))
	}
	if routes["status-check"] != "status-check-gen" {
		t.Errorf("routes[status-check] = %v, want status-check-gen", routes["status-check"])
	}

	// classify_prompt：每个 skill 的名称+描述注入判断标准，chat 兜底显式说明
	cp, _ := classifier.Config.CustomConfig["classify_prompt"].(string)
	if cp == "" {
		t.Fatal("classifier classify_prompt should be injected")
	}
	for _, key := range []string{
		"- status-check：查询centag当前运行状态",
		"- config-analysis：分析当前配置",
		"- error-diagnosis：错误诊断",
		"- log-analysis：日志分析",
		"- strategy-recommend：策略调整建议",
		"- billing-audit：计费审计",
		"- cost-analysis：成本分析",
		"- chat：问候、闲聊或与 centag 运维无关的问题；无法判断时也返回 chat",
		"{{.input}}",
	} {
		if !strings.Contains(cp, key) {
			t.Errorf("classify_prompt missing %q", key)
		}
	}

	// status-check 分支
	var branch *pipeline.PipelineNodeConfig
	for i := range pp.Nodes {
		if pp.Nodes[i].ID == "status-check-gen" {
			branch = &pp.Nodes[i]
			break
		}
	}
	if branch == nil {
		t.Fatal("status-check-gen branch not found")
	}
	if branch.Type != pipeline.NodeTypeGenerator || branch.Implementation != "builtin.generator" {
		t.Errorf("branch = %+v", branch)
	}
	if !strings.Contains(branch.Config.SystemPrompt, "你是一个 centag 运维助手，正在执行 skill: status-check") {
		t.Errorf("branch system_prompt should be manifest prompt")
	}
	if branch.Config.PromptTemplate != "{{input}}" {
		t.Errorf("branch prompt_template = %q, want {{input}}", branch.Config.PromptTemplate)
	}
	if branch.RouteConfig == nil || branch.RouteConfig.RouterNodeID != agentSkillRouterClassifierID {
		t.Errorf("branch route_config = %+v", branch.RouteConfig)
	}
	if branch.RouteConfig.RouteValue != "status-check-gen" {
		t.Errorf("branch route_value = %q, want status-check-gen", branch.RouteConfig.RouteValue)
	}
	if len(branch.DependsOn) != 1 || branch.DependsOn[0] != agentSkillRouterClassifierID {
		t.Errorf("branch depends_on = %v, want [%s]", branch.DependsOn, agentSkillRouterClassifierID)
	}

	// 默认 chat 分支
	var chat *pipeline.PipelineNodeConfig
	for i := range pp.Nodes {
		if pp.Nodes[i].ID == agentSkillRouterChatID {
			chat = &pp.Nodes[i]
			break
		}
	}
	if chat == nil {
		t.Fatal("chat-gen branch not found")
	}
	if chat.RouteConfig == nil || !chat.RouteConfig.IsDefault {
		t.Error("chat-gen should be default route")
	}

	if !pp.Metadata[skillPipelineMetadataKey].(bool) {
		t.Error("metadata centag_ops_pipeline should be true")
	}
}

func TestBuildSkillRouterPipeline_NoToolCallInjector(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	pp, err := BuildSkillRouterPipeline(reg.ListAll(), "b", "m")
	if err != nil {
		t.Fatalf("BuildSkillRouterPipeline failed: %v", err)
	}
	for _, n := range pp.Nodes {
		if strings.Contains(string(n.Type), "tool_call_injector") || n.Implementation == "builtin.tool_call_injector" {
			t.Errorf("skill router pipeline should not contain tool_call_injector, got %s", n.Implementation)
		}
	}
}

func TestRegisterSkillRouter(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))
	reg := loadBuiltinSkillPlugins()
	pr := pipeline.NewPipelineRegistry()

	id := registerSkillRouter(reg, pr, "default-backend", "default-model")
	if id != centagOpsRouterPipelineID {
		t.Fatalf("registered id = %q, want %q", id, centagOpsRouterPipelineID)
	}
	if pr.Get(centagOpsRouterPipelineID) == nil {
		t.Errorf("pipeline %s not in registry", centagOpsRouterPipelineID)
	}

	// 幂等：重复注册不报错
	id2 := registerSkillRouter(reg, pr, "default-backend", "default-model")
	if id2 != centagOpsRouterPipelineID {
		t.Fatalf("re-register id = %q, want %q", id2, centagOpsRouterPipelineID)
	}
}

func TestRegisterSkillRouter_Nil(t *testing.T) {
	if id := registerSkillRouter(nil, nil, "b", "m"); id != "" {
		t.Errorf("registerSkillRouter(nil,nil) = %q, want empty", id)
	}
	pr := pipeline.NewPipelineRegistry()
	if id := registerSkillRouter(nil, pr, "b", "m"); id != "" {
		t.Errorf("registerSkillRouter(nil,pr) = %q, want empty", id)
	}
}

// TestSkillRouterTemplate_NotSeeded 验证路由真源单一化：
// 1) initdata 不再提供 centag-ops-router 流水线模板（已删除 agent-skill-router.yaml）；
// 2) skill manifest 仍不被当作 pipeline 模板；
// 3) 路由管线仅由代码从 manifest 生成（TestBuildSkillRouterPipeline 覆盖结构）。
func TestSkillRouterTemplate_NotSeeded(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustFindProjectRoot(t))

	found := map[string]bool{}
	for _, tpl := range resolvePipelineTemplatesWithEdition("team") {
		found[tpl.ID] = true
	}
	if found[centagOpsRouterPipelineID] {
		t.Errorf("initdata must NOT provide a %s template — router is generated from skill manifests only", centagOpsRouterPipelineID)
	}
	if found["agent-skill-status-check"] {
		t.Error("skill manifest agent-skill-status-check should NOT be a pipeline template")
	}
}

// TestLoadSkillPluginRegistry_CustomManifestsReloaded P1-C：自定义 skill
// （data/agent-skills/）必须在重启后重新载入注册表；内置 + 自定义皆空时回退 nil。
func TestLoadSkillPluginRegistry_CustomManifestsReloaded(t *testing.T) {
	dataDir := t.TempDir()

	// 空目录（且无 initdata）：返回 nil，保持 minimal 回退语义。
	if reg := loadSkillPluginRegistry(dataDir); reg != nil {
		t.Fatalf("empty stores must yield nil registry, got %d plugins", len(reg.ListAll()))
	}

	// 写入一个自定义 manifest → 重启载入后应出现在注册表且标记 custom。
	customStore := skills.NewFileManifestStore(filepath.Join(dataDir, "agent-skills"))
	manifest := `api_version: centag.agent-skill/v1alpha1
implementation: custom.agent-skill-custom-restart-check
name: 重启自检
kind: agent.skill
version: 1.0.0
description: custom skill persisted across restarts
permissions:
  - agent.tool.read_config
skill:
  name: custom-restart-check
  category: system
  enabled: true
  internal: false
  tools:
    - read_config
  system_prompt: 你是自定义 skill
`
	if err := customStore.Save("custom-restart-check", []byte(manifest)); err != nil {
		t.Fatalf("save custom manifest: %v", err)
	}
	reg := loadSkillPluginRegistry(dataDir)
	if reg == nil {
		t.Fatal("registry must load custom manifests from data dir")
	}
	p, ok := reg.Get("custom-restart-check")
	if !ok {
		t.Fatalf("custom skill missing after reload; have %v", reg.ListAll())
	}
	if p.Internal() {
		t.Error("reloaded skill must be custom (not internal)")
	}
}
