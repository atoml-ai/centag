package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// AuthConfig holds optional dependencies for ProxyAuthMiddleware.
type AuthConfig struct {
	RateLimiter RateLimiter
	IsDesktop   bool // true for SQLite (Desktop Edition) — skips rate/budget/model checks
}

// JWTMiddleware validates the Bearer token from the Authorization header and
// sets user context values.  Returns 401 on missing / invalid tokens.
func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearerToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "authentication required",
			})
			return
		}

		claims, err := ValidateAccessToken(tokenStr)
		if err != nil {
			status := http.StatusUnauthorized
			msg := "invalid token"
			if err == ErrTokenExpired {
				msg = "token expired"
			}
			c.AbortWithStatusJSON(status, gin.H{"success": false, "error": msg})
			return
		}

		SetUserContext(c, claims)
		c.Next()
	}
}

// AdminOnlyMiddleware ensures the authenticated user has the admin role.
// Must be used after JWTMiddleware.
func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "administrator access required",
			})
			return
		}
		c.Next()
	}
}

// OptionalJWTMiddleware attempts to validate the Bearer token if present, but
// does not abort on missing tokens.  Useful for endpoints that behave
// differently depending on auth state.
func OptionalJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearerToken(c)
		if tokenStr != "" {
			if claims, err := ValidateAccessToken(tokenStr); err == nil {
				SetUserContext(c, claims)
			}
		}
		c.Next()
	}
}

func extractBearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	// Also accept token in query param for WebSocket / download use-cases.
	return c.Query("token")
}

// ExtractBearerToken is the exported form of extractBearerToken for other packages.
func ExtractBearerToken(c *gin.Context) string {
	return extractBearerToken(c)
}

// apiKeyPrefix is the well-known prefix for all LLM-Proxy API keys.
const apiKeyPrefix = "llmproxy_"

// ProxyAuthMiddleware authenticates incoming proxy requests (e.g. /v1/chat/completions).
// It accepts both JWT Bearer tokens and llmproxy_* API keys.
// Anonymous requests (no Bearer / ?token=) are rejected with 401.
//
// Authentication priority:
//  1. Bearer or ?token= starting with "llmproxy_" → validate as API key
//  2. Any other non-empty token → validate as JWT
//  3. Empty → 401
//
// When cfg is non-nil and not desktop (IsDesktop=false), the middleware also
// enforces model whitelist, rate limits, and budget caps for API-key requests.
func ProxyAuthMiddleware(cfg *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "需要认证：请在 Authorization 使用 Bearer（JWT 或 llmproxy_* API Key），或查询参数 token=",
			})
			return
		}

		if strings.HasPrefix(token, apiKeyPrefix) {
			// ── API Key path ─────────────────────────────────────────────
			hash := SHA256Hex(token)
			db := database.Get()
			key, err := db.APIKeyStore().GetByHash(c.Request.Context(), hash)
			if err != nil {
				logger.Warnf("Proxy auth rejected: invalid API key (path=%s, client=%s)", c.Request.URL.Path, c.ClientIP())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "无效的 API key",
				})
				return
			}
			if !key.Enabled {
				logger.Warnf("Proxy auth rejected: API key disabled (path=%s)", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "API key 已禁用",
				})
				return
			}
			if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
				logger.Warnf("Proxy auth rejected: API key expired (path=%s)", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "API key 已过期",
				})
				return
			}

			// ── Virtual key checks (Team edition only) ────────────────
			if cfg != nil && !cfg.IsDesktop {
				if !checkVirtualKeyChecks(c, key, cfg.RateLimiter, hash) {
					return
				}
			}

			// Look up owner to set full user context.
			user, err := db.UserStore().GetByID(c.Request.Context(), key.UserID)
			if err == nil {
				c.Set(CtxKeyUserID, user.ID)
				c.Set(CtxKeyUsername, user.Username)
				c.Set(CtxKeyRole, string(user.Role))
				if user.TenantID != nil {
					c.Set(CtxKeyTenantID, *user.TenantID)
				}
			} else {
				// At minimum store the user ID so downstream code can use it.
				c.Set(CtxKeyUserID, key.ID)
				if key.TenantID != nil {
					c.Set(CtxKeyTenantID, *key.TenantID)
				}
			}

			// Update last_used_at asynchronously to not block the request.
			keyID := key.ID
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = db.APIKeyStore().UpdateLastUsed(ctx, keyID, time.Now())
			}()

			c.Next()
			return
		}

		// ── JWT path ─────────────────────────────────────────────────────
		claims, err := ValidateAccessToken(token)
		if err != nil {
			// Invalid JWT but there was an attempt — reject rather than silently pass.
			// Common case: client sent a third-party API key (e.g. OpenAI/百炼 key) as Bearer token;
			// we only accept llmproxy_* keys or valid JWTs here.
			status := http.StatusUnauthorized
			msg := "invalid token"
			if err == ErrTokenExpired {
				msg = "token expired"
			}
			logger.Warnf("Proxy auth rejected: %s (path=%s, client=%s); tip: Bearer llmproxy_* or valid JWT", msg, c.Request.URL.Path, c.ClientIP())
			c.AbortWithStatusJSON(status, gin.H{"success": false, "error": msg})
			return
		}
		SetUserContext(c, claims)
		c.Next()
	}
}

