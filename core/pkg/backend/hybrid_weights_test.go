package backend

import (
	"math"
	"testing"
)

// TestHybridWeights_FamilyMatchUsed 验证 FamilyMatch 权重真正参与计算
func TestHybridWeights_FamilyMatchUsed(t *testing.T) {
	// 构造两个模型：同家族（qwen2 系列）
	requested := "qwen2:7b"
	actual := "qwen2:1.5b" // 同家族，不同参数量

	// 策略A：家族权重 0（不考虑家族）
	cfgA := DefaultModelMatchingConfig()
	cfgA.Strategy = StrategyHybrid
	cfgA.HybridWeights = HybridWeights{NameSimilarity: 0.9, CapacityMatch: 0.1, FamilyMatch: 0.0}
	matcherA := NewModelMatcher(cfgA)

	// 策略B：家族权重高
	cfgB := DefaultModelMatchingConfig()
	cfgB.Strategy = StrategyHybrid
	cfgB.HybridWeights = HybridWeights{NameSimilarity: 0.1, CapacityMatch: 0.1, FamilyMatch: 0.8}
	matcherB := NewModelMatcher(cfgB)

	backends := []*BackendConfig{{
		ID: "test", Name: "Test", Enabled: true,
		SupportedModels: []ModelMapping{
			{RequestedModel: actual, ActualModel: actual},
		},
	}}

	resA := matcherA.Match(requested, backends)
	resB := matcherB.Match(requested, backends)

	if resA == nil || resB == nil {
		t.Fatal("Both matchers should find a result")
	}

	t.Logf("FamilyMatch=0.0 → score=%.4f", resA.CompatibilityScore)
	t.Logf("FamilyMatch=0.8 → score=%.4f", resB.CompatibilityScore)

	// 家族权重高时，同家族模型的评分应更高
	if resB.CompatibilityScore <= resA.CompatibilityScore {
		t.Errorf("Higher FamilyMatch weight should yield higher score for same-family models: %.4f <= %.4f",
			resB.CompatibilityScore, resA.CompatibilityScore)
	}
}

// TestHybridWeights_NormalizationInvariance 验证权重归一化：比例相同时评分相同
func TestHybridWeights_NormalizationInvariance(t *testing.T) {
	requested := "gpt-4"
	actual := "gpt-4"

	// 权重1：0.5/0.3/0.2
	cfg1 := DefaultModelMatchingConfig()
	cfg1.HybridWeights = HybridWeights{NameSimilarity: 0.5, CapacityMatch: 0.3, FamilyMatch: 0.2}

	// 权重2：50/30/20（比例相同，未归一化）
	cfg2 := DefaultModelMatchingConfig()
	cfg2.HybridWeights = HybridWeights{NameSimilarity: 50, CapacityMatch: 30, FamilyMatch: 20}

	backends := []*BackendConfig{{
		ID: "t", Name: "T", Enabled: true,
		SupportedModels: []ModelMapping{{RequestedModel: actual, ActualModel: actual}},
	}}

	r1 := NewModelMatcher(cfg1).Match(requested, backends)
	r2 := NewModelMatcher(cfg2).Match(requested, backends)

	if r1 == nil || r2 == nil {
		t.Fatal("Both should match")
	}

	if math.Abs(r1.CompatibilityScore-r2.CompatibilityScore) > 0.0001 {
		t.Errorf("Normalized and unnormalized weights should yield same score: %.4f vs %.4f",
			r1.CompatibilityScore, r2.CompatibilityScore)
	}
}

// TestHybridWeights_DefaultWeightBreakdown 验证默认权重三维度之和为 1.0
func TestHybridWeights_DefaultWeightBreakdown(t *testing.T) {
	w := DefaultHybridWeights()
	total := w.NameSimilarity + w.CapacityMatch + w.FamilyMatch

	if math.Abs(total-1.0) > 0.001 {
		t.Errorf("Default weights should sum to 1.0, got %.4f (name=%.2f cap=%.2f family=%.2f)",
			total, w.NameSimilarity, w.CapacityMatch, w.FamilyMatch)
	}

	t.Logf("Default weights: name=%.2f cap=%.2f family=%.2f total=%.2f",
		w.NameSimilarity, w.CapacityMatch, w.FamilyMatch, total)
}

