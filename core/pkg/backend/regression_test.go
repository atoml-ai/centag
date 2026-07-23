package backend

import (
	"context"
	"testing"
)

// TestNilAccountPoolCompat 验证无 account_pool 后端回归
// 确保没有配置账户池的后端仍然正常工作
func TestNilAccountPoolCompat(t *testing.T) {
	// 测试 nil 配置
	if HasAccountPool(nil) {
		t.Error("expected HasAccountPool(nil) to return false")
	}

	// 测试空配置
	cfg := &BackendConfig{}
	if HasAccountPool(cfg) {
		t.Error("expected HasAccountPool(empty config) to return false")
	}

	// 测试有账户池配置
	cfgWithPool := &BackendConfig{
		AccountPool: &AccountPoolConfig{
			Accounts: []BackendAccount{
				{ID: "key-1", APIKey: "sk-1"},
			},
		},
	}
	if !HasAccountPool(cfgWithPool) {
		t.Error("expected HasAccountPool(config with pool) to return true")
	}

	// 测试 GetEffectiveAPIKey 在无账户池时返回主 Key
	cfgNoPool := &BackendConfig{
		APIKey: "sk-main",
	}
	key := GetEffectiveAPIKey(cfgNoPool)
	if key != "sk-main" {
		t.Errorf("expected 'sk-main', got '%s'", key)
	}
}

// TestAccountPool429Failover 验证 429 自动切换
// 当某个账户返回 429 时，应该自动切换到下一个账户
func TestAccountPool429Failover(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
			{ID: "key-3", APIKey: "sk-3", Enabled: true},
		},
	}

	// 第一次选择
	result1, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 模拟 429 错误，禁用账户
	for i := range pool.Accounts {
		if pool.Accounts[i].ID == result1.Account.ID {
			pool.Accounts[i].Enabled = false
			break
		}
	}

	// 第二次选择应该选择不同的账户
	result2, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.Account.ID == result2.Account.ID {
		t.Errorf("expected different account after 429 failover, got same: %s", result1.Account.ID)
	}

	// 恢复第一个账户
	for i := range pool.Accounts {
		if pool.Accounts[i].ID == result1.Account.ID {
			pool.Accounts[i].Enabled = true
			break
		}
	}

	// 第三次选择应该可以选择恢复的账户
	result3, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证选择逻辑正常
	if result3.Account.ID == "" {
		t.Error("expected valid account selection")
	}
}

// TestAccountScopedBreaker 验证账户级熔断
// 确保每个账户有独立的熔断器状态
func TestAccountScopedBreaker(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: true},
		},
	}

	// 选择第一个账户
	result1, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 选择第二个账户
	result2, err := selector.SelectAccountForRequest(context.Background(), pool, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证选择不同的账户
	if result1.Account.ID == result2.Account.ID {
		t.Errorf("expected different accounts, got same: %s", result1.Account.ID)
	}

	// 验证计数器独立
	selector.mu.RLock()
	count1 := selector.counters["key-1"]
	count2 := selector.counters["key-2"]
	selector.mu.RUnlock()

	if count1 != 1 || count2 != 1 {
		t.Errorf("expected each account to have count 1, got key-1: %d, key-2: %d", count1, count2)
	}
}

