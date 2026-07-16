package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const envAPIKeyStorageSecret = "LLM_PROXY_API_KEY_STORAGE_SECRET"

// APIKeyStorageKey returns a 32-byte AES-256 key derived from
// LLM_PROXY_API_KEY_STORAGE_SECRET (SHA-256 of the UTF-8 secret), or nil if unset.
// When nil, API keys are not stored in a revealable form (legacy one-time display only).
func APIKeyStorageKey() []byte {
	s := strings.TrimSpace(os.Getenv(envAPIKeyStorageSecret))
	if s == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// APIKeyMetadataFromFullKey derives hash and display prefix for any full API key string.
func APIKeyMetadataFromFullKey(fullKey string) (keyHash, keyPrefix string) {
	keyHash = SHA256Hex(fullKey)
	if len(fullKey) >= 16 {
		keyPrefix = fullKey[:16]
	} else {
		keyPrefix = fullKey
	}
	return keyHash, keyPrefix
}

// EncryptAPIKeyPlaintext encrypts the full API key for at-rest storage (AES-GCM).
func EncryptAPIKeyPlaintext(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("api key storage: invalid key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKeyPlaintext decrypts a value produced by EncryptAPIKeyPlaintext.
func DecryptAPIKeyPlaintext(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("api key storage: invalid key length")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("api key storage: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
