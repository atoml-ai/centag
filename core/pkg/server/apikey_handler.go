package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// APIKeyHandler manages user self-service API keys (/api/v1/user/apikeys*).
// Team admin key management lives in centag-pro/internal/teamadmin (E2R).
type APIKeyHandler struct{}

func NewAPIKeyHandler() *APIKeyHandler { return &APIKeyHandler{} }

// ── GET /api/v1/user/apikeys ─────────────────────────────────────────────────

func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	keys, err := database.Get().APIKeyStore().ListByUserID(c.Request.Context(), userID)
	if err != nil {
		logger.Errorf("list api keys user %d: %v", userID, err)
		RespondInternalError(c, "failed to list API keys")
		return
	}

	RespondSuccess(c, ToAPIKeyResponses(keys))
}

// ── GET /api/v1/user/apikeys/:id ─────────────────────────────────────────────

func (h *APIKeyHandler) GetAPIKey(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	ctx := c.Request.Context()
	db := database.Get()
	key, err := db.APIKeyStore().GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to get API key")
		}
		return
	}
	if key.UserID != userID {
		RespondError(c, http.StatusForbidden, "not your API key")
		return
	}

	resp := APIKeyDetailResponse{
		APIKeyResponse: ToAPIKeyResponse(key),
	}
	if plain, ok := RevealAPIKeyPlaintext(key); ok {
		resp.FullKey = plain
		resp.RevealAvailable = true
	}

	RespondSuccess(c, resp)
}

// ── POST /api/v1/user/apikeys ────────────────────────────────────────────────

type createAPIKeyRequest struct {
	Name           string  `json:"name"       binding:"required"`
	ExpiresIn      *int    `json:"expires_in"` // days; nil = no expiry
	BudgetUSD      float64 `json:"budget_usd"`
	RateLimitRPM   int     `json:"rate_limit_rpm"`
	RateLimitTPM   int     `json:"rate_limit_tpm"`
	ModelWhitelist string  `json:"model_whitelist"`
}

func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	existing, err := database.Get().APIKeyStore().ListByUserID(c.Request.Context(), userID)
	if err != nil {
		logger.Errorf("check duplicate api key name user %d: %v", userID, err)
		RespondInternalError(c, "failed to create API key")
		return
	}
	if userAPIKeyNameConflict(existing, req.Name, 0) {
		RespondError(c, http.StatusBadRequest, "API key name already exists")
		return
	}

	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		RespondInternalError(c, "failed to generate API key")
		return
	}

	key := &database.APIKey{
		UserID:         userID,
		Name:           req.Name,
		KeyHash:        keyHash,
		KeyPrefix:      keyPrefix,
		Enabled:        true,
		BudgetUSD:      req.BudgetUSD,
		RateLimitRPM:   req.RateLimitRPM,
		RateLimitTPM:   req.RateLimitTPM,
		ModelWhitelist: req.ModelWhitelist,
	}

	tenantID := auth.GetTenantID(c)
	if tenantID != "" {
		key.TenantID = &tenantID
	}

	enc, encErr := auth.EncryptAPIKeyForStorage(fullKey)
	if encErr != nil {
		logger.Errorf("encrypt api key user %d: %v", userID, encErr)
		RespondInternalError(c, "failed to persist API key")
		return
	}
	key.KeySecretEnc = enc
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		key.ExpiresAt = &t
	}

	if err := database.Get().APIKeyStore().Create(c.Request.Context(), key); err != nil {
		logger.Errorf("create api key user %d: %v", userID, err)
		RespondInternalError(c, "failed to create API key")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": APIKeyCreateResponse{
			APIKeyResponse: ToAPIKeyResponse(key),
			FullKey:        fullKey,
		},
	})
}

// ── PUT /api/v1/user/apikeys/:id ─────────────────────────────────────────────

type updateAPIKeyRequest struct {
	Name           *string  `json:"name"`
	Enabled        *bool    `json:"enabled"`
	BudgetUSD      *float64 `json:"budget_usd"`
	RateLimitRPM   *int     `json:"rate_limit_rpm"`
	RateLimitTPM   *int     `json:"rate_limit_tpm"`
	ModelWhitelist *string  `json:"model_whitelist"`
}

func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()
	key, err := db.APIKeyStore().GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to get API key")
		}
		return
	}
	if key.UserID != userID {
		RespondError(c, http.StatusForbidden, "not your API key")
		return
	}

	var req updateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if req.Name != nil {
		existing, listErr := db.APIKeyStore().ListByUserID(ctx, userID)
		if listErr != nil {
			RespondInternalError(c, "failed to update API key")
			return
		}
		if userAPIKeyNameConflict(existing, *req.Name, key.ID) {
			RespondError(c, http.StatusBadRequest, "API key name already exists")
			return
		}
		key.Name = *req.Name
	}
	if req.Enabled != nil {
		key.Enabled = *req.Enabled
	}
	if req.BudgetUSD != nil {
		key.BudgetUSD = *req.BudgetUSD
	}
	if req.RateLimitRPM != nil {
		key.RateLimitRPM = *req.RateLimitRPM
	}
	if req.RateLimitTPM != nil {
		key.RateLimitTPM = *req.RateLimitTPM
	}
	if req.ModelWhitelist != nil {
		key.ModelWhitelist = *req.ModelWhitelist
	}

	if err := db.APIKeyStore().Update(ctx, key); err != nil {
		RespondInternalError(c, "failed to update API key")
		return
	}

	RespondSuccess(c, ToAPIKeyResponse(key))
}

// ── DELETE /api/v1/user/apikeys/:id ─────────────────────────────────────────

