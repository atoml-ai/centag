package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	tenantID := ownTenantID(user)
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

	tenantID := ownTenantID(user)
	accessToken, err := auth.IssueAccessToken(user.ID, user.Username, string(user.Role), tenantID)
	if err != nil {
		RespondInternalError(c, "could not issue token")
		return
	}

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		logger.Errorf("login: generate refresh token: %v", err)
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

// UserResponse is the public JSON shape for user records (profile + admin).
// Exported so commercial plugins (centag-pro) can reuse the same DTO without forking.
type UserResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
	DefaultPipelineID        string `json:"default_pipeline_id,omitempty"`
	CanAddOwnBackends        bool   `json:"can_add_own_backends"`
	CanAddOwnPipelines       bool   `json:"can_add_own_pipelines"`
	CanChangeDefaultPipeline bool   `json:"can_change_default_pipeline"`
	CreatedAt                string `json:"created_at"`
}

// Deprecated: use UserResponse.
type userResponse = UserResponse

// ToUserResponse maps a DB user to the API DTO (no password).
func ToUserResponse(u *database.User) *UserResponse {
	return toUserResponse(u)
}

func toUserResponse(u *database.User) *UserResponse {
	return &userResponse{
		ID:                       u.ID,
		Username:                 u.Username,
		Role:                     string(u.Role),
		DisplayName:              u.DisplayName,
		Email:                    u.Email,
		Enabled:                  u.Enabled,
		DefaultPipelineID:        u.DefaultPipelineID,
		CanAddOwnBackends:        u.CanAddOwnBackends,
		CanAddOwnPipelines:       u.CanAddOwnPipelines,
		CanChangeDefaultPipeline: u.CanChangeDefaultPipeline,
		CreatedAt:                u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// ── GET /api/v1/auth/bootstrap-status ────────────────────────────────────────

func (h *AuthHandler) BootstrapStatus(c *gin.Context) {
	db := database.Get()
	ctx := c.Request.Context()

	count, err := db.UserStore().Count(ctx)
	if err != nil {
		logger.Errorf("bootstrap-status: count users: %v", err)
		RespondInternalError(c, "failed to check bootstrap status")
		return
	}

	// 获取edition环境变量
	edition := strings.ToLower(strings.TrimSpace(os.Getenv("CENTAG_EDITION")))
	if edition == "" {
		edition = "team"
	}

	// 如果没有用户，则未初始化
	initialized := count > 0

	RespondSuccess(c, gin.H{
		"initialized": initialized,
		"username":    "admin",
		"edition":     edition,
	})
}

// ── POST /api/v1/auth/setup ──────────────────────────────────────────────────

func (h *AuthHandler) Setup(c *gin.Context) {
	db := database.Get()
	ctx := c.Request.Context()

	// 检查是否已有用户（已初始化）
	count, err := db.UserStore().Count(ctx)
	if err != nil {
		logger.Errorf("setup: count users: %v", err)
		RespondInternalError(c, "failed to check setup status")
		return
	}
	if count > 0 {
		RespondError(c, http.StatusConflict, "管理密码已设置")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		RespondBadRequest(c, "请提供密码")
		return
	}
	if len(req.Password) < 6 {
		RespondBadRequest(c, "密码至少 6 位")
		return
	}

	// 获取用户名（从环境变量或默认值）
	username := "admin"
	if envUser := strings.TrimSpace(os.Getenv("LLM_PROXY_ADMIN_USERNAME")); envUser != "" {
		username = envUser
	}

	// 哈希密码
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		logger.Errorf("setup: hash password: %v", err)
		RespondInternalError(c, "无法保存密码")
		return
	}

	// 创建管理员用户
	user := &database.User{
		Username:                 username,
		Password:                 hash,
		Role:                     database.RoleAdmin,
		DisplayName:              "Administrator",
		Enabled:                  true,
		AllowedBackendIDs:        []string{},
		AllowedModelIDs:          []string{},
		AllowedPipelineIDs:       []string{},
		CanAddOwnBackends:        true,
		CanAddOwnPipelines:       true,
		CanChangeDefaultPipeline: true,
	}
	if err := db.UserStore().Create(ctx, user); err != nil {
		logger.Errorf("setup: create user: %v", err)
		RespondInternalError(c, "无法保存密码")
		return
	}

	// 为新用户创建默认API key
	if err := createDefaultAPIKey(ctx, db, user); err != nil {
		logger.Errorf("setup: create default api key: %v", err)
		// 不返回错误，API key创建失败不影响用户创建
	}

	// 签发令牌
	accessToken, err := auth.IssueAccessToken(user.ID, user.Username, string(user.Role), "")
	if err != nil {
		logger.Errorf("setup: issue token: %v", err)
		RespondInternalError(c, "签发令牌失败")
		return
	}

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		logger.Errorf("setup: generate refresh token: %v", err)
		RespondInternalError(c, "签发令牌失败")
		return
	}

	rt := &database.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(auth.RefreshTokenTTL),
	}
	if err := db.RefreshTokenStore().Create(ctx, rt); err != nil {
		logger.Errorf("setup: store refresh token: %v", err)
		RespondInternalError(c, "签发令牌失败")
		return
	}

	RespondSuccess(c, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(auth.AccessTokenTTL.Seconds()),
		User:         toUserResponse(user),
	})
}

// createDefaultAPIKey creates a default API key for a user if none exists.
func createDefaultAPIKey(ctx context.Context, db *database.Manager, user *database.User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("invalid user")
	}

	// 确保API key存储已初始化
	if err := auth.EnsureAPIKeyStorage(ctx); err != nil {
		return fmt.Errorf("ensure api key storage: %w", err)
	}

	// 生成API key
	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("generate api key: %w", err)
	}

	// 加密存储
	enc, err := auth.EncryptAPIKeyForStorage(fullKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	key := &database.APIKey{
		UserID:       user.ID,
		TenantID:     user.TenantID,
		Name:         "default",
		KeyHash:      keyHash,
		KeyPrefix:    keyPrefix,
		KeySecretEnc: enc,
		Enabled:      true,
	}
	if err := db.APIKeyStore().Create(ctx, key); err != nil {
		return err
	}

	logger.Infof("已为用户 %q 创建默认 API key", user.Username)
	return nil
}
