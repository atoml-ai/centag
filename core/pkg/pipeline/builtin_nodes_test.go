package pipeline

import (
	"context"
	"strings"
	"testing"
)

// mockBackendClient 用于测试的模拟后端客户端
type mockBackendClient struct {
	model    string
	response string
	tokens   int
	err      error
}

func (m *mockBackendClient) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &LLMResponse{
		Model:      m.model,
		Content:    m.response,
		TokenUsage: m.tokens,
	}, nil
}

// mockCapabilityBroker 用于测试的模拟能力代理
type mockCapabilityBroker struct {
	llmClient  LLMClient
	httpClient HTTPClient
}

func (m *mockCapabilityBroker) GetLLMClient(ctx context.Context, permissions []string) (LLMClient, error) {
	return m.llmClient, nil
}
func (m *mockCapabilityBroker) GetLLMStreamClient(ctx context.Context, permissions []string) (LLMStreamClient, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetStorage(ctx context.Context, permissions []string) (Storage, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetMemory(ctx context.Context, permissions []string) (Memory, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetSecretsResolver(ctx context.Context, permissions []string) (SecretsResolver, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetHTTPClient(ctx context.Context, permissions []string) (HTTPClient, error) {
	if m.httpClient != nil {
		return m.httpClient, nil
	}
	return nil, nil
}
func (m *mockCapabilityBroker) GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (CacheStrategyCapability, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetVectorCache(ctx context.Context, permissions []string) (VectorCacheCapability, error) {
	return nil, nil
}
func (m *mockCapabilityBroker) GetEmbeddingService(ctx context.Context, permissions []string) (EmbeddingCapability, error) {
	return nil, nil
}

func TestGeneratorNode(t *testing.T) {
	config := NodeConfig{
		Backend:      "test-backend",
		Model:        "gpt-4",
		SystemPrompt: "You are a helpful assistant",
	}

	node, err := NewGeneratorNode(config)
	if err != nil {
		t.Fatalf("Failed to create generator node: %v", err)
	}

	// 验证类型
	if node.Type() != NodeTypeGenerator {
		t.Errorf("Expected type generator, got %v", node.Type())
	}

	// 验证验证
	if err := node.Validate(); err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// 测试执行 - 注入 mock capability broker
	mockClient := &mockBackendClient{
		model:    "gpt-4",
		response: "Mock response: Hello back!",
		tokens:   10,
	}
	if withBroker, ok := node.(interface{ SetCapabilityBroker(CapabilityBroker) }); ok {
		withBroker.SetCapabilityBroker(&mockCapabilityBroker{llmClient: mockClient})
	}
	ctx := context.Background()
	input := &NodeInput{
		Content: "Hello, world!",
	}

	output, err := node.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content == "" {
		t.Error("Output content should not be empty")
	}

	if output.Metadata == nil {
		t.Error("Output metadata should not be nil")
	}

	// 验证metadata包含预期字段
	if _, ok := output.Metadata["model"]; !ok {
		t.Error("Metadata should contain 'model' field")
	}
}

func TestGeneratorNodeValidation(t *testing.T) {
	// 缺少backend
	config := NodeConfig{
		Model: "gpt-4",
	}
	node, _ := NewGeneratorNode(config)
	if err := node.Validate(); err == nil {
		t.Error("Should fail validation without backend")
	}

	// 缺少model
	config = NodeConfig{
		Backend: "test-backend",
	}
	node, _ = NewGeneratorNode(config)
	if err := node.Validate(); err == nil {
		t.Error("Should fail validation without model")
	}
}

func TestProcessorNode(t *testing.T) {
	config := NodeConfig{
		Backend:        "test-backend",
		Model:          "gpt-4",
		PromptTemplate: "Optimize: {{.input}}",
		CustomConfig: map[string]interface{}{
			"operation": "optimize",
		},
	}

	node, err := NewProcessorNode(config)
	if err != nil {
		t.Fatalf("Failed to create processor node: %v", err)
	}

	if node.Type() != NodeTypeProcessor {
		t.Errorf("Expected type processor, got %v", node.Type())
	}

	// 测试执行 - 注入 mock capability broker
	mockClient := &mockBackendClient{
		model:    "gpt-4",
		response: "Optimized: Original content",
		tokens:   5,
	}
	if withBroker, ok := node.(interface{ SetCapabilityBroker(CapabilityBroker) }); ok {
		withBroker.SetCapabilityBroker(&mockCapabilityBroker{llmClient: mockClient})
	}
	ctx := context.Background()
	input := &NodeInput{
		Content: "Original content",
	}

	output, err := node.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content == "" {
		t.Error("Output content should not be empty")
	}

	// 验证metadata包含operation
	if op, ok := output.Metadata["operation"]; !ok || op != "optimize" {
		t.Errorf("Metadata should contain operation='optimize', got %v", op)
	}
}

func TestProcessorNodeTemplateRendering(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
		CustomConfig: map[string]interface{}{
			"operation":   "translate",
			"target_lang": "Chinese",
		},
	}

	node, _ := NewProcessorNode(config)
	processor := node.(*ProcessorNode)

	data := map[string]interface{}{
		"input":       "Hello",
		"target_lang": "Chinese",
	}

	result, err := processor.renderTemplate(processor.PromptTemplate, data)
	if err != nil {
		t.Fatalf("Template rendering failed: %v", err)
	}

	if result == "" {
		t.Error("Rendered template should not be empty")
	}

	// 验证模板包含目标语言
	if result != "Chinese" && len(result) < 10 {
		t.Logf("Rendered template: %s", result)
	}
}

func TestReviewerNode(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
		CustomConfig: map[string]interface{}{
			"criteria":  []string{"accuracy", "completeness"},
			"min_score": 0.8,
		},
	}

	node, err := NewReviewerNode(config)
	if err != nil {
		t.Fatalf("Failed to create reviewer node: %v", err)
	}

	if node.Type() != NodeTypeReviewer {
		t.Errorf("Expected type reviewer, got %v", node.Type())
	}

	// 测试执行 - 注入 mock capability broker
	mockClient := &mockBackendClient{
		model:    "gpt-4",
		response: `{"passed": true, "score": 0.9, "feedback": "Looks good", "suggestions": []}`,
		tokens:   20,
	}
	if withBroker, ok := node.(interface{ SetCapabilityBroker(CapabilityBroker) }); ok {
		withBroker.SetCapabilityBroker(&mockCapabilityBroker{llmClient: mockClient})
	}
	ctx := context.Background()
	input := &NodeInput{
		Content: "Test answer",
		Metadata: map[string]interface{}{
			"question": "Test question",
		},
	}

	output, err := node.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 审核节点不改变内容
	if output.Content != input.Content {
		t.Error("Reviewer should not change content")
	}

	// 审核结论通过顶层字段暴露（不再重复写入 Metadata）
	if output.Passed == nil {
		t.Error("Output.Passed should be set by reviewer node")
	}
	if output.Score == nil {
		t.Error("Output.Score should be set by reviewer node")
	}
	// Metadata 仅保留执行统计（model/tokens/prompt_tokens），不包含审核结论字段
	if _, ok := output.Metadata["passed"]; ok {
		t.Error("Metadata should NOT contain 'passed' (it is a top-level field now)")
	}
	if _, ok := output.Metadata["model"]; !ok {
		t.Error("Metadata should contain 'model' execution stat")
	}
}

func TestReviewerNodeParseResult(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
	}

	node, _ := NewReviewerNode(config)
	reviewer := node.(*ReviewerNode)

	// 测试有效JSON
	validJSON := `{"passed": true, "score": 0.9, "feedback": "Good", "suggestions": ["suggestion1"]}`
	result, err := reviewer.parseReviewResult(validJSON)
	if err != nil {
		t.Fatalf("Parse valid JSON failed: %v", err)
	}
	if !result.Passed {
		t.Error("Expected passed=true")
	}
	if result.Score != 0.9 {
		t.Errorf("Expected score=0.9, got %v", result.Score)
	}

	// 测试无效JSON（应返回默认值）
	invalidJSON := "not valid json"
	result, err = reviewer.parseReviewResult(invalidJSON)
	if err != nil {
		t.Logf("Invalid JSON returns default: %v", err)
	}
	if result == nil {
		t.Error("Should return default result for invalid JSON")
	}
}

func TestRouterNode(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"routes": map[string]interface{}{
				"condition1": "node1",
				"condition2": "node2",
			},
		},
	}

	node, err := NewRouterNode(config)
	if err != nil {
		t.Fatalf("Failed to create router node: %v", err)
	}

	if node.Type() != NodeTypeRouter {
		t.Errorf("Expected type router, got %v", node.Type())
	}

	router := node.(*RouterNode)
	if len(router.legacyRoutes) != 2 {
		t.Errorf("Expected 2 legacy routes, got %d", len(router.legacyRoutes))
	}
	if len(router.rules) != 2 {
		t.Errorf("Expected 2 compiled rules, got %d", len(router.rules))
	}

	ctx := context.Background()
	input := &NodeInput{Content: "test"}

	output, err := node.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 路由节点不改变内容
	if output.Content != input.Content {
		t.Error("Router should not change content")
	}
}

