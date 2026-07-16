package scheduler

import (
	"encoding/json"
	"strings"
	"testing"

	"centag/core/pkg/backend"
)

// TestAutoAssignAnalyzer 测试自动分配分析器
func TestAutoAssignAnalyzer(t *testing.T) {
	// 准备测试后端配置
	testBackends := []*backend.BackendConfig{
		{
			ID:       "ollama-local",
			Name:     "本地 Ollama",
			Type:     "ollama",
			Enabled:  true,
			Priority: 5,
			Weight:   10,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "qwen2.5:1.5b", ActualModel: "qwen2.5:1.5b"},
				{RequestedModel: "llama3.2:1b", ActualModel: "llama3.2:1b"},
			},
		},
		{
			ID:       "bigmodel",
			Name:     "智谱 AI",
			Type:     "openai",
			Enabled:  true,
			Priority: 9,
			Weight:   90,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "glm-4-flash", ActualModel: "glm-4-flash"},
				{RequestedModel: "glm-5", ActualModel: "glm-5"},
			},
		},
		{
			ID:       "ppinfra",
			Name:     "PPIO",
			Type:     "openai",
			Enabled:  true,
			Priority: 7,
			Weight:   70,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-v3", ActualModel: "deepseek-v3"},
				{RequestedModel: "kimi", ActualModel: "kimi"},
			},
		},
	}

	t.Run("初始化分析器", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)
		if analyzer == nil {
			t.Fatal("分析器初始化失败")
		}
	})

	t.Run("构建后端信息", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)
		info := analyzer.buildBackendsInfo(testBackends)

		// 验证输出包含所有启用的后端 ID
		expectedIDs := []string{"ollama-local", "bigmodel", "ppinfra"}
		for _, id := range expectedIDs {
			if !strings.Contains(info, id) {
				t.Errorf("后端信息中缺少 %s", id)
			}
		}

		// 验证包含模型信息
		expectedModels := []string{"qwen2.5:1.5b", "glm-4-flash", "deepseek-v3"}
		for _, model := range expectedModels {
			if !strings.Contains(info, model) {
				t.Errorf("后端信息中缺少模型 %s", model)
			}
		}

		// 验证禁用的后端不会被包含
		disabledBackends := []*backend.BackendConfig{
			{ID: "disabled", Name: "禁用后端", Enabled: false},
		}
		infoDisabled := analyzer.buildBackendsInfo(disabledBackends)
		if strings.Contains(infoDisabled, "disabled") {
			t.Error("禁用的后端不应该被包含在信息中")
		}
	})

	t.Run("提示词模板替换", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 验证提示词包含必要元素
		prompt := config.Prompt
		requiredElements := []string{
			"任务类型",
			"code_generation",
			"simple_chat",
			"后端配置",
			"{{.backends_info}}",
			"JSON 格式",
			"task_assignments",
		}

		for _, elem := range requiredElements {
			if !strings.Contains(prompt, elem) {
				t.Errorf("提示词中缺少必要元素：%s", elem)
			}
		}

		// 验证替换后不包含占位符
		backendsInfo := analyzer.buildBackendsInfo(testBackends)
		replacedPrompt := strings.ReplaceAll(prompt, "{{.backends_info}}", backendsInfo)
		if strings.Contains(replacedPrompt, "{{.backends_info}}") {
			t.Error("提示词替换后仍包含占位符")
		}
	})

	t.Run("解析分析结果 - 正常情况", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 模拟 AI 返回的 JSON 响应
		mockResponse := `{
			"task_assignments": [
				{
					"task_type": "code_generation",
					"recommended_backend_id": "bigmodel",
					"recommended_model": "glm-4-flash",
					"reason": "代码生成任务需要强大的代码能力",
					"confidence": 0.95
				},
				{
					"task_type": "simple_chat",
					"recommended_backend_id": "ollama-local",
					"recommended_model": "qwen2.5:1.5b",
					"reason": "简单对话使用本地模型节省成本",
					"confidence": 0.90
				},
				{
					"task_type": "complex_reasoning",
					"recommended_backend_id": "bigmodel",
					"recommended_model": "glm-5",
					"reason": "复杂推理需要高质量模型",
					"confidence": 0.92
				}
			]
		}`

		result := analyzer.parseAnalysisResult(mockResponse, testBackends)

		if len(result.TaskAssignments) != 3 {
			t.Errorf("期望 3 个分配结果，实际 %d 个", len(result.TaskAssignments))
		}

		// 验证第一个分配结果
		first := result.TaskAssignments[0]
		if first.TaskType != TaskCodeGeneration {
			t.Errorf("期望任务类型为 code_generation，实际 %s", first.TaskType)
		}
		if first.RecommendedBackendID != "bigmodel" {
			t.Errorf("期望后端为 bigmodel，实际 %s", first.RecommendedBackendID)
		}
		if first.Confidence != 0.95 {
			t.Errorf("期望置信度为 0.95，实际 %f", first.Confidence)
		}
	})

	t.Run("解析分析结果 - 带 Markdown 标记", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 模拟带 Markdown 代码块标记的响应
		mockResponse := `好的，我来分析一下：

` + "```json" + `
{
  "task_assignments": [
    {
      "task_type": "translation",
      "recommended_backend_id": "ppinfra",
      "recommended_model": "deepseek-v3",
      "reason": "翻译任务性价比高",
      "confidence": 0.88
    }
  ]
}
` + "```" + `

希望这个分析对你有帮助！`

		result := analyzer.parseAnalysisResult(mockResponse, testBackends)

		if len(result.TaskAssignments) != 1 {
			t.Errorf("期望 1 个分配结果，实际 %d 个", len(result.TaskAssignments))
		}

		assign := result.TaskAssignments[0]
		if assign.TaskType != TaskTranslation {
			t.Errorf("期望任务类型为 translation，实际 %s", assign.TaskType)
		}
		if assign.RecommendedBackendID != "ppinfra" {
			t.Errorf("期望后端为 ppinfra，实际 %s", assign.RecommendedBackendID)
		}
	})

	t.Run("解析分析结果 - 无效 JSON", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		invalidResponse := `这不是有效的 JSON 格式`
		result := analyzer.parseAnalysisResult(invalidResponse, testBackends)

		// 应该返回错误信息
		if result.Error == "" {
			t.Error("期望返回错误信息，但为空")
		}
	})

	t.Run("解析分析结果 - 空响应", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		result := analyzer.parseAnalysisResult("", testBackends)

		// 应该返回错误信息
		if result.Error == "" {
			t.Error("期望返回错误信息，但为空")
		}
	})

	t.Run("验证任务类型完整性", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 模拟完整的 8 种任务类型响应
		mockResponse := `{
			"task_assignments": [
				{"task_type": "code_generation", "recommended_backend_id": "bigmodel", "recommended_model": "glm-4-flash", "reason": "代码", "confidence": 0.9},
				{"task_type": "simple_chat", "recommended_backend_id": "ollama-local", "recommended_model": "qwen2.5:1.5b", "reason": "聊天", "confidence": 0.9},
				{"task_type": "complex_reasoning", "recommended_backend_id": "bigmodel", "recommended_model": "glm-5", "reason": "推理", "confidence": 0.9},
				{"task_type": "long_text", "recommended_backend_id": "ppinfra", "recommended_model": "kimi", "reason": "长文本", "confidence": 0.9},
				{"task_type": "embedding", "recommended_backend_id": "ollama-local", "recommended_model": "bge-m3", "reason": "向量", "confidence": 0.9},
				{"task_type": "translation", "recommended_backend_id": "ppinfra", "recommended_model": "deepseek-v3", "reason": "翻译", "confidence": 0.9},
				{"task_type": "creative", "recommended_backend_id": "bigmodel", "recommended_model": "glm-4-flash", "reason": "创意", "confidence": 0.9},
				{"task_type": "analysis", "recommended_backend_id": "bigmodel", "recommended_model": "glm-5", "reason": "分析", "confidence": 0.9}
			]
		}`

		result := analyzer.parseAnalysisResult(mockResponse, testBackends)

		if len(result.TaskAssignments) != 8 {
			t.Errorf("期望 8 个任务类型，实际 %d 个", len(result.TaskAssignments))
		}

		// 验证所有任务类型都存在
		taskTypes := make(map[TaskType]bool)
		for _, assign := range result.TaskAssignments {
			taskTypes[assign.TaskType] = true
		}

		expectedTypes := []TaskType{
			TaskCodeGeneration, TaskSimpleChat, TaskComplexReasoning, TaskLongText,
			TaskEmbedding, TaskTranslation, TaskCreative, TaskAnalysis,
		}

		for _, expected := range expectedTypes {
			if !taskTypes[expected] {
				t.Errorf("缺少任务类型：%s", expected)
			}
		}
	})

	t.Run("置信度规范化", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 测试置信度超出范围的情况
		mockResponse := `{
			"task_assignments": [
				{"task_type": "code_generation", "recommended_backend_id": "bigmodel", "recommended_model": "glm-4-flash", "reason": "代码", "confidence": 1.5},
				{"task_type": "simple_chat", "recommended_backend_id": "ollama-local", "recommended_model": "qwen2.5:1.5b", "reason": "聊天", "confidence": -0.2},
				{"task_type": "complex_reasoning", "recommended_backend_id": "bigmodel", "recommended_model": "glm-5", "reason": "推理", "confidence": 0.0}
			]
		}`

		result := analyzer.parseAnalysisResult(mockResponse, testBackends)

		// 验证置信度被规范化到 [0, 1] 范围
		if result.TaskAssignments[0].Confidence != 1.0 {
			t.Errorf("期望置信度规范化为 1.0，实际 %f", result.TaskAssignments[0].Confidence)
		}
		if result.TaskAssignments[1].Confidence != 0.5 {
			t.Errorf("期望置信度规范化为 0.5（负数处理），实际 %f", result.TaskAssignments[1].Confidence)
		}
		if result.TaskAssignments[2].Confidence != 0.5 {
			t.Errorf("期望置信度规范化为 0.5（零值处理），实际 %f", result.TaskAssignments[2].Confidence)
		}
	})

	t.Run("后端 ID 验证", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		// 测试推荐的后端 ID 不存在的情况
		mockResponse := `{
			"task_assignments": [
				{"task_type": "code_generation", "recommended_backend_id": "non-existent", "recommended_model": "glm-4-flash", "reason": "代码", "confidence": 0.9}
			]
		}`

		result := analyzer.parseAnalysisResult(mockResponse, testBackends)

		// 应该降级到第一个可用后端
		if result.TaskAssignments[0].RecommendedBackendID == "non-existent" {
			t.Error("期望无效后端 ID 被处理，但保留了原值")
		}
	})
}

