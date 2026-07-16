package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// APIKeyHandler manages API keys for the authenticated user.
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

	RespondSuccess(c, toAPIKeyResponses(keys))
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

	resp := apiKeyDetailResponse{
		apiKeyResponse: toAPIKeyResponse(key),
	}
	storageKey := auth.APIKeyStorageKey()
	if key.KeySecretEnc != "" && storageKey != nil {
		plain, decErr := auth.DecryptAPIKeyPlaintext(key.KeySecretEnc, storageKey)
		if decErr != nil {
			logger.Warnf("decrypt api key id=%d: %v", keyID, decErr)
		} else {
			resp.FullKey = plain
		}
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

type apiKeyCreateResponse struct {
	apiKeyResponse
	FullKey string `json:"full_key"` // always returned on create; also retrievable via GET :id when encrypted storage is enabled
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

	// 多租户：自动注入 tenant_id
	tenantID := auth.GetTenantID(c)
	if tenantID != "" {
		key.TenantID = &tenantID
	}

	if sk := auth.APIKeyStorageKey(); sk != nil {
		enc, encErr := auth.EncryptAPIKeyPlaintext(fullKey, sk)
		if encErr != nil {
			logger.Errorf("encrypt api key user %d: %v", userID, encErr)
			RespondInternalError(c, "failed to persist API key")
			return
		}
		key.KeySecretEnc = enc
	}
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
		"data": apiKeyCreateResponse{
			apiKeyResponse: toAPIKeyResponse(key),
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

	RespondSuccess(c, toAPIKeyResponse(key))
}

// ── Admin: GET /api/v1/admin/users/:user_id/apikeys ──────────────────────────────

func (h *APIKeyHandler) ListUserAPIKeys(c *gin.Context) {
	// Verify admin permissions
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	// Check if user is admin
	db := database.Get()
	ctx := c.Request.Context()
	user, err := db.UserStore().GetByID(ctx, userID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}
	if user.Role != database.RoleAdmin {
		RespondError(c, http.StatusForbidden, "admin access required")
		return
	}

	// Get target user ID from URL parameter
	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid user id")
		return
	}

	// List API keys for the target user
	keys, err := db.APIKeyStore().ListByUserID(ctx, targetUserID)
	if err != nil {
		logger.Errorf("list api keys for user %d: %v", targetUserID, err)
		RespondInternalError(c, "failed to list API keys")
		return
	}

	RespondSuccess(c, toAPIKeyResponses(keys))
}

// ── Admin: GET /api/v1/admin/users/:user_id/apikeys/:id ───────────────────────────

func (h *APIKeyHandler) GetAdminAPIKey(c *gin.Context) {
	// Verify admin permissions
	adminID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	// Check if user is admin
	db := database.Get()
	ctx := c.Request.Context()
	admin, err := db.UserStore().GetByID(ctx, adminID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}
	if admin.Role != database.RoleAdmin {
		RespondError(c, http.StatusForbidden, "admin access required")
		return
	}

	// Get target user ID and key ID from URL parameters
	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid user id")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	// Get the specific API key
	key, err := db.APIKeyStore().GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to get API key")
		}
		return
	}

	// Verify that the key belongs to the target user
	if key.UserID != targetUserID {
		RespondError(c, http.StatusForbidden, "API key does not belong to the specified user")
		return
	}

	resp := apiKeyDetailResponse{
		apiKeyResponse: toAPIKeyResponse(key),
	}
	storageKey := auth.APIKeyStorageKey()
	if key.KeySecretEnc != "" && storageKey != nil {
		plain, decErr := auth.DecryptAPIKeyPlaintext(key.KeySecretEnc, storageKey)
		if decErr != nil {
			logger.Warnf("decrypt api key id=%d: %v", keyID, decErr)
		} else {
			resp.FullKey = plain
		}
	}

	RespondSuccess(c, resp)
}

// ── Admin: GET /api/v1/admin/api-keys ──────────────────────────────────────────

type listAllAPIKeysRequest struct {
	Offset int `form:"offset"`
	Limit  int `form:"limit"  binding:"max=100"`
}

func (h *APIKeyHandler) ListAllAPIKeys(c *gin.Context) {
	var req listAllAPIKeysRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	keys, total, err := database.Get().APIKeyStore().ListAll(c.Request.Context(), req.Offset, req.Limit)
	if err != nil {
		logger.Errorf("list all api keys: %v", err)
		RespondInternalError(c, "failed to list API keys")
		return
	}

	RespondSuccess(c, gin.H{
		"keys":   toAPIKeyResponses(keys),
		"total":  total,
		"limit":  req.Limit,
		"offset": req.Offset,
	})
}

// ── Admin: POST /api/v1/admin/api-keys ──────────────────────────────────────────

type adminCreateAPIKeyRequest struct {
	UserID         int64   `json:"user_id" binding:"required"`
	Name           string  `json:"name" binding:"required"`
	ExpiresIn      *int    `json:"expires_in"`
	BudgetUSD      float64 `json:"budget_usd"`
	RateLimitRPM   int     `json:"rate_limit_rpm"`
	RateLimitTPM   int     `json:"rate_limit_tpm"`
	ModelWhitelist string  `json:"model_whitelist"`
}

