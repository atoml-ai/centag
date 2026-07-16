package scheduler

import (
	"testing"
	"time"

	"centag/core/pkg/backend"
)

func TestFastMatcher_MatchCategory(t *testing.T) {
	m := NewFastMatcher()

	tests := []struct {
		name     string
		category string
		expected TaskType
	}{
		{
			name:     "code keyword",
			category: "code",
			expected: TaskCodeGeneration,
		},
		{
			name:     "python keyword",
			category: "python",
			expected: TaskCodeGeneration,
		},
		{
			name:     "java keyword",
			category: "java",
			expected: TaskCodeGeneration,
		},
		{
			name:     "translate keyword",
			category: "translate",
			expected: TaskTranslation,
		},
		{
			name:     "summary keyword",
			category: "summary",
			expected: TaskLongText,
		},
		{
			name:     "chat keyword",
			category: "chat",
			expected: TaskSimpleChat,
		},
		{
			name:     "unknown keyword defaults to chat",
			category: "unknown",
			expected: TaskSimpleChat,
		},
		{
			name:     "chinese code keyword",
			category: "代码",
			expected: TaskCodeGeneration,
		},
		{
			name:     "chinese translate keyword",
			category: "翻译",
			expected: TaskTranslation,
		},
		{
			name:     "chinese summary keyword",
			category: "总结",
			expected: TaskLongText,
		},
		{
			name:     "partial match code",
			category: "my-code-project",
			expected: TaskCodeGeneration,
		},
		{
			name:     "case insensitive",
			category: "CODE",
			expected: TaskCodeGeneration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.MatchCategory(tt.category)
			if result != tt.expected {
				t.Errorf("MatchCategory(%q) = %v, want %v", tt.category, result, tt.expected)
			}
		})
	}
}

func TestFastMatcher_RecommendBackend(t *testing.T) {
	m := NewFastMatcher()

	// 设置后端缓存
	backends := []*backend.BackendConfig{
		{
			ID:      "ollama-local",
			Name:    "Ollama Local",
			Type:    "ollama",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "llama3", ActualModel: "llama3"},
				{RequestedModel: "codellama", ActualModel: "codellama"},
			},
		},
		{
			ID:      "bigmodel",
			Name:    "智谱 AI",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-chat", ActualModel: "deepseek-chat"},
				{RequestedModel: "deepseek-coder", ActualModel: "deepseek-coder"},
			},
		},
	}
	m.UpdateBackendCache(backends)

	tests := []struct {
		name            string
		taskType        TaskType
		strategy        string
		expectedBackend string
		expectedModel   string
	}{
		{
			name:            "code generation recommends bigmodel first",
			taskType:        TaskCodeGeneration,
			strategy:        "fast",
			expectedBackend: "bigmodel",
			expectedModel:   "GLM-4-flash",
		},
		{
			name:            "simple chat recommends ollama first",
			taskType:        TaskSimpleChat,
			strategy:        "fast",
			expectedBackend: "ollama-local",
			expectedModel:   "llama3",
		},
		{
			name:            "cost strategy prefers ollama",
			taskType:        TaskCodeGeneration,
			strategy:        "cost",
			expectedBackend: "ollama-local",
			expectedModel:   "codellama",
		},
		{
			name:            "quality strategy prefers bigmodel",
			taskType:        TaskSimpleChat,
			strategy:        "quality",
			expectedBackend: "bigmodel",
			expectedModel:   "GLM-4-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := m.RecommendBackend(tt.taskType, tt.strategy)
			if decision.RecommendedBackendID != tt.expectedBackend {
				t.Errorf("RecommendBackend() backend = %v, want %v", decision.RecommendedBackendID, tt.expectedBackend)
			}
			if decision.RecommendedModel != tt.expectedModel {
				t.Errorf("RecommendBackend() model = %v, want %v", decision.RecommendedModel, tt.expectedModel)
			}
		})
	}
}

func TestFastMatcher_RecommendBackend_NoAvailableBackend(t *testing.T) {
	m := NewFastMatcher()

	// 不设置后端缓存
	decision := m.RecommendBackend(TaskCodeGeneration, "fast")
	if decision.Reason != "无可用后端" {
		t.Errorf("RecommendBackend() reason = %v, want '无可用后端'", decision.Reason)
	}
}

