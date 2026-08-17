package pipeline

import (
	"context"
	"testing"
)

func TestKeywordStrategy_Classify(t *testing.T) {
	tests := []struct {
		name           string
		routes         map[string]string
		matchMode      string
		content        string
		expectedTarget string
		expectedConf   float64
		wantErr        bool
	}{
		{
			name: "contains模式匹配",
			routes: map[string]string{
				"代码": "code-generator",
				"翻译": "translate-gen",
			},
			matchMode:      "contains",
			content:        "请帮我写一段代码",
			expectedTarget: "code-generator",
			expectedConf:   1.0,
			wantErr:        false,
		},
		{
			name: "contains模式不匹配",
			routes: map[string]string{
				"代码": "code-generator",
				"翻译": "translate-gen",
			},
			matchMode:      "contains",
			content:        "你好",
			expectedTarget: "",
			expectedConf:   0,
			wantErr:        false,
		},
		{
			name: "prefix模式匹配",
			routes: map[string]string{
				"翻译": "translate-gen",
			},
			matchMode:      "prefix",
			content:        "翻译这段文字",
			expectedTarget: "translate-gen",
			expectedConf:   1.0,
			wantErr:        false,
		},
		{
			name: "prefix模式不匹配",
			routes: map[string]string{
				"翻译": "translate-gen",
			},
			matchMode:      "prefix",
			content:        "请翻译这段文字",
			expectedTarget: "",
			expectedConf:   0,
			wantErr:        false,
		},
		{
			name: "regex模式匹配",
			routes: map[string]string{
				"^代码.*": "code-generator",
			},
			matchMode:      "regex",
			content:        "代码生成",
			expectedTarget: "code-generator",
			expectedConf:   1.0,
			wantErr:        false,
		},
		{
			name: "空路由",
			routes:         nil,
			matchMode:      "contains",
			content:        "测试",
			expectedTarget: "",
			expectedConf:   0,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewKeywordStrategy(tt.routes, tt.matchMode)
			ctx := context.Background()

			target, conf, err := strategy.Classify(ctx, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Classify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if target != tt.expectedTarget {
				t.Errorf("Classify() target = %v, want %v", target, tt.expectedTarget)
			}
			if conf != tt.expectedConf {
				t.Errorf("Classify() confidence = %v, want %v", conf, tt.expectedConf)
			}
		})
	}
}