func TestRouterNodeValidation(t *testing.T) {
	// 没有路由配置
	config := NodeConfig{}
	node, _ := NewRouterNode(config)
	if err := node.Validate(); err == nil {
		t.Error("Should fail validation without routes")
	}
}

// TestRouterNodeKeywordRouting 验证 keyword_contains 策略下各类关键词的正确路由。
// 与 config/initdata/pipeline-templates/11-router-mode.yaml 中的 routes 配置对齐。
func TestRouterNodeKeywordRouting(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"routing_strategy": "keyword_contains",
			"default_route":    "chat-generator",
			"routes": map[string]interface{}{
				// 代码生成
				"code":         "code-generator",
				"代码":         "code-generator",
				"python":       "code-generator",
				"java":         "code-generator",
				"go":           "code-generator",
				"golang":       "code-generator",
				"rust":         "code-generator",
				"javascript":   "code-generator",
				"js":           "code-generator",
				"typescript":   "code-generator",
				"ts":           "code-generator",
				"cpp":          "code-generator",
				"c++":          "code-generator",
				"csharp":       "code-generator",
				"c#":           "code-generator",
				"php":          "code-generator",
				"ruby":         "code-generator",
				"swift":        "code-generator",
				"kotlin":       "code-generator",
				"sql":          "code-generator",
				"shell":        "code-generator",
				"bash":         "code-generator",
				"程序":         "code-generator",
				"函数":         "code-generator",
				"方法":         "code-generator",
				"脚本":         "code-generator",
				"算法":         "code-generator",
				"类":           "code-generator",
				"接口":         "code-generator",
				"模块":         "code-generator",
				"库":           "code-generator",
				"leetcode":     "code-generator",
				"实现":         "code-generator",
				"编写":         "code-generator",
				// 翻译
				"translate":    "translate-gen",
				"translation":  "translate-gen",
				"翻译":         "translate-gen",
				// 摘要
				"summary":      "summary-gen",
				"摘要":         "summary-gen",
				"总结":         "summary-gen",
			},
		},
	}

	node, err := NewRouterNode(config)
	if err != nil {
		t.Fatalf("Failed to create router node: %v", err)
	}

	router := node.(*RouterNode)

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		// 代码生成（编程语言）
		{"写一段python的hello world", "code-generator", "python-language"},
		{"用go写一个排序算法", "code-generator", "go-language"},
		{"给我一段java代码", "code-generator", "java-language"},
		{"javascript怎么写循环", "code-generator", "javascript-language"},
		{"帮我写个js脚本", "code-generator", "js-language"},
		{"用rust实现一个web服务器", "code-generator", "rust-language"},
		{"c++的vector怎么用", "code-generator", "cpp-language"},
		{"csharp的linq查询", "code-generator", "csharp-language"},
		{"写个php接口", "code-generator", "php-language"},
		{"ruby的元编程", "code-generator", "ruby-language"},
		{"swift的闭包语法", "code-generator", "swift-language"},
		{"kotlin的协程", "code-generator", "kotlin-language"},
		{"sql查询优化", "code-generator", "sql-language"},
		{"写个shell脚本", "code-generator", "shell-language"},
		{"bash的if语句", "code-generator", "bash-language"},
		{"ts的类型定义", "code-generator", "ts-language"},
		{"golang的channel", "code-generator", "golang-language"},
		// 代码生成（中文意图词）
		{"帮我写个函数", "code-generator", "chinese-function"},
		{"写个排序算法", "code-generator", "chinese-algorithm"},
		{"实现一个登录模块", "code-generator", "chinese-implement"},
		{"编写一个爬虫程序", "code-generator", "chinese-write"},
		{"这个程序怎么运行", "code-generator", "chinese-program"},
		{"写个类来管理用户", "code-generator", "chinese-class"},
		{"定义一个接口", "code-generator", "chinese-interface"},
		{"引入一个第三方库", "code-generator", "chinese-library"},
		{"leetcode第1题", "code-generator", "leetcode-keyword"},
		// 翻译
		{"翻译这句话", "translate-gen", "translate-chinese"},
		{"translate this sentence", "translate-gen", "translate-english"},
		{"请做translation", "translate-gen", "translation-keyword"},
		// 摘要
		{"总结一下这篇文章", "summary-gen", "summary-chinese"},
		{"给个摘要", "summary-gen", "abstract-chinese"},
		{"总结会议记录", "summary-gen", "summarize-chinese"},
		{"summary of the report", "summary-gen", "summary-english"},
		// 默认（对话）
		{"今天天气怎么样", "chat-generator", "default-chat-weather"},
		{"你好", "chat-generator", "default-chat-greeting"},
		{"讲个笑话", "chat-generator", "default-chat-joke"},
		{"什么是人工智能", "chat-generator", "default-chat-concept"},
		// 边界：不含任何关键词但含 code 子串
		{"decode这个字符串", "code-generator", "edge-contains-code-substring"},
		// 边界：不含任何关键词
		{"推荐几本书", "chat-generator", "edge-no-keyword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, matched, _ := router.selectRoute(context.Background(), tt.input)
			if target != tt.expected {
				t.Errorf("input=%q: expected target=%q, got target=%q (matched=%q)",
					tt.input, tt.expected, target, matched)
			}
		})
	}
}

