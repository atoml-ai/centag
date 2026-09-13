// MCP observation endpoint dedicated token management.  A long-lived JWT is
// issued on demand (admin settings page); only its SHA-256 hash is stored in
// system_config so the raw token never touches the database.  Deleting the
// stored hash revokes the token immediately.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
)

const systemKeyMCPTokenHash = "mcp_token_hash"

// readStoredHash returns the stored token hash, unwrapping the JSON string
// quoting applied by SystemConfigStore.Set.
func readStoredHash(ctx context.Context) (string, error) {
	raw, err := database.Get().SystemConfigStore().Get(ctx, systemKeyMCPTokenHash)
	if err != nil {
		return "", err
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "\"") {
		var s string
		if json.Unmarshal([]byte(raw), &s) == nil {
			return s, nil
		}
	}
	return raw, nil
}

// IssueMCPObservationToken generates a new long-lived MCP token, stores its
// SHA-256 hash in system_config (replacing any previous token, thereby
// revoking it), and returns the raw token string.
func (h *ConfigHandler) IssueMCPObservationToken(ctx context.Context, username string) (string, error) {
	tok, err := auth.IssueMCPToken(username)
	if err != nil {
		return "", err
	}
	hash := hashToken(tok)
	if err := database.Get().SystemConfigStore().Set(ctx, systemKeyMCPTokenHash, hash); err != nil {
		return "", err
	}
	return tok, nil
}

// MCPObservationTokenActive reports whether a dedicated MCP token exists.
func (h *ConfigHandler) MCPObservationTokenActive(ctx context.Context) bool {
	_, err := database.Get().SystemConfigStore().Get(ctx, systemKeyMCPTokenHash)
	return err == nil
}

// RevokeMCPObservationToken deletes the stored hash; the previously issued
// token stops working immediately.
func (h *ConfigHandler) RevokeMCPObservationToken(ctx context.Context) error {
	err := database.Get().SystemConfigStore().Delete(ctx, systemKeyMCPTokenHash)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	return err
}

// ValidateMCPObservationToken checks a Bearer token against the stored hash.
// Returns false when no dedicated token has been issued or on any mismatch.
func (h *ConfigHandler) ValidateMCPObservationToken(raw string) bool {
	if raw == "" {
		return false
	}
	// Reject ordinary access tokens: a dedicated MCP token must carry the
	// purpose claim, keeping the two credential types non-interchangeable.
	claims, err := auth.ValidateAccessToken(raw)
	if err != nil || claims.Purpose != "mcp" {
		return false
	}
	stored, err := readStoredHash(context.Background())
	if err != nil || stored == "" {
		return false
	}
	return stored == hashToken(raw)
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}
