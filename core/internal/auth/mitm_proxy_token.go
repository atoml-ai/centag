package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/database"
)

// ValidateMITMProxyToken validates a client credential used for MITM Proxy-Authorization.
// Accepts llmproxy_* API keys (preferred for employees) or a valid JWT access token.
// Does not accept empty tokens. Does not treat the shared egress key specially —
// any valid personal/admin llmproxy_* key works.
func ValidateMITMProxyToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("empty token")
	}
	if strings.HasPrefix(token, apiKeyPrefix) {
		db := database.Get()
		if db == nil {
			return fmt.Errorf("database not initialized")
		}
		hash := SHA256Hex(token)
		key, err := db.APIKeyStore().GetByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("invalid API key")
		}
		if !key.Enabled {
			return fmt.Errorf("API key disabled")
		}
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			return fmt.Errorf("API key expired")
		}
		return nil
	}
	if _, err := ValidateAccessToken(token); err != nil {
		if err == ErrTokenExpired {
			return fmt.Errorf("token expired")
		}
		return fmt.Errorf("invalid token")
	}
	return nil
}
