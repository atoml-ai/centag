package pipeline

import (
	"testing"

	"centag/core/pkg/bootstrap"
)

func TestEducationSceneTemplate_ValidateAndTopology(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadEducationSceneTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)
	if p == nil {
		t.Fatal("CreatePipelineFromTemplate returned nil")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.ShortcutCode != "#edu" {
		t.Fatalf("shortcut = %q, want #edu", p.ShortcutCode)
	}
	if len(p.Nodes) != 7 {
		t.Fatalf("node_count = %d, want 7", len(p.Nodes))
	}

	graph := NewExecutionGraph(p)
	layers, err := graph.LayeredTopologicalSort()
	if err != nil {
		t.Fatalf("LayeredTopologicalSort: %v", err)
	}
	if len(layers) < 2 {
		t.Fatalf("layer_count = %d, want >= 2", len(layers))
	}

	nodeByID := map[string]PipelineNodeConfig{}
	for _, n := range p.Nodes {
		nodeByID[n.ID] = n
	}

	// 验证 scene_router 节点
	router, ok := nodeByID["scene_router"]
	if !ok {
		t.Fatal("missing node scene_router")
	}
	if router.Type != NodeTypeRouter {
		t.Fatalf("scene_router type = %v, want router", router.Type)
	}
	if router.Inputs["content"] != "input.metadata.scene" {
		t.Fatalf("scene_router inputs.content = %q, want input.metadata.scene", router.Inputs["content"])
	}

	// 验证 6 个 generator 节点
	generatorIDs := []string{"problem-solver", "explainer", "knowledge-thinker", "essay-reviewer", "speaking-practice", "qa-generator"}
	for _, gid := range generatorIDs {
		g, ok := nodeByID[gid]
		if !ok {
			t.Fatalf("missing node %s", gid)
		}
		if g.Type != NodeTypeGenerator {
			t.Fatalf("%s type = %v, want generator", gid, g.Type)
		}
		if g.Inputs["content"] != "context.input" {
			t.Fatalf("%s inputs.content = %q, want context.input", gid, g.Inputs["content"])
		}
		if len(g.DependsOn) != 1 || g.DependsOn[0] != "scene_router" {
			t.Fatalf("%s depends_on = %v, want [scene_router]", gid, g.DependsOn)
		}
		if g.RouteConfig == nil {
			t.Fatalf("%s route_config is nil", gid)
		}
		if g.RouteConfig.RouterNodeID != "scene_router" {
			t.Fatalf("%s route_config.router_node_id = %q, want scene_router", gid, g.RouteConfig.RouterNodeID)
		}
	}

	// 验证 route_config 的 route_value 映射
	routeValueMap := map[string]string{
		"problem-solver":    "problem_solving",
		"explainer":         "explain",
		"knowledge-thinker": "knowledge_thinking",
		"essay-reviewer":    "essay_review",
		"speaking-practice": "speaking",
		"qa-generator":      "qa",
	}
	for gid, expectedRoute := range routeValueMap {
		g := nodeByID[gid]
		if g.RouteConfig.RouteValue != expectedRoute {
			t.Fatalf("%s route_config.route_value = %q, want %q", gid, g.RouteConfig.RouteValue, expectedRoute)
		}
	}

	// 验证 qa-generator 是默认分支
	qaGen := nodeByID["qa-generator"]
	if !qaGen.RouteConfig.IsDefault {
		t.Fatal("qa-generator route_config.is_default should be true")
	}
}

func TestEducationSceneTemplate_RouterKeywordRouting(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadEducationSceneTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)

	// 找到 scene_router 节点的 routes 配置
	var routes map[string]interface{}
	for _, n := range p.Nodes {
		if n.ID == "scene_router" {
			routes = n.Config.CustomConfig["routes"].(map[string]interface{})
			break
		}
	}
	if routes == nil {
		t.Fatal("scene_router routes not found")
	}

	// 验证所有教育场景关键词
	expectedRoutes := map[string]string{
		"problem_solving":    "problem-solver",
		"explain":            "explainer",
		"knowledge_thinking": "knowledge-thinker",
		"essay_review":       "essay-reviewer",
		"speaking":           "speaking-practice",
		"qa":                 "qa-generator",
		"解题":                "problem-solver",
		"讲解":                "explainer",
		"思考":                "knowledge-thinker",
		"知识点":              "knowledge-thinker",
		"作文":                "essay-reviewer",
		"批改":                "essay-reviewer",
		"口语":                "speaking-practice",
		"答疑":                "qa-generator",
	}
	for keyword, expectedTarget := range expectedRoutes {
		if target, ok := routes[keyword]; !ok || target != expectedTarget {
			t.Fatalf("route[%q] = %q, want %q", keyword, target, expectedTarget)
		}
	}
}

