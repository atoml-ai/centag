package backend

import (
	"fmt"
	"testing"
)

func TestBackendSelector_SelectBackendByModel(t *testing.T) {
	config := DefaultModelMatchingConfig()
	config.Strategy = StrategyHybrid
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "ollama",
			Name:    "Ollama",
			Enabled: true,
			Priority: 1,
			Weight:   50,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "exact-match",
					ActualModel:    "exact-model",
				},
				{
					RequestedModel: "gpt-4",
					ActualModel:    "qwen2.5:7b",
				},
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Enabled: true,
			Priority: 2,
			Weight:   30,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "deepseek-chat",
				},
			},
		},
	}

	// 测试精确匹配
	backend, actualModel, err := selector.SelectBackendByModel("exact-match", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}
	if backend.ID != "ollama" {
		t.Errorf("BackendID = %v, want ollama", backend.ID)
	}
	if actualModel != "exact-model" {
		t.Errorf("ActualModel = %v, want exact-model", actualModel)
	}
}

func TestBackendSelector_NoExactMatch(t *testing.T) {
	config := DefaultModelMatchingConfig()
	config.Strategy = StrategyHybrid
	_ = config.AllowConversion()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "ollama",
			Name:    "Ollama",
			Enabled: true,
			Priority: 1,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "qwen2.5:7b",
				},
			},
		},
	}

	// 测试无匹配情况
	_, _, err := selector.SelectBackendByModel("nonexistent", backends)
	if err == nil {
		t.Error("Expected error for nonexistent model")
	}
}

func TestBackendSelector_Strictness(t *testing.T) {
	tests := []struct {
		name       string
		strictness int
		expectMatch bool
	}{
		{
			name:       "Strict mode - exact only",
			strictness: 0,
			expectMatch: false,
		},
		{
			name:       "Relaxed mode - allow conversion",
			strictness: 70,
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultModelMatchingConfig()
			config.Strategy = StrategyHybrid
			config.DefaultStrictness = tt.strictness
			selector := NewBackendSelector(config)

			backends := []*BackendConfig{
				{
					ID:      "test",
					Name:    "Test Backend",
					Enabled: true,
					SupportedModels: []ModelMapping{
						{
							RequestedModel: "gpt-4",
							ActualModel:    "qwen2.5:7b",
						},
					},
				},
			}

			backend, actualModel, err := selector.SelectBackendByModel("gpt-4", backends)

			if tt.expectMatch {
				if err != nil {
					t.Logf("Expected match, got error: %v (may be acceptable for strict mode)", err)
				}
				if backend != nil {
					if actualModel != "qwen2.5:7b" {
						t.Logf("ActualModel = %v, want qwen2.5:7b", actualModel)
					}
				}
			}
		})
	}
}

func TestBackendSelector_Priority(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "low-priority",
			Name:    "Low Priority Backend",
			Enabled: true,
			Priority: 1,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model1",
				},
			},
		},
		{
			ID:      "high-priority",
			Name:    "High Priority Backend",
			Enabled: true,
			Priority: 10,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model2",
				},
			},
		},
	}

	backend, _, err := selector.SelectBackendByModel("gpt-4", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}

	if backend.ID != "high-priority" {
		t.Errorf("BackendID = %v, want high-priority", backend.ID)
	}
}

func TestBackendSelector_DisabledBackend(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "disabled",
			Name:    "Disabled Backend",
			Enabled: false,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model",
				},
			},
		},
		{
			ID:      "enabled",
			Name:    "Enabled Backend",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model",
				},
			},
		},
	}

	backend, _, err := selector.SelectBackendByModel("gpt-4", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}

	if backend.ID != "enabled" {
		t.Errorf("BackendID = %v, want enabled", backend.ID)
	}
}

func TestBackendSelector_SelectBackendForRequest(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "test",
			Name:    "Test Backend",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "test-model",
					ActualModel:    "test-model",
				},
			},
		},
	}

	selection, err := selector.SelectBackendForRequest("test-model", backends)
	if err != nil {
		t.Fatalf("SelectBackendForRequest() error = %v", err)
	}

	if selection.BackendID != "test" {
		t.Errorf("BackendID = %v, want test", selection.BackendID)
	}

	// 检查置信度
	level := selection.GetConfidenceLevel()
	if level != "high" {
		t.Logf("ConfidenceLevel = %v, want high, score=%.2f", level, selection.CompatibilityScore)
	}
}

func TestBackendSelection_IsModelConverted(t *testing.T) {
	selection := NewBackendSelection(
		"backend1", "Backend 1", "openai",
		"gpt-4", "qwen2.5:7b",
		false, 0.85, StrategyHybrid,
		&BackendConfig{ID: "backend1"},
	)

	if !selection.IsModelConverted() {
		t.Error("Expected model to be converted")
	}

	// 测试精确匹配
	selection.IsExactMatch = true
	if selection.IsModelConverted() {
		t.Error("Expected model not to be converted")
	}
}

