package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestOpenAILLMService_Generate 测试 OpenAI LLM 服务生成文本
// 注意：这个测试需要真实的 API Key，在 CI/CD 中会被跳过
func TestOpenAILLMService_Generate(t *testing.T) {
	// 从环境变量读取 API Key，没有则跳过测试
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
	}

	config := &LLMConfig{
		Provider:    "openai",
		ModelName:   "gpt-4o-mini",
		BaseURL:     "https://api.openai.com/v1",
		APIKey:      apiKey,
		Temperature: 0.7,
		MaxTokens:   100,
		Timeout:     60,
	}

	service, err := NewOpenAILLMService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI LLM service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := "Say hello in one word."
	response, err := service.Generate(ctx, prompt)
	if err != nil {
		t.Fatalf("Failed to generate text: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response")
	}

	t.Logf("Generated response: %s", response)
}

// TestOpenAILLMService_GenerateJSON 测试 OpenAI LLM 服务生成 JSON
func TestOpenAILLMService_GenerateJSON(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
	}

	config := &LLMConfig{
		Provider:    "openai",
		ModelName:   "gpt-4o-mini",
		BaseURL:     "https://api.openai.com/v1",
		APIKey:      apiKey,
		Temperature: 0.3,
		MaxTokens:   100,
		Timeout:     60,
	}

	service, err := NewOpenAILLMService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI LLM service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type TestResult struct {
		Word   string `json:"word"`
		Number int    `json:"number"`
	}

	prompt := `Generate a JSON object with:
- word: any English word
- number: any integer between 1 and 10`

	var result TestResult
	err = service.GenerateJSON(ctx, prompt, &result)
	if err != nil {
		t.Fatalf("Failed to generate JSON: %v", err)
	}

	if result.Word == "" {
		t.Error("Expected non-empty word")
	}
	if result.Number < 1 || result.Number > 10 {
		t.Errorf("Expected number between 1 and 10, got %d", result.Number)
	}

	t.Logf("Generated JSON: %+v", result)
}

// TestOllamaLLMService_Generate 测试 Ollama LLM 服务生成文本
// 注意：这个测试需要 Ollama 服务运行在本地
func TestOllamaLLMService_Generate(t *testing.T) {
	// 检查 Ollama 服务是否可用（简单检查）
	// 实际测试中应该尝试连接 Ollama

	config := &LLMConfig{
		Provider:    "ollama",
		ModelName:   "llama3.2:3b",
		BaseURL:     "http://localhost:21434",
		Temperature: 0.7,
		MaxTokens:   100,
		Timeout:     60,
	}

	service, err := NewOllamaLLMService(config)
	if err != nil {
		t.Fatalf("Failed to create Ollama LLM service: %v", err)
	}

	// 验证配置
	if service.GetProvider() != "ollama" {
		t.Errorf("Expected provider 'ollama', got '%s'", service.GetProvider())
	}

	if service.GetModelName() != "llama3.2:3b" {
		t.Errorf("Expected model 'llama3.2:3b', got '%s'", service.GetModelName())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 尝试调用（可能会失败，这是预期的）
	prompt := "Say hello."
	_, err = service.Generate(ctx, prompt)
	if err != nil {
		t.Logf("Ollama service not available or model not downloaded: %v", err)
		t.Skip("Ollama service not available")
	}

	t.Log("Ollama service test passed")
}

// TestCreateLLMService 测试 LLM 服务工厂函数
func TestCreateLLMService(t *testing.T) {
	tests := []struct {
		name      string
		config    *LLMConfig
		expectErr bool
	}{
		{
			name: "OpenAI provider",
			config: &LLMConfig{
				Provider:  "openai",
				ModelName: "gpt-4o-mini",
			},
			expectErr: false,
		},
		{
			name: "Ollama provider",
			config: &LLMConfig{
				Provider:  "ollama",
				ModelName: "llama3.2:3b",
			},
			expectErr: false,
		},
		{
			name: "Invalid provider",
			config: &LLMConfig{
				Provider:  "invalid",
				ModelName: "test",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := CreateLLMService(tt.config)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to create LLM service: %v", err)
			}

			if service.GetProvider() != tt.config.Provider {
				t.Errorf("Expected provider '%s', got '%s'",
					tt.config.Provider, service.GetProvider())
			}

			if service.GetModelName() != tt.config.ModelName {
				t.Errorf("Expected model '%s', got '%s'",
					tt.config.ModelName, service.GetModelName())
			}
		})
	}
}

// TestDefaultLLMConfig 测试默认配置
func TestDefaultLLMConfig(t *testing.T) {
	config := DefaultLLMConfig()

	if config.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", config.Provider)
	}

	if config.ModelName != "gpt-4o-mini" {
		t.Errorf("Expected model 'gpt-4o-mini', got '%s'", config.ModelName)
	}

	if config.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %.2f", config.Temperature)
	}

	if config.MaxTokens != 2000 {
		t.Errorf("Expected max_tokens 2000, got %d", config.MaxTokens)
	}

	if config.Timeout != 60 {
		t.Errorf("Expected timeout 60, got %d", config.Timeout)
	}
}

// BenchmarkOpenAILLMService_Generate 基准测试 OpenAI 生成性能
func BenchmarkOpenAILLMService_Generate(b *testing.B) {
	apiKey := "test-api-key"
	if apiKey == "" {
		b.Skip("Skipping benchmark: OPENAI_API_KEY not set")
	}

	config := &LLMConfig{
		Provider:    "openai",
		ModelName:   "gpt-4o-mini",
		BaseURL:     "https://api.openai.com/v1",
		APIKey:      apiKey,
		Temperature: 0.7,
		MaxTokens:   100,
		Timeout:     60,
	}

	service, err := NewOpenAILLMService(config)
	if err != nil {
		b.Fatalf("Failed to create OpenAI LLM service: %v", err)
	}

	ctx := context.Background()
	prompt := "Say hello."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Generate(ctx, prompt)
		if err != nil {
			b.Fatalf("Failed to generate: %v", err)
		}
	}
}