func TestRegisterBuiltinNodes(t *testing.T) {
	registry := NewNodeRegistry()

	err := RegisterBuiltinNodes(registry)
	if err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	// 验证所有内置类型都已注册
	expectedTypes := []NodeType{
		NodeTypeGenerator,
		NodeTypeProcessor,
		NodeTypeReviewer,
		NodeTypeRouter,
		NodeTypeUserPromptOps,
		NodeTypeOutputPostOps,
	}

	for _, nodeType := range expectedTypes {
		if !registry.IsRegistered(nodeType) {
			t.Errorf("Expected %s to be registered", nodeType)
		}
	}

	// 验证可以创建节点
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
	}

	for _, nodeType := range expectedTypes {
		node, err := registry.Create(nodeType, config)
		if err != nil {
			t.Errorf("Failed to create %s: %v", nodeType, err)
		}
		if node == nil {
			t.Errorf("Created %s is nil", nodeType)
		}
	}
}

func TestProcessorNodeDefaultTemplates(t *testing.T) {
	tests := []struct {
		operation string
		wantErr   bool
	}{
		{"optimize", false},
		{"translate", false},
		{"summarize", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			config := NodeConfig{
				Backend: "test-backend",
				Model:   "gpt-4",
				CustomConfig: map[string]interface{}{
					"operation": tt.operation,
				},
			}

			node, err := NewProcessorNode(config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProcessorNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			processor := node.(*ProcessorNode)
			if processor.PromptTemplate == "" {
				t.Error("Default prompt template should not be empty")
			}
		})
	}
}