// checkVirtualKeyChecks performs model whitelist, rate limit, and budget
// checks for virtual key requests. Returns false if the request should be aborted.
func checkVirtualKeyChecks(c *gin.Context, key *database.APIKey, limiter RateLimiter, keyHash string) bool {
	// ── 1. Model whitelist ─────────────────────────────────────────
	if key.ModelWhitelist != "" && key.ModelWhitelist != "*" {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Warnf("Virtual key check: failed to read body for model check (key=%d)", key.ID)
		} else {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var chatReq struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(bodyBytes, &chatReq); err == nil && chatReq.Model != "" {
				if !isModelAllowed(chatReq.Model, key.ModelWhitelist) {
					logger.Warnf("Virtual key check: model %q not in whitelist (key=%d)", chatReq.Model, key.ID)
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"success": false,
						"error":   "model not allowed for this API key",
					})
					return false
				}
			}
			// Restore body after peek.
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	// ── 2. Rate limit ────────────────────────────────────────
	if limiter != nil && (key.RateLimitRPM > 0 || key.RateLimitTPM > 0) {
		ok, remRPM, remTPM, err := limiter.Allow(c.Request.Context(), keyHash, key.RateLimitRPM, key.RateLimitTPM)
		c.Header("X-RateLimit-RPM-Remaining", strconv.FormatInt(int64(remRPM), 10))
		c.Header("X-RateLimit-TPM-Remaining", strconv.FormatInt(int64(remTPM), 10))
		if err != nil {
			logger.Warnf("Virtual key check: rate limiter error (%v), allowing (key=%d)", err, key.ID)
		} else if !ok {
			logger.Warnf("Virtual key check: rate limit exceeded (key=%d, rpm=%d, tpm=%d)", key.ID, key.RateLimitRPM, key.RateLimitTPM)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "rate limit exceeded",
			})
			return false
		}
	}

	// ── 3. Budget check ──────────────────────────────────────────
	if key.BudgetUSD > 0 {
		result := (BudgetChecker{}).Check(key.UsedUSD, key.BudgetUSD)
		c.Header("X-Budget-Remaining", strconv.FormatFloat(result.Remaining, 'f', -1, 64))
		if !result.OK {
			logger.Warnf("Virtual key check: budget exhausted (key=%d, used=%.4f, budget=%.4f)", key.ID, key.UsedUSD, key.BudgetUSD)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "budget exhausted",
			})
			return false
		}
	}

	return true
}

// isModelAllowed checks whether the requested model is in the whitelist.
// The whitelist is stored as a JSON array string, e.g. `["gpt-4","gpt-3.5-turbo"]`.
func isModelAllowed(model, whitelist string) bool {
	var models []string
	if err := json.Unmarshal([]byte(whitelist), &models); err != nil {
		// Fallback: treat as comma-separated.
		for _, m := range strings.Split(whitelist, ",") {
			if strings.TrimSpace(m) == model {
				return true
			}
		}
		return false
	}
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}
