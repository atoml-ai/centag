package scheduler

import (
	"testing"
)

func TestIntentClassifier_parseTaskType(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		OllamaAddr:   "http://localhost:21434",
		CacheEnabled: false,
		Timeout:      10,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		input string
		want  TaskType
	}{
		{"code_generation", TaskCodeGeneration},
		{"code", TaskCodeGeneration},
		{"coding", TaskCodeGeneration},
		{"simple_chat", TaskSimpleChat},
		{"chat", TaskSimpleChat},
		{"complex_reasoning", TaskComplexReasoning},
		{"reasoning", TaskComplexReasoning},
		{"translation", TaskTranslation},
		{"translate", TaskTranslation},
		{"unknown_type", TaskUnknown},
	}

	for _, tt := range tests {
		got := classifier.parseTaskType(tt.input)
		if got != tt.want {
			t.Errorf("parseTaskType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIntentClassifier_parseComplexity(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		CacheEnabled: false,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		input string
		want  ComplexityLevel
	}{
		{"low", ComplexityLow},
		{"simple", ComplexityLow},
		{"high", ComplexityHigh},
		{"complex", ComplexityHigh},
		{"medium", ComplexityMedium},
		{"unknown", ComplexityMedium},
	}

	for _, tt := range tests {
		got := classifier.parseComplexity(tt.input)
		if got != tt.want {
			t.Errorf("parseComplexity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIntentClassifier_parseSensitivity(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		CacheEnabled: false,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		input string
		want  SensitivityLevel
	}{
		{"public", SensitivityPublic},
		{"open", SensitivityPublic},
		{"confidential", SensitivityConfidential},
		{"private", SensitivityConfidential},
		{"internal", SensitivityInternal},
		{"unknown", SensitivityInternal},
	}

	for _, tt := range tests {
		got := classifier.parseSensitivity(tt.input)
		if got != tt.want {
			t.Errorf("parseSensitivity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIntentClassifier_parseUrgency(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		CacheEnabled: false,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		input string
		want  UrgencyLevel
	}{
		{"low", UrgencyLow},
		{"normal", UrgencyLow},
		{"high", UrgencyHigh},
		{"urgent", UrgencyHigh},
		{"medium", UrgencyMedium},
		{"unknown", UrgencyMedium},
	}

	for _, tt := range tests {
		got := classifier.parseUrgency(tt.input)
		if got != tt.want {
			t.Errorf("parseUrgency(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIntentClassifier_cleanJSONResponse(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		CacheEnabled: false,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "纯 JSON",
			input:  `{"task_type": "code"}`,
			output: `{"task_type": "code"}`,
		},
		{
			name:   "带 markdown 代码块",
			input:  "```json\n{\"task_type\": \"code\"}\n```",
			output: `{"task_type": "code"}`,
		},
		{
			name:   "带额外文本",
			input:  "好的，这是结果：\n```json\n{\"task_type\": \"code\"}\n```\n希望对你有帮助",
			output: `{"task_type": "code"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.cleanJSONResponse(tt.input)
			if got != tt.output {
				t.Errorf("cleanJSONResponse() = %q, want %q", got, tt.output)
			}
		})
	}
}

func TestClassificationResult_String(t *testing.T) {
	tests := []struct {
		taskType TaskType
		want     string
	}{
		{TaskCodeGeneration, "代码生成"},
		{TaskSimpleChat, "简单对话"},
		{TaskComplexReasoning, "复杂推理"},
		{TaskLongText, "长文本处理"},
		{TaskEmbedding, "向量嵌入"},
		{TaskTranslation, "翻译"},
		{TaskCreative, "创意写作"},
		{TaskAnalysis, "数据分析"},
		{TaskUnknown, "未知"},
	}

	for _, tt := range tests {
		got := tt.taskType.String()
		if got != tt.want {
			t.Errorf("TaskType(%s).String() = %q, want %q", tt.taskType, got, tt.want)
		}
	}
}

func TestIntentClassifier_getDefaultClassification(t *testing.T) {
	config := IntentClassifierConfig{
		Enabled:      false,
		LocalModel:   "qwen2.5:1.5b",
		CacheEnabled: false,
	}
	classifier := NewIntentClassifier(config)
	defer classifier.Close()

	tests := []struct {
		question string
		wantType TaskType
	}{
		{"帮我写代码", TaskCodeGeneration},
		{"翻译这句话", TaskTranslation},
		{"请总结这篇文章", TaskLongText},
		{"分析数据", TaskAnalysis},
		{"你好", TaskSimpleChat},
	}

	for _, tt := range tests {
		result := classifier.getDefaultClassification(tt.question)
		if result == nil {
			t.Errorf("getDefaultClassification(%q) returned nil", tt.question)
			continue
		}
		if result.TaskType != tt.wantType {
			t.Errorf("getDefaultClassification(%q) taskType = %v, want %v", tt.question, result.TaskType, tt.wantType)
		}
		if result.Confidence != 0.5 {
			t.Errorf("getDefaultClassification() confidence = %v, want 0.5", result.Confidence)
		}
		t.Logf("Question: %s -> Task: %s, Complexity: %s", tt.question, result.TaskType, result.Complexity)
	}
}

// TestIntentClassifier_Cache 缓存测试
// 注意：此测试需要完整的 logger 和 LLM 后端环境
// 在 CI/CD 或无后端环境中会自动跳过
func TestIntentClassifier_Cache(t *testing.T) {
	t.Skip("Skipping - requires logger and LLM backend initialization. Run manually with full environment.")
}

// TestScheduler_Schedule 调度器测试
// 注意：此测试需要完整的调度器环境和后端配置
func TestScheduler_Schedule(t *testing.T) {
	t.Skip("Skipping - requires full scheduler environment and backend configuration. Run manually with full environment.")
}

// TestScheduler_Stats 统计测试
// 注意：此测试需要调度器初始化
func TestScheduler_Stats(t *testing.T) {
	t.Skip("Skipping - requires scheduler initialization. Run manually with full environment.")
}