func TestReviewerNodeDefaultCriteria(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "gpt-4",
	}

	node, _ := NewReviewerNode(config)
	reviewer := node.(*ReviewerNode)

	// 默认criteria应该是空的
	if len(reviewer.Criteria) != 0 {
		t.Logf("Default criteria: %v", reviewer.Criteria)
	}

	// 默认min_score应该是0.8
	if reviewer.MinScore != 0.8 {
		t.Errorf("Expected default min_score=0.8, got %v", reviewer.MinScore)
	}
}

func TestBuiltinNodeSchemasComplete(t *testing.T) {
	nodeTypes := []NodeType{
		NodeTypeGenerator,
		NodeTypeProcessor,
		NodeTypeReviewer,
		NodeTypeRouter,
		NodeTypeAggregator,
		NodeTypeMemory,
	}
	for _, nt := range nodeTypes {
		schemas, ok := BuiltinNodeSchemas[nt]
		if !ok {
			t.Errorf("NodeType %q has no schema fixture", nt)
			continue
		}
		if schemas.InputSchema == nil {
			t.Errorf("NodeType %q InputSchema is nil", nt)
		}
		if schemas.OutputSchema == nil {
			t.Errorf("NodeType %q OutputSchema is nil", nt)
		}
		if schemas.ConfigSchema == nil {
			t.Errorf("NodeType %q ConfigSchema is nil", nt)
		}
	}
}

func TestBuiltinSchemasJSONSchemaValidity(t *testing.T) {
	for nt, schemas := range BuiltinNodeSchemas {
		for name, schema := range map[string]JSONSchema{"input": schemas.InputSchema, "output": schemas.OutputSchema, "config": schemas.ConfigSchema} {
			if schema["type"] != "object" {
				t.Errorf("NodeType %q %s schema type = %v, want object", nt, name, schema["type"])
			}
			props, ok := schema["properties"].(map[string]interface{})
			if !ok {
				t.Errorf("NodeType %q %s schema has no properties map", nt, name)
				continue
			}
			if len(props) == 0 {
				t.Errorf("NodeType %q %s schema has empty properties", nt, name)
			}
		}
	}
}

