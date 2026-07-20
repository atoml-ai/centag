package database

import (
	"testing"
	"time"
)

func TestUserRole_Constants(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %s, want admin", RoleAdmin)
	}
	if RoleNormal != "normal" {
		t.Errorf("RoleNormal = %s, want normal", RoleNormal)
	}
}

func TestUser_Fields(t *testing.T) {
	now := time.Now()
	quotaResetDate := now.Add(30 * 24 * time.Hour)
	user := &User{
		ID:          1,
		Username:    "testuser",
		Password:    "hashed_password",
		Role:        RoleAdmin,
		DisplayName: "Test User",
		Email:       "test@example.com",
		Enabled:     true,
		// v2.1 quota fields
		DefaultPipelineID: "pipeline-default",
		DailyTokenLimit:   100000,
		MonthlyTokenLimit: 3000000,
		DailyTokenUsed:   50000,
		MonthlyTokenUsed: 1500000,
		QuotaResetDate:   &quotaResetDate,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if user.ID != 1 {
		t.Errorf("User.ID = %d, want 1", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("User.Username = %s, want testuser", user.Username)
	}
	if user.Role != RoleAdmin {
		t.Errorf("User.Role = %s, want admin", user.Role)
	}
	if !user.Enabled {
		t.Error("User.Enabled should be true")
	}
	// v2.1 quota fields
	if user.DefaultPipelineID != "pipeline-default" {
		t.Errorf("User.DefaultPipelineID = %s, want pipeline-default", user.DefaultPipelineID)
	}
	if user.DailyTokenLimit != 100000 {
		t.Errorf("User.DailyTokenLimit = %d, want 100000", user.DailyTokenLimit)
	}
	if user.MonthlyTokenLimit != 3000000 {
		t.Errorf("User.MonthlyTokenLimit = %d, want 3000000", user.MonthlyTokenLimit)
	}
	if user.DailyTokenUsed != 50000 {
		t.Errorf("User.DailyTokenUsed = %d, want 50000", user.DailyTokenUsed)
	}
	if user.MonthlyTokenUsed != 1500000 {
		t.Errorf("User.MonthlyTokenUsed = %d, want 1500000", user.MonthlyTokenUsed)
	}
	if user.QuotaResetDate == nil {
		t.Error("User.QuotaResetDate should not be nil")
	}
}

func TestUser_DefaultQuotaFields(t *testing.T) {
	user := &User{
		ID:       2,
		Username: "newuser",
		Role:     RoleNormal,
	}

	// Default values for quota fields
	if user.DefaultPipelineID != "" {
		t.Errorf("default DefaultPipelineID should be empty, got %s", user.DefaultPipelineID)
	}
	if user.DailyTokenLimit != 0 {
		t.Errorf("default DailyTokenLimit should be 0, got %d", user.DailyTokenLimit)
	}
	if user.MonthlyTokenLimit != 0 {
		t.Errorf("default MonthlyTokenLimit should be 0, got %d", user.MonthlyTokenLimit)
	}
	if user.DailyTokenUsed != 0 {
		t.Errorf("default DailyTokenUsed should be 0, got %d", user.DailyTokenUsed)
	}
	if user.MonthlyTokenUsed != 0 {
		t.Errorf("default MonthlyTokenUsed should be 0, got %d", user.MonthlyTokenUsed)
	}
	if user.QuotaResetDate != nil {
		t.Error("default QuotaResetDate should be nil")
	}
}

func TestAPIKey_Fields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	lastUsedAt := now

	key := &APIKey{
		ID:             1,
		UserID:         100,
		Name:           "Test Key",
		KeyHash:        "abc123hash",
		KeySecretEnc:   "encrypted_data",
		KeyPrefix:      "llmproxy_abc123",
		ExpiresAt:      &expiresAt,
		LastUsedAt:     &lastUsedAt,
		Enabled:        true,
		BudgetUSD:      100.50,
		UsedUSD:        25.25,
		RateLimitRPM:   60,
		RateLimitTPM:   100000,
		ModelWhitelist: `["gpt-4","gpt-3.5-turbo"]`,
		CreatedAt:      now,
	}

	if key.ID != 1 {
		t.Errorf("APIKey.ID = %d, want 1", key.ID)
	}
	if key.UserID != 100 {
		t.Errorf("APIKey.UserID = %d, want 100", key.UserID)
	}
	if key.KeyPrefix != "llmproxy_abc123" {
		t.Errorf("APIKey.KeyPrefix = %s, want llmproxy_abc123", key.KeyPrefix)
	}
	if key.ExpiresAt == nil {
		t.Error("APIKey.ExpiresAt should not be nil")
	}
	if key.BudgetUSD != 100.50 {
		t.Errorf("APIKey.BudgetUSD = %f, want 100.50", key.BudgetUSD)
	}
	if key.UsedUSD != 25.25 {
		t.Errorf("APIKey.UsedUSD = %f, want 25.25", key.UsedUSD)
	}
	if key.RateLimitRPM != 60 {
		t.Errorf("APIKey.RateLimitRPM = %d, want 60", key.RateLimitRPM)
	}
	if key.RateLimitTPM != 100000 {
		t.Errorf("APIKey.RateLimitTPM = %d, want 100000", key.RateLimitTPM)
	}
	if key.ModelWhitelist != `["gpt-4","gpt-3.5-turbo"]` {
		t.Errorf("APIKey.ModelWhitelist = %s, want JSON array string", key.ModelWhitelist)
	}
}

func TestAPIKey_DefaultLimitFields(t *testing.T) {
	key := &APIKey{
		ID:     2,
		UserID: 200,
		Name:   "Default Key",
	}
	if key.BudgetUSD != 0 {
		t.Errorf("default BudgetUSD should be 0, got %f", key.BudgetUSD)
	}
	if key.UsedUSD != 0 {
		t.Errorf("default UsedUSD should be 0, got %f", key.UsedUSD)
	}
	if key.RateLimitRPM != 0 {
		t.Errorf("default RateLimitRPM should be 0, got %d", key.RateLimitRPM)
	}
	if key.RateLimitTPM != 0 {
		t.Errorf("default RateLimitTPM should be 0, got %d", key.RateLimitTPM)
	}
	if key.ModelWhitelist != "" {
		t.Errorf("default ModelWhitelist should be empty, got %s", key.ModelWhitelist)
	}
}

func TestAPIKey_NilExpiration(t *testing.T) {
	key := &APIKey{
		ID:       1,
		UserID:   100,
		Name:     "No Expiry Key",
		Enabled:  true,
	}

	if key.ExpiresAt != nil {
		t.Error("APIKey.ExpiresAt should be nil for non-expiring key")
	}
	if key.LastUsedAt != nil {
		t.Error("APIKey.LastUsedAt should be nil when never used")
	}
}

func TestUserConfig_Fields(t *testing.T) {
	config := &UserConfig{
		ID:            1,
		UserID:        100,
		Backends:      `[{"name":"test"}]`,
		ProxySettings: `{"enabled":true}`,
		CacheSettings: `{"ttl":3600}`,
		Embedding:     `{"model":"text-embedding"}`,
		QASplit:       `{"enabled":false}`,
		PresetModes:   `[{"name":"default"}]`,
		Scheduling:    `{"mode":"round-robin"}`,
		CacheControl:  `{"enabled":true}`,
		AuthSettings:  `{"require_api_key":true}`,
	}

	if config.ID != 1 {
		t.Errorf("UserConfig.ID = %d, want 1", config.ID)
	}
	if config.UserID != 100 {
		t.Errorf("UserConfig.UserID = %d, want 100", config.UserID)
	}
}

func TestAuthSettings_Struct(t *testing.T) {
	settings := AuthSettings{
		RequireAPIKey: true,
		AllowNoAuth:   false,
	}

	if !settings.RequireAPIKey {
		t.Error("AuthSettings.RequireAPIKey should be true")
	}
	if settings.AllowNoAuth {
		t.Error("AuthSettings.AllowNoAuth should be false")
	}
}

func TestRefreshToken_Fields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour)

	token := &RefreshToken{
		ID:        1,
		UserID:    100,
		TokenHash: "token_hash_value",
		ExpiresAt: expiresAt,
		CreatedAt: now,
		Revoked:   false,
	}

	if token.ID != 1 {
		t.Errorf("RefreshToken.ID = %d, want 1", token.ID)
	}
	if token.Revoked {
		t.Error("RefreshToken.Revoked should be false")
	}
}

