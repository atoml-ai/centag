package authapi

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTripShape(t *testing.T) {
	hash, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "secret-pass" {
		t.Fatalf("expected non-empty hashed value, got %q", hash)
	}
}

func TestGenerateAPIKey_PrefixAndParts(t *testing.T) {
	full, hash, prefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if full == "" || hash == "" || prefix == "" {
		t.Fatalf("empty parts: full=%q hash=%q prefix=%q", full, hash, prefix)
	}
	if !strings.HasPrefix(full, "llmproxy_") {
		t.Fatalf("full key %q should start with llmproxy_", full)
	}
	// keyPrefix is a list-safe display mask (head…tail), not a literal prefix of full.
	if !strings.Contains(prefix, "…") {
		t.Fatalf("prefix %q should contain ellipsis", prefix)
	}
	head, _, ok := strings.Cut(prefix, "…")
	if !ok || !strings.HasPrefix(full, head) {
		t.Fatalf("prefix head %q should be a prefix of full key %q", head, full)
	}
	if full == hash {
		t.Fatal("full key must differ from storage hash")
	}
}

func TestEncryptAPIKeyForStorage_RequiresStorageReady(t *testing.T) {
	_, err := EncryptAPIKeyForStorage("llmproxy_test_key")
	if err == nil {
		t.Fatal("expected error when storage secret is not initialized")
	}
}