func TestBuiltinConfigSchemaSpecificFields(t *testing.T) {
	processorSchema := BuiltinNodeSchemas[NodeTypeProcessor].ConfigSchema
	cc, ok := processorSchema["properties"].(map[string]interface{})["custom_config"].(map[string]interface{})
	if !ok {
		t.Fatal("Processor ConfigSchema missing custom_config")
	}
	if cc["type"] != "object" {
		t.Errorf("Processor custom_config type = %v", cc["type"])
	}
	customProps := cc["properties"].(map[string]interface{})
	if _, ok := customProps["operation"]; !ok {
		t.Error("Processor custom_config missing operation field")
	}
	if _, ok := customProps["target_lang"]; !ok {
		t.Error("Processor custom_config missing target_lang field")
	}

	routerSchema := BuiltinNodeSchemas[NodeTypeRouter].ConfigSchema
	routerCC := routerSchema["properties"].(map[string]interface{})["custom_config"].(map[string]interface{})
	routerProps := routerCC["properties"].(map[string]interface{})
	if _, ok := routerProps["routing_strategy"]; !ok {
		t.Error("Router custom_config missing routing_strategy field")
	}

	aggregatorSchema := BuiltinNodeSchemas[NodeTypeAggregator].ConfigSchema
	aggCC := aggregatorSchema["properties"].(map[string]interface{})["custom_config"].(map[string]interface{})
	aggProps := aggCC["properties"].(map[string]interface{})
	strategy := aggProps["strategy"].(map[string]interface{})
	enum, ok := strategy["enum"].([]string)
	if !ok {
		t.Fatal("Aggregator strategy missing enum")
	}
	expected := []string{"concat", "merge", "summarize", "vote", "best"}
	if len(enum) != len(expected) {
		t.Errorf("Aggregator strategy enum length = %d, want %d", len(enum), len(expected))
	}
	for i, e := range expected {
		if enum[i] != e {
			t.Errorf("Aggregator strategy enum[%d] = %q, want %q", i, enum[i], e)
		}
	}
}

func TestGetBuiltinSchemasReturnsCorrect(t *testing.T) {
	for nt := range BuiltinNodeSchemas {
		schemas := GetBuiltinSchemas(nt)
		if schemas.ConfigSchema == nil || schemas.InputSchema == nil || schemas.OutputSchema == nil {
			t.Errorf("GetBuiltinSchemas(%q) returned nil schema", nt)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// llm_classify 策略测试
// ─────────────────────────────────────────────────────────────────────────────

// toRouteMap 将 map[string]string 转为 NodeConfig.routes 所需的 map[string]interface{}
func toRouteMap(routes map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(routes))
	for k, v := range routes {
		out[k] = v
	}
	return out
}

// newLLMClassifyRouter 构造一个启用了 mock LLM 的 llm_classify 路由节点
func newLLMClassifyRouter(t *testing.T, response string, err error, routes map[string]string) *RouterNode {
	t.Helper()
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes":           toRouteMap(routes),
		},
	}
	node, err2 := NewRouterNode(config)
	if err2 != nil {
		t.Fatalf("NewRouterNode failed: %v", err2)
	}
	router := node.(*RouterNode)
	router.SetID("classifier")
	router.SetName("classifier")
	router.SetType(NodeTypeRouter)
	mockClient := &mockBackendClient{
		model:    "glm-4-flash",
		response: response,
		err:      err,
	}
	router.SetCapabilityBroker(&mockCapabilityBroker{llmClient: mockClient})
	return router
}

// TestRouterNode_LLMClassify_Success 验证 LLM 返回合法类别名时正确路由
func TestRouterNode_LLMClassify_Success(t *testing.T) {
	router := newLLMClassifyRouter(t, "code", nil, map[string]string{
		"code":      "code-generator",
		"translate": "translate-gen",
		"summary":   "summary-gen",
		"chat":      "chat-generator",
	})

	ctx := context.Background()
	input := &NodeInput{Content: "用python写个hello world"}
	output, err := router.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Metadata["selected_route"] != "code-generator" {
		t.Errorf("expected selected_route=code-generator, got %v", output.Metadata["selected_route"])
	}
	if output.Metadata["matched"] != "code" {
		t.Errorf("expected matched=code, got %v", output.Metadata["matched"])
	}
	if output.Metadata["llm_raw_response"] != "code" {
		t.Errorf("expected llm_raw_response=code, got %v", output.Metadata["llm_raw_response"])
	}
	if output.Content != input.Content {
		t.Errorf("Router should not change content, got %q", output.Content)
	}
}

// TestRouterNode_LLMClassify_DefaultFallback 验证 LLM 返回未定义类别时 fallback 到 default
func TestRouterNode_LLMClassify_DefaultFallback(t *testing.T) {
	router := newLLMClassifyRouter(t, "unknown_category", nil, map[string]string{
		"code":      "code-generator",
		"translate": "translate-gen",
	})

	ctx := context.Background()
	input := &NodeInput{Content: "今天天气怎么样"}
	output, err := router.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Metadata["selected_route"] != "chat-generator" {
		t.Errorf("expected selected_route=chat-generator (default), got %v", output.Metadata["selected_route"])
	}
	if output.Metadata["matched"] != "__default__" {
		t.Errorf("expected matched=__default__, got %v", output.Metadata["matched"])
	}
}