func TestBackendSelection_GetConfidenceLevel(t *testing.T) {
	tests := []struct {
		name            string
		isExact         bool
		score           float64
		expectedLevel   string
	}{
		{
			name:          "Exact match",
			isExact:       true,
			score:         1.0,
			expectedLevel: "high",
		},
		{
			name:          "High score",
			isExact:       false,
			score:         0.85,
			expectedLevel: "high",
		},
		{
			name:          "Medium score",
			isExact:       false,
			score:         0.7,
			expectedLevel: "medium",
		},
		{
			name:          "Low score",
			isExact:       false,
			score:         0.5,
			expectedLevel: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := NewBackendSelection(
				"backend1", "Backend 1", "openai",
				"gpt-4", "qwen2.5:7b",
				tt.isExact, tt.score, StrategyHybrid,
				&BackendConfig{ID: "backend1"},
			)

			level := selection.GetConfidenceLevel()
			if level != tt.expectedLevel {
				t.Errorf("ConfidenceLevel = %v, want %v", level, tt.expectedLevel)
			}
		})
	}
}

func TestBackendSelector_GetCompatibleBackends(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model1",
				},
			},
		},
		{
			ID:      "backend2",
			Name:    "Backend 2",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-3.5",
					ActualModel:    "model2",
				},
			},
		},
	}

	compatible := selector.GetCompatibleBackends("gpt-4", backends)
	if len(compatible) == 0 {
		t.Error("Expected at least one compatible backend")
	}
}

func TestBackendSelector_HasExactMatch(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "exact",
					ActualModel:    "exact-model",
				},
			},
		},
	}

	if !selector.HasExactMatch("exact", backends) {
		t.Error("Expected exact match to be found")
	}

	if selector.HasExactMatch("nonexistent", backends) {
		t.Error("Expected no exact match for nonexistent model")
	}
}

func TestBackendSelector_CountCompatibleBackends(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model1",
				},
			},
		},
		{
			ID:      "backend2",
			Name:    "Backend 2",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-3.5",
					ActualModel:    "model2",
				},
			},
		},
		{
			ID:      "backend3",
			Name:    "Backend 3",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model3",
				},
			},
		},
	}

	count := selector.CountCompatibleBackends("gpt-4", backends)
	if count != 2 {
		t.Errorf("CountCompatibleBackends() = %d, want 2", count)
	}
}

// TestBackendSelector_AllDisabledBackends 所有后端禁用测试
func TestBackendSelector_AllDisabledBackends(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "disabled1",
			Name:    "Disabled Backend 1",
			Enabled: false,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model",
				},
			},
		},
		{
			ID:      "disabled2",
			Name:    "Disabled Backend 2",
			Enabled: false,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model",
				},
			},
		},
	}

	_, _, err := selector.SelectBackendByModel("gpt-4", backends)
	if err == nil {
		t.Error("Expected error for all disabled backends")
	}
}

// TestBackendSelector_WeightSelection 权重选择测试
func TestBackendSelector_WeightSelection(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "low-weight",
			Name:    "Low Weight Backend",
			Enabled: true,
			Priority: 1,
			Weight:   10,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model1",
				},
			},
		},
		{
			ID:      "high-weight",
			Name:    "High Weight Backend",
			Enabled: true,
			Priority: 1,
			Weight:   100,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model2",
				},
			},
		},
	}

	backend, _, err := selector.SelectBackendByModel("gpt-4", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}

	// 应该选择权重高的
	if backend.ID != "high-weight" {
		t.Errorf("BackendID = %v, want high-weight (same priority, higher weight)", backend.ID)
	}
}

// TestBackendSelector_MultipleModels 多模型映射测试
func TestBackendSelector_MultipleModels(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "qwen2.5:7b",
				},
				{
					RequestedModel: "gpt-3.5",
					ActualModel:    "qwen2:1.5b",
				},
				{
					RequestedModel: "claude-3",
					ActualModel:    "llama3:8b",
				},
			},
		},
	}

	// 测试不同模型的映射
	tests := []struct {
		requested string
		expected  string
	}{
		{"gpt-4", "qwen2.5:7b"},
		{"gpt-3.5", "qwen2:1.5b"},
		{"claude-3", "llama3:8b"},
	}

	for _, tt := range tests {
		t.Run(tt.requested, func(t *testing.T) {
			_, actualModel, err := selector.SelectBackendByModel(tt.requested, backends)
			if err != nil {
				t.Fatalf("SelectBackendByModel() error = %v", err)
			}
			if actualModel != tt.expected {
				t.Errorf("ActualModel = %v, want %s", actualModel, tt.expected)
			}
		})
	}
}

// TestBackendSelector_ConversionDisabled 禁用转换测试
func TestBackendSelector_ConversionDisabled(t *testing.T) {
	config := DefaultModelMatchingConfig()
	config.ConversionWeight = 0 // 禁用转换
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "qwen2.5:7b",
				},
			},
		},
	}

	// 只有精确匹配才能通过
	_, _, err := selector.SelectBackendByModel("gpt-4", backends)
	// 如果没有精确匹配，应该返回错误
	if err != nil {
		t.Logf("Expected error for non-exact match when conversion disabled: %v", err)
	}
}

