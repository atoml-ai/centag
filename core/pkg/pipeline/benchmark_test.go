package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkNodeExecution 测试节点执行性能
func BenchmarkNodeExecution(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_ = reqCtx
		cancel()
	}
}

// BenchmarkNodeConfigAccess 测试节点配置访问性能
func BenchmarkNodeConfigAccess(b *testing.B) {
	config := NodeConfig{
		Backend:        "openai",
		Model:          "gpt-4",
		PromptTemplate: "Hello {{name}}",
		Temperature:    floatPtr(0.7),
		MaxTokens:      intPtr(100),
		CustomConfig: map[string]interface{}{
			"timeout": 30,
			"retry":   3,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Backend
		_ = config.Model
		_ = config.CustomConfig["timeout"]
	}
}

// BenchmarkNodeInputCreation 测试节点输入创建性能
func BenchmarkNodeInputCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := &NodeInput{
			Content: fmt.Sprintf("Test input %d", i),
			Metadata: map[string]interface{}{
				"index": i,
			},
		}
		_ = input
	}
}

// BenchmarkNodeOutputCreation 测试节点输出创建性能
func BenchmarkNodeOutputCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output := &NodeOutput{
			Content: fmt.Sprintf("Test output %d", i),
			Metadata: map[string]interface{}{
				"tokens": i * 10,
			},
		}
		_ = output
	}
}

// BenchmarkPipelineContextCreation 测试流水线上下文创建性能
func BenchmarkPipelineContextCreation(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipelineCtx := &PipelineContext{
			TraceID:   fmt.Sprintf("trace-%d", i),
			RequestID: fmt.Sprintf("req-%d", i),
			StartTime: time.Now(),
		}
		_ = pipelineCtx
		_ = ctx
	}
}

// BenchmarkNodeIDGeneration 测试节点 ID 生成性能
func BenchmarkNodeIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		_ = nodeID
	}
}

func BenchmarkConfigCompatLayer_GetActualPipelineID(b *testing.B) {
	compat := NewConfigCompatLayer()

	// 基准测试ID转换性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compat.GetActualPipelineID("transparent-proxy")
		compat.GetActualPipelineID("direct-backend")
		compat.GetActualPipelineID("fixed-egress")
		compat.GetActualPipelineID("router-mode")
		compat.GetActualPipelineID("agent-skill-router")
		compat.GetActualPipelineID("cache-hit")
		compat.GetActualPipelineID("cache-mode")
		compat.GetActualPipelineID("18-rag-mode")
	}
}

func BenchmarkConfigCompatLayer_IsLegacyPipeline(b *testing.B) {
	compat := NewConfigCompatLayer()

	configs := []map[string]interface{}{
		{"id": "transparent-proxy"},
		{"id": "direct-backend"},
		{"id": "fixed-egress"},
		{"id": "transparent-passthrough"},
		{"id": "transparent"},
		{"id": "router-mode"},
		{"id": "agent-skill-router"},
		{"id": "cache-hit"},
		{"id": "cache-mode"},
		{"id": "18-rag-mode"},
		{"id": "router-pipeline"},
		{"id": "centag-ops-router"},
		{"id": "cache-pipeline"},
	}

	// 基准测试旧版检测性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, config := range configs {
			compat.IsLegacyPipeline(config)
		}
	}
}

func BenchmarkConfigCompatLayer_ConvertPipelineConfig(b *testing.B) {
	compat := NewConfigCompatLayer()

	oldConfigs := []map[string]interface{}{
		{
			"id":            "transparent-proxy",
			"shortcut_code": "#t",
			"nodes": []interface{}{
				map[string]interface{}{
					"id":   "forward",
					"type": "transparent_forward",
					"config": map[string]interface{}{
						"custom_config": map[string]interface{}{
							"route_policy":         "match_model",
							"inject_system_prompt": false,
						},
					},
				},
			},
		},
		{
			"id":            "router-mode",
			"shortcut_code": "#r",
			"nodes": []interface{}{
				map[string]interface{}{
					"id":   "classifier",
					"type": "router",
					"config": map[string]interface{}{
						"custom_config": map[string]interface{}{
							"routing_strategy": "keyword_contains",
						},
					},
				},
			},
		},
		{
			"id":            "cache-hit",
			"shortcut_code": "#cache",
			"nodes": []interface{}{
				map[string]interface{}{
					"id":   "cache_read",
					"type": "cache",
					"config": map[string]interface{}{
						"custom_config": map[string]interface{}{
							"operation": "read",
							"strategy":  "exact",
						},
					},
				},
			},
		},
	}

	// 基准测试配置转换性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, config := range oldConfigs {
			compat.ConvertPipelineConfig(config)
		}
	}
}

func BenchmarkKeywordStrategy_Classify(b *testing.B) {
	strategy := NewKeywordStrategy(map[string]string{
		"代码":   "code-generator",
		"翻译":   "translate-gen",
		"摘要":   "summary-gen",
		"python": "code-generator",
		"java":   "code-generator",
		"go":     "code-generator",
	}, "contains")

	content := "请帮我写一段python代码"

	// 基准测试关键词匹配性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Classify(nil, content)
	}
}

func BenchmarkRoutingStrategyRegistry_Get(b *testing.B) {
	registry := GetRoutingStrategyRegistry()

	strategyNames := []string{
		"keyword_contains",
		"keyword_prefix",
		"regex_only",
		"llm_classify",
		"keyword_then_intent",
	}

	// 基准测试策略获取性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range strategyNames {
			registry.Get(name)
		}
	}
}

// PipelineContext 流水线上下文
type PipelineContext struct {
	TraceID   string
	RequestID string
	StartTime time.Time
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
