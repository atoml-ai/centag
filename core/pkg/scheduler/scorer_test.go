package scheduler

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestModelPriceTable(t *testing.T) {
	pt := NewModelPriceTable()

	t.Run("获取免费模型价格", func(t *testing.T) {
		price := pt.GetPrice("ollama-local", "qwen2.5:1.5b")
		if price.InputPrice != 0 {
			t.Errorf("ollama-local input price = %f, want 0", price.InputPrice)
		}
		if price.Tier != "free" {
			t.Errorf("ollama-local tier = %s, want free", price.Tier)
		}
	})

	t.Run("获取低价模型价格", func(t *testing.T) {
		price := pt.GetPrice("ppinfra", "qwen3.5-plus")
		if price.InputPrice != 1.5 {
			t.Errorf("ppinfra qwen3.5-plus input price = %f, want 1.5", price.InputPrice)
		}
		if price.Tier != "low" {
			t.Errorf("ppinfra qwen3.5-plus tier = %s, want low", price.Tier)
		}
	})

	t.Run("获取高价模型价格", func(t *testing.T) {
		price := pt.GetPrice("bigmodel", "glm-5")
		if price.InputPrice != 20.0 {
			t.Errorf("bigmodel glm-5 input price = %f, want 20.0", price.InputPrice)
		}
		if price.Tier != "high" {
			t.Errorf("bigmodel glm-5 tier = %s, want high", price.Tier)
		}
	})

	t.Run("估算成本", func(t *testing.T) {
		cost := pt.EstimateCost("ppinfra", "qwen3.5-plus", 1000, 1000)
		expected := (1000.0/1_000_000)*1.5 + (1000.0/1_000_000)*1.5
		if cost != expected {
			t.Errorf("Estimated cost = %f, want %f", cost, expected)
		}
	})
}

func TestPerfMetricsCollector(t *testing.T) {
	c := NewPerfMetricsCollector()

	t.Run("记录请求", func(t *testing.T) {
		c.RecordRequest(RequestRecord{
			BackendID: "test-backend",
			Model:     "test-model",
			LatencyMs: 100,
			Success:   true,
		})

		stats := c.GetStats("test-backend")
		if stats == nil {
			t.Fatal("stats is nil")
		}
		if stats.TotalRequests != 1 {
			t.Errorf("TotalRequests = %d, want 1", stats.TotalRequests)
		}
		if stats.SuccessRate != 1.0 {
			t.Errorf("SuccessRate = %f, want 1.0", stats.SuccessRate)
		}
	})

	t.Run("记录质量反馈", func(t *testing.T) {
		c.RecordQualityFeedback("test-backend", 0.9)
		stats := c.GetStats("test-backend")
		if stats.QualityScore < 0.8 {
			t.Errorf("QualityScore = %f, want > 0.8", stats.QualityScore)
		}
	})

	t.Run("获取性能评分", func(t *testing.T) {
		score := c.GetPerformanceScore("test-backend")
		if score < 0.5 {
			t.Errorf("PerformanceScore = %f, want > 0.5", score)
		}
	})
}

func TestLatencyMonitor(t *testing.T) {
	m := NewLatencyMonitor(100)

	t.Run("记录延迟", func(t *testing.T) {
		m.RecordLatency("test-backend", 150)
		m.RecordLatency("test-backend", 160)
		m.RecordLatency("test-backend", 140)

		avgMs, trend, ok := m.GetLatency("test-backend")
		if !ok {
			t.Fatal("GetLatency returned false")
		}
		if avgMs < 140 || avgMs > 160 {
			t.Errorf("AvgLatency = %d, want between 140-160", avgMs)
		}
		t.Logf("AvgLatency: %dms, Trend: %s", avgMs, trend)
	})

	t.Run("获取延迟评分", func(t *testing.T) {
		score := m.GetLatencyScore("test-backend")
		if score < 0.5 {
			t.Errorf("LatencyScore = %f, want > 0.5", score)
		}
	})

	t.Run("健康检查", func(t *testing.T) {
		if !m.IsHealthy("test-backend") {
			t.Error("Backend should be healthy")
		}
	})
}

