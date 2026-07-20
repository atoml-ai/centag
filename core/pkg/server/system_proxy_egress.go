package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// SystemProxyEgressKeyName is the dedicated API key name for MITM egress injection.
const SystemProxyEgressKeyName = "system-proxy-egress"

// EnsureSystemProxyEgressAPIKey ensures cfg.SystemProxy.EgressAPIKey is set so MITM
// can inject a Centag llmproxy_* key. PreferUserID owns the key when > 0; otherwise
// the bootstrap admin user is used. Returns whether cfg was mutated (caller should SaveConfig).
func EnsureSystemProxyEgressAPIKey(ctx context.Context, cfg *config.Config, preferUserID int64) (changed bool, err error) {
	if cfg == nil {
		return false, fmt.Errorf("nil config")
	}
	if resolved := config.ResolveSystemProxyEgressAPIKey(&cfg.SystemProxy); resolved != "" {
		// Persist env-resolved key into config so Web shows "configured" and survives without env.
		if strings.TrimSpace(cfg.SystemProxy.EgressAPIKey) == "" {
			cfg.SystemProxy.EgressAPIKey = resolved
			return true, nil
		}
		return false, nil
	}

	db := database.Get()
	if db == nil {
		return false, fmt.Errorf("database not initialized")
	}

	userID, err := resolveEgressOwnerUserID(ctx, db, preferUserID)
	if err != nil {
		return false, err
	}

	// Reuse existing named key if decryptable.
	if plain, ok := findDecryptableKeyByName(ctx, db, userID, SystemProxyEgressKeyName); ok {
		cfg.SystemProxy.EgressAPIKey = plain
		logger.Infof("system_proxy: bound existing egress API key name=%q user_id=%d", SystemProxyEgressKeyName, userID)
		return true, nil
	}

	fullKey, err := createSystemProxyEgressKey(ctx, db, userID)
	if err != nil {
		return false, err
	}
	cfg.SystemProxy.EgressAPIKey = fullKey
	logger.Infof("system_proxy: created egress API key name=%q user_id=%d", SystemProxyEgressKeyName, userID)
	return true, nil
}

// BindSystemProxyEgressAPIKeyByID decrypts the given API key and writes it into cfg.
func BindSystemProxyEgressAPIKeyByID(ctx context.Context, cfg *config.Config, keyID int64) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if keyID <= 0 {
		return fmt.Errorf("invalid api key id")
	}
	db := database.Get()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	key, err := db.APIKeyStore().GetByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	if !key.Enabled {
		return fmt.Errorf("api key is disabled")
	}
	plain, err := decryptAPIKeyPlain(key)
	if err != nil {
		return err
	}
	cfg.SystemProxy.EgressAPIKey = plain
	return nil
}

func resolveEgressOwnerUserID(ctx context.Context, db *database.Manager, preferUserID int64) (int64, error) {
	if preferUserID > 0 {
		return preferUserID, nil
	}
	admin, err := db.UserStore().GetByUsername(ctx, bootstrap.AdminUsername())
	if err != nil {
		return 0, fmt.Errorf("resolve admin user for egress key: %w", err)
	}
	return admin.ID, nil
}

func findDecryptableKeyByName(ctx context.Context, db *database.Manager, userID int64, name string) (string, bool) {
	keys, err := db.APIKeyStore().ListByUserID(ctx, userID)
	if err != nil {
		return "", false
	}
	for _, k := range keys {
		if k == nil || !k.Enabled || k.Name != name {
			continue
		}
		plain, err := decryptAPIKeyPlain(k)
		if err != nil {
			continue
		}
		return plain, true
	}
	return "", false
}

func decryptAPIKeyPlain(key *database.APIKey) (string, error) {
	if key == nil {
		return "", fmt.Errorf("nil api key")
	}
	sk := auth.APIKeyStorageKey()
	if key.KeySecretEnc == "" || sk == nil {
		return "", fmt.Errorf("api key id=%d has no recoverable plaintext (storage secret missing or reveal-once mode)", key.ID)
	}
	plain, err := auth.DecryptAPIKeyPlaintext(key.KeySecretEnc, sk)
	if err != nil {
		return "", fmt.Errorf("decrypt api key id=%d: %w", key.ID, err)
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", fmt.Errorf("empty plaintext for api key id=%d", key.ID)
	}
	return plain, nil
}

func createSystemProxyEgressKey(ctx context.Context, db *database.Manager, userID int64) (string, error) {
	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", fmt.Errorf("generate egress api key: %w", err)
	}

	user, err := db.UserStore().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user %d: %w", userID, err)
	}

	rec := &database.APIKey{
		UserID:    userID,
		TenantID:  user.TenantID,
		Name:      SystemProxyEgressKeyName,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}
	enc, encErr := auth.EncryptAPIKeyForStorage(fullKey)
	if encErr != nil {
		return "", fmt.Errorf("encrypt egress api key: %w", encErr)
	}
	rec.KeySecretEnc = enc
	if enc == "" && auth.APIKeyRevealOnce() {
		logger.Warn("system_proxy: created egress key in reveal-once mode; key stored only in system_proxy.egress_api_key")
	}
	if err := db.APIKeyStore().Create(ctx, rec); err != nil {
		return "", fmt.Errorf("create egress api key: %w", err)
	}
	return fullKey, nil
}