func (h *APIKeyHandler) CreateAdminAPIKey(c *gin.Context) {
	var req adminCreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		RespondInternalError(c, "failed to generate API key")
		return
	}

	key := &database.APIKey{
		UserID:         req.UserID,
		Name:           req.Name,
		KeyHash:        keyHash,
		KeyPrefix:      keyPrefix,
		Enabled:        true,
		BudgetUSD:      req.BudgetUSD,
		RateLimitRPM:   req.RateLimitRPM,
		RateLimitTPM:   req.RateLimitTPM,
		ModelWhitelist: req.ModelWhitelist,
	}

	if sk := auth.APIKeyStorageKey(); sk != nil {
		enc, encErr := auth.EncryptAPIKeyPlaintext(fullKey, sk)
		if encErr != nil {
			RespondInternalError(c, "failed to persist API key")
			return
		}
		key.KeySecretEnc = enc
	}
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		key.ExpiresAt = &t
	}

	if err := database.Get().APIKeyStore().Create(c.Request.Context(), key); err != nil {
		RespondInternalError(c, "failed to create API key")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": apiKeyCreateResponse{
			apiKeyResponse: toAPIKeyResponse(key),
			FullKey:        fullKey,
		},
	})
}

// ── Admin: PUT /api/v1/admin/api-keys/:id ───────────────────────────────────────

type adminUpdateAPIKeyRequest struct {
	Name           *string  `json:"name"`
	Enabled        *bool    `json:"enabled"`
	BudgetUSD      *float64 `json:"budget_usd"`
	RateLimitRPM   *int     `json:"rate_limit_rpm"`
	RateLimitTPM   *int     `json:"rate_limit_tpm"`
	ModelWhitelist *string  `json:"model_whitelist"`
	ExpiresIn      *int     `json:"expires_in"`
}

func (h *APIKeyHandler) UpdateAdminAPIKey(c *gin.Context) {
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

	var req adminUpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if req.Name != nil {
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
	if req.ExpiresIn != nil {
		if *req.ExpiresIn > 0 {
			t := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
			key.ExpiresAt = &t
		} else {
			key.ExpiresAt = nil
		}
	}

	if err := db.APIKeyStore().Update(ctx, key); err != nil {
		RespondInternalError(c, "failed to update API key")
		return
	}

	RespondSuccess(c, toAPIKeyResponse(key))
}

// ── Admin: DELETE /api/v1/admin/api-keys/:id ─────────────────────────────────────

func (h *APIKeyHandler) DeleteAdminAPIKey(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	if err := database.Get().APIKeyStore().Delete(c.Request.Context(), keyID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to delete API key")
		}
		return
	}

	RespondSuccessWithMessage(c, "API key deleted")
}

// ── Admin: GET /api/v1/admin/api-keys/:id/stats ────────────────────────────────

func (h *APIKeyHandler) GetAPIKeyStats(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid key id")
		return
	}

	key, err := database.Get().APIKeyStore().GetByID(c.Request.Context(), keyID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "API key not found")
		} else {
			RespondInternalError(c, "failed to get API key")
		}
		return
	}

	RespondSuccess(c, gin.H{
		"total_used_usd": key.UsedUSD,
		"budget_usd":     key.BudgetUSD,
	})
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
		RespondInternalError(c, "failed to delete API key")
		return
	}

	RespondSuccessWithMessage(c, "API key deleted")
}

// ── response helpers ─────────────────────────────────────────────────────────

type apiKeyResponse struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	KeyPrefix       string  `json:"key_prefix"` // 16-char prefix
	MaskedKey       string  `json:"masked_key"` // display-safe
	RevealAvailable bool    `json:"reveal_available"`
	Enabled         bool    `json:"enabled"`
	TenantID        string  `json:"tenant_id,omitempty"` // 多租户：所属租户
	BudgetUSD       float64 `json:"budget_usd"`
	UsedUSD         float64 `json:"used_usd"`
	RateLimitRPM    int     `json:"rate_limit_rpm"`
	RateLimitTPM    int     `json:"rate_limit_tpm"`
	ModelWhitelist  string  `json:"model_whitelist"`
	ExpiresAt       *string `json:"expires_at"`
	LastUsedAt      *string `json:"last_used_at"`
	CreatedAt       string  `json:"created_at"`
}

type apiKeyDetailResponse struct {
	apiKeyResponse
	FullKey string `json:"full_key,omitempty"`
}

func toAPIKeyResponse(k *database.APIKey) apiKeyResponse {
	r := apiKeyResponse{
		ID:              k.ID,
		Name:            k.Name,
		KeyPrefix:       k.KeyPrefix,
		MaskedKey:       auth.MaskAPIKey(k.KeyPrefix),
		RevealAvailable: k.KeySecretEnc != "",
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

func toAPIKeyResponses(keys []*database.APIKey) []apiKeyResponse {
	resp := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toAPIKeyResponse(k))
	}
	return resp
}
