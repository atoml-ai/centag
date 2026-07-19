package server

import (
	"testing"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
)

func TestRevealAPIKeyPlaintext_FromEnvFallback(t *testing.T) {
	const full = "llmproxy_packaged_test_key_abcdef0123456789"
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", full)
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_API_KEY_STORAGE_SECRET", "")

	hash, prefix := auth.APIKeyMetadataFromFullKey(full)
	key := &database.APIKey{ID: 1, KeyHash: hash, KeyPrefix: prefix, KeySecretEnc: ""}
	if !apiKeyRevealAvailable(key) {
		t.Fatal("expected reveal available via env fallback")
	}
	got, ok := revealAPIKeyPlaintext(key)
	if !ok || got != full {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRevealAPIKeyPlaintext_Encrypted(t *testing.T) {
	const full = "llmproxy_enc_test_key_xyz"
	t.Setenv("LLM_PROXY_API_KEY_STORAGE_SECRET", "unit-test-storage-secret")
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")

	sk := auth.APIKeyStorageKey()
	if sk == nil {
		t.Fatal("storage key nil")
	}
	enc, err := auth.EncryptAPIKeyPlaintext(full, sk)
	if err != nil {
		t.Fatal(err)
	}
	hash, prefix := auth.APIKeyMetadataFromFullKey(full)
	key := &database.APIKey{ID: 2, KeyHash: hash, KeyPrefix: prefix, KeySecretEnc: enc}
	got, ok := revealAPIKeyPlaintext(key)
	if !ok || got != full {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRevealAPIKeyPlaintext_NoMatch(t *testing.T) {
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "llmproxy_other")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_API_KEY_STORAGE_SECRET", "")
	key := &database.APIKey{ID: 3, KeyHash: "deadbeef", KeySecretEnc: ""}
	if apiKeyRevealAvailable(key) {
		t.Fatal("expected not revealable")
	}
	if _, ok := revealAPIKeyPlaintext(key); ok {
		t.Fatal("expected miss")
	}
}
