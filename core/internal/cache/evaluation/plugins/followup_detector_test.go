package plugins

import (
	"context"
	"testing"

	"centag/core/internal/cache/evaluation/plugin"
)

func TestFollowUpDetectorPlugin_Evaluate(t *testing.T) {
	p := NewFollowUpDetectorPlugin()
	if err := p.Init(); err != nil {
		t.Fatalf("Failed to init plugin: %v", err)
	}

	tests := []struct {
		name           string
		question       string
		history        []plugin.Message
		expectPassed   bool
		expectIsFollowUp bool
		expectLabels   []string
	}{
		{
			name:           "第一轮问题 - 不可能是追问",
			question:       "什么是机器学习？",
			history:        []plugin.Message{},
			expectPassed:   true,
			expectIsFollowUp: false,
			expectLabels:   []string{"first_round"},
		},
		{
			name:     "简单指代 - 它是追问",
			question: "它有什么应用？",
			history: []plugin.Message{
				{Role: "user", Content: "什么是机器学习？"},
				{Role: "assistant", Content: "机器学习是人工智能的一个分支..."},
			},
			expectPassed:     false,
			expectIsFollowUp: true,
			expectLabels:     []string{"has_pronouns"},
		},
		{
			name:     "复杂指代 - 这/那",
			question: "这个技术有什么优缺点？",
			history: []plugin.Message{
				{Role: "user", Content: "介绍一下Python"},
				{Role: "assistant", Content: "Python是一种编程语言..."},
			},
			expectPassed:     false,
			expectIsFollowUp: true,
			expectLabels:     []string{"has_pronouns"},
		},
		{
			name:     "短问题 - 可能是追问",
			question: "还有呢？",
			history: []plugin.Message{
				{Role: "user", Content: "列举一些编程语言"},
				{Role: "assistant", Content: "Java, Python, Go..."},
			},
			expectPassed:     false,
			expectIsFollowUp: true,
			expectLabels:     []string{"continuation"}, // 匹配到 continuation 模式
		},
		{
			name:     "详细说说模式",
			question: "详细说说它的原理",
			history: []plugin.Message{
				{Role: "user", Content: "什么是神经网络？"},
				{Role: "assistant", Content: "神经网络是一种..."},
			},
			expectPassed:     false,
			expectIsFollowUp: true,
			expectLabels:     []string{"has_pronouns", "detail_request"},
		},
		{
			name:     "独立问题 - 不是追问",
			question: "Python和Java有什么区别？",
			history: []plugin.Message{
				{Role: "user", Content: "什么是机器学习？"},
				{Role: "assistant", Content: "机器学习是..."},
			},
			expectPassed:     true,
			expectIsFollowUp: false,
			expectLabels:     []string{}, // 无标签表示未检测到追问特征
		},
		{
			name:     "确认性问题",
			question: "对吗？",
			history: []plugin.Message{
				{Role: "user", Content: "Python是解释型语言"},
				{Role: "assistant", Content: "是的，Python是解释型语言..."},
			},
			expectPassed:     true,  // score=1 < threshold=3，不是追问
			expectIsFollowUp: false,
			expectLabels:     []string{"short_question"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &plugin.EvalInput{
				OriginalQuestion: tt.question,
				HistoryMessages:  tt.history,
			}

		output, err := p.Evaluate(context.Background(), input)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}

		if output.Passed != tt.expectPassed {
			t.Errorf("Expected passed=%v, got=%v (score=%.1f)",
				tt.expectPassed, output.Passed, output.Score)
		}

			isFollowUp, ok := output.Metadata["is_follow_up"].(bool)
			if !ok || isFollowUp != tt.expectIsFollowUp {
				t.Errorf("Expected isFollowUp=%v, got=%v",
					tt.expectIsFollowUp, isFollowUp)
			}

			// 检查标签
			for _, expectedLabel := range tt.expectLabels {
				found := false
				for _, actualLabel := range output.Labels {
					if actualLabel == expectedLabel {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected label '%s' not found in %v",
						expectedLabel, output.Labels)
				}
			}
		})
	}
}

func TestFollowUpDetectorPlugin_Config(t *testing.T) {
	p := NewFollowUpDetectorPlugin()
	if err := p.Init(); err != nil {
		t.Fatalf("Failed to init plugin: %v", err)
	}

	// 测试配置schema
	schema := p.GetConfigSchema()
	if schema == nil {
		t.Fatal("Config schema should not be nil")
	}

	if len(schema.Fields) == 0 {
		t.Fatal("Config schema should have fields")
	}

	// 测试设置配置
	config := map[string]interface{}{
		"enabled":         true,
		"score_threshold": 5.0,
		"short_threshold": 20.0,
	}

	if err := p.SetConfig(config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	// 验证配置已设置
	currentConfig := p.GetConfig()
	if enabled, ok := currentConfig["enabled"].(bool); !ok || !enabled {
		t.Error("Enabled config not set correctly")
	}

	if threshold, ok := currentConfig["score_threshold"].(float64); !ok || threshold != 5.0 {
		t.Errorf("Score threshold not set correctly, got %v", threshold)
	}
}

func TestFollowUpDetectorPlugin_ValidateConfig(t *testing.T) {
	p := NewFollowUpDetectorPlugin()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"score_threshold": 5.0,
			},
			wantErr: false,
		},
		{
			name: "threshold too low",
			config: map[string]interface{}{
				"score_threshold": 0.5,
			},
			wantErr: true,
		},
		{
			name: "threshold too high",
			config: map[string]interface{}{
				"score_threshold": 15.0,
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
