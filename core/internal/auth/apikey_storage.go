package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"centag/core/pkg/database"
)

const (
	envAPIKeyStorageSecret = "LLM_PROXY_API_KEY_STORAGE_SECRET"
	// envAPIKeyRevealOnce disables encrypted at-rest storage so full keys are
	// only available in the create response (legacy one-time display).
	envAPIKeyRevealOnce          = "LLM_PROXY_API_KEY_REVEAL_ONCE"
	systemKeyAPIKeyStorageSecret = "api_key_storage_secret"
)

var (
	apiKeyStorageMu     sync.RWMutex
	apiKeyStorageSecret string // raw secret material (env or persisted)
)

// APIKeyRevealOnce reports whether secondary reveal is disabled.
// Default is false (re-reveal enabled). Set LLM_PROXY_API_KEY_REVEAL_ONCE=true|1|yes|on to opt out.
func APIKeyRevealOnce() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envAPIKeyRevealOnce)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// APIKeyStorageKey returns a 32-byte AES-256 key for encrypting API key plaintext
// at rest, or nil when reveal-once mode is on / storage has not been ensured yet.
func APIKeyStorageKey() []byte {
	if APIKeyRevealOnce() {
		return nil
	}
	if s := strings.TrimSpace(os.Getenv(envAPIKeyStorageSecret)); s != "" {
		sum := sha256.Sum256([]byte(s))
		return sum[:]
	}
	apiKeyStorageMu.RLock()
	defer apiKeyStorageMu.RUnlock()
	if apiKeyStorageSecret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(apiKeyStorageSecret))
	return sum[:]
}

// EnsureAPIKeyStorage loads or generates a persistent storage secret so newly
// created keys can be revealed again in the Web UI. No-op when reveal-once is set.
// Prefer LLM_PROXY_API_KEY_STORAGE_SECRET when set; otherwise use system_config.
func EnsureAPIKeyStorage(ctx context.Context) error {
	if APIKeyRevealOnce() {
		apiKeyStorageMu.Lock()
		apiKeyStorageSecret = ""
		apiKeyStorageMu.Unlock()
		return nil
	}
	if s := strings.TrimSpace(os.Getenv(envAPIKeyStorageSecret)); s != "" {
		apiKeyStorageMu.Lock()
		apiKeyStorageSecret = s
		apiKeyStorageMu.Unlock()
		return nil
	}

	db := database.Get()
	if db == nil {
		return fmt.Errorf("api key storage: database not initialised")
	}
	val, err := db.SystemConfigStore().Get(ctx, systemKeyAPIKeyStorageSecret)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("load api key storage secret: %w", err)
	}
	if errors.Is(err, database.ErrNotFound) || strings.TrimSpace(val) == "" {
		secret, genErr := generateAPIKeyStorageSecret()
		if genErr != nil {
			return fmt.Errorf("generate api key storage secret: %w", genErr)
		}
		if setErr := db.SystemConfigStore().Set(ctx, systemKeyAPIKeyStorageSecret, secret); setErr != nil {
			return fmt.Errorf("persist api key storage secret: %w", setErr)
		}
		val = secret
	}

	apiKeyStorageMu.Lock()
	apiKeyStorageSecret = strings.TrimSpace(val)
	apiKeyStorageMu.Unlock()
	return nil
}

// EnsureFileAPIKeyStorage loads or generates a storage secret from path
// (minimal / file-based mode without DB system_config).
func EnsureFileAPIKeyStorage(path string) error {
	if APIKeyRevealOnce() {
		apiKeyStorageMu.Lock()
		apiKeyStorageSecret = ""
		apiKeyStorageMu.Unlock()
		return nil
	}
	if s := strings.TrimSpace(os.Getenv(envAPIKeyStorageSecret)); s != "" {
		apiKeyStorageMu.Lock()
		apiKeyStorageSecret = s
		apiKeyStorageMu.Unlock()
		return nil
	}
	if data, err := os.ReadFile(path); err == nil {
		s := strings.TrimSpace(string(data))
		if s != "" {
			apiKeyStorageMu.Lock()
			apiKeyStorageSecret = s
			apiKeyStorageMu.Unlock()
			return nil
		}
	}
	secret, err := generateAPIKeyStorageSecret()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return err
	}
	apiKeyStorageMu.Lock()
	apiKeyStorageSecret = secret
	apiKeyStorageMu.Unlock()
	return nil
}

// EncryptAPIKeyForStorage encrypts plaintext when secondary reveal is enabled.
// In reveal-once mode it returns ("", nil).
func EncryptAPIKeyForStorage(plaintext string) (string, error) {
	if APIKeyRevealOnce() {
		return "", nil
	}
	sk := APIKeyStorageKey()
	if sk == nil {
		return "", fmt.Errorf("api key storage not ready; call EnsureAPIKeyStorage first or set %s", envAPIKeyStorageSecret)
	}
	return EncryptAPIKeyPlaintext(plaintext, sk)
}

func generateAPIKeyStorageSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// resetAPIKeyStorageForTest clears cached storage state (unit tests only).
func resetAPIKeyStorageForTest() {
	apiKeyStorageMu.Lock()
	apiKeyStorageSecret = ""
	apiKeyStorageMu.Unlock()
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