// TestRouterNode_LLMClassify_LLMError 验证 LLM 调用失败时 fallback 到 default
func TestRouterNode_LLMClassify_LLMError(t *testing.T) {
	router := newLLMClassifyRouter(t, "", errMock("network timeout"), map[string]string{
		"code": "code-generator",
	})

	ctx := context.Background()
	input := &NodeInput{Content: "用go写个排序"}
	output, err := router.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute should not fail on LLM error, got: %v", err)
	}
	if output.Metadata["selected_route"] != "chat-generator" {
		t.Errorf("expected fallback to chat-generator, got %v", output.Metadata["selected_route"])
	}
	if output.Metadata["matched"] != "__llm_error_fallback__" {
		t.Errorf("expected matched=__llm_error_fallback__, got %v", output.Metadata["matched"])
	}
}

// TestRouterNode_LLMClassify_NoBroker 验证无 CapabilityBroker 时直接走 fallback
func TestRouterNode_LLMClassify_NoBroker(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes":           toRouteMap(map[string]string{"code": "code-generator"}),
		},
	}
	node, _ := NewRouterNode(config)
	router := node.(*RouterNode)
	router.SetID("classifier")
	router.SetType(NodeTypeRouter)
	// 注意：未设置 capability broker

	ctx := context.Background()
	output, err := router.Execute(ctx, &NodeInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute should not fail when broker is nil, got: %v", err)
	}
	if output.Metadata["selected_route"] != "chat-generator" {
		t.Errorf("expected fallback to chat-generator, got %v", output.Metadata["selected_route"])
	}
	if output.Metadata["matched"] != "__llm_error_fallback__" {
		t.Errorf("expected matched=__llm_error_fallback__, got %v", output.Metadata["matched"])
	}
}

// TestRouterNode_LLMClassify_PromptRender 验证 {{.input}} 正确注入
func TestRouterNode_LLMClassify_PromptRender(t *testing.T) {
	router := newLLMClassifyRouter(t, "code", nil, map[string]string{
		"code": "code-generator",
	})
	router.classifyPrompt = "Custom: classify {{.input}}"

	ctx := context.Background()
	_, err := router.Execute(ctx, &NodeInput{Content: "hello world"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// 直接调用 render 验证
	prompt, err := renderGoTemplate("classify_prompt", router.classifyPrompt, map[string]interface{}{
		"input": "hello world",
	})
	if err != nil {
		t.Fatalf("renderGoTemplate failed: %v", err)
	}
	if prompt != "Custom: classify hello world" {
		t.Errorf("expected prompt to inject input, got %q", prompt)
	}
}

// TestRouterNode_LLMClassify_Validation 验证 Validate() 强制 backend/model/routes
func TestRouterNode_LLMClassify_Validation(t *testing.T) {
	// 缺少 backend
	cfg := NodeConfig{
		Model: "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes":           toRouteMap(map[string]string{"code": "code-generator"}),
		},
	}
	node, _ := NewRouterNode(cfg)
	if err := node.Validate(); err == nil {
		t.Error("llm_classify without backend should fail validation")
	}

	// 缺少 model
	cfg = NodeConfig{
		Backend: "test-backend",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes":           toRouteMap(map[string]string{"code": "code-generator"}),
		},
	}
	node, _ = NewRouterNode(cfg)
	if err := node.Validate(); err == nil {
		t.Error("llm_classify without model should fail validation")
	}

	// 缺少 routes
	cfg = NodeConfig{
		Backend: "test-backend",
		Model:   "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
		},
	}
	node, _ = NewRouterNode(cfg)
	if err := node.Validate(); err == nil {
		t.Error("llm_classify without routes should fail validation")
	}

	// 全部齐备：通过
	cfg = NodeConfig{
		Backend: "test-backend",
		Model:   "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes":           toRouteMap(map[string]string{"code": "code-generator"}),
		},
	}
	node, _ = NewRouterNode(cfg)
	if err := node.Validate(); err != nil {
		t.Errorf("llm_classify with full config should pass, got: %v", err)
	}
}

