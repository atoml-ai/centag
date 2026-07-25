package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "test_password_123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty hash")
	}

	if strings.Contains(hash, password) {
		t.Error("Hash should not contain plaintext password")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "test_password_123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	if CheckPassword("wrong_password", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	fullKey, keyHash, keyPrefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if fullKey == "" {
		t.Error("fullKey should not be empty")
	}

	if !strings.HasPrefix(fullKey, "llmproxy_") {
		t.Errorf("fullKey should have prefix 'llmproxy_', got: %s", fullKey)
	}

	if keyHash == "" {
		t.Error("keyHash should not be empty")
	}

	if keyPrefix == "" {
		t.Error("keyPrefix should not be empty")
	}

	wantPrefix := APIKeyDisplayPrefix(fullKey)
	if keyPrefix != wantPrefix {
		t.Errorf("keyPrefix = %q, want display prefix %q", keyPrefix, wantPrefix)
	}
	if !strings.Contains(keyPrefix, "…") {
		t.Errorf("keyPrefix should contain ellipsis, got: %s", keyPrefix)
	}
	if !strings.HasPrefix(fullKey, strings.Split(keyPrefix, "…")[0]) {
		t.Error("keyPrefix head should be a prefix of fullKey")
	}
}

func TestSHA256Hex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}

	for _, tt := range tests {
		result := SHA256Hex(tt.input)
		if result != tt.expected {
			t.Errorf("SHA256Hex(%q) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestSHA256Hex_Consistency(t *testing.T) {
	input := "llmproxy_test"
	result1 := SHA256Hex(input)
	result2 := SHA256Hex(input)

	if result1 != result2 {
		t.Error("SHA256Hex should return consistent results for same input")
	}

	if len(result1) != 64 {
		t.Errorf("SHA256Hex should return 64 character hex string, got: %d", len(result1))
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	if raw == "" {
		t.Error("raw token should not be empty")
	}

	if hash == "" {
		t.Error("hash should not be empty")
	}

	if len(raw) != 96 {
		t.Errorf("raw token should be 96 hex characters (48 bytes), got: %d", len(raw))
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"llmproxy_ab12cdef…9f3a12", "llmproxy_ab12cdef…9f3a12"},
		{"llmproxy_ab12cd", "llmproxy_ab…12cd"},     // len14 → head10…tail4
		{"llmproxy_12345678", "llmproxy_1234…5678"}, // len16 → head12…tail4
		{"12345678", "12345678…"},
	}

	for _, tt := range tests {
		result := MaskAPIKey(tt.prefix)
		if result != tt.want {
			t.Errorf("MaskAPIKey(%q) = %s, want %s", tt.prefix, result, tt.want)
		}
	}
}

func TestAPIKeyDisplayPrefix(t *testing.T) {
	full := "llmproxy_" + strings.Repeat("ab", 32) // 9 + 64
	got := APIKeyDisplayPrefix(full)
	want := full[:32] + "…" + full[len(full)-12:]
	if got != want {
		t.Fatalf("APIKeyDisplayPrefix = %q, want %q", got, want)
	}
}

func TestMaskAPIKey_ShortPrefix(t *testing.T) {
	result := MaskAPIKey("123")
	expected := strings.Repeat("*", 16)
	if result != expected {
		t.Errorf("MaskAPIKey for short prefix should return 16 asterisks, got: %s", result)
	}
}