// TestAccountPoolReset 验证账户池重置功能
func TestAccountPoolReset(t *testing.T) {
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

	// 验证会话绑定
	selector.mu.RLock()
	boundID := selector.sessions[sessionKey]
	selector.mu.RUnlock()

	if boundID != result1.Account.ID {
		t.Errorf("expected session bound to %s, got %s", result1.Account.ID, boundID)
	}

	// 重置选择器
	selector = NewAccountPoolSelector()

	// 再次选择应该可以选择不同的账户
	result2, err := selector.SelectAccountForRequest(context.Background(), pool, sessionKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证会话重新绑定
	selector.mu.RLock()
	newBoundID := selector.sessions[sessionKey]
	selector.mu.RUnlock()

	if newBoundID != result2.Account.ID {
		t.Errorf("expected session bound to %s, got %s", result2.Account.ID, newBoundID)
	}
}

// TestAccountPoolHealthCheck 验证账户健康检查逻辑
func TestAccountPoolHealthCheck(t *testing.T) {
	selector := NewAccountPoolSelector()

	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: true},
			{ID: "key-2", APIKey: "sk-2", Enabled: false},
			{ID: "key-3", APIKey: "sk-3", Enabled: true},
		},
	}

	// 多次选择应该只选择启用的账户
	selected := make(map[string]int)
	for i := 0; i < 6; i++ {
		result, err := selector.SelectAccountForRequest(context.Background(), pool, "")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		selected[result.Account.ID]++
	}

	// 验证只选择了启用的账户
	if _, exists := selected["key-2"]; exists {
		t.Error("expected disabled account 'key-2' not to be selected")
	}

	// 验证每个启用的账户都被选择
	if selected["key-1"] != 3 {
		t.Errorf("expected key-1 to be selected 3 times, got %d", selected["key-1"])
	}
	if selected["key-3"] != 3 {
		t.Errorf("expected key-3 to be selected 3 times, got %d", selected["key-3"])
	}
}

// TestAccountPoolErrorHandling 验证错误处理
func TestAccountPoolErrorHandling(t *testing.T) {
	selector := NewAccountPoolSelector()

	// 测试 nil pool
	_, err := selector.SelectAccountForRequest(context.Background(), nil, "")
	if err == nil {
		t.Error("expected error for nil pool")
	}

	// 测试空 pool
	_, err = selector.SelectAccountForRequest(context.Background(), &AccountPoolConfig{}, "")
	if err == nil {
		t.Error("expected error for empty pool")
	}

	// 测试所有账户禁用
	_, err = selector.SelectAccountForRequest(context.Background(), &AccountPoolConfig{
		Accounts: []BackendAccount{
			{ID: "key-1", APIKey: "sk-1", Enabled: false},
		},
	}, "")
	if err == nil {
		t.Error("expected error when all accounts are disabled")
	}
}

// TestAccountPoolSessionKeyExtraction 验证会话亲和 key 提取
func TestAccountPoolSessionKeyExtraction(t *testing.T) {
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

// TestAccountPoolValidation 验证账户池配置验证
func TestAccountPoolValidation(t *testing.T) {
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

// TestAccountPoolNormalization 验证账户池规范化
func TestAccountPoolNormalization(t *testing.T) {
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

// TestAccountPoolCRUDOperations 验证账户 CRUD 操作
func TestAccountPoolCRUDOperations(t *testing.T) {
	pool := &AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []BackendAccount{},
	}

	// 添加账户
	acc1 := BackendAccount{ID: "key-1", APIKey: "sk-1", Label: "Account 1"}
	if err := AddAccount(pool, acc1); err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acc2 := BackendAccount{ID: "key-2", APIKey: "sk-2", Label: "Account 2"}
	if err := AddAccount(pool, acc2); err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	if len(pool.Accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(pool.Accounts))
	}

	// 获取账户
	acc, err := GetAccountByID(pool, "key-1")
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}
	if acc.Label != "Account 1" {
		t.Errorf("expected label 'Account 1', got '%s'", acc.Label)
	}

	// 更新账户
	updatedAcc := BackendAccount{ID: "key-1", APIKey: "sk-1", Label: "Updated Account 1"}
	if err := UpdateAccount(pool, updatedAcc); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}

	acc, err = GetAccountByID(pool, "key-1")
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}
	if acc.Label != "Updated Account 1" {
		t.Errorf("expected label 'Updated Account 1', got '%s'", acc.Label)
	}

	// 删除账户
	if err := RemoveAccount(pool, "key-1"); err != nil {
		t.Fatalf("RemoveAccount failed: %v", err)
	}

	if len(pool.Accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(pool.Accounts))
	}

	// 获取不存在的账户
	_, err = GetAccountByID(pool, "key-1")
	if err == nil {
		t.Error("expected error for non-existent account")
	}

	// 删除不存在的账户
	err = RemoveAccount(pool, "key-1")
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

// TestAccountPoolConcurrentOperations 验证并发操作安全性
func TestAccountPoolConcurrentOperations(t *testing.T) {
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
