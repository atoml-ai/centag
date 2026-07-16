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
// The prefix is stored in key_prefix for masked display.
func GenerateAPIKey() (fullKey, keyHash, keyPrefix string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	fullKey = "llmproxy_" + hex.EncodeToString(raw)
	keyHash = SHA256Hex(fullKey)
	keyPrefix = fullKey[:16] // first 16 chars for masked display
	return
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

// MaskAPIKey returns a display-safe version of an API key:
//
//	llmproxy_ab12cd** … **
func MaskAPIKey(prefix string) string {
	if len(prefix) < 8 {
		return strings.Repeat("*", 16)
	}
	return prefix + "****…****"
}
