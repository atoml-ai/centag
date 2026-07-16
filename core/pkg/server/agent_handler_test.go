package server

import (
	"testing"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
)

func TestSelectDecryptableProxyAPIKey(t *testing.T) {
	storageKey := make([]byte, 32)
	for i := range storageKey {
		storageKey[i] = byte(i + 1)
	}

	now := time.Now()
	expired := now.Add(-time.Hour)

	encOld, err := auth.EncryptAPIKeyPlaintext("llmproxy_old", storageKey)
	if err != nil {
		t.Fatalf("encrypt old key: %v", err)
	}
	encLatest, err := auth.EncryptAPIKeyPlaintext("llmproxy_latest", storageKey)
	if err != nil {
		t.Fatalf("encrypt latest key: %v", err)
	}
	encExpired, err := auth.EncryptAPIKeyPlaintext("llmproxy_expired", storageKey)
	if err != nil {
		t.Fatalf("encrypt expired key: %v", err)
	}

	keys := []*database.APIKey{
		{
			ID:           1,
			Enabled:      true,
			KeySecretEnc: encOld,
			CreatedAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:           2,
			Enabled:      true,
			KeySecretEnc: encExpired,
			CreatedAt:    now.Add(-30 * time.Minute),
			ExpiresAt:    &expired,
		},
		{
			ID:           3,
			Enabled:      true,
			KeySecretEnc: encLatest,
			CreatedAt:    now.Add(-10 * time.Minute),
		},
	}

	got, ok := selectDecryptableProxyAPIKey(keys, now, storageKey)
	if !ok {
		t.Fatalf("expected usable key, got none")
	}
	if got != "llmproxy_latest" {
		t.Fatalf("selected key = %q, want %q", got, "llmproxy_latest")
	}
}

func TestSelectDecryptableProxyAPIKey_RejectNonProxyPrefix(t *testing.T) {
	storageKey := make([]byte, 32)
	for i := range storageKey {
		storageKey[i] = byte(i + 1)
	}

	now := time.Now()
	encOther, err := auth.EncryptAPIKeyPlaintext("sk-provider-xxx", storageKey)
	if err != nil {
		t.Fatalf("encrypt non-proxy key: %v", err)
	}

	keys := []*database.APIKey{
		{
			ID:           1,
			Enabled:      true,
			KeySecretEnc: encOther,
			CreatedAt:    now,
		},
	}

	if got, ok := selectDecryptableProxyAPIKey(keys, now, storageKey); ok {
		t.Fatalf("expected no key, got %q", got)
	}
}

func TestLoadDefaultProxyAdminAPIKey(t *testing.T) {
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")

	if _, ok := loadDefaultProxyAdminAPIKey(); ok {
		t.Fatalf("expected no env key")
	}

	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "sk-provider-123")
	if _, ok := loadDefaultProxyAdminAPIKey(); ok {
		t.Fatalf("should ignore non-llmproxy key")
	}

	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "llmproxy_default_admin")
	got, ok := loadDefaultProxyAdminAPIKey()
	if !ok {
		t.Fatalf("expected default admin key from env")
	}
	if got != "llmproxy_default_admin" {
		t.Fatalf("got %q, want %q", got, "llmproxy_default_admin")
	}
}

func TestResolveModelName_PipelineOnly(t *testing.T) {
	got := resolveModelName("glm-4-flash", "direct-backend", []backend.ModelMapping{
		{ActualModel: "gpt-4o"},
	})
	if got != "pipeline.direct-backend" {
		t.Fatalf("model = %q, want %q", got, "pipeline.direct-backend")
	}
}

func TestResolveModelName_NoPipelineUseDefault(t *testing.T) {
	got := resolveModelName("", "", []backend.ModelMapping{
		{ActualModel: "gpt-4o-mini"},
	})
	if got != "gpt-4o-mini" {
		t.Fatalf("model = %q, want %q", got, "gpt-4o-mini")
	}
}