func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()

	key, err := db.APIKeyStore().GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to get API key")
		}
		return
	}
	if key.UserID != userID {
		RespondError(c, http.StatusForbidden, "not your API key")
		return
	}

	if err := db.APIKeyStore().Delete(ctx, keyID); err != nil {
		logger.Errorf("delete api key %d user %d: %v", keyID, userID, err)
		RespondInternalError(c, "failed to delete API key")
		return
	}

	RespondSuccessWithMessage(c, "API key deleted")
}

// ── response helpers (exported for teamadmin) ────────────────────────────────

// APIKeyResponse is the public JSON shape for API key records.
type APIKeyResponse struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	KeyPrefix       string  `json:"key_prefix"`
	MaskedKey       string  `json:"masked_key"`
	RevealAvailable bool    `json:"reveal_available"`
	Enabled         bool    `json:"enabled"`
	TenantID        string  `json:"tenant_id,omitempty"`
	BudgetUSD       float64 `json:"budget_usd"`
	UsedUSD         float64 `json:"used_usd"`
	RateLimitRPM    int     `json:"rate_limit_rpm"`
	RateLimitTPM    int     `json:"rate_limit_tpm"`
	ModelWhitelist  string  `json:"model_whitelist"`
	ExpiresAt       *string `json:"expires_at"`
	LastUsedAt      *string `json:"last_used_at"`
	CreatedAt       string  `json:"created_at"`
}

// Deprecated: use APIKeyResponse.
type apiKeyResponse = APIKeyResponse

// APIKeyDetailResponse includes an optional full key reveal.
type APIKeyDetailResponse struct {
	APIKeyResponse
	FullKey string `json:"full_key,omitempty"`
}

// Deprecated: use APIKeyDetailResponse.
type apiKeyDetailResponse = APIKeyDetailResponse

// APIKeyCreateResponse is returned on create (always includes FullKey once).
type APIKeyCreateResponse struct {
	APIKeyResponse
	FullKey string `json:"full_key"`
}

// Deprecated: use APIKeyCreateResponse.
type apiKeyCreateResponse = APIKeyCreateResponse

// RevealAPIKeyPlaintext returns the full key when encrypted storage is available,
// or when the record matches the bootstrap default key still present in env.
func RevealAPIKeyPlaintext(key *database.APIKey) (string, bool) {
	return revealAPIKeyPlaintext(key)
}

func revealAPIKeyPlaintext(key *database.APIKey) (string, bool) {
	if key == nil {
		return "", false
	}
	if sk := auth.APIKeyStorageKey(); key.KeySecretEnc != "" && sk != nil {
		plain, err := auth.DecryptAPIKeyPlaintext(key.KeySecretEnc, sk)
		if err != nil {
			logger.Warnf("decrypt api key id=%d: %v", key.ID, err)
		} else if plain != "" {
			return plain, true
		}
	}
	if raw := bootstrap.DefaultAdminAPIKeyString(); raw != "" {
		hash, _ := auth.APIKeyMetadataFromFullKey(raw)
		if hash == key.KeyHash {
			return raw, true
		}
	}
	return "", false
}

func apiKeyRevealAvailable(k *database.APIKey) bool {
	if k == nil {
		return false
	}
	_, ok := revealAPIKeyPlaintext(k)
	return ok
}

// ToAPIKeyResponse maps a DB key to the API DTO.
func ToAPIKeyResponse(k *database.APIKey) APIKeyResponse {
	return toAPIKeyResponse(k)
}

func maskedAPIKeyForList(k *database.APIKey) string {
	// Prefer full-key head…tail when ciphertext is available (legacy rows only store 16-char prefix).
	if plain, ok := revealAPIKeyPlaintext(k); ok {
		return auth.APIKeyDisplayPrefix(plain)
	}
	return auth.MaskAPIKey(k.KeyPrefix)
}

func toAPIKeyResponse(k *database.APIKey) APIKeyResponse {
	r := APIKeyResponse{
		ID:              k.ID,
		Name:            k.Name,
		KeyPrefix:       k.KeyPrefix,
		MaskedKey:       maskedAPIKeyForList(k),
		RevealAvailable: apiKeyRevealAvailable(k),
		Enabled:         k.Enabled,
		BudgetUSD:       k.BudgetUSD,
		UsedUSD:         k.UsedUSD,
		RateLimitRPM:    k.RateLimitRPM,
		RateLimitTPM:    k.RateLimitTPM,
		ModelWhitelist:  k.ModelWhitelist,
		CreatedAt:       k.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if k.TenantID != nil {
		r.TenantID = *k.TenantID
	}
	if k.ExpiresAt != nil {
		s := k.ExpiresAt.Format("2006-01-02 15:04:05")
		r.ExpiresAt = &s
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.Format("2006-01-02 15:04:05")
		r.LastUsedAt = &s
	}
	return r
}

// ToAPIKeyResponses maps a list of DB keys.
func ToAPIKeyResponses(keys []*database.APIKey) []APIKeyResponse {
	return toAPIKeyResponses(keys)
}

func toAPIKeyResponses(keys []*database.APIKey) []APIKeyResponse {
	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toAPIKeyResponse(k))
	}
	return resp
}

// userAPIKeyNameConflict reports whether any of the user's API keys already uses
// the given name (case-insensitive, trimmed). excludeID skips the key being updated.
func userAPIKeyNameConflict(keys []*database.APIKey, name string, excludeID int64) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, k := range keys {
		if k == nil || k.ID == excludeID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k.Name), name) {
			return true
		}
	}
	return false
}
