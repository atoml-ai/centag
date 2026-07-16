package scheduler

import (
	"testing"
	"time"

	"centag/core/pkg/backend"
)

func BenchmarkFastMatcher_MatchCategory(b *testing.B) {
	m := NewFastMatcher()

	categories := []string{
		"code",
		"python",
		"java",
		"translate",
		"summary",
		"chat",
		"unknown",
		"代码",
		"翻译",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cat := range categories {
			m.MatchCategory(cat)
		}
	}
}

func BenchmarkFastMatcher_RecommendBackend(b *testing.B) {
	m := NewFastMatcher()

	// 设置后端缓存
	backends := []*backend.BackendConfig{
		{
			ID:      "ollama-local",
			Name:    "Ollama Local",
			Type:    "ollama",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "llama3", ActualModel: "llama3"},
				{RequestedModel: "codellama", ActualModel: "codellama"},
			},
		},
		{
			ID:      "bigmodel",
			Name:    "智谱 AI",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-chat", ActualModel: "deepseek-chat"},
				{RequestedModel: "deepseek-coder", ActualModel: "deepseek-coder"},
			},
		},
	}
	m.UpdateBackendCache(backends)

	taskTypes := []TaskType{
		TaskCodeGeneration,
		TaskSimpleChat,
		TaskComplexReasoning,
		TaskLongText,
		TaskTranslation,
	}

	strategies := []string{"fast", "cost", "quality", "latency"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, taskType := range taskTypes {
			for _, strategy := range strategies {
				m.RecommendBackend(taskType, strategy)
			}
		}
	}
}

func TestFastMatcher_Performance(t *testing.T) {
	m := NewFastMatcher()

	// 设置后端缓存
	backends := []*backend.BackendConfig{
		{
			ID:      "ollama-local",
			Name:    "Ollama Local",
			Type:    "ollama",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "llama3", ActualModel: "llama3"},
			},
		},
		{
			ID:      "bigmodel",
			Name:    "智谱 AI",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
		},
	}
	m.UpdateBackendCache(backends)

	// 测试快速匹配性能
	start := time.Now()
	iterations := 10000

	for i := 0; i < iterations; i++ {
		m.MatchCategory("code")
		m.RecommendBackend(TaskCodeGeneration, "fast")
	}

	elapsed := time.Since(start)
	avgNs := elapsed.Nanoseconds() / int64(iterations)

	t.Logf("Fast matcher performance:")
	t.Logf("  Total iterations: %d", iterations)
	t.Logf("  Total time: %v", elapsed)
	t.Logf("  Average per operation: %d ns", avgNs)

	// 快速匹配应该非常快（<1ms）
	if avgNs > 1000000 { // 1ms = 1000000 ns
		t.Errorf("Fast matcher too slow: %d ns per operation (expected < 1ms)", avgNs)
	}
}
