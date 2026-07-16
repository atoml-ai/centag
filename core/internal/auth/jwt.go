// Package auth provides JWT-based authentication and API key validation for
// Centag.  The JWT secret is loaded from the system_config table on first
// call to LoadSecret; if no secret exists yet, a random 64-byte secret is
// generated and persisted.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"centag/core/pkg/database"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// AccessTokenTTL is the lifetime of a freshly issued access token.
	AccessTokenTTL = 24 * time.Hour
	// RefreshTokenTTL is the lifetime of a refresh token stored in the DB.
	RefreshTokenTTL = 7 * 24 * time.Hour

	systemKeyJWTSecret = "jwt_secret"
)

// Claims are the JWT payload fields embedded in every access token.
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"sub"`
	Role     string `json:"role"`
	TenantID string `json:"tid,omitempty"` // 多租户：租户 ID
	jwt.RegisteredClaims
}

var (
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenInvalid  = errors.New("token is invalid")
	ErrSecretMissing = errors.New("jwt secret not loaded; call LoadSecret first")
)

// jwtSecret is the active signing key (loaded once at startup).
var jwtSecret []byte

// LoadSecret reads the JWT signing secret from the database.  If no secret
// exists, a fresh random secret is generated and persisted.  This must be
// called before IssueAccessToken or ValidateAccessToken.
func LoadSecret(ctx context.Context) error {
	db := database.Get()
	val, err := db.SystemConfigStore().Get(ctx, systemKeyJWTSecret)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("load jwt secret: %w", err)
	}

	if errors.Is(err, database.ErrNotFound) || val == "" {
		secret, genErr := generateSecret()
		if genErr != nil {
			return fmt.Errorf("generate jwt secret: %w", genErr)
		}
		if setErr := db.SystemConfigStore().Set(ctx, systemKeyJWTSecret, secret); setErr != nil {
			return fmt.Errorf("persist jwt secret: %w", setErr)
		}
		val = secret
	}

	jwtSecret = []byte(val)
	return nil
}

// InitSecret sets the JWT signing secret directly (for minimal / file-based mode without DB).
func InitSecret(secret string) {
	jwtSecret = []byte(secret)
}

// EnsureFileSecret loads a JWT secret from path, or generates and persists one.
func EnsureFileSecret(path string) error {
	if data, err := os.ReadFile(path); err == nil {
		s := strings.TrimSpace(string(data))
		if s != "" {
			jwtSecret = []byte(s)
			return nil
		}
	}
	secret, err := generateSecret()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return err
	}
	jwtSecret = []byte(secret)
	return nil
}

func generateSecret() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IssueAccessToken creates a signed JWT for the given user.
func IssueAccessToken(userID int64, username, role, tenantID string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", ErrSecretMissing
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			Issuer:    "centag",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(jwtSecret)
}

// ValidateAccessToken parses and validates a JWT string, returning the embedded
// Claims on success.
func ValidateAccessToken(tokenStr string) (*Claims, error) {
	if len(jwtSecret) == 0 {
		return nil, ErrSecretMissing
	}
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