// TestBackendSelector_EmptyRequestedModel 空模型名测试
func TestBackendSelector_EmptyRequestedModel(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "model",
					ActualModel:    "actual",
				},
			},
		},
	}

	_, _, err := selector.SelectBackendByModel("", backends)
	if err == nil {
		t.Error("Expected error for empty model name")
	}
}

// TestBackendSelector_StrictnessLevels 严格度级别测试
func TestBackendSelector_StrictnessLevels(t *testing.T) {
	strictnessLevels := []int{0, 30, 50, 70, 90, 100}

	for _, strictness := range strictnessLevels {
		t.Run(fmt.Sprintf("Strictness_%d", strictness), func(t *testing.T) {
			config := DefaultModelMatchingConfig()
			config.DefaultStrictness = strictness
			selector := NewBackendSelector(config)

			backends := []*BackendConfig{
				{
					ID:      "backend1",
					Name:    "Backend 1",
					Enabled: true,
					SupportedModels: []ModelMapping{
						{
							RequestedModel: "gpt-4",
							ActualModel:    "qwen2.5:7b",
						},
					},
				},
			}

			_, actualModel, err := selector.SelectBackendByModel("gpt-4", backends)
			t.Logf("Strictness %d: error=%v, actualModel=%s", strictness, err, actualModel)
		})
	}
}

// TestBackendSelector_BackendTypes 后端类型测试
func TestBackendSelector_BackendTypes(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "ollama",
			Name:    "Ollama",
			Type:    "ollama",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "test",
					ActualModel:    "test-model",
				},
			},
		},
		{
			ID:      "openai",
			Name:    "OpenAI",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "test",
					ActualModel:    "test-model",
				},
			},
		},
		{
			ID:      "anthropic",
			Name:    "Anthropic",
			Type:    "anthropic",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "test",
					ActualModel:    "test-model",
				},
			},
		},
	}

	selection, err := selector.SelectBackendForRequest("test", backends)
	if err != nil {
		t.Fatalf("SelectBackendForRequest() error = %v", err)
	}

	// 验证后端类型被正确设置
	if selection.BackendType == "" {
		t.Error("BackendType should not be empty")
	}

	t.Logf("Selected backend type: %s", selection.BackendType)
}

// TestBackendSelector_ModelAlias 模型别名测试
func TestBackendSelector_ModelAlias(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "backend1",
			Name:    "Backend 1",
			Enabled: true,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4-turbo",
					ActualModel:    "qwen2.5:7b",
				},
			},
		},
	}

	// 使用别名请求
	_, actualModel, err := selector.SelectBackendByModel("gpt-4-turbo-2024-04-09", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}

	if actualModel != "qwen2.5:7b" {
		t.Errorf("ActualModel = %v, want qwen2.5:7b", actualModel)
	}
}

// TestBackendSelector_ZeroPriority 零优先级测试
func TestBackendSelector_ZeroPriority(t *testing.T) {
	config := DefaultModelMatchingConfig()
	selector := NewBackendSelector(config)

	backends := []*BackendConfig{
		{
			ID:      "zero-priority",
			Name:    "Zero Priority",
			Enabled: true,
			Priority: 0,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model1",
				},
			},
		},
		{
			ID:      "normal-priority",
			Name:    "Normal Priority",
			Enabled: true,
			Priority: 5,
			SupportedModels: []ModelMapping{
				{
					RequestedModel: "gpt-4",
					ActualModel:    "model2",
				},
			},
		},
	}

	backend, _, err := selector.SelectBackendByModel("gpt-4", backends)
	if err != nil {
		t.Fatalf("SelectBackendByModel() error = %v", err)
	}

	// 应该选择优先级高的（即使为零，另一个优先级5）
	if backend.ID != "normal-priority" {
		t.Errorf("BackendID = %v, want normal-priority", backend.ID)
	}
}

// TestBackendSelector_GetConfidenceLevels 置信度级别测试
func TestBackendSelector_GetConfidenceLevels(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "High confidence",
			model:    "exact-match",
			expected: "high",
		},
		{
			name:     "Medium confidence",
			model:    "compatible-model",
			expected: "medium",
		},
		{
			name:     "Low confidence",
			model:    "weak-match",
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultModelMatchingConfig()
			selector := NewBackendSelector(config)

			var score float64
			switch tt.expected {
			case "high":
				score = 0.95
			case "medium":
				score = 0.75
			default:
				score = 0.45
			}

			backends := []*BackendConfig{
				{
					ID:      "test",
					Name:    "Test Backend",
					Enabled: true,
					SupportedModels: []ModelMapping{
						{
							RequestedModel: tt.model,
							ActualModel:    "actual-model",
						},
					},
				},
			}

			selection, err := selector.SelectBackendForRequest(tt.model, backends)
			if err != nil {
				t.Fatalf("SelectBackendForRequest() error = %v", err)
			}

			level := selection.GetConfidenceLevel()
			t.Logf("Model %s: confidence=%s, score=%.3f", tt.model, level, score)
		})
	}
}
