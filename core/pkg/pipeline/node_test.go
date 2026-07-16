package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNodeTypeIsValid(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		want     bool
	}{
		{NodeTypeGenerator, true},
		{NodeTypeProcessor, true},
		{NodeTypeReviewer, true},
		{NodeTypeRouter, true},
		{NodeTypeAggregator, true},
		{NodeTypeMemory, true},
		{NodeTypeAudit, true},
		{NodeTypeOptimize, true},
		{NodeTypeCache, true},
		{NodeTypeTokenUsage, true},
		{NodeType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType.String(), func(t *testing.T) {
			if got := tt.nodeType.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryConfigCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		BackoffStrategy: "exponential",
		InitialDelay:    1000,
		MaxDelay:        30000,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 0},
		{1, 1000 * time.Millisecond},
		{2, 2000 * time.Millisecond},
		{3, 4000 * time.Millisecond},
		{4, 8000 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.attempt)), func(t *testing.T) {
			got := config.CalculateDelay(tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestRetryConfigFixedDelay(t *testing.T) {
	config := &RetryConfig{
		BackoffStrategy: "fixed",
		InitialDelay:    500,
	}

	for i := 1; i <= 3; i++ {
		got := config.CalculateDelay(i)
		want := 500 * time.Millisecond
		if got != want {
			t.Errorf("Fixed delay attempt %d = %v, want %v", i, got, want)
		}
	}
}

func TestNodeRegistry(t *testing.T) {
	registry := NewNodeRegistry()

	// 注册测试工厂
	testFactory := func(config NodeConfig) (PipelineNode, error) {
		return &BaseNode{
			id:   "test",
			name: "Test Node",
		}, nil
	}

	// 测试注册
	err := registry.Register(NodeTypeGenerator, testFactory)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 测试重复注册
	err = registry.Register(NodeTypeGenerator, testFactory)
	if err != nil {
		t.Fatalf("Re-register should succeed: %v", err)
	}

	// 测试创建
	node, err := registry.Create(NodeTypeGenerator, NodeConfig{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if node == nil {
		t.Fatal("Created node is nil")
	}

	// 测试未知类型
	_, err = registry.Create(NodeType("unknown"), NodeConfig{})
	if err == nil {
		t.Error("Create unknown type should fail")
	}

	// 测试无效类型
	err = registry.Register(NodeType("invalid"), testFactory)
	if err == nil {
		t.Error("Register invalid type should fail")
	}

	// 测试nil工厂
	err = registry.Register(NodeTypeProcessor, nil)
	if err == nil {
		t.Error("Register nil factory should fail")
	}
}

func TestNodeRegistryGetRegisteredTypes(t *testing.T) {
	registry := NewNodeRegistry()
	factory := func(config NodeConfig) (PipelineNode, error) {
		return &BaseNode{}, nil
	}

	registry.Register(NodeTypeGenerator, factory)
	registry.Register(NodeTypeProcessor, factory)

	types := registry.GetRegisteredTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 registered types, got %d", len(types))
	}
}

func TestNodeRegistryGetPlugin(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	plugin, ok := registry.GetPlugin("builtin.generator")
	if !ok {
		t.Fatal("expected to find builtin.generator plugin")
	}
	if plugin.Descriptor().Implementation != "builtin.generator" {
		t.Errorf("Implementation = %q, want %q", plugin.Descriptor().Implementation, "builtin.generator")
	}
	if plugin.Descriptor().Kind != "llm.generate" {
		t.Errorf("Kind = %q, want %q", plugin.Descriptor().Kind, "llm.generate")
	}

	_, ok = registry.GetPlugin("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent plugin")
	}
}

func TestBaseNode(t *testing.T) {
	node := &BaseNode{
		id:       "test-id",
		name:     "Test Node",
		nodeType: NodeTypeGenerator,
		timeout:  60,
	}

	if node.ID() != "test-id" {
		t.Errorf("ID() = %v, want test-id", node.ID())
	}
	if node.Name() != "Test Node" {
		t.Errorf("Name() = %v, want Test Node", node.Name())
	}
	if node.Type() != NodeTypeGenerator {
		t.Errorf("Type() = %v, want generator", node.Type())
	}
	if node.GetTimeout() != 60 {
		t.Errorf("GetTimeout() = %v, want 60", node.GetTimeout())
	}
}

func TestNodeInputOutput(t *testing.T) {
	input := &NodeInput{
		Content: "test content",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	if input.Content != "test content" {
		t.Errorf("Content = %v", input.Content)
	}

	output := &NodeOutput{
		Content: "response",
		Metadata: map[string]interface{}{
			"tokens": 100,
		},
	}

	if output.Content != "response" {
		t.Errorf("Content = %v", output.Content)
	}

	passed := true
	output.Passed = &passed
	if *output.Passed != true {
		t.Error("Passed should be true")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	if config.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %v, want 3", config.MaxAttempts)
	}
	if config.BackoffStrategy != "exponential" {
		t.Errorf("BackoffStrategy = %v, want exponential", config.BackoffStrategy)
	}
	if config.InitialDelay != 1000 {
		t.Errorf("InitialDelay = %v, want 1000", config.InitialDelay)
	}
	if config.MaxDelay != 30000 {
		t.Errorf("MaxDelay = %v, want 30000", config.MaxDelay)
	}
}

func TestEstimateInputSize(t *testing.T) {
	input := &NodeInput{
		Content:  "hello",
		Messages: []Message{{Role: "user", Content: "world"}},
		Metadata: map[string]interface{}{"key": "value"},
	}
	size := estimateInputSize(input)
	if size <= 0 {
		t.Errorf("estimateInputSize returned %d, want > 0", size)
	}
	if size < int64(len("hello")+len("user")+len("world")) {
		t.Errorf("estimateInputSize too small: %d", size)
	}
}

func TestNodeConfigMaxInputBytes(t *testing.T) {
	config := NodeConfig{
		Backend:       "test",
		MaxInputBytes: 1 << 20,
	}
	if config.MaxInputBytes != 1<<20 {
		t.Errorf("MaxInputBytes = %d, want %d", config.MaxInputBytes, 1<<20)
	}
}

func TestPluginBackedNodeExecuteRejectsOversizedInput(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	mockPlugin := &mockSizeCheckPlugin{returnOutput: &NodeOutput{Content: "ok"}}
	registry.RegisterPlugin(mockPlugin)

	node := NewPluginBackedNode(PipelineNodeConfig{
		ID:             "test",
		Implementation: "mock.size",
	}, NodeConfig{MaxInputBytes: 10}, mockPlugin)

	largeInput := &NodeInput{
		Content: string(make([]byte, 100)),
	}

	_, err := node.Execute(context.Background(), largeInput)
	if err == nil {
		t.Error("expected error for oversized input, got nil")
	}
	if err != nil && !contains(err.Error(), "exceeds limit") {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockSizeCheckPlugin struct {
	returnOutput *NodeOutput
}

func (m *mockSizeCheckPlugin) Descriptor() NodePluginDescriptor {
	return NodePluginDescriptor{
		Name:           "mock",
		Implementation: "mock.size",
		Version:        "1.0.0",
	}
}

func (m *mockSizeCheckPlugin) ValidateConfig(config NodeConfig) error {
	return nil
}

func (m *mockSizeCheckPlugin) Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error) {
	if req.MaxInputBytes > 0 && req.Input != nil {
		size := estimateInputSize(req.Input)
		if size > req.MaxInputBytes {
			return nil, fmt.Errorf("input size %d exceeds limit %d", size, req.MaxInputBytes)
		}
	}
	return &NodeExecutionResponse{Output: m.returnOutput}, nil
}