func TestKeywordStrategy_GetStrategyName(t *testing.T) {
	tests := []struct {
		name      string
		matchMode string
		expected  string
	}{
		{
			name:      "contains模式",
			matchMode: "contains",
			expected:  "keyword_contains",
		},
		{
			name:      "prefix模式",
			matchMode: "prefix",
			expected:  "keyword_prefix",
		},
		{
			name:      "regex模式",
			matchMode: "regex",
			expected:  "keyword_regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewKeywordStrategy(nil, tt.matchMode)
			if got := strategy.GetStrategyName(); got != tt.expected {
				t.Errorf("GetStrategyName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestKeywordStrategy_Validate(t *testing.T) {
	tests := []struct {
		name      string
		routes    map[string]string
		matchMode string
		wantErr   bool
	}{
		{
			name: "有效配置",
			routes: map[string]string{
				"代码": "code-generator",
			},
			matchMode: "contains",
			wantErr:   false,
		},
		{
			name:      "空路由",
			routes:    nil,
			matchMode: "contains",
			wantErr:   true,
		},
		{
			name: "无效匹配模式",
			routes: map[string]string{
				"代码": "code-generator",
			},
			matchMode: "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewKeywordStrategy(tt.routes, tt.matchMode)
			if err := strategy.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLLMClassifyStrategy_GetStrategyName(t *testing.T) {
	strategy := NewLLMClassifyStrategy("test-backend", "test-model", "", nil)
	if got := strategy.GetStrategyName(); got != "llm_classify" {
		t.Errorf("GetStrategyName() = %v, want %v", got, "llm_classify")
	}
}

func TestLLMClassifyStrategy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		model   string
		routes  map[string]string
		wantErr bool
	}{
		{
			name:    "有效配置",
			backend: "test-backend",
			model:   "test-model",
			routes: map[string]string{
				"status-check": "status-check-gen",
			},
			wantErr: false,
		},
		{
			name:    "空backend",
			backend: "",
			model:   "test-model",
			routes: map[string]string{
				"status-check": "status-check-gen",
			},
			wantErr: true,
		},
		{
			name:    "空model",
			backend: "test-backend",
			model:   "",
			routes: map[string]string{
				"status-check": "status-check-gen",
			},
			wantErr: true,
		},
		{
			name:    "空路由",
			backend: "test-backend",
			model:   "test-model",
			routes:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewLLMClassifyStrategy(tt.backend, tt.model, "", tt.routes)
			if err := strategy.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHybridStrategy_GetStrategyName(t *testing.T) {
	keywordStrategy := NewKeywordStrategy(map[string]string{"代码": "code-generator"}, "contains")
	strategy := NewHybridStrategy(keywordStrategy, nil, 0.55)
	if got := strategy.GetStrategyName(); got != "keyword_then_intent" {
		t.Errorf("GetStrategyName() = %v, want %v", got, "keyword_then_intent")
	}
}

func TestHybridStrategy_Validate(t *testing.T) {
	tests := []struct {
		name              string
		keywordStrategy   *KeywordStrategy
		llmStrategy       *LLMClassifyStrategy
		confidenceThreshold float64
		wantErr           bool
	}{
		{
			name:              "有效配置",
			keywordStrategy:   NewKeywordStrategy(map[string]string{"代码": "code-generator"}, "contains"),
			llmStrategy:       nil,
			confidenceThreshold: 0.55,
			wantErr:           false,
		},
		{
			name:              "空关键词策略",
			keywordStrategy:   nil,
			llmStrategy:       nil,
			confidenceThreshold: 0.55,
			wantErr:           true,
		},
		{
			name:              "无效置信度阈值",
			keywordStrategy:   NewKeywordStrategy(map[string]string{"代码": "code-generator"}, "contains"),
			llmStrategy:       nil,
			confidenceThreshold: 1.5,
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewHybridStrategy(tt.keywordStrategy, tt.llmStrategy, tt.confidenceThreshold)
			if err := strategy.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRoutingStrategyRegistry(t *testing.T) {
	registry := NewRoutingStrategyRegistry()

	// 测试注册策略
	keywordStrategy := NewKeywordStrategy(map[string]string{"代码": "code-generator"}, "contains")
	registry.Register("keyword_contains", keywordStrategy)

	// 测试获取策略
	strategy, ok := registry.Get("keyword_contains")
	if !ok {
		t.Error("Get() returned false for registered strategy")
	}
	if strategy == nil {
		t.Error("Get() returned nil for registered strategy")
	}

	// 测试获取不存在的策略
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Get() returned true for nonexistent strategy")
	}

	// 测试获取所有策略
	allStrategies := registry.GetAll()
	if len(allStrategies) != 1 {
		t.Errorf("GetAll() returned %d strategies, want 1", len(allStrategies))
	}
}

func TestRoutingStrategyRegistry_Validate(t *testing.T) {
	registry := NewRoutingStrategyRegistry()

	// 注册有效策略
	keywordStrategy := NewKeywordStrategy(map[string]string{"代码": "code-generator"}, "contains")
	registry.Register("keyword_contains", keywordStrategy)

	// 验证所有策略
	if err := registry.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestGetRoutingStrategyRegistry(t *testing.T) {
	registry := GetRoutingStrategyRegistry()
	if registry == nil {
		t.Error("GetRoutingStrategyRegistry() returned nil")
	}

	// 检查内置策略是否已注册
	builtinStrategies := []string{
		"keyword_contains",
		"keyword_prefix",
		"regex_only",
		"llm_classify",
		"keyword_then_intent",
	}

	for _, name := range builtinStrategies {
		_, ok := registry.Get(name)
		if !ok {
			t.Errorf("builtin strategy %s not registered", name)
		}
	}
}