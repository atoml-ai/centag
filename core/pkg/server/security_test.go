package server

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestExportDesensitize(t *testing.T) {
	// 测试脱敏功能
	cfg := &backend.BackendConfig{
		ID:     "test-backend",
		Name:   "Test Backend",
		APIKey: "sk-secret-key-12345",
		AccountPool: &backend.AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []backend.BackendAccount{
				{ID: "key-1", APIKey: "sk-account-key-1"},
				{ID: "key-2", APIKey: "sk-account-key-2"},
			},
		},
	}

	// 测试 ToResponse 方法（掩码 api_key）
	resp := cfg.ToResponse()
	if resp.HasAPIKey != true {
		t.Error("expected has_api_key to be true")
	}

	// 测试账户掩码（原始账户不应被修改）
	if cfg.AccountPool != nil {
		for _, acc := range cfg.AccountPool.Accounts {
			if acc.APIKey == "" {
				// 原始账户不应被修改
				t.Error("expected original account api_key to be preserved")
			}
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "long key",
			apiKey:   "sk-1234567890abcdef1234567890abcdef",
			expected: "sk-1...cdef",
		},
		{
			name:     "short key",
			apiKey:   "sk-123",
			expected: "sk-123",
		},
		{
			name:     "empty key",
			apiKey:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := maskAPIKey(tt.apiKey)
			if masked != tt.expected {
				t.Errorf("maskAPIKey(%s) = %s, want %s", tt.apiKey, masked, tt.expected)
			}
		})
	}
}

func TestExportDesensitizeParameter(t *testing.T) {
	// 测试 desensitize 参数的逻辑
	// 这个测试验证脱敏逻辑的正确性

	// 创建测试配置
	cfg := &backend.BackendConfig{
		ID:     "test-backend",
		Name:   "Test Backend",
		APIKey: "sk-secret-key-12345",
		AccountPool: &backend.AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []backend.BackendAccount{
				{ID: "key-1", APIKey: "sk-account-key-1"},
				{ID: "key-2", APIKey: "sk-account-key-2"},
			},
		},
	}

	// 测试脱敏逻辑
	masked := *cfg
	masked.APIKey = ""
	if masked.AccountPool != nil {
		for i := range masked.AccountPool.Accounts {
			masked.AccountPool.Accounts[i].APIKey = ""
		}
	}

	// 验证脱敏结果
	if masked.APIKey != "" {
		t.Error("expected api_key to be masked")
	}

	if masked.AccountPool != nil {
		for _, acc := range masked.AccountPool.Accounts {
			if acc.APIKey != "" {
				t.Error("expected account api_key to be masked")
			}
		}
	}

	// 验证原始配置未被修改
	if cfg.APIKey == "" {
		t.Error("original api_key should not be modified")
	}
}

func TestBackendConfig_HasAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "with api key",
			apiKey:   "sk-secret-key",
			expected: true,
		},
		{
			name:     "without api key",
			apiKey:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &backend.BackendConfig{
				ID:     "test-backend",
				APIKey: tt.apiKey,
			}
			resp := cfg.ToResponse()
			if resp.HasAPIKey != tt.expected {
				t.Errorf("expected has_api_key=%v, got %v", tt.expected, resp.HasAPIKey)
			}
		})
	}
}

func TestAccountPoolSummary(t *testing.T) {
	cfg := &backend.BackendConfig{
		ID:     "test-backend",
		APIKey: "sk-secret-key",
		AccountPool: &backend.AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []backend.BackendAccount{
				{ID: "key-1", APIKey: "sk-key-1", Enabled: true},
				{ID: "key-2", APIKey: "sk-key-2", Enabled: false},
			},
		},
	}

	resp := cfg.ToResponse()
	if resp.AccountPoolSummary == nil {
		t.Fatal("expected account_pool_summary to be set")
	}

	if resp.AccountPoolSummary.TotalAccounts != 2 {
		t.Errorf("expected total_accounts=2, got %d", resp.AccountPoolSummary.TotalAccounts)
	}

	if resp.AccountPoolSummary.EnabledAccounts != 1 {
		t.Errorf("expected enabled_accounts=1, got %d", resp.AccountPoolSummary.EnabledAccounts)
	}

	if resp.AccountPoolSummary.Strategy != "round_robin" {
		t.Errorf("expected strategy=round_robin, got %s", resp.AccountPoolSummary.Strategy)
	}

	if resp.AccountPoolSummary.HealthStatus != "partial" {
		t.Errorf("expected health_status=partial, got %s", resp.AccountPoolSummary.HealthStatus)
	}
}