// TestRouterNode_CleanClassifyResponse 测试响应清洗逻辑
func TestRouterNode_CleanClassifyResponse(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
		name     string
	}{
		{"code", "code", "plain"},
		{"  CODE  ", "code", "uppercase-with-spaces"},
		{"\"translate\"", "translate", "quoted"},
		{"'summary'", "summary", "single-quoted"},
		{"```code```", "code", "markdown-fence"},
		{"类别：code", "code", "chinese-prefix"},
		{"category: chat", "chat", "english-prefix"},
		{"code\n更多解释", "code", "multi-line-first"},
		{"  `chat`  ", "chat", "backticked"},
		{"", "", "empty"},
		{"\n\n  \n", "", "whitespace-only"},
		{"未知类别", "未知类别", "unmatched-chinese-pass-through"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanClassifyResponse(tt.raw)
			if got != tt.expected {
				t.Errorf("cleanClassifyResponse(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}

// TestRouterNode_LLMClassify_NoKeywordCompile 验证 llm_classify 不会把 routes 编译为规则
func TestRouterNode_LLMClassify_NoKeywordCompile(t *testing.T) {
	cfg := NodeConfig{
		Backend: "test-backend",
		Model:   "glm-4-flash",
		CustomConfig: map[string]interface{}{
			"routing_strategy": "llm_classify",
			"default_route":    "chat-generator",
			"routes": toRouteMap(map[string]string{
				"code": "code-generator",
				"chat": "chat-generator",
			}),
		},
	}
	node, _ := NewRouterNode(cfg)
	router := node.(*RouterNode)
	// llm_classify 不应该把 routes 编译为关键词规则
	if len(router.rules) != 0 {
		t.Errorf("llm_classify should not compile rules, got %d", len(router.rules))
	}
	// 但 legacyRoutes 应该保留
	if len(router.legacyRoutes) != 2 {
		t.Errorf("expected 2 legacyRoutes, got %d", len(router.legacyRoutes))
	}
}

// errMock 简单的 error 字符串类型
type errMock string

func (e errMock) Error() string { return string(e) }

// 防止 strings 包未使用告警（保留 import）
var _ = strings.TrimSpace

// =============================================================================
// BaseNode.CallLLM 抽象测试
// 覆盖：nil broker / 无 perms / backend+model 推断 / 显式 perms / 错误透传 / nil req
// =============================================================================

// capturingBroker 捕获被请求的 permissions，便于断言节点构造的权限是否正确
type capturingBroker struct {
	llmClient     LLMClient
	llmGetErr     error
	returnNilClient bool
	receivedPerms []string
	calls         int
}

func (c *capturingBroker) GetLLMClient(ctx context.Context, permissions []string) (LLMClient, error) {
	c.calls++
	c.receivedPerms = permissions
	if c.llmGetErr != nil {
		return nil, c.llmGetErr
	}
	if c.returnNilClient {
		return nil, nil
	}
	return c.llmClient, nil
}
func (c *capturingBroker) GetLLMStreamClient(ctx context.Context, permissions []string) (LLMStreamClient, error) {
	return nil, nil
}
func (c *capturingBroker) GetStorage(ctx context.Context, permissions []string) (Storage, error) {
	return nil, nil
}
func (c *capturingBroker) GetMemory(ctx context.Context, permissions []string) (Memory, error) {
	return nil, nil
}
func (c *capturingBroker) GetSecretsResolver(ctx context.Context, permissions []string) (SecretsResolver, error) {
	return nil, nil
}
func (c *capturingBroker) GetHTTPClient(ctx context.Context, permissions []string) (HTTPClient, error) {
	return nil, nil
}
func (c *capturingBroker) GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (CacheStrategyCapability, error) {
	return nil, nil
}
func (c *capturingBroker) GetVectorCache(ctx context.Context, permissions []string) (VectorCacheCapability, error) {
	return nil, nil
}
func (c *capturingBroker) GetEmbeddingService(ctx context.Context, permissions []string) (EmbeddingCapability, error) {
	return nil, nil
}

// newTestGenerator 构造一个带已知 id 的 generator 节点
func newTestGenerator(t *testing.T, backend, model string) *GeneratorNode {
	t.Helper()
	node, err := NewGeneratorNode(NodeConfig{
		Backend: backend,
		Model:   model,
	})
	if err != nil {
		t.Fatalf("NewGeneratorNode: %v", err)
	}
	return node.(*GeneratorNode)
}

func TestBaseNode_CallLLM_NilBroker(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	// 不注入 broker

	resp, err := node.CallLLM(context.Background(), "generator", &LLMRequest{})
	if err == nil {
		t.Fatal("expected error for nil broker, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "capability broker not available") {
		t.Errorf("error should mention broker unavailability: %v", err)
	}
	if !strings.Contains(err.Error(), "generator") {
		t.Errorf("error should include kind=generator: %v", err)
	}
}

func TestBaseNode_CallLLM_AutoPerms_BackendAndModel(t *testing.T) {
	node := newTestGenerator(t, "b-explicit", "gpt-4o")
	cap := &capturingBroker{llmClient: &mockBackendClient{response: "ok"}}
	node.SetCapabilityBroker(cap)

	resp, err := node.CallLLM(context.Background(), "generator", &LLMRequest{Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("CallLLM: %v", err)
	}
	if resp == nil || resp.Content != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if cap.calls != 1 {
		t.Errorf("expected 1 broker call, got %d", cap.calls)
	}
	if len(cap.receivedPerms) != 1 || cap.receivedPerms[0] != "llm.call:b-explicit:gpt-4o" {
		t.Errorf("expected auto-inferred perm, got %v", cap.receivedPerms)
	}
}

func TestBaseNode_CallLLM_AutoPerms_NoBackendNoModel(t *testing.T) {
	node := newTestGenerator(t, "", "")
	cap := &capturingBroker{llmClient: &mockBackendClient{response: "ok"}}
	node.SetCapabilityBroker(cap)

	_, err := node.CallLLM(context.Background(), "audit", &LLMRequest{})
	if err != nil {
		t.Fatalf("CallLLM: %v", err)
	}
	if len(cap.receivedPerms) != 1 || cap.receivedPerms[0] != "llm.call" {
		t.Errorf("expected fallback perm 'llm.call', got %v", cap.receivedPerms)
	}
}

func TestBaseNode_CallLLM_ExplicitPermsWin(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	node.SetPermissions([]string{"custom.perm:foo"})
	cap := &capturingBroker{llmClient: &mockBackendClient{response: "ok"}}
	node.SetCapabilityBroker(cap)

	_, err := node.CallLLM(context.Background(), "router", &LLMRequest{})
	if err != nil {
		t.Fatalf("CallLLM: %v", err)
	}
	if len(cap.receivedPerms) != 1 || cap.receivedPerms[0] != "custom.perm:foo" {
		t.Errorf("explicit perm should win, got %v", cap.receivedPerms)
	}
}

func TestBaseNode_CallLLM_BrokerGetError(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	cap := &capturingBroker{llmGetErr: errMock("broker exploded")}
	node.SetCapabilityBroker(cap)

	resp, err := node.CallLLM(context.Background(), "reviewer", &LLMRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on broker error, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "broker exploded") {
		t.Errorf("error should wrap original cause: %v", err)
	}
	if !strings.Contains(err.Error(), "reviewer") {
		t.Errorf("error should include kind=reviewer: %v", err)
	}
}

func TestBaseNode_CallLLM_NilClientFromBroker(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	cap := &capturingBroker{returnNilClient: true}
	node.SetCapabilityBroker(cap)

	resp, err := node.CallLLM(context.Background(), "optimize", &LLMRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "llm client is nil") {
		t.Errorf("error should mention nil client: %v", err)
	}
}

func TestBaseNode_CallLLM_ChatErrorPropagates(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	cap := &capturingBroker{llmClient: &mockBackendClient{err: errMock("chat blew up")}}
	node.SetCapabilityBroker(cap)

	resp, err := node.CallLLM(context.Background(), "processor", &LLMRequest{})
	if err == nil {
		t.Fatal("expected chat error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "chat blew up") {
		t.Errorf("error should wrap chat failure: %v", err)
	}
}

func TestBaseNode_CallLLM_NilRequest(t *testing.T) {
	node := newTestGenerator(t, "b1", "m1")
	cap := &capturingBroker{llmClient: &mockBackendClient{response: "ok"}}
	node.SetCapabilityBroker(cap)

	resp, err := node.CallLLM(context.Background(), "generator", nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "nil llm request") {
		t.Errorf("error should mention nil request: %v", err)
	}
}

func TestBaseNode_CallLLM_KindAppearsInError(t *testing.T) {
	// 不同 node kind 出现在错误信息中，便于日志排查
	node := newTestGenerator(t, "b1", "m1")
	for _, kind := range []string{"generator", "processor", "reviewer", "router", "audit", "optimize"} {
		_, err := node.CallLLM(context.Background(), kind, &LLMRequest{})
		if err == nil {
			t.Fatalf("[%s] expected error, got nil", kind)
		}
		if !strings.Contains(err.Error(), kind) {
			t.Errorf("[%s] error should include kind: %v", kind, err)
		}
	}
}

func TestLoggerFromContext(t *testing.T) {
	// nil ctx
	if l := LoggerFromContext(nil); l != nil {
		t.Errorf("nil ctx should yield nil logger, got %T", l)
	}
	// empty ctx
	if l := LoggerFromContext(context.Background()); l != nil {
		t.Errorf("empty ctx should yield nil logger, got %T", l)
	}
	// ctx with wrong type
	ctx := context.WithValue(context.Background(), loggerContextKey{}, "not-a-logger")
	if l := LoggerFromContext(ctx); l != nil {
		t.Errorf("wrong-typed ctx value should yield nil logger, got %T", l)
	}
}

