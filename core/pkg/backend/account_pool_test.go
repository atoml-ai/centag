package backend

import (
	"context"
	"testing"
)

func TestAccountPoolSelector_RoundRobin(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
			{ID: "key-3", APIKey: "sk-3", Enabled: true},
		},
	}

	// 多次选择应该轮询
	selected := make(map[string]int)
	for i := 0; i < 6; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[result.Account.ID]++
	}

	// 每个账户应该被选择 2 次
	for _, count := range selected {
		if count != 2 {
			t.Errorf("expected each account to be selected 2 times, got %d", count)
		}
	}
}

func TestAccountPoolSelector_LeastUsage(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "least_usage",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	// 第一次选择
	result1, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 第二次应该选择另一个（请求次数相同）
	result2, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.Account.ID == result2.Account.ID {
		t.Errorf("expected different accounts, got same: %s", result1.Account.ID)
	}
}

func TestAccountPoolSelector_StickySession(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "sticky_session",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	sessionKey := "user-123"

	// 第一次选择
	result1, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 第二次应该选择同一个（会话亲和）
	result2, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.Account.ID != result2.Account.ID {
		t.Errorf("expected same account for sticky session, got %s and %s", result1.Account.ID, result2.Account.ID)
	}
}

func TestAccountPoolSelector_FilterDisabled(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: false},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	// 应该只选择启用的账户
	result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Account.ID != "key-2" {
		t.Errorf("expected key-2, got %s", result.Account.ID)
	}
}

func TestAccountPoolSelector_AllDisabled(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: false},
			{ID: "key-2", APIKey: "sk-2", Enabled: false},
		},
	}

	_, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err == nil {
		t.Error("expected error when all accounts are disabled")
	}
}

func TestAccountPoolSelector_EmptyPool(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{},
	}

	_, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err == nil {
		t.Error("expected error for empty pool")
	}
}