// TestAutoAssignPrompt 测试提示词模板
func TestAutoAssignPrompt(t *testing.T) {
	t.Run("默认提示词包含必要元素", func(t *testing.T) {
		requiredElements := []string{
			"任务类型",
			"code_generation",
			"simple_chat",
			"complex_reasoning",
			"long_text",
			"embedding",
			"translation",
			"creative",
			"analysis",
			"后端配置",
			"{{.backends_info}}",
			"JSON 格式",
			"task_assignments",
			"recommended_backend_id",
			"recommended_model",
			"reason",
			"confidence",
		}

		for _, elem := range requiredElements {
			if !strings.Contains(DefaultAnalysisPrompt, elem) {
				t.Errorf("默认提示词中缺少必要元素：%s", elem)
			}
		}
	})

	t.Run("提示词长度合理", func(t *testing.T) {
		// 提示词应该在 500-5000 字符之间
		length := len(DefaultAnalysisPrompt)
		if length < 500 {
			t.Errorf("提示词过短：%d 字符，可能缺少必要说明", length)
		}
		if length > 5000 {
			t.Errorf("提示词过长：%d 字符，可能超出模型上下文限制", length)
		}
	})
}

// TestAutoAssignResult_JSON 测试结果序列化
func TestAutoAssignResult_JSON(t *testing.T) {
	result := &AutoAssignResult{
		Success: true,
		TaskAssignments: []*TaskTypeAssignment{
			{
				TaskType:             TaskCodeGeneration,
				RecommendedBackendID: "bigmodel",
				RecommendedModel:     "glm-4-flash",
				Reason:               "代码生成任务需要强大的代码能力",
				Confidence:           0.95,
			},
			{
				TaskType:             TaskSimpleChat,
				RecommendedBackendID: "ollama-local",
				RecommendedModel:     "qwen2.5:1.5b",
				Reason:               "简单对话使用本地模型节省成本",
				Confidence:           0.90,
			},
		},
	}

	// 测试序列化
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}

	// 验证 JSON 格式
	var decoded AutoAssignResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}

	if len(decoded.TaskAssignments) != 2 {
		t.Errorf("期望 2 个分配结果，实际 %d 个", len(decoded.TaskAssignments))
	}
}

