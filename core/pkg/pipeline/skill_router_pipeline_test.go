package pipeline

import (
	"context"
	"strings"
	"testing"
)

// skillRouterEchoClient 返回请求中 system 消息内容作为响应，用于区分被执行的 skill 分支。
type skillRouterEchoClient struct{}

func (m *skillRouterEchoClient) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	sys := ""
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			sys = msg.Content
			break
		}
	}
	return &LLMResponse{Model: req.Model, Content: "exec:" + sys}, nil
}

// buildSkillRouterTestPipeline 构造与 server.BuildSkillRouterPipeline 结构一致的
// 最小路由管线：classifier(llm_classify) + a/b 两个 skill 分支 + 默认 chat 分支。
func buildSkillRouterTestPipeline() *AgentPatternPipeline {
	classifier := PipelineNodeConfig{
		ID:             "skill-classifier",
		Type:           NodeTypeRouter,
		Kind:           "routing.decide",
		Implementation: "builtin.router",
		Name:           "Skill Classify",
		Backend:        "default-backend",
		Model:          "default-model",
		Config: NodeConfig{
			Backend: "default-backend",
			Model:   "default-model",
			CustomConfig: map[string]interface{}{
				"routing_strategy": "llm_classify",
				"default_route":    "chat-gen",
				"routes": map[string]interface{}{
					"a": "a-gen",
					"b": "b-gen",
				},
			},
		},
		Timeout: 15,
	}

	gen := func(id, prompt string, def bool) PipelineNodeConfig {
		return PipelineNodeConfig{
			ID:             id,
			Type:           NodeTypeGenerator,
			Kind:           "llm.generate",
			Implementation: "builtin.generator",
			Name:           id,
			Backend:        "default-backend",
			Model:          "default-model",
			Config: NodeConfig{
				Backend:        "default-backend",
				Model:          "default-model",
				PromptTemplate: "{{input}}",
				SystemPrompt:   prompt,
			},
			Timeout: 120,
			DependsOn: []string{
				"skill-classifier",
			},
			RouteConfig: &RouteConfig{
				RouterNodeID: "skill-classifier",
				RouteValue:   id,
				IsDefault:    def,
			},
		}
	}

	return &AgentPatternPipeline{
		SchemaVersion: "centag.pipeline/v1alpha1",
		ID:            "agent-skill-router",
		Name:          "Agent Skill Router",
		Nodes: []PipelineNodeConfig{
			classifier,
			gen("a-gen", "prompt-a", false),
			gen("b-gen", "prompt-b", false),
			gen("chat-gen", "prompt-chat", true),
		},
		GlobalConfig: GlobalPipelineConfig{
			Timeout:       180,
			MaxRetries:    3,
			BypassOnError: true,
			ParallelLimit: 4,
			LogLevel:      "info",
		},
	}
}

// TestSkillRouterPipeline_ForcedRoute 端到端验证强制路由：
// forced_route 时 router 跳过 LLM 分类，直接执行对应 skill 分支。
func TestSkillRouterPipeline_ForcedRoute(t *testing.T) {
	engine, pr := newTestPipelineEngineWithRegistry()
	engine.SetCapabilityBroker(&mockCapabilityBroker{llmClient: &skillRouterEchoClient{}})
	if err := pr.Register(buildSkillRouterTestPipeline()); err != nil {
		t.Fatalf("register: %v", err)
	}

	// forced_route=b → b-gen 分支执行（输出含 prompt-b）
	out, err := engine.Execute(context.Background(), "agent-skill-router", &PipelineInput{
		Content:  "hello",
		Metadata: map[string]interface{}{"forced_route": "b"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Content, "prompt-b") {
		t.Errorf("forced_route=b: output = %q, want contain prompt-b", out.Content)
	}
	// router 节点元数据保留 selected_route（证明分类节点输出）
	var selected string
	for _, no := range out.NodeOutputs {
		if no != nil && no.Metadata != nil {
			if v, ok := no.Metadata["selected_route"].(string); ok && v != "" {
				selected = v
			}
		}
	}
	if selected != "b-gen" {
		t.Errorf("selected_route = %q, want b-gen", selected)
	}
}

// TestSkillRouterPipeline_AutoClassify 验证未指定 forced_route 时走 LLM 分类。
// echo 客户端对分类请求（无 system 消息）返回空 category → 回退默认 chat 分支。
func TestSkillRouterPipeline_AutoClassify(t *testing.T) {
	engine, pr := newTestPipelineEngineWithRegistry()
	engine.SetCapabilityBroker(&mockCapabilityBroker{llmClient: &skillRouterEchoClient{}})
	if err := pr.Register(buildSkillRouterTestPipeline()); err != nil {
		t.Fatalf("register: %v", err)
	}

	out, err := engine.Execute(context.Background(), "agent-skill-router", &PipelineInput{
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// echo 客户端返回的 category 无法匹配 routes → 回退 chat-gen
	if !strings.Contains(out.Content, "prompt-chat") {
		t.Errorf("auto classify fallback: output = %q, want contain prompt-chat", out.Content)
	}
}

// newTestPipelineEngineWithRegistry 返回带独立 pipeline registry 的引擎（供注册被测管线）。
func newTestPipelineEngineWithRegistry() (*PipelineEngine, *PipelineRegistry) {
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		panic(err)
	}
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, nil, NewPipelineLogger(), nil)
	return engine, pipelineRegistry
}
