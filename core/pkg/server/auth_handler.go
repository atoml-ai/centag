package server

import (
	"errors"
	"net/http"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthHandler handles login / logout / token refresh / profile endpoints.
type AuthHandler struct{}

func NewAuthHandler() *AuthHandler { return &AuthHandler{} }

// ── POST /api/auth/login ─────────────────────────────────────────────────────

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int64         `json:"expires_in"` // seconds
	User         *userResponse `json:"user"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "请输入用户名和密码")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()

	user, err := db.UserStore().GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusUnauthorized, "用户名或密码错误，请重新输入")
			return
		}
		logger.Errorf("login: db error: %v", err)
		logger.Errorf("login: db error details: %+v", err)
		RespondInternalError(c, "internal error")
		return
	}

	if !user.Enabled {
		RespondError(c, http.StatusUnauthorized, "该账户已被禁用，如有疑问请联系管理员")
		return
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		RespondError(c, http.StatusUnauthorized, "用户名或密码错误，请重新输入")
		return
	}

	// Issue access token (include tenant ID for multi-tenant isolation).
	tenantID := ""
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}
	accessToken, err := auth.IssueAccessToken(user.ID, user.Username, string(user.Role), tenantID)
	if err != nil {
		logger.Errorf("login: issue access token: %v", err)
		RespondInternalError(c, "could not issue token")
		return
	}

	// Generate & store refresh token.
	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		logger.Errorf("login: generate refresh token: %v", err)
		RespondInternalError(c, "could not generate refresh token")
		return
	}

	rt := &database.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(auth.RefreshTokenTTL),
	}
	if err := db.RefreshTokenStore().Create(ctx, rt); err != nil {
		logger.Errorf("login: store refresh token: %v", err)
		RespondInternalError(c, "could not store refresh token")
		return
	}

	RespondSuccess(c, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(auth.AccessTokenTTL.Seconds()),
		User:         toUserResponse(user),
	})
}

// ── POST /api/auth/refresh ───────────────────────────────────────────────────

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "refresh_token is required")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()

	hash := auth.SHA256Hex(req.RefreshToken)
	logger.Info("auth/refresh: looking up refresh token",
		zap.String("hash_prefix", hash[:8]))
	
	rt, err := db.RefreshTokenStore().GetByHash(ctx, hash)
	if err != nil {
		logger.Warn("auth/refresh: token not found or error",
			zap.Error(err))
		RespondError(c, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	logger.Info("auth/refresh: token found",
		zap.Int64("user_id", rt.UserID),
		zap.Time("expires_at", rt.ExpiresAt),
		zap.Bool("revoked", rt.Revoked))

	if rt.ExpiresAt.Before(time.Now()) {
		_ = db.RefreshTokenStore().Revoke(ctx, hash)
		logger.Warn("auth/refresh: token expired")
		RespondError(c, http.StatusUnauthorized, "refresh token expired")
		return
	}

	user, err := db.UserStore().GetByID(ctx, rt.UserID)
	if err != nil || !user.Enabled {
		logger.Warn("auth/refresh: user not found or disabled",
			zap.Int64("user_id", rt.UserID),
			zap.Error(err))
		RespondError(c, http.StatusUnauthorized, "user not found or disabled")
		return
	}

	// Rotate: revoke old, issue new pair.
	_ = db.RefreshTokenStore().Revoke(ctx, hash)
	logger.Info("auth/refresh: old token revoked, issuing new pair",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username))

	tenantID := ""
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}
	accessToken, err := auth.IssueAccessToken(user.ID, user.Username, string(user.Role), tenantID)
	if err != nil {
		RespondInternalError(c, "could not issue token")
		return
	}

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		RespondInternalError(c, "could not generate refresh token")
		return
	}
	newRT := &database.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(auth.RefreshTokenTTL),
	}
	if err := db.RefreshTokenStore().Create(ctx, newRT); err != nil {
		RespondInternalError(c, "could not store refresh token")
		return
	}

	logger.Info("auth/refresh: success",
		zap.Int64("user_id", user.ID),
		zap.String("new_token_hash_prefix", refreshHash[:8]))

	RespondSuccess(c, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(auth.AccessTokenTTL.Seconds()),
		User:         toUserResponse(user),
	})
}

// ── POST /api/auth/logout ────────────────────────────────────────────────────

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req)

	if req.RefreshToken != "" {
		db := database.Get()
		hash := auth.SHA256Hex(req.RefreshToken)
		_ = db.RefreshTokenStore().Revoke(c.Request.Context(), hash)
	}

	RespondSuccessWithMessage(c, "logged out")
}

// ── GET /api/auth/me ─────────────────────────────────────────────────────────

func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	user, err := database.Get().UserStore().GetByID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	RespondSuccess(c, toUserResponse(user))
}

// ── shared helpers ───────────────────────────────────────────────────────────

type userResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
	// v2.1: Quota fields
	DefaultPipelineID string `json:"default_pipeline_id,omitempty"`
	DailyTokenLimit   int64  `json:"daily_token_limit"`
	MonthlyTokenLimit int64  `json:"monthly_token_limit"`
	DailyTokenUsed   int64  `json:"daily_token_used"`
	MonthlyTokenUsed int64  `json:"monthly_token_used"`
	// Team: shared resource access
	AllowedBackendIDs        []string `json:"allowed_backend_ids"`
	AllowedModelIDs          []string `json:"allowed_model_ids"`
	AllowedPipelineIDs       []string `json:"allowed_pipeline_ids"`
	CanAddOwnBackends        bool     `json:"can_add_own_backends"`
	CanAddOwnPipelines       bool     `json:"can_add_own_pipelines"`
	CanChangeDefaultPipeline bool     `json:"can_change_default_pipeline"`
	CreatedAt                string   `json:"created_at"`
}

func toUserResponse(u *database.User) *userResponse {
	backends := u.AllowedBackendIDs
	if backends == nil {
		backends = []string{}
	}
	models := u.AllowedModelIDs
	if models == nil {
		models = []string{}
	}
	pipelines := u.AllowedPipelineIDs
	if pipelines == nil {
		pipelines = []string{}
	}
	return &userResponse{
		ID:                       u.ID,
		Username:                 u.Username,
		Role:                     string(u.Role),
		DisplayName:              u.DisplayName,
		Email:                    u.Email,
		Enabled:                  u.Enabled,
		DefaultPipelineID:        u.DefaultPipelineID,
		DailyTokenLimit:          u.DailyTokenLimit,
		MonthlyTokenLimit:        u.MonthlyTokenLimit,
		DailyTokenUsed:           u.DailyTokenUsed,
		MonthlyTokenUsed:         u.MonthlyTokenUsed,
		AllowedBackendIDs:        backends,
		AllowedModelIDs:          models,
		AllowedPipelineIDs:       pipelines,
		CanAddOwnBackends:        u.CanAddOwnBackends,
		CanAddOwnPipelines:       u.CanAddOwnPipelines,
		CanChangeDefaultPipeline: u.CanChangeDefaultPipeline,
		CreatedAt:                u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}