// TestAutoAssignAnalyzer_EdgeCases 边界情况测试
func TestAutoAssignAnalyzer_EdgeCases(t *testing.T) {
	t.Run("空后端列表", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)
		info := analyzer.buildBackendsInfo([]*backend.BackendConfig{})
		if info == "" {
			t.Error("期望返回空字符串，但为 nil")
		}
	})

	t.Run("单个后端", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		backends := []*backend.BackendConfig{
			{ID: "single", Name: "单个后端", Type: "ollama", Enabled: true},
		}
		info := analyzer.buildBackendsInfo(backends)
		if !strings.Contains(info, "single") {
			t.Error("单个后端信息生成失败")
		}
	})

	t.Run("所有后端都禁用", func(t *testing.T) {
		config := DefaultAutoAssignConfig()
		analyzer := NewAutoAssignAnalyzer(config)

		backends := []*backend.BackendConfig{
			{ID: "disabled1", Name: "禁用后端 1", Enabled: false},
			{ID: "disabled2", Name: "禁用后端 2", Enabled: false},
		}
		info := analyzer.buildBackendsInfo(backends)
		// 禁用的后端不应该被包含
		if strings.Contains(info, "disabled1") || strings.Contains(info, "disabled2") {
			t.Error("禁用的后端不应该被包含在信息中")
		}
	})
}