func mustLoadEducationSceneTemplate(t *testing.T) PatternTemplate {
	t.Helper()
	for _, raw := range bootstrap.LoadInitialPipelineTemplatesFromFiles() {
		if raw.ID == "education-scene" {
			return convertBootstrapTemplate(raw)
		}
	}
	t.Fatal("education-scene template not loaded from initdata")
	return PatternTemplate{}
}

func TestSceneInjection_IntoExecutionContext(t *testing.T) {
	execCtx := NewExecutionContext(nil)

	// 设置 metadata 包含 scene
	metadata := map[string]interface{}{
		"scene": "essay_review",
		"mode":  "education-scene",
	}
	execCtx.SetVariable("metadata", metadata)

	// 模拟 engine.go 中的 scene 注入逻辑
	if inputMetadata, ok := execCtx.GetVariable("metadata"); ok {
		if metaMap, ok := inputMetadata.(map[string]interface{}); ok {
			if scene, ok := metaMap["scene"].(string); ok && scene != "" {
				execCtx.SetVariable("scene", scene)
			}
		}
	}

	// 验证 scene 变量已设置
	scene, ok := execCtx.GetVariable("scene")
	if !ok {
		t.Fatal("scene variable not set in ExecutionContext")
	}
	if scene != "essay_review" {
		t.Fatalf("scene = %q, want essay_review", scene)
	}
}

func TestSceneResolution_ViaTemplateVarResolver(t *testing.T) {
	execCtx := NewExecutionContext(nil)
	execCtx.SetVariable("scene", "speaking")

	input := &NodeInput{
		Content:  "用户输入",
		Metadata: map[string]interface{}{"scene": "speaking"},
	}

	resolver := NewTemplateVarResolver(input, execCtx)

	// 测试 context.scene 路径
	val, err := resolver.Resolve("context.scene")
	if err != nil {
		t.Fatalf("Resolve(context.scene) error: %v", err)
	}
	if val != "speaking" {
		t.Fatalf("context.scene = %q, want speaking", val)
	}

	// 测试 input.metadata.scene 路径
	val, err = resolver.Resolve("input.metadata.scene")
	if err != nil {
		t.Fatalf("Resolve(input.metadata.scene) error: %v", err)
	}
	if val != "speaking" {
		t.Fatalf("input.metadata.scene = %q, want speaking", val)
	}
}

func TestRouterNode_EducationKeywordMatching(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadEducationSceneTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)

	// 找到 scene_router 节点
	var routerConfig NodeConfig
	for _, n := range p.Nodes {
		if n.ID == "scene_router" {
			routerConfig = n.Config
			break
		}
	}

	// 创建 router 节点
	routerNode, err := NewRouterNode(routerConfig)
	if err != nil {
		t.Fatalf("NewRouterNode: %v", err)
	}

	// 测试关键词匹配
	tests := []struct {
		scene    string
		expected string
	}{
		{"problem_solving", "problem-solver"},
		{"explain", "explainer"},
		{"knowledge_thinking", "knowledge-thinker"},
		{"essay_review", "essay-reviewer"},
		{"speaking", "speaking-practice"},
		{"qa", "qa-generator"},
		{"解题", "problem-solver"},
		{"讲解", "explainer"},
		{"思考", "knowledge-thinker"},
		{"知识点", "knowledge-thinker"},
		{"作文", "essay-reviewer"},
		{"批改", "essay-reviewer"},
		{"口语", "speaking-practice"},
		{"答疑", "qa-generator"},
		{"unknown", "qa-generator"}, // 默认路由
	}

	for _, tt := range tests {
		t.Run(tt.scene, func(t *testing.T) {
			input := &NodeInput{
				Content: tt.scene,
			}
			output, err := routerNode.Execute(nil, input)
			if err != nil {
				t.Fatalf("RouterNode.Execute(%q) error: %v", tt.scene, err)
			}
			selectedRoute, ok := output.Metadata["selected_route"].(string)
			if !ok {
				t.Fatalf("selected_route not found in metadata: %v", output.Metadata)
			}
			if selectedRoute != tt.expected {
				t.Fatalf("selected_route = %q, want %q", selectedRoute, tt.expected)
			}
		})
	}
}
