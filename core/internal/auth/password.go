package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword returns a bcrypt hash of password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateAPIKey returns a new random API key and its SHA-256 hash.
// The key has the form:  llmproxy_<32 random hex bytes>
// key_prefix stores a display mask: llmproxy_xxxxxxxx…xxxxxx (head…tail).
func GenerateAPIKey() (fullKey, keyHash, keyPrefix string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	fullKey = "llmproxy_" + hex.EncodeToString(raw)
	keyHash = SHA256Hex(fullKey)
	keyPrefix = APIKeyDisplayPrefix(fullKey)
	return
}

// APIKeyDisplayPrefix returns a list-safe identifier: llmproxy_xxxxxxxx…xxxxxx.
const (
	apiKeyDisplayHead = 32 // "llmproxy_" + 23 hex — 列表脱敏尽量铺满列宽
	apiKeyDisplayTail = 12
)

func APIKeyDisplayPrefix(fullKey string) string {
	if len(fullKey) < apiKeyDisplayHead+apiKeyDisplayTail+1 {
		if len(fullKey) < 8 {
			return fullKey
		}
		return MaskAPIKey(fullKey)
	}
	return fullKey[:apiKeyDisplayHead] + "…" + fullKey[len(fullKey)-apiKeyDisplayTail:]
}

// SHA256Hex returns the lowercase hex-encoded SHA-256 digest of s.
func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// GenerateRefreshToken returns a random opaque token and its SHA-256 hash.
func GenerateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 48)
	if _, err = rand.Read(b); err != nil {
		return
	}
	raw = hex.EncodeToString(b)
	hash = SHA256Hex(raw)
	return
}

// MaskAPIKey returns a display-safe version of an API key / stored prefix:
//
//	llmproxy_xxxxxxxx…xxxxxx
//
// Already-masked prefixes (containing "…") are returned as-is. Legacy short
// prefixes (first 16 chars only) are reshaped into head…tail form.
func MaskAPIKey(prefix string) string {
	if prefix == "" {
		return strings.Repeat("*", 16)
	}
	if strings.Contains(prefix, "…") || strings.Contains(prefix, "...") {
		return prefix
	}
	if len(prefix) < 8 {
		return strings.Repeat("*", 16)
	}
	if len(prefix) >= apiKeyDisplayHead+apiKeyDisplayTail {
		return prefix[:apiKeyDisplayHead] + "…" + prefix[len(prefix)-apiKeyDisplayTail:]
	}
	// Legacy short prefixes (often first 16 chars): keep both ends when possible.
	tail := 4
	if len(prefix) < 12 {
		return prefix + "…"
	}
	head := len(prefix) - tail
	if head < 8 {
		head = 8
	}
	if head+tail >= len(prefix) {
		head = len(prefix) - tail
	}
	return prefix[:head] + "…" + prefix[len(prefix)-tail:]
}