// BenchmarkAutoAssignAnalyzer 性能测试
func BenchmarkAutoAssignAnalyzer_BuildBackendsInfo(b *testing.B) {
	config := DefaultAutoAssignConfig()
	analyzer := NewAutoAssignAnalyzer(config)

	backends := []*backend.BackendConfig{
		{ID: "ollama-local", Name: "本地 Ollama", Type: "ollama", Enabled: true},
		{ID: "bigmodel", Name: "智谱 AI", Type: "openai", Enabled: true},
		{ID: "ppinfra", Name: "PPIO", Type: "openai", Enabled: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.buildBackendsInfo(backends)
	}
}

func BenchmarkAutoAssignAnalyzer_ParseAnalysisResult(b *testing.B) {
	config := DefaultAutoAssignConfig()
	analyzer := NewAutoAssignAnalyzer(config)

	mockResponse := `{
		"task_assignments": [
			{"task_type": "code_generation", "recommended_backend_id": "bigmodel", "recommended_model": "glm-4-flash", "reason": "代码", "confidence": 0.9},
			{"task_type": "simple_chat", "recommended_backend_id": "ollama-local", "recommended_model": "qwen2.5:1.5b", "reason": "聊天", "confidence": 0.9},
			{"task_type": "complex_reasoning", "recommended_backend_id": "bigmodel", "recommended_model": "glm-5", "reason": "推理", "confidence": 0.9},
			{"task_type": "long_text", "recommended_backend_id": "ppinfra", "recommended_model": "kimi", "reason": "长文本", "confidence": 0.9},
			{"task_type": "embedding", "recommended_backend_id": "ollama-local", "recommended_model": "bge-m3", "reason": "向量", "confidence": 0.9},
			{"task_type": "translation", "recommended_backend_id": "ppinfra", "recommended_model": "deepseek-v3", "reason": "翻译", "confidence": 0.9},
			{"task_type": "creative", "recommended_backend_id": "bigmodel", "recommended_model": "glm-4-flash", "reason": "创意", "confidence": 0.9},
			{"task_type": "analysis", "recommended_backend_id": "bigmodel", "recommended_model": "glm-5", "reason": "分析", "confidence": 0.9}
		]
	}`

	backends := []*backend.BackendConfig{
		{ID: "ollama-local", Name: "本地 Ollama", Type: "ollama", Enabled: true},
		{ID: "bigmodel", Name: "智谱 AI", Type: "openai", Enabled: true},
		{ID: "ppinfra", Name: "PPIO", Type: "openai", Enabled: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.parseAnalysisResult(mockResponse, backends)
	}
}
