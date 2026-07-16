package backend

import (
	"encoding/json"
	"testing"
)

func TestModelMatchStrategy_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		strategy ModelMatchStrategy
		expected string
	}{
		{"exact", StrategyExact, "exact"},
		{"family", StrategyFamily, "family"},
		{"capacity", StrategyCapacity, "capacity"},
		{"hybrid", StrategyHybrid, "hybrid"},
		{"custom", StrategyCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.strategy)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(data) != `"`+tt.expected+`"` {
				t.Errorf("MarshalJSON() = %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestModelMatchStrategy_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ModelMatchStrategy
	}{
		{"exact", `"exact"`, StrategyExact},
		{"family", `"family"`, StrategyFamily},
		{"capacity", `"capacity"`, StrategyCapacity},
		{"hybrid", `"hybrid"`, StrategyHybrid},
		{"custom", `"custom"`, StrategyCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var strategy ModelMatchStrategy
			err := json.Unmarshal([]byte(tt.input), &strategy)
			if err != nil {
				t.Fatalf("UnmarshalJSON() error = %v", err)
			}
			if strategy != tt.expected {
				t.Errorf("UnmarshalJSON() = %v, want %v", strategy, tt.expected)
			}
		})
	}
}

func TestGetMinCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		strictness  int
		expected    float64
	}{
		{"strict mode", 0, 1.0},
		{"conservative", 10, 1.0},
		{"conservative boundary", 30, 1.0},
		{"balanced", 50, 0.8},
		{"balanced boundary", 70, 0.8},
		{"relaxed", 80, 0.6},
		{"relaxed boundary", 90, 0.6},
		{"very relaxed", 95, 0.4},
		{"max relaxed", 100, 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMinCompatibility(tt.strictness)
			if result != tt.expected {
				t.Errorf("GetMinCompatibility(%d) = %f, want %f", tt.strictness, result, tt.expected)
			}
		})
	}
}

func TestAllowConversion(t *testing.T) {
	tests := []struct {
		name        string
		strictness  int
		expected    bool
	}{
		{"strict mode", 0, false},
		{"non-strict", 1, true},
		{"medium", 50, true},
		{"very relaxed", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllowConversion(tt.strictness)
			if result != tt.expected {
				t.Errorf("AllowConversion(%d) = %v, want %v", tt.strictness, result, tt.expected)
			}
		})
	}
}

func TestPreferExact(t *testing.T) {
	tests := []struct {
		name        string
		strictness  int
		expected    bool
	}{
		{"strict mode", 0, true},
		{"conservative", 30, true},
		{"balanced", 70, false},
		{"very relaxed", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreferExact(tt.strictness)
			if result != tt.expected {
				t.Errorf("PreferExact(%d) = %v, want %v", tt.strictness, result, tt.expected)
			}
		})
	}
}

func TestDefaultModelMatchingConfig(t *testing.T) {
	cfg := DefaultModelMatchingConfig()

	if cfg.Strategy != StrategyHybrid {
		t.Errorf("Strategy = %v, want %v", cfg.Strategy, StrategyHybrid)
	}

	// HybridWeights should match DefaultHybridWeights()
	defaultWeights := DefaultHybridWeights()
	if cfg.HybridWeights.NameSimilarity != defaultWeights.NameSimilarity {
		t.Errorf("NameSimilarity = %f, want %f", cfg.HybridWeights.NameSimilarity, defaultWeights.NameSimilarity)
	}

	if cfg.HybridWeights.CapacityMatch != defaultWeights.CapacityMatch {
		t.Errorf("CapacityMatch = %f, want %f", cfg.HybridWeights.CapacityMatch, defaultWeights.CapacityMatch)
	}

	if cfg.HybridWeights.FamilyMatch != defaultWeights.FamilyMatch {
		t.Errorf("FamilyMatch = %f, want %f", cfg.HybridWeights.FamilyMatch, defaultWeights.FamilyMatch)
	}

	if cfg.CapacityTolerance != 0.2 {
		t.Errorf("CapacityTolerance = %f, want 0.2", cfg.CapacityTolerance)
	}

	if cfg.DefaultStrictness != 70 {
		t.Errorf("DefaultStrictness = %d, want 70", cfg.DefaultStrictness)
	}
}

func TestModelMapping_JSONRoundTrip(t *testing.T) {
	original := ModelMapping{
		RequestedModel:    "gpt-4",
		ActualModel:       "qwen2.5:7b",
		CompatibilityScore: 0.85,
		IsExact:          false,
	}

	// 序列化
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// 反序列化
	var result ModelMapping
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// 验证
	if result.RequestedModel != original.RequestedModel {
		t.Errorf("RequestedModel = %s, want %s", result.RequestedModel, original.RequestedModel)
	}
	if result.ActualModel != original.ActualModel {
		t.Errorf("ActualModel = %s, want %s", result.ActualModel, original.ActualModel)
	}
	if result.CompatibilityScore != original.CompatibilityScore {
		t.Errorf("CompatibilityScore = %f, want %f", result.CompatibilityScore, original.CompatibilityScore)
	}
	if result.IsExact != original.IsExact {
		t.Errorf("IsExact = %v, want %v", result.IsExact, original.IsExact)
	}
}

func TestModelCapabilities_JSONRoundTrip(t *testing.T) {
	original := ModelCapabilities{
		MaxContextTokens: 32768,
		Features:         []string{"streaming", "function_calling"},
		SupportsImages:   false,
		SupportsTools:    true,
	}

	// 序列化
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// 反序列化
	var result ModelCapabilities
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// 验证
	if result.MaxContextTokens != original.MaxContextTokens {
		t.Errorf("MaxContextTokens = %d, want %d", result.MaxContextTokens, original.MaxContextTokens)
	}
	if len(result.Features) != len(original.Features) {
		t.Errorf("Features length = %d, want %d", len(result.Features), len(original.Features))
	}
	if result.SupportsImages != original.SupportsImages {
		t.Errorf("SupportsImages = %v, want %v", result.SupportsImages, original.SupportsImages)
	}
	if result.SupportsTools != original.SupportsTools {
		t.Errorf("SupportsTools = %v, want %v", result.SupportsTools, original.SupportsTools)
	}
}
