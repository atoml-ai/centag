// Package authapi exposes a minimal auth surface for commercial plugins.
// Implementations live in core/internal/auth; plugins must import this package
// instead of centag/core/internal/*.
package authapi

import (
	"centag/core/internal/auth"

	"github.com/gin-gonic/gin"
)

func GetUserID(c *gin.Context) (int64, error) { return auth.GetUserID(c) }

func GetTenantID(c *gin.Context) string { return auth.GetTenantID(c) }

func HashPassword(password string) (string, error) { return auth.HashPassword(password) }

func GenerateAPIKey() (fullKey, keyHash, keyPrefix string, err error) {
	return auth.GenerateAPIKey()
}

func EncryptAPIKeyForStorage(plaintext string) (string, error) {
	return auth.EncryptAPIKeyForStorage(plaintext)
}
