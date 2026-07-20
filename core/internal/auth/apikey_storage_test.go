package auth

import (
	"strings"
	"testing"
)

func TestAPIKeyRevealOnce(t *testing.T) {
	t.Setenv(envAPIKeyRevealOnce, "")
	if APIKeyRevealOnce() {
		t.Fatal("default should allow secondary reveal")
	}
	t.Setenv(envAPIKeyRevealOnce, "true")
	if !APIKeyRevealOnce() {
		t.Fatal("true should enable reveal-once")
	}
	t.Setenv(envAPIKeyRevealOnce, "1")
	if !APIKeyRevealOnce() {
		t.Fatal("1 should enable reveal-once")
	}
}

func TestAPIKeyStorageKey_Unset(t *testing.T) {
	resetAPIKeyStorageForTest()
	t.Setenv(envAPIKeyStorageSecret, "")
	t.Setenv(envAPIKeyRevealOnce, "")

	key := APIKeyStorageKey()
	if key != nil {
		t.Error("APIKeyStorageKey should return nil when env var is unset and not ensured")
	}
}

func TestAPIKeyStorageKey_RevealOnce(t *testing.T) {
	resetAPIKeyStorageForTest()
	t.Setenv(envAPIKeyStorageSecret, "present-but-ignored")
	t.Setenv(envAPIKeyRevealOnce, "true")
	defer t.Setenv(envAPIKeyRevealOnce, "")

	if APIKeyStorageKey() != nil {
		t.Error("reveal-once mode must not expose storage key")
	}
	enc, err := EncryptAPIKeyForStorage("llmproxy_x")
	if err != nil {
		t.Fatalf("EncryptAPIKeyForStorage: %v", err)
	}
	if enc != "" {
		t.Error("reveal-once should skip encryption")
	}
}

func TestAPIKeyStorageKey_Set(t *testing.T) {
	resetAPIKeyStorageForTest()
	t.Setenv(envAPIKeyRevealOnce, "")
	testSecret := "test_secret_12345"
	t.Setenv(envAPIKeyStorageSecret, testSecret)

	key := APIKeyStorageKey()
	if key == nil {
		t.Fatal("APIKeyStorageKey should not return nil when env var is set")
	}

	if len(key) != 32 {
		t.Errorf("APIKeyStorageKey should return 32-byte key, got: %d bytes", len(key))
	}
}

func TestAPIKeyStorageKey_EmptyString(t *testing.T) {
	resetAPIKeyStorageForTest()
	t.Setenv(envAPIKeyRevealOnce, "")
	t.Setenv(envAPIKeyStorageSecret, "   ")

	key := APIKeyStorageKey()
	if key != nil {
		t.Error("APIKeyStorageKey should return nil for whitespace-only secret")
	}
}

func TestAPIKeyMetadataFromFullKey(t *testing.T) {
	fullKey := "llmproxy_abcdef1234567890"

	keyHash, keyPrefix := APIKeyMetadataFromFullKey(fullKey)

	if keyHash == "" {
		t.Error("keyHash should not be empty")
	}

	expectedHash := SHA256Hex(fullKey)
	if keyHash != expectedHash {
		t.Errorf("keyHash = %s, want %s", keyHash, expectedHash)
	}

	if keyPrefix != "llmproxy_abcdef1" {
		t.Errorf("keyPrefix = %s, want llmproxy_abcdef1", keyPrefix)
	}

	if len(keyPrefix) != 16 {
		t.Errorf("keyPrefix should be 16 characters, got: %d", len(keyPrefix))
	}
}

func TestAPIKeyMetadataFromFullKey_ShortKey(t *testing.T) {
	shortKey := "short"
	keyHash, keyPrefix := APIKeyMetadataFromFullKey(shortKey)

	if keyHash == "" {
		t.Error("keyHash should not be empty")
	}

	if keyPrefix != shortKey {
		t.Errorf("keyPrefix should be the full short key, got: %s", keyPrefix)
	}
}

func TestEncryptDecryptAPIKeyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "llmproxy_test_key_12345"

	encrypted, err := EncryptAPIKeyPlaintext(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptAPIKeyPlaintext failed: %v", err)
	}

	if encrypted == "" {
		t.Error("encrypted should not be empty")
	}

	if encrypted == plaintext {
		t.Error("encrypted should not equal plaintext")
	}

	decrypted, err := DecryptAPIKeyPlaintext(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptAPIKeyPlaintext failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptAPIKeyPlaintext_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)

	_, err := EncryptAPIKeyPlaintext("test", shortKey)
	if err == nil {
		t.Error("EncryptAPIKeyPlaintext should fail with short key")
	}

	if !strings.Contains(err.Error(), "invalid key length") {
		t.Errorf("expected 'invalid key length' error, got: %v", err)
	}
}

func TestDecryptAPIKeyPlaintext_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)

	_, err := DecryptAPIKeyPlaintext("dGVzdA==", shortKey)
	if err == nil {
		t.Error("DecryptAPIKeyPlaintext should fail with short key")
	}
}

func TestDecryptAPIKeyPlaintext_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	_, err := DecryptAPIKeyPlaintext("invalid_base64!", key)
	if err == nil {
		t.Error("DecryptAPIKeyPlaintext should fail with invalid base64")
	}
}

func TestDecryptAPIKeyPlaintext_CiphertextTooShort(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	shortCiphertext := "AAAA" // too short for AES-GCM

	_, err := DecryptAPIKeyPlaintext(shortCiphertext, key)
	if err == nil {
		t.Error("DecryptAPIKeyPlaintext should fail with ciphertext too short")
	}
}

func TestEncryptAPIKeyForStorage_RequiresKey(t *testing.T) {
	resetAPIKeyStorageForTest()
	t.Setenv(envAPIKeyRevealOnce, "")
	t.Setenv(envAPIKeyStorageSecret, "")
	_, err := EncryptAPIKeyForStorage("llmproxy_x")
	if err == nil {
		t.Fatal("expected error when storage not ready")
	}
}