func TestClashRule_Fields(t *testing.T) {
	now := time.Now()

	rule := &ClashRule{
		ID:            1,
		UserID:        100,
		Name:          "Test Rule",
		RuleContent:    "rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT",
		SubscribeToken: "sub_token_abc123",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if rule.ID != 1 {
		t.Errorf("ClashRule.ID = %d, want 1", rule.ID)
	}
	if rule.Name != "Test Rule" {
		t.Errorf("ClashRule.Name = %s, want Test Rule", rule.Name)
	}
	if rule.SubscribeToken != "sub_token_abc123" {
		t.Errorf("ClashRule.SubscribeToken = %s, want sub_token_abc123", rule.SubscribeToken)
	}
}

func TestTeamQuota_Fields(t *testing.T) {
	now := time.Now()
	tq := &TeamQuota{
		ID:               1,
		TenantID:         "tenant-001",
		DailyTokenLimit:   1000000,
		MonthlyTokenLimit: 30000000,
		DailyTokenUsed:   500000,
		MonthlyTokenUsed: 15000000,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if tq.ID != 1 {
		t.Errorf("TeamQuota.ID = %d, want 1", tq.ID)
	}
	if tq.TenantID != "tenant-001" {
		t.Errorf("TeamQuota.TenantID = %s, want tenant-001", tq.TenantID)
	}
	if tq.DailyTokenLimit != 1000000 {
		t.Errorf("TeamQuota.DailyTokenLimit = %d, want 1000000", tq.DailyTokenLimit)
	}
	if tq.MonthlyTokenLimit != 30000000 {
		t.Errorf("TeamQuota.MonthlyTokenLimit = %d, want 30000000", tq.MonthlyTokenLimit)
	}
	if tq.DailyTokenUsed != 500000 {
		t.Errorf("TeamQuota.DailyTokenUsed = %d, want 500000", tq.DailyTokenUsed)
	}
	if tq.MonthlyTokenUsed != 15000000 {
		t.Errorf("TeamQuota.MonthlyTokenUsed = %d, want 15000000", tq.MonthlyTokenUsed)
	}
}

func TestTeamQuota_DefaultFields(t *testing.T) {
	tq := &TeamQuota{
		ID:       1,
		TenantID: "tenant-002",
	}

	// Default values
	if tq.DailyTokenLimit != 0 {
		t.Errorf("default DailyTokenLimit should be 0, got %d", tq.DailyTokenLimit)
	}
	if tq.MonthlyTokenLimit != 0 {
		t.Errorf("default MonthlyTokenLimit should be 0, got %d", tq.MonthlyTokenLimit)
	}
	if tq.DailyTokenUsed != 0 {
		t.Errorf("default DailyTokenUsed should be 0, got %d", tq.DailyTokenUsed)
	}
	if tq.MonthlyTokenUsed != 0 {
		t.Errorf("default MonthlyTokenUsed should be 0, got %d", tq.MonthlyTokenUsed)
	}
}

func TestSchedulerDecision_Fields(t *testing.T) {
	now := time.Now()
	d := &SchedulerDecision{
		ID:        1,
		RequestID: "req-abc123",
		UserID:    100,
		TenantID:  "tenant-001",
		Model:     "gpt-4",
		Backend:   "openai",
		Strategy:  "weighted-random",
		Score:     0.85,
		Reason:    "high score due to low latency and good health",
		CreatedAt: now,
	}

	if d.ID != 1 {
		t.Errorf("SchedulerDecision.ID = %d, want 1", d.ID)
	}
	if d.RequestID != "req-abc123" {
		t.Errorf("SchedulerDecision.RequestID = %s, want req-abc123", d.RequestID)
	}
	if d.UserID != 100 {
		t.Errorf("SchedulerDecision.UserID = %d, want 100", d.UserID)
	}
	if d.TenantID != "tenant-001" {
		t.Errorf("SchedulerDecision.TenantID = %s, want tenant-001", d.TenantID)
	}
	if d.Model != "gpt-4" {
		t.Errorf("SchedulerDecision.Model = %s, want gpt-4", d.Model)
	}
	if d.Backend != "openai" {
		t.Errorf("SchedulerDecision.Backend = %s, want openai", d.Backend)
	}
	if d.Strategy != "weighted-random" {
		t.Errorf("SchedulerDecision.Strategy = %s, want weighted-random", d.Strategy)
	}
	if d.Score != 0.85 {
		t.Errorf("SchedulerDecision.Score = %f, want 0.85", d.Score)
	}
	if d.Reason != "high score due to low latency and good health" {
		t.Errorf("SchedulerDecision.Reason = %s, want expected reason", d.Reason)
	}
}

func TestSchedulerDecision_DefaultFields(t *testing.T) {
	d := &SchedulerDecision{
		ID:        1,
		RequestID: "req-xyz789",
		Model:     "gpt-3.5-turbo",
		Backend:   "ollama",
		Strategy:  "round-robin",
	}

	// Default values
	if d.UserID != 0 {
		t.Errorf("default UserID should be 0, got %d", d.UserID)
	}
	if d.TenantID != "" {
		t.Errorf("default TenantID should be empty, got %s", d.TenantID)
	}
	if d.Score != 0 {
		t.Errorf("default Score should be 0, got %f", d.Score)
	}
	if d.Reason != "" {
		t.Errorf("default Reason should be empty, got %s", d.Reason)
	}
}
