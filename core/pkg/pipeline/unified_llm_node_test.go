package pipeline

import (
	"testing"
)

func TestUnifiedLLMNode_Type(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	if node.Type() != NodeTypeGenerator {
		t.Errorf("Type() = %v, want %v", node.Type(), NodeTypeGenerator)
	}
}

func TestUnifiedLLMNode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  NodeConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: NodeConfig{
				Backend: "test-backend",
				Model:   "test-model",
			},
			wantErr: false,
		},
		{
			name: "缺少backend",
			config: NodeConfig{
				Model: "test-model",
			},
			wantErr: true,
		},
		{
			name: "缺少model",
			config: NodeConfig{
				Backend: "test-backend",
			},
			wantErr: true,
		},
		{
			name: "translate操作缺少target_lang",
			config: NodeConfig{
				Backend: "test-backend",
				Model:   "test-model",
				CustomConfig: map[string]interface{}{
					"operation":  "translate",
					"target_lang": "",
				},
			},
			wantErr: true,
		},
		{
			name: "translate操作有target_lang",
			config: NodeConfig{
				Backend: "test-backend",
				Model:   "test-model",
				CustomConfig: map[string]interface{}{
					"operation":  "translate",
					"target_lang": "en",
				},
			},
			wantErr: false,
		},
		{
			name: "无效操作类型",
			config: NodeConfig{
				Backend: "test-backend",
				Model:   "test-model",
				CustomConfig: map[string]interface{}{
					"operation": "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := NewUnifiedLLMNode(tt.config)
			if err != nil {
				t.Fatalf("NewUnifiedLLMNode() error = %v", err)
			}

			if err := node.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnifiedLLMNode_GetOperationName(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{
			name:      "generate操作",
			operation: "generate",
			want:      "generate",
		},
		{
			name:      "optimize操作",
			operation: "optimize",
			want:      "optimize",
		},
		{
			name:      "translate操作",
			operation: "translate",
			want:      "translate",
		},
		{
			name:      "summarize操作",
			operation: "summarize",
			want:      "summarize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NodeConfig{
				Backend: "test-backend",
				Model:   "test-model",
				CustomConfig: map[string]interface{}{
					"operation": tt.operation,
				},
			}

			node, err := NewUnifiedLLMNode(config)
			if err != nil {
				t.Fatalf("NewUnifiedLLMNode() error = %v", err)
			}

			// 使用类型断言访问UnifiedLLMNode的方法
			unifiedNode, ok := node.(*UnifiedLLMNode)
			if !ok {
				t.Fatal("node is not *UnifiedLLMNode")
			}

			if got := unifiedNode.GetOperationName(); got != tt.want {
				t.Errorf("GetOperationName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiedLLMNode_SetOperation(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	// 使用类型断言访问UnifiedLLMNode的方法
	unifiedNode, ok := node.(*UnifiedLLMNode)
	if !ok {
		t.Fatal("node is not *UnifiedLLMNode")
	}

	// 测试设置操作类型
	unifiedNode.SetOperation("optimize")
	if unifiedNode.GetOperationName() != "optimize" {
		t.Errorf("SetOperation() failed, got %v, want optimize", unifiedNode.GetOperationName())
	}

	unifiedNode.SetOperation("translate")
	if unifiedNode.GetOperationName() != "translate" {
		t.Errorf("SetOperation() failed, got %v, want translate", unifiedNode.GetOperationName())
	}
}

func TestUnifiedLLMNode_SetTargetLang(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
		CustomConfig: map[string]interface{}{
			"operation": "translate",
		},
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	// 使用类型断言访问UnifiedLLMNode的方法
	unifiedNode, ok := node.(*UnifiedLLMNode)
	if !ok {
		t.Fatal("node is not *UnifiedLLMNode")
	}

	// 测试设置目标语言
	unifiedNode.SetTargetLang("en")
	if unifiedNode.TargetLang != "en" {
		t.Errorf("SetTargetLang() failed, got %v, want en", unifiedNode.TargetLang)
	}

	unifiedNode.SetTargetLang("zh")
	if unifiedNode.TargetLang != "zh" {
		t.Errorf("SetTargetLang() failed, got %v, want zh", unifiedNode.TargetLang)
	}
}

func TestUnifiedLLMNode_IsDeprecated(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	// 使用类型断言访问UnifiedLLMNode的方法
	unifiedNode, ok := node.(*UnifiedLLMNode)
	if !ok {
		t.Fatal("node is not *UnifiedLLMNode")
	}

	// 统一LLM节点不应该被标记为废弃
	if unifiedNode.IsDeprecated() {
		t.Error("IsDeprecated() should return false for UnifiedLLMNode")
	}
}

func TestUnifiedLLMNode_GetDeprecatedReason(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	// 使用类型断言访问UnifiedLLMNode的方法
	unifiedNode, ok := node.(*UnifiedLLMNode)
	if !ok {
		t.Fatal("node is not *UnifiedLLMNode")
	}

	// 统一LLM节点不应该有废弃原因
	if unifiedNode.GetDeprecatedReason() != "" {
		t.Error("GetDeprecatedReason() should return empty string for UnifiedLLMNode")
	}
}

func TestUnifiedLLMNode_GetReplacementNode(t *testing.T) {
	config := NodeConfig{
		Backend: "test-backend",
		Model:   "test-model",
	}

	node, err := NewUnifiedLLMNode(config)
	if err != nil {
		t.Fatalf("NewUnifiedLLMNode() error = %v", err)
	}

	// 使用类型断言访问UnifiedLLMNode的方法
	unifiedNode, ok := node.(*UnifiedLLMNode)
	if !ok {
		t.Fatal("node is not *UnifiedLLMNode")
	}

	// 统一LLM节点不应该有替代节点
	if unifiedNode.GetReplacementNode() != "" {
		t.Error("GetReplacementNode() should return empty string for UnifiedLLMNode")
	}
}