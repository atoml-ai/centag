package pipeline

import (
	"fmt"
	"testing"
)

func TestPipelineMetadataResolution(t *testing.T) {
	// 模拟流水线 metadata（与 cache-pipeline.yaml 中的配置一致）
	metadata := map[string]interface{}{
		"cache_strategy":        "exact",
		"cache_storage":         "memory",
		"cache_read_storage":    "memory",
		"cache_write_storage":   "memory",
		"token_usage_operation": "record",
	}

	// 创建流水线
	pipeline := &AgentPatternPipeline{
		ID:       "cache-pipeline",
		Metadata: metadata,
	}

	// 创建执行上下文
	execCtx := NewExecutionContext(pipeline)

	// 创建输入
	input := &NodeInput{
		Content: "test",
		Metadata: map[string]interface{}{
			"model": "test-model",
		},
	}

	// 创建解析器
	resolver := NewTemplateVarResolver(input, execCtx)

	// 测试解析 pipeline 变量
	tests := []struct {
		path     string
		expected string
	}{
		{"pipeline.cache_strategy", "exact"},
		{"pipeline.cache_storage", "memory"},
		{"pipeline.cache_read_storage", "memory"},
		{"pipeline.cache_write_storage", "memory"},
		{"pipeline.token_usage_operation", "record"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result, err := resolver.Resolve(tt.path)
			if err != nil {
				t.Errorf("Resolve(%q) error: %v", tt.path, err)
				return
			}
			if fmt.Sprintf("%v", result) != tt.expected {
				t.Errorf("Resolve(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestEmptyStringDefaultResolution(t *testing.T) {
	// 回归：显式 default: '' 必须生效（cache-pipeline.yaml 的
	// vector_storage_name / embedding_backend_id / embedding_model 均使用该语法），
	// 解析失败时应返回空字符串而非残留字面 {{...}} 模板。
	pipeline := &AgentPatternPipeline{
		ID:       "cache-pipeline",
		Metadata: map[string]interface{}{},
	}
	execCtx := NewExecutionContext(pipeline)
	input := &NodeInput{Content: "test"}
	resolver := NewTemplateVarResolver(input, execCtx)

	tests := []struct {
		path     string
		expected string
	}{
		{"pipeline.vector_storage | default: ''", ""},
		{"pipeline.embedding_backend | default: ''", ""},
		{"pipeline.embedding_model | default: ''", ""},
		{"pipeline.missing_key | default: 'postgresql'", "postgresql"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result, err := resolver.Resolve(tt.path)
			if err != nil {
				t.Errorf("Resolve(%q) error: %v", tt.path, err)
				return
			}
			s, ok := result.(string)
			if !ok {
				t.Errorf("Resolve(%q) = %T, want string", tt.path, result)
				return
			}
			if s != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.path, s, tt.expected)
			}
		})
	}

	// 无 default 时缺失 key 仍应返回错误（不静默吞掉）
	if _, err := resolver.Resolve("pipeline.missing_key"); err == nil {
		t.Error("Resolve without default should error on missing key")
	}
}

func TestCacheNodeTemplateResolution(t *testing.T) {
	// 模拟流水线 metadata
	metadata := map[string]interface{}{
		"cache_strategy":      "exact",
		"cache_storage":       "memory",
		"cache_read_storage":  "memory",
		"cache_write_storage": "memory",
	}

	// 创建流水线
	pipeline := &AgentPatternPipeline{
		ID:       "cache-pipeline",
		Metadata: metadata,
	}

	// 创建执行上下文
	execCtx := NewExecutionContext(pipeline)

	// 创建 CacheNode，使用模板变量
	node := &CacheNode{
		BaseNode: BaseNode{
			config: NodeConfig{},
		},
		Operation:        "read",
		Strategy:         "{{pipeline.cache_strategy | default: 'exact'}}",
		ReadStorageName:  "{{pipeline.cache_read_storage | default: 'memory'}}",
		WriteStorageName: "{{pipeline.cache_write_storage | default: 'memory'}}",
	}

	// 创建输入
	input := &NodeInput{
		Content: "test",
		Metadata: map[string]interface{}{
			"model": "test-model",
		},
	}

	// 创建解析器
	resolver := NewTemplateVarResolver(input, execCtx)

	// 模拟 Execute 中的模板变量解析逻辑
	if containsTemplate(node.ReadStorageName) {
		resolved, err := resolver.Resolve(extractTemplatePath(node.ReadStorageName))
		if err != nil {
			t.Errorf("Resolve ReadStorageName error: %v", err)
		} else {
			if s, ok := resolved.(string); ok {
				node.ReadStorageName = s
			}
		}
	}

	if containsTemplate(node.WriteStorageName) {
		resolved, err := resolver.Resolve(extractTemplatePath(node.WriteStorageName))
		if err != nil {
			t.Errorf("Resolve WriteStorageName error: %v", err)
		} else {
			if s, ok := resolved.(string); ok {
				node.WriteStorageName = s
			}
		}
	}

	if containsTemplate(node.Strategy) {
		resolved, err := resolver.Resolve(extractTemplatePath(node.Strategy))
		if err != nil {
			t.Errorf("Resolve Strategy error: %v", err)
		} else {
			if s, ok := resolved.(string); ok {
				node.Strategy = s
			}
		}
	}

	// 验证解析结果
	if node.ReadStorageName != "memory" {
		t.Errorf("ReadStorageName = %q, want 'memory'", node.ReadStorageName)
	}
	if node.WriteStorageName != "memory" {
		t.Errorf("WriteStorageName = %q, want 'memory'", node.WriteStorageName)
	}
	if node.Strategy != "exact" {
		t.Errorf("Strategy = %q, want 'exact'", node.Strategy)
	}
}

func containsTemplate(s string) bool {
	return len(s) > 2 && s[:2] == "{{"
}

func extractTemplatePath(s string) string {
	if len(s) > 4 && s[:2] == "{{" && s[len(s)-2:] == "}}" {
		return s[2 : len(s)-2]
	}
	return s
}