func TestFastMatcher_RecommendBackend_DisabledBackend(t *testing.T) {
	m := NewFastMatcher()

	// 设置后端缓存（全部禁用）
	backends := []*backend.BackendConfig{
		{
			ID:      "ollama-local",
			Name:    "Ollama Local",
			Type:    "ollama",
			Enabled: false,
		},
	}
	m.UpdateBackendCache(backends)

	decision := m.RecommendBackend(TaskCodeGeneration, "fast")
	if decision.Reason != "无可用后端" {
		t.Errorf("RecommendBackend() reason = %v, want '无可用后端'", decision.Reason)
	}
}

func TestFastMatcher_RecommendBackend_BalancePrefersLowerLatencyHealthy(t *testing.T) {
	m := NewFastMatcher()

	backends := []*backend.BackendConfig{
		{
			ID:      "bigmodel",
			Name:    "BigModel",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 1100,
			},
		},
		{
			ID:      "ppio",
			Name:    "PPIO",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek/deepseek-v4-flash", ActualModel: "deepseek/deepseek-v4-flash"},
				{RequestedModel: "qwen/qwen3.5-plus", ActualModel: "qwen/qwen3.5-plus"},
				{RequestedModel: "moonshotai/kimi-k2.5", ActualModel: "moonshotai/kimi-k2.5"},
				{RequestedModel: "zai-org/glm-5", ActualModel: "zai-org/glm-5"},
				{RequestedModel: "minimax/minimax-m2.7-highspeed", ActualModel: "minimax/minimax-m2.7-highspeed"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 4000,
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-chat", ActualModel: "deepseek-chat"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 1500,
			},
		},
	}
	m.UpdateBackendCache(backends)

	decision := m.RecommendBackend(TaskSimpleChat, "balance")
	if decision.RecommendedBackendID != "bigmodel" {
		t.Fatalf("balance should prefer lower latency healthy backend, got %s", decision.RecommendedBackendID)
	}
}

func TestFastMatcher_RecommendBackend_AvailabilityFirstSkipsUnhealthy(t *testing.T) {
	m := NewFastMatcher()
	backends := []*backend.BackendConfig{
		{
			ID:      "bigmodel",
			Name:    "BigModel",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "unhealthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				LastError:    "HTTP 403 forbidden",
				ResponseTime: 100,
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-chat", ActualModel: "deepseek-chat"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 900,
			},
		},
	}
	m.UpdateBackendCache(backends)

	decision := m.RecommendBackend(TaskSimpleChat, "fast")
	if decision.RecommendedBackendID != "deepseek" {
		t.Fatalf("availability-first should skip unhealthy backend, got %s", decision.RecommendedBackendID)
	}
}

func TestFastMatcher_RecommendBackend_AvailabilityFirstSkipsBalanceError(t *testing.T) {
	m := NewFastMatcher()
	backends := []*backend.BackendConfig{
		{
			ID:      "bigmodel",
			Name:    "BigModel",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				LastError:    "API error (status 403): {\"reason\":\"NOT_ENOUGH_BALANCE\"}",
				ResponseTime: 100,
			},
		},
		{
			ID:      "deepseek",
			Name:    "DeepSeek",
			Type:    "openai",
			Enabled: true,
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "deepseek-chat", ActualModel: "deepseek-chat"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 900,
			},
		},
	}
	m.UpdateBackendCache(backends)

	decision := m.RecommendBackend(TaskSimpleChat, "balance")
	if decision.RecommendedBackendID != "deepseek" {
		t.Fatalf("availability-first should skip balance-insufficient backend, got %s", decision.RecommendedBackendID)
	}
}

func TestFastMatcher_RecommendBackend_UsesProbeModelAsStableChoice(t *testing.T) {
	m := NewFastMatcher()

	backends := []*backend.BackendConfig{
		{
			ID:         "bigmodel",
			Name:       "BigModel",
			Type:       "openai",
			Enabled:    true,
			ProbeModel: "GLM-4-flash",
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "GLM-4-32K", ActualModel: "GLM-4-32K"},
				{RequestedModel: "GLM-4-flash", ActualModel: "GLM-4-flash"},
			},
			HealthStatus: &backend.BackendHealthStatus{
				Status:       "healthy",
				LastCheckAt:  time.Now().Format(time.RFC3339),
				ResponseTime: 500,
			},
		},
	}
	m.UpdateBackendCache(backends)

	decision := m.RecommendBackend(TaskLongText, "balance")
	if decision.RecommendedBackendID != "bigmodel" {
		t.Fatalf("expected backend bigmodel, got %s", decision.RecommendedBackendID)
	}
	if decision.RecommendedModel != "GLM-4-flash" {
		t.Fatalf("expected probe model GLM-4-flash, got %s", decision.RecommendedModel)
	}
}
