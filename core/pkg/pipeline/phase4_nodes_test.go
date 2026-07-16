package pipeline

import (
	"testing"
)

// TestPhase4NodeTypes 测试 Phase 4 新增的节点类型
func TestPhase4NodeTypes(t *testing.T) {
	tests := []struct {
		name     NodeType
		expected bool
	}{
		{NodeTypeCache, true},
		{NodeTypeTokenUsage, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			if got := tt.name.IsValid(); got != tt.expected {
				t.Errorf("NodeType(%s).IsValid() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// TestPhase4NodeKinds 测试 Phase 4 节点的 Kind 映射
func TestPhase4NodeKinds(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		expected string
	}{
		{NodeTypeCache, "cache.access"},
		{NodeTypeTokenUsage, "metrics.token_usage"},
	}

	for _, tt := range tests {
		t.Run(string(tt.nodeType), func(t *testing.T) {
			got := KindForBuiltinType(tt.nodeType)
			if got != tt.expected {
				t.Errorf("KindForBuiltinType(%s) = %q, want %q", tt.nodeType, got, tt.expected)
			}
		})
	}
}

// TestPhase4NodeSchemas 测试 Phase 4 节点的 Schema
func TestPhase4NodeSchemas(t *testing.T) {
	tests := []struct {
		name NodeType
	}{
		{NodeTypeCache},
		{NodeTypeTokenUsage},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			schemas := GetBuiltinSchemas(tt.name)
			if schemas.ConfigSchema == nil {
				t.Errorf("GetBuiltinSchemas(%s).ConfigSchema is nil", tt.name)
			}
			if schemas.InputSchema == nil {
				t.Errorf("GetBuiltinSchemas(%s).InputSchema is nil", tt.name)
			}
			if schemas.OutputSchema == nil {
				t.Errorf("GetBuiltinSchemas(%s).OutputSchema is nil", tt.name)
			}
		})
	}
}

// TestCacheNodeCreation 测试缓存节点创建
func TestCacheNodeCreation(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"ttl":          float64(3600),
			"key_template": "{{model}}:{{hash}}",
		},
	}

	node, err := NewCacheNode(config)
	if err != nil {
		t.Fatalf("NewCacheNode failed: %v", err)
	}

	if node.Type() != NodeTypeCache {
		t.Errorf("node.Type() = %v, want %v", node.Type(), NodeTypeCache)
	}

	if err := node.Validate(); err != nil {
		t.Errorf("node.Validate() failed: %v", err)
	}
}

// TestTokenUsageNodeCreation 测试 Token 计量节点创建
func TestTokenUsageNodeCreation(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":     "record",
			"storage_type":  "memory",
			"record_fields": []interface{}{"prompt_tokens", "completion_tokens", "total_tokens"},
		},
	}

	node, err := NewTokenUsageNode(config)
	if err != nil {
		t.Fatalf("NewTokenUsageNode failed: %v", err)
	}

	if node.Type() != NodeTypeTokenUsage {
		t.Errorf("node.Type() = %v, want %v", node.Type(), NodeTypeTokenUsage)
	}

	if err := node.Validate(); err != nil {
		t.Errorf("node.Validate() failed: %v", err)
	}
}

// TestCacheNodeInvalidOperation 测试缓存节点无效操作
func TestCacheNodeInvalidOperation(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "invalid",
		},
	}

	node, err := NewCacheNode(config)
	if err != nil {
		t.Fatalf("NewCacheNode failed: %v", err)
	}

	if err := node.Validate(); err == nil {
		t.Error("node.Validate() should fail for invalid operation")
	}
}

// TestTokenUsageNodeInvalidStorage 测试 Token 计量节点无效存储
func TestTokenUsageNodeInvalidStorage(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "record",
			"storage_type": "invalid",
		},
	}

	node, err := NewTokenUsageNode(config)
	if err != nil {
		t.Fatalf("NewTokenUsageNode failed: %v", err)
	}

	if err := node.Validate(); err == nil {
		t.Error("node.Validate() should fail for invalid storage_type")
	}
}