func TestValidateAccountPool(t *testing.T) {
	tests := []struct {
		name    string
		pool    *AccountPoolConfig
		wantErr bool
	}{
		{
			name:    "nil pool",
			pool:    nil,
			wantErr: false,
		},
		{
			name: "valid pool",
			pool: &AccountPoolConfig{
				Strategy: "round_robin",
				Accounts: []BackendAccount{
					{ID: "key-1", APIKey: "sk-1"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid strategy",
			pool: &AccountPoolConfig{
				Strategy: "invalid",
				Accounts: []BackendAccount{
					{ID: "key-1", APIKey: "sk-1"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty accounts",
			pool: &AccountPoolConfig{
				Strategy: "round_robin",
				Accounts: []BackendAccount{},
			},
			wantErr: true,
		},
		{
			name: "missing account id",
			pool: &AccountPoolConfig{
				Strategy: "round_robin",
				Accounts: []BackendAccount{
					{APIKey: "sk-1"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate account id",
			pool: &AccountPoolConfig{
				Strategy: "round_robin",
				Accounts: []BackendAccount{
					{ID: "key-1", APIKey: "sk-1"},
					{ID: "key-1", APIKey: "sk-2"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing api key",
			pool: &AccountPoolConfig{
				Strategy: "round_robin",
				Accounts: []BackendAccount{
					{ID: "key-1"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccountPool(tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccountPool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddAccount(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{},
	}

	acc := BackendAccount{
		ID:     "key-1",
		APIKey: "sk-1",
	}

	if err := AddAccount(pool, acc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pool.Accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(pool.Accounts))
	}

	// 重复添加应该失败
	if err := AddAccount(pool, acc); err == nil {
		t.Error("expected error for duplicate account id")
	}
}

func TestUpdateAccount(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Label: "old"},
		},
	}

	acc := BackendAccount{
		ID:     "key-1",
		APIKey: "sk-1",
		Label:  "new",
	}

	if err := UpdateAccount(pool, acc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pool.Accounts[0].Label != "new" {
		t.Errorf("expected label 'new', got '%s'", pool.Accounts[0].Label)
	}
}

func TestRemoveAccount(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1"},
			{ID: "key-2", APIKey: "sk-2"},
		},
	}

	if err := RemoveAccount(pool, "key-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pool.Accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(pool.Accounts))
	}

	if pool.Accounts[0].ID != "key-2" {
		t.Errorf("expected remaining account to be key-2, got %s", pool.Accounts[0].ID)
	}
}

func TestExtractSessionKey(t *testing.T) {
	tests := []struct {
		name            string
		body            []byte
		headerSessionID string
		expected        string
	}{
		{
			name:            "header session id",
			headerSessionID: "session-123",
			expected:        "session-123",
		},
		{
			name:     "openai user",
			body:     []byte(`{"user": "user-456"}`),
			expected: "openai:user-456",
		},
		{
			name:     "anthropic user_id",
			body:     []byte(`{"metadata": {"user_id": "user-789"}}`),
			expected: "anthropic:user-789",
		},
		{
			name:     "no key",
			body:     []byte(`{}`),
			expected: "",
		},
		{
			name:     "invalid json",
			body:     []byte(`invalid`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSessionKey(context.Background(), tt.body, tt.headerSessionID)
			if result != tt.expected {
				t.Errorf("ExtractSessionKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasAccountPool(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *BackendConfig
		expected bool
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: false,
		},
		{
			name:     "no account pool",
			cfg:      &BackendConfig{},
			expected: false,
		},
		{
			name: "empty accounts",
			cfg: &BackendConfig{
				AccountPool: &AccountPoolConfig{
					Accounts: []BackendAccount{},
				},
			},
			expected: false,
		},
		{
			name: "has accounts",
			cfg: &BackendConfig{
				AccountPool: &AccountPoolConfig{
					Accounts: []BackendAccount{
						{ID: "key-1", APIKey: "sk-1"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAccountPool(tt.cfg)
			if result != tt.expected {
				t.Errorf("HasAccountPool() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetEffectiveAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *BackendConfig
		expected string
	}{
		{
			name: "no account pool",
			cfg: &BackendConfig{
				APIKey: "sk-single",
			},
			expected: "sk-single",
		},
		{
			name: "with account pool",
			cfg: &BackendConfig{
				APIKey: "sk-single",
				AccountPool: &AccountPoolConfig{
					Accounts: []BackendAccount{
						{ID: "key-1", APIKey: "sk-pool-1", Enabled: true},
						{ID: "key-2", APIKey: "sk-pool-2", Enabled: true},
					},
				},
			},
			expected: "sk-pool-1",
		},
		{
			name: "all disabled in pool",
			cfg: &BackendConfig{
				APIKey: "sk-single",
				AccountPool: &AccountPoolConfig{
					Accounts: []BackendAccount{
						{ID: "key-1", APIKey: "sk-pool-1", Enabled: false},
					},
				},
			},
			expected: "sk-single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEffectiveAPIKey(tt.cfg)
			if result != tt.expected {
				t.Errorf("GetEffectiveAPIKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBackendConfig_AccountPoolInResponse(t *testing.T) {
	cfg := &BackendConfig{
		ID:     "backend-1",
		Name:   "Test Backend",
		APIKey: "sk-main",
		AccountPool: &AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []BackendAccount{
				{ID: "key-1", APIKey: "sk-1", Enabled: true},
				{ID: "key-2", APIKey: "sk-2", Enabled: true},
				{ID: "key-3", APIKey: "sk-3", Enabled: false},
			},
		},
	}

	resp := cfg.ToResponse()

	if resp.AccountPoolSummary == nil {
		t.Fatal("expected AccountPoolSummary to be set")
	}

	if resp.AccountPoolSummary.TotalAccounts != 3 {
		t.Errorf("expected TotalAccounts=3, got %d", resp.AccountPoolSummary.TotalAccounts)
	}

	if resp.AccountPoolSummary.EnabledAccounts != 2 {
		t.Errorf("expected EnabledAccounts=2, got %d", resp.AccountPoolSummary.EnabledAccounts)
	}

	if resp.AccountPoolSummary.Strategy != "round_robin" {
		t.Errorf("expected Strategy=round_robin, got %s", resp.AccountPoolSummary.Strategy)
	}

	if resp.AccountPoolSummary.HealthStatus != "partial" {
		t.Errorf("expected HealthStatus=partial, got %s", resp.AccountPoolSummary.HealthStatus)
	}
}

// TestAccountPoolStreamParity 验证流式/非流式路径账户选择一致性
// 流式和非流式请求应该使用相同的账户选择逻辑
func TestAccountPoolStreamParity(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
			{ID: "key-3", APIKey: "sk-3", Enabled: true},
		},
	}

	// 模拟流式请求（连续快速选择）
	streamResults := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("stream request %d: unexpected error: %v", i, err)
		}
		streamResults = append(streamResults, result.Account.ID)
	}

	// 重置选择器
	selector = NewAccountPoolSelector()

	// 模拟非流式请求（相同模式）
	nonStreamResults := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("non-stream request %d: unexpected error: %v", i, err)
		}
		nonStreamResults = append(nonStreamResults, result.Account.ID)
	}

	// 验证选择模式一致
	for i := 0; i < 6; i++ {
		if streamResults[i] != nonStreamResults[i] {
			t.Errorf("request %d: stream selected %s, non-stream selected %s",
				i, streamResults[i], nonStreamResults[i])
		}
	}
}

// TestAccountPoolStickySessionConsistency 验证粘性会话在流式/非流式路径的一致性
func TestAccountPoolStickySessionConsistency(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "sticky_session",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	sessionKey := "user-123"

	// 流式请求
	result1, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("stream request 1: unexpected error: %v", err)
	}

	result2, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("stream request 2: unexpected error: %v", err)
	}

	// 非流式请求（相同 session key）
	result3, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("non-stream request 1: unexpected error: %v", err)
	}

	result4, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("non-stream request 2: unexpected error: %v", err)
	}

	// 验证所有请求都选择同一个账户
	if result1.Account.ID != result2.Account.ID || result2.Account.ID != result3.Account.ID || result3.Account.ID != result4.Account.ID {
		t.Errorf("sticky session inconsistency: %s, %s, %s, %s",
			result1.Account.ID, result2.Account.ID, result3.Account.ID, result4.Account.ID)
	}
}

// TestAccountPoolLeastUsageConsistency 验证最少使用策略在流式/非流式路径的一致性
func TestAccountPoolLeastUsageConsistency(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "least_usage",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
			{ID: "key-3", APIKey: "sk-3", Enabled: true},
		},
	}

	// 模拟 3 个流式请求
	results := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		results = append(results, result.Account.ID)
	}

	// 验证每个账户都被选择一次
	seen := make(map[string]bool)
	for _, id := range results {
		seen[id] = true
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 different accounts, got %d: %v", len(seen), results)
	}
}

// TestAccountPoolWeightedSelection 验证加权选择在流式/非流式路径的一致性
// 注意：当前实现是简单的轮询，权重字段保留但未在选择逻辑中使用
// 此测试验证轮询行为的一致性
func TestAccountPoolWeightedSelection(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true, Weight: 2},
			{ID: "key-2", APIKey: "sk-2", Enabled: true, Weight: 1},
		},
	}

	// 选择 6 次
	results := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		results = append(results, result.Account.ID)
	}

	// 验证轮询行为（key-1 和 key-2 交替选择）
	expected := []string{"key-1", "key-2", "key-1", "key-2", "key-1", "key-2"}
	for i, id := range results {
		if id != expected[i] {
			t.Errorf("request %d: expected %s, got %s", i, expected[i], id)
		}
	}
}

// TestAccountPoolConcurrentAccess 验证并发访问安全性
func TestAccountPoolConcurrentAccess(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	// 并发选择
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, err := selector.SelectAccountForRequest(context.Background(), pool, "")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAccountPoolSessionKeyExtractionPriority 验证会话亲和 key 提取优先级
func TestAccountPoolSessionKeyExtractionPriority(t *testing.T) {
	tests := []struct {
		name            string
		body            []byte
		headerSessionID string
		expected        string
		description     string
	}{
		{
			name:            "header takes precedence",
			headerSessionID: "header-session",
			body:            []byte(`{"user": "openai-user"}`),
			expected:        "header-session",
			description:     "Header X-Session-ID should take precedence over body fields",
		},
		{
			name:     "openai user field",
			body:     []byte(`{"user": "openai-user"}`),
			expected: "openai:openai-user",
			description: "OpenAI user field should be used when no header",
		},
		{
			name:     "anthropic user_id field",
			body:     []byte(`{"metadata": {"user_id": "anthropic-user"}}`),
			expected: "anthropic:anthropic-user",
			description: "Anthropic user_id field should be used when no header",
		},
		{
			name:     "no key available",
			body:     []byte(`{}`),
			expected: "",
			description: "Empty string should be returned when no key available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSessionKey(context.Background(), tt.body, tt.headerSessionID)
			if result != tt.expected {
				t.Errorf("ExtractSessionKey() = %v, want %v (%s)", result, tt.expected, tt.description)
			}
		})
	}
}

// TestAccountPoolDisabledAccountSelection 验证禁用账户不被选择
func TestAccountPoolDisabledAccountSelection(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: false},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
			{ID: "key-3", APIKey: "sk-3", Enabled: false},
		},
	}

	// 选择 5 次，应该只选择 key-2
	for i := 0; i < 5; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if result.Account.ID != "key-2" {
			t.Errorf("request %d: expected key-2, got %s", i, result.Account.ID)
		}
	}
}

// TestAccountPoolGetAccountByID 验证通过 ID 获取账户
func TestAccountPoolGetAccountByID(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Label: "Account 1"},
			{ID: "key-2", APIKey: "sk-2", Label: "Account 2"},
		},
	}

	// 获取存在的账户
	acc, err := GetAccountByID(pool, "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID != "key-1" || acc.Label != "Account 1" {
		t.Errorf("expected Account 1, got %v", acc)
	}

	// 获取不存在的账户
	_, err = GetAccountByID(pool, "key-3")
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

// TestAccountPoolNormalize 验证规范化账户池配置
func TestAccountPoolNormalize(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Weight: 0, CreatedAt: ""},
			{ID: "key-2", APIKey: "sk-2", Weight: -1, CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}

	NormalizeAccountPool(pool)

	if pool.Strategy != "round_robin" {
		t.Errorf("expected default strategy 'round_robin', got '%s'", pool.Strategy)
	}

	if pool.Accounts[0].Weight != 1 {
		t.Errorf("expected weight 1, got %d", pool.Accounts[0].Weight)
	}

	if pool.Accounts[0].CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}

	if pool.Accounts[1].Weight != 1 {
		t.Errorf("expected weight 1, got %d", pool.Accounts[1].Weight)
	}
}