func TestMultiDimensionScorer(t *testing.T) {
	t.Run("默认权重", func(t *testing.T) {
		weights := DefaultWeights()
		total := weights.Price + weights.Performance + weights.Quality +
			weights.Latency + weights.Privacy + weights.Match
		if total < 0.99 || total > 1.01 {
			t.Errorf("Weights sum = %f, want ~1.0", total)
		}
	})

	t.Run("成本优化工权重", func(t *testing.T) {
		weights := CostOptimizedWeights()
		if weights.Price < 0.35 {
			t.Errorf("Cost optimized price weight = %f, want > 0.35", weights.Price)
		}
	})

	t.Run("质量优化权重", func(t *testing.T) {
		weights := QualityOptimizedWeights()
		if weights.Quality < 0.35 {
			t.Errorf("Quality optimized quality weight = %f, want > 0.35", weights.Quality)
		}
	})
}

func TestScoringDimensions(t *testing.T) {
	s := NewMultiDimensionScorer()

	// 测试价格评分
	t.Run("价格评分", func(t *testing.T) {
		freeScore := s.calculatePriceScore("ollama-local", "qwen2.5")
		if freeScore != 1.0 {
			t.Errorf("Free backend score = %f, want 1.0", freeScore)
		}

		lowScore := s.calculatePriceScore("ppinfra", "qwen3.5-plus")
		if lowScore != 0.8 {
			t.Errorf("Low price backend score = %f, want 0.8", lowScore)
		}

		highScore := s.calculatePriceScore("bigmodel", "glm-5")
		if highScore != 0.2 {
			t.Errorf("High price backend score = %f, want 0.2", highScore)
		}
	})

	// 测试隐私评分
	t.Run("隐私评分", func(t *testing.T) {
		localScore := s.calculatePrivacyScore(&backend.BackendConfig{ID: "ollama-local"})
		if localScore != 1.0 {
			t.Errorf("Local backend privacy score = %f, want 1.0", localScore)
		}
	})
}

func TestGetWeightsForIntent(t *testing.T) {
	s := NewMultiDimensionScorer()

	t.Run("简单对话 - 成本优先", func(t *testing.T) {
		weights := s.getWeightsForIntent(&ClassificationResult{
			TaskType: TaskSimpleChat,
		})
		if weights.Price < 0.35 {
			t.Errorf("Simple chat price weight = %f, want > 0.35", weights.Price)
		}
	})

	t.Run("复杂推理 - 质量优先", func(t *testing.T) {
		weights := s.getWeightsForIntent(&ClassificationResult{
			TaskType: TaskComplexReasoning,
		})
		if weights.Quality < 0.35 {
			t.Errorf("Complex reasoning quality weight = %f, want > 0.35", weights.Quality)
		}
	})

	t.Run("代码生成 - 质量 + 性能", func(t *testing.T) {
		weights := s.getWeightsForIntent(&ClassificationResult{
			TaskType: TaskCodeGeneration,
		})
		if weights.Quality < 0.3 || weights.Performance < 0.2 {
			t.Errorf("Code generation weights not as expected")
		}
	})
}

func TestBackendScore_Calculation(t *testing.T) {
	s := NewMultiDimensionScorer()

	backend := &backend.BackendConfig{
		ID:      "test-backend",
		Name:    "Test Backend",
		Enabled: true,
	}

	intent := &ClassificationResult{
		TaskType:   TaskSimpleChat,
		Complexity: ComplexityLow,
	}

	score := s.Score(&ScoreRequest{
		Backend:      backend,
		Model:        "test-model",
		Intent:       intent,
		InputTokens:  100,
		OutputTokens: 100,
		Weights:      DefaultWeights(),
	})

	if score.TotalScore < 0 || score.TotalScore > 1 {
		t.Errorf("TotalScore = %f, want between 0-1", score.TotalScore)
	}

	t.Logf("Backend: %s, TotalScore: %.2f, Reason: %s",
		score.BackendName, score.TotalScore, score.Reason)
	t.Logf("Dimensions: Price=%.2f, Perf=%.2f, Quality=%.2f, Latency=%.2f, Privacy=%.2f, Match=%.2f",
		score.Dimensions.PriceScore,
		score.Dimensions.PerformanceScore,
		score.Dimensions.QualityScore,
		score.Dimensions.LatencyScore,
		score.Dimensions.PrivacyScore,
		score.Dimensions.MatchScore)
}
