package server

import (
	"testing"
	"time"

	"centag/core/pkg/database"
)

func TestToAPIKeyResponse_Fields(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	exp := now.Add(30 * 24 * time.Hour)
	lastUsed := now.Add(-1 * time.Hour)

	key := &database.APIKey{
		ID:             42,
		Name:           "production-key",
		KeyPrefix:      "llmproxy_abcd1234",
		KeySecretEnc:   "encrypted-content",
		Enabled:        true,
		BudgetUSD:      100.0,
		UsedUSD:        25.5,
		RateLimitRPM:   60,
		RateLimitTPM:   100000,
		ModelWhitelist: `["gpt-4","gpt-3.5-turbo"]`,
		ExpiresAt:      &exp,
		LastUsedAt:     &lastUsed,
		CreatedAt:      now,
	}

	resp := toAPIKeyResponse(key)

	if resp.ID != 42 {
		t.Errorf("ID = %d, want 42", resp.ID)
	}
	if resp.Name != "production-key" {
		t.Errorf("Name = %s, want production-key", resp.Name)
	}
	if resp.BudgetUSD != 100.0 {
		t.Errorf("BudgetUSD = %f, want 100.0", resp.BudgetUSD)
	}
	if resp.UsedUSD != 25.5 {
		t.Errorf("UsedUSD = %f, want 25.5", resp.UsedUSD)
	}
	if resp.RateLimitRPM != 60 {
		t.Errorf("RateLimitRPM = %d, want 60", resp.RateLimitRPM)
	}
	if resp.RateLimitTPM != 100000 {
		t.Errorf("RateLimitTPM = %d, want 100000", resp.RateLimitTPM)
	}
	if resp.ModelWhitelist != `["gpt-4","gpt-3.5-turbo"]` {
		t.Errorf("ModelWhitelist = %s", resp.ModelWhitelist)
	}
	if resp.CreatedAt != "2026-01-15 10:30:00" {
		t.Errorf("CreatedAt = %s, want 2026-01-15 10:30:00", resp.CreatedAt)
	}
	if resp.ExpiresAt == nil || *resp.ExpiresAt == "" {
		t.Error("ExpiresAt should be set")
	}
	if resp.LastUsedAt == nil || *resp.LastUsedAt == "" {
		t.Error("LastUsedAt should be set")
	}
	if !resp.RevealAvailable {
		t.Error("RevealAvailable should be true when KeySecretEnc is set")
	}
}

func TestToAPIKeyResponse_MaskedKey(t *testing.T) {
	key := &database.APIKey{
		ID:        1,
		Name:      "test",
		KeyPrefix: "llmproxy_12345678",
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	resp := toAPIKeyResponse(key)
	if resp.MaskedKey == "" {
		t.Error("MaskedKey should not be empty")
	}
	// MaskedKey should not expose the full prefix
	if resp.MaskedKey == "llmproxy_12345678" {
		t.Error("MaskedKey should not be the full prefix")
	}
}

func TestToAPIKeyResponse_NoEncyption(t *testing.T) {
	key := &database.APIKey{
		ID:          1,
		Name:        "no-encrypt",
		KeyPrefix:   "llmproxy_xxx",
		Enabled:     true,
		CreatedAt:   time.Now(),
		// KeySecretEnc empty — cannot reveal
	}
	resp := toAPIKeyResponse(key)
	if resp.RevealAvailable {
		t.Error("RevealAvailable should be false when KeySecretEnc is empty")
	}
}

func TestToAPIKeyResponse_NilTimestamps(t *testing.T) {
	key := &database.APIKey{
		ID:        1,
		Name:      "no-timestamps",
		KeyPrefix: "llmproxy_xxx",
		Enabled:   true,
		CreatedAt: time.Now(),
		// ExpiresAt & LastUsedAt nil
	}
	resp := toAPIKeyResponse(key)
	if resp.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil when key has no expiry")
	}
	if resp.LastUsedAt != nil {
		t.Error("LastUsedAt should be nil when key has never been used")
	}
}

func TestToAPIKeyResponses_Empty(t *testing.T) {
	resps := toAPIKeyResponses(nil)
	if len(resps) != 0 {
		t.Error("nil input should return empty slice")
	}
	resps = toAPIKeyResponses([]*database.APIKey{})
	if len(resps) != 0 {
		t.Error("empty input should return empty slice")
	}
}

func TestToAPIKeyResponses_Multiple(t *testing.T) {
	now := time.Now()
	keys := []*database.APIKey{
		{ID: 1, Name: "key-1", KeyPrefix: "llmproxy_a", Enabled: true, CreatedAt: now},
		{ID: 2, Name: "key-2", KeyPrefix: "llmproxy_b", Enabled: false, CreatedAt: now},
		{ID: 3, Name: "key-3", KeyPrefix: "llmproxy_c", Enabled: true, CreatedAt: now},
	}
	resps := toAPIKeyResponses(keys)
	if len(resps) != 3 {
		t.Fatalf("len = %d, want 3", len(resps))
	}
	if resps[0].ID != 1 || resps[1].ID != 2 || resps[2].ID != 3 {
		t.Error("response order mismatch")
	}
}

func TestToAPIKeyResponse_VirtualKeyFields(t *testing.T) {
	now := time.Now()
	key := &database.APIKey{
		ID:             1,
		Name:           "vk-test",
		KeyPrefix:      "llmproxy_vk",
		Enabled:        true,
		BudgetUSD:      50.0,
		UsedUSD:        0,
		RateLimitRPM:   0,
		RateLimitTPM:   0,
		ModelWhitelist: "*",
		CreatedAt:      now,
	}
	resp := toAPIKeyResponse(key)
	if resp.BudgetUSD != 50.0 {
		t.Errorf("BudgetUSD = %f, want 50.0", resp.BudgetUSD)
	}
	if resp.UsedUSD != 0 {
		t.Errorf("UsedUSD = %f, want 0", resp.UsedUSD)
	}
	if resp.RateLimitRPM != 0 {
		t.Errorf("RateLimitRPM = %d, want 0", resp.RateLimitRPM)
	}
	if resp.RateLimitTPM != 0 {
		t.Errorf("RateLimitTPM = %d, want 0", resp.RateLimitTPM)
	}
	if resp.ModelWhitelist != "*" {
		t.Errorf("ModelWhitelist = %s, want *", resp.ModelWhitelist)
	}
}
