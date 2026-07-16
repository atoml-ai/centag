package auth

import (
	"os"
	"strings"
	"testing"
)

func TestAPIKeyStorageKey_Unset(t *testing.T) {
	os.Unsetenv(envAPIKeyStorageSecret)

	key := APIKeyStorageKey()
	if key != nil {
		t.Error("APIKeyStorageKey should return nil when env var is unset")
	}
}

func TestAPIKeyStorageKey_Set(t *testing.T) {
	testSecret := "test_secret_12345"
	os.Setenv(envAPIKeyStorageSecret, testSecret)
	defer os.Unsetenv(envAPIKeyStorageSecret)

	key := APIKeyStorageKey()
	if key == nil {
		t.Fatal("APIKeyStorageKey should not return nil when env var is set")
	}

	if len(key) != 32 {
		t.Errorf("APIKeyStorageKey should return 32-byte key, got: %d bytes", len(key))
	}
}

func TestAPIKeyStorageKey_EmptyString(t *testing.T) {
	os.Setenv(envAPIKeyStorageSecret, "   ")
	defer os.Unsetenv(envAPIKeyStorageSecret)

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
