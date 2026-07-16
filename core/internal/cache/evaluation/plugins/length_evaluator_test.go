package plugins

import (
	"context"
	"strings"
	"testing"

	"centag/core/internal/cache/evaluation/plugin"
)

func TestLengthEvaluatorPlugin_Evaluate(t *testing.T) {
	p := NewLengthEvaluatorPlugin()
	if err := p.Init(); err != nil {
		t.Fatalf("Failed to init plugin: %v", err)
	}

	tests := []struct {
		name         string
		answer       string
		expectPassed bool
		expectLabel  string
		minScore     float64
	}{
		{
			name:         "太短 - 不缓存",
			answer:       "是的。",
			expectPassed: false,
			expectLabel:  "too_short",
			minScore:     0,
		},
		{
			name:         "偏短但可接受",
			answer:       strings.Repeat("这是一个测试句子，用于验证长度评估功能。", 4), // ~120 chars, between min(50) and optimal_min(200)
			expectPassed: true,
			expectLabel:  "short_but_acceptable",
			minScore:     20,
		},
		{
			name:         "最佳长度",
			answer:       strings.Repeat("这是一个详细的测试句子，包含丰富的信息内容。", 15), // ~450 chars, in optimal range
			expectPassed: true,
			expectLabel:  "optimal_length",
			minScore:     70,
		},
		{
			name:         "偏长但可接受",
			answer:       strings.Repeat("这是一个较长的测试句子，用于测试长度评估插件的各种功能和边界条件。", 25), // ~1800 chars
			expectPassed: true,
			expectLabel:  "optimal_length", // Still in optimal range (200-2000)
			minScore:     60,
		},
		{
			name:         "太长 - 不缓存",
			answer:       strings.Repeat("abcdefghij", 600), // 6000 chars > max(5000)
			expectPassed: false,
			expectLabel:  "too_long",
			minScore:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &plugin.EvalInput{
				Answer: tt.answer,
			}

			output, err := p.Evaluate(context.Background(), input)
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}

			if output.Passed != tt.expectPassed {
				t.Errorf("Expected passed=%v, got=%v (score=%.1f)",
					tt.expectPassed, output.Passed, output.Score)
			}

			if output.Score < tt.minScore {
				t.Errorf("Expected score >= %.1f, got %.1f", tt.minScore, output.Score)
			}

			found := false
			for _, label := range output.Labels {
				if label == tt.expectLabel {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected label '%s' not found in %v", tt.expectLabel, output.Labels)
			}

			// 验证详情中包含长度信息
			if length, ok := output.Details["answer_length"].(int); !ok || length == 0 {
				t.Error("Details should contain answer_length")
			}
		})
	}
}

func TestLengthEvaluatorPlugin_ConfigValidation(t *testing.T) {
	p := NewLengthEvaluatorPlugin()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"min_length":  50,
				"optimal_min": 200,
				"optimal_max": 2000,
				"max_length":  5000,
			},
			wantErr: false,
		},
		{
			name: "min >= optimal_min",
			config: map[string]interface{}{
				"min_length":  300,
				"optimal_min": 200,
			},
			wantErr: true,
		},
		{
			name: "optimal_min >= optimal_max",
			config: map[string]interface{}{
				"optimal_min": 2000,
				"optimal_max": 1000,
			},
			wantErr: true,
		},
		{
			name: "optimal_max >= max_length",
			config: map[string]interface{}{
				"optimal_max": 6000,
				"max_length":  5000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.ValidateConfig(tt.config)
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestLengthEvaluatorPlugin_SetAndGetConfig(t *testing.T) {
	p := NewLengthEvaluatorPlugin()
	if err := p.Init(); err != nil {
		t.Fatalf("Failed to init: %v", err)
	}

	config := map[string]interface{}{
		"enabled":     true,
		"min_length":  100.0,
		"optimal_min": 500.0,
		"optimal_max": 3000.0,
		"max_length":  8000.0,
	}

	if err := p.SetConfig(config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	currentConfig := p.GetConfig()

	if minLength := getIntFromConfig(currentConfig, "min_length", 0); minLength != 100 {
		t.Errorf("Expected min_length=100, got %d", minLength)
	}

	if optimalMin := getIntFromConfig(currentConfig, "optimal_min", 0); optimalMin != 500 {
		t.Errorf("Expected optimal_min=500, got %d", optimalMin)
	}
}
