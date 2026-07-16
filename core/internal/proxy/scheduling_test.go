package proxy

import (
	"testing"

	"centag/core/pkg/backend"
)

// TestBuildMatchingConfig_Builtin 测试内置策略 ID 能正确映射
func TestBuildMatchingConfig_Builtin(t *testing.T) {
	cases := []struct {
		strategyID       string
		expectedStrategy backend.ModelMatchStrategy
	}{
		{"exact", backend.StrategyExact},
		{"family", backend.StrategyFamily},
		{"capacity", backend.StrategyCapacity},
		{"hybrid", backend.StrategyHybrid},
	}

	for _, c := range cases {
		t.Run(string(c.strategyID), func(t *testing.T) {
			cfg := buildMatchingConfig(c.strategyID)
			if cfg.Strategy != c.expectedStrategy {
				t.Errorf("buildMatchingConfig(%q).Strategy = %q, want %q",
					c.strategyID, cfg.Strategy, c.expectedStrategy)
			}
		})
	}
}

// TestBuildMatchingConfig_Empty 无策略 ID 时使用全局配置默认值
func TestBuildMatchingConfig_Empty(t *testing.T) {
	cfg := buildMatchingConfig("")
	// 没有全局配置时，应该回退到 DefaultModelMatchingConfig
	if cfg.Strategy == "" {
		t.Error("Strategy should not be empty")
	}
	t.Logf("Default strategy from config: %s", cfg.Strategy)
}

// TestBuildMatchingConfig_CustomResolver 自定义策略解析函数被正确调用
func TestBuildMatchingConfig_CustomResolver(t *testing.T) {
	// 注册测试用的自定义策略解析函数
	original := globalCustomStrategyResolver
	defer func() { globalCustomStrategyResolver = original }()

	SetCustomStrategyResolver(func(id string) *CustomStrategyWeights {
		if id == "my-test-strategy" {
			return &CustomStrategyWeights{
				NameSimilarity: 0.1,
				CapacityMatch:  0.2,
				FamilyMatch:    0.7,
				Strictness:     80,
				Tolerance:      0.15,
			}
		}
		return nil
	})

	cfg := buildMatchingConfig("my-test-strategy")

	// 自定义策略应该被映射为 hybrid 策略
	if cfg.Strategy != backend.StrategyHybrid {
		t.Errorf("Custom strategy should be treated as hybrid, got %q", cfg.Strategy)
	}

	if cfg.HybridWeights.FamilyMatch != 0.7 {
		t.Errorf("FamilyMatch = %.2f, want 0.7", cfg.HybridWeights.FamilyMatch)
	}
	if cfg.HybridWeights.NameSimilarity != 0.1 {
		t.Errorf("NameSimilarity = %.2f, want 0.1", cfg.HybridWeights.NameSimilarity)
	}
	if cfg.DefaultStrictness != 80 {
		t.Errorf("Strictness = %d, want 80", cfg.DefaultStrictness)
	}
	if cfg.CapacityTolerance != 0.15 {
		t.Errorf("Tolerance = %.2f, want 0.15", cfg.CapacityTolerance)
	}
}

// TestBuildMatchingConfig_UnknownStrategyFallback 未知策略 ID 回退到默认配置（不 panic）
func TestBuildMatchingConfig_UnknownStrategyFallback(t *testing.T) {
	original := globalCustomStrategyResolver
	defer func() { globalCustomStrategyResolver = original }()

	// 解析器返回 nil（模拟未找到）
	SetCustomStrategyResolver(func(id string) *CustomStrategyWeights {
		return nil
	})

	cfg := buildMatchingConfig("nonexistent-strategy-xyz")

	// 不应该 panic，应该返回有效配置
	if cfg.Strategy == "" {
		t.Error("Strategy should not be empty after fallback")
	}
	t.Logf("Fallback strategy: %s", cfg.Strategy)
}

// TestCustomStrategyWeights_Fields 验证 CustomStrategyWeights 结构字段完整性
func TestCustomStrategyWeights_Fields(t *testing.T) {
	w := &CustomStrategyWeights{
		NameSimilarity: 0.4,
		CapacityMatch:  0.3,
		FamilyMatch:    0.3,
		Strictness:     60,
		Tolerance:      0.25,
	}

	total := w.NameSimilarity + w.CapacityMatch + w.FamilyMatch
	if total < 0.99 || total > 1.01 {
		t.Errorf("Weights should sum to ~1.0, got %.4f", total)
	}

	if w.Strictness < 0 || w.Strictness > 100 {
		t.Errorf("Strictness out of range: %d", w.Strictness)
	}
}

// TestResolveCustomStrategy_WithoutResolver 未注册解析器时返回 nil
func TestResolveCustomStrategy_WithoutResolver(t *testing.T) {
	original := globalCustomStrategyResolver
	defer func() { globalCustomStrategyResolver = original }()

	globalCustomStrategyResolver = nil
	result := resolveCustomStrategy("any-id")
	if result != nil {
		t.Error("Expected nil when no resolver registered")
	}
}
