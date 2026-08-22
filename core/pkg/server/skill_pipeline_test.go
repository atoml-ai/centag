package server

import (
	"strings"
	"testing"

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
	// classifier + 5 skill 分支 + chat-gen
	wantNodes := 7
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
	if len(routes) != 5 {
		t.Errorf("classifier routes = %d, want 5", len(routes))
	}
	if routes["status-check"] != "status-check-gen" {
		t.Errorf("routes[status-check] = %v, want status-check-gen", routes["status-check"])
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