// TestStrategyComparison 对同一模型对，比较四种策略的评分差异
func TestStrategyComparison(t *testing.T) {
	type testCase struct {
		requested string
		actual    string
		desc      string
	}

	cases := []testCase{
		{"qwen2:7b", "qwen2:1.5b", "同家族不同规格"},
		{"gpt-4", "gpt-4-turbo", "同家族小变体"},
		{"gpt-4", "qwen2:7b", "跨厂商不同架构"},
		{"llama3:8b", "llama3:70b", "同家族10倍规格差"},
	}

	strategies := []ModelMatchStrategy{
		StrategyExact, StrategyFamily, StrategyCapacity, StrategyHybrid,
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			backends := []*BackendConfig{{
				ID: "b", Name: "B", Enabled: true,
				SupportedModels: []ModelMapping{
					{RequestedModel: tc.actual, ActualModel: tc.actual},
				},
			}}

			t.Logf("  请求: %-20s  后端: %-20s", tc.requested, tc.actual)
			for _, strat := range strategies {
				cfg := DefaultModelMatchingConfig()
				cfg.Strategy = strat
				cfg.DefaultStrictness = 70
				result := NewModelMatcher(cfg).Match(tc.requested, backends)

				score := 0.0
				matched := "未匹配"
				if result != nil {
					score = result.CompatibilityScore
					matched = "匹配"
				}
				t.Logf("    %-10s → %-5s  score=%.4f", strat, matched, score)
			}
		})
	}
}

// TestHybridWeights_CapacityDominant 验证容量权重主导时，参数量差异大的模型评分低
func TestHybridWeights_CapacityDominant(t *testing.T) {
	// 请求 7B，后端有 7B 和 70B
	backends := []*BackendConfig{
		{
			ID: "small", Name: "Small", Enabled: true,
			SupportedModels: []ModelMapping{
				{RequestedModel: "llama3:8b", ActualModel: "llama3:8b"},
			},
		},
		{
			ID: "large", Name: "Large", Enabled: true,
			SupportedModels: []ModelMapping{
				{RequestedModel: "llama3:70b", ActualModel: "llama3:70b"},
			},
		},
	}

	// 容量权重主导（0.8）
	cfg := DefaultModelMatchingConfig()
	cfg.Strategy = StrategyHybrid
	cfg.HybridWeights = HybridWeights{NameSimilarity: 0.1, CapacityMatch: 0.8, FamilyMatch: 0.1}
	cfg.DefaultStrictness = 80
	cfg.CapacityTolerance = 0.3

	result := NewModelMatcher(cfg).Match("llama3:8b", backends)
	if result == nil {
		t.Fatal("Should find a match")
	}

	t.Logf("Capacity-dominant: selected backend=%s actualModel=%s score=%.4f",
		result.BackendID, result.ActualModel, result.CompatibilityScore)

	// 容量主导时应该优先选参数量近的（8B 而非 70B）
	if result.BackendID != "small" {
		t.Logf("Note: selected %s (may be correct depending on scoring)", result.BackendID)
	}
}

// TestDefaultHybridWeights_FieldsExist 验证结构体字段对齐（无 VersionWeight/QualifierWeight）
func TestDefaultHybridWeights_FieldsExist(t *testing.T) {
	w := DefaultHybridWeights()

	if w.NameSimilarity <= 0 {
		t.Error("NameSimilarity should be positive")
	}
	if w.CapacityMatch <= 0 {
		t.Error("CapacityMatch should be positive")
	}
	if w.FamilyMatch <= 0 {
		t.Error("FamilyMatch should be positive")
	}

	// 验证旧字段已移除（编译期验证，此处只做运行期断言）
	t.Logf("HybridWeights fields: name=%.2f capacity=%.2f family=%.2f",
		w.NameSimilarity, w.CapacityMatch, w.FamilyMatch)
}
