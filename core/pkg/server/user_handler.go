package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// UserHandler manages user CRUD (admin) and self-profile endpoints.
type UserHandler struct {
	provisioner *TenantProvisioner
}

func NewUserHandler(provisioner *TenantProvisioner) *UserHandler {
	return &UserHandler{provisioner: provisioner}
}

// ── Admin: GET /api/v1/admin/users ──────────────────────────────────────────

func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := database.Get().UserStore().List(c.Request.Context())
	if err != nil {
		logger.Errorf("list users: %v", err)
		RespondInternalError(c, "failed to list users")
		return
	}
	resp := make([]*userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	RespondSuccess(c, resp)
}

// ── Admin: POST /api/v1/admin/users ─────────────────────────────────────────

type createUserRequest struct {
	Username         string `json:"username"           binding:"required"`
	Password         string `json:"password"           binding:"required"`
	Role             string `json:"role"`
	DisplayName      string `json:"display_name"`
	Email            string `json:"email"`
	DefaultPipelineID string `json:"default_pipeline_id"` // v2.1: 默认流水线
	DailyTokenLimit   int64  `json:"daily_token_limit"`   // v2.1: 每日 Token 限额
	MonthlyTokenLimit int64  `json:"monthly_token_limit"` // v2.1: 每月 Token 限额
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if req.Role == "" {
		req.Role = string(database.RoleNormal)
	}
	if req.Role != string(database.RoleAdmin) && req.Role != string(database.RoleNormal) {
		RespondBadRequest(c, "role must be 'admin' or 'normal'")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		RespondInternalError(c, "failed to hash password")
		return
	}

	user := &database.User{
		Username:                 req.Username,
		Password:                 hash,
		Role:                     database.UserRole(req.Role),
		DisplayName:              req.DisplayName,
		Email:                    req.Email,
		Enabled:                  true,
		DefaultPipelineID:        req.DefaultPipelineID,
		DailyTokenLimit:          req.DailyTokenLimit,
		MonthlyTokenLimit:        req.MonthlyTokenLimit,
		AllowedBackendIDs:        []string{},
		AllowedModelIDs:          []string{},
		AllowedPipelineIDs:       []string{},
		CanAddOwnBackends:        true,
		CanAddOwnPipelines:       true,
		CanChangeDefaultPipeline: true,
	}

	if err := database.Get().UserStore().Create(c.Request.Context(), user); err != nil {
		errStr := err.Error()
		// SQLite: "UNIQUE constraint failed: ..."
		// PostgreSQL/pgx: "duplicate key value violates unique constraint ..."
		if strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "duplicate key") {
			RespondError(c, http.StatusConflict, "username already exists")
			return
		}
		logger.Errorf("create user: %v", err)
		RespondInternalError(c, "failed to create user")
		return
	}

	// ── 多租户：自动创建租户并复制系统预设 ─────────────────────────────────────
	if h.provisioner != nil {
		tenant, rawKey, err := h.provisioner.ProvisionForUser(c.Request.Context(), user)
		if err != nil {
			// 租户创建失败不阻塞用户创建，记录警告
			logger.Warnf("Failed to provision tenant for user %d: %v", user.ID, err)
		} else {
			logger.Info("User provisioned with tenant",
				logger.GetField("user_id", user.ID),
				logger.GetField("tenant_id", tenant.ID))
			// 返回租户信息和默认 API Key
			c.JSON(http.StatusCreated, gin.H{
				"success": true,
				"data":    toUserResponse(user),
				"tenant": gin.H{
					"id":          tenant.ID,
					"name":        tenant.Name,
					"description": tenant.Description,
					"status":      tenant.Status,
				},
				"default_api_key": rawKey, // 仅返回一次，需提醒用户保存
			})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": toUserResponse(user)})
}

// ── Admin: PUT /api/v1/admin/users/:id ──────────────────────────────────────

type updateUserRequest struct {
	DisplayName       *string  `json:"display_name"`
	Email             *string  `json:"email"`
	Role              *string  `json:"role"`
	Enabled           *bool    `json:"enabled"`
	DefaultPipelineID *string  `json:"default_pipeline_id"` // v2.1: 默认流水线
	DailyTokenLimit   *int64   `json:"daily_token_limit"`   // v2.1: 每日 Token 限额
	MonthlyTokenLimit *int64   `json:"monthly_token_limit"` // v2.1: 每月 Token 限额
	AllowedBackendIDs  *[]string `json:"allowed_backend_ids"`
	AllowedModelIDs    *[]string `json:"allowed_model_ids"`
	AllowedPipelineIDs *[]string `json:"allowed_pipeline_ids"`
	CanAddOwnBackends        *bool `json:"can_add_own_backends"`
	CanAddOwnPipelines       *bool `json:"can_add_own_pipelines"`
	CanChangeDefaultPipeline *bool `json:"can_change_default_pipeline"`
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		RespondBadRequest(c, "invalid user id")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()

	user, err := db.UserStore().GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "user not found")
		} else {
			RespondInternalError(c, "failed to get user")
		}
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Role != nil {
		if *req.Role != string(database.RoleAdmin) && *req.Role != string(database.RoleNormal) {
			RespondBadRequest(c, "role must be 'admin' or 'normal'")
			return
		}
		user.Role = database.UserRole(*req.Role)
	}
	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	}
	// v2.1: Quota fields
	if req.DefaultPipelineID != nil {
		user.DefaultPipelineID = *req.DefaultPipelineID
	}
	if req.DailyTokenLimit != nil {
		user.DailyTokenLimit = *req.DailyTokenLimit
	}
	if req.MonthlyTokenLimit != nil {
		user.MonthlyTokenLimit = *req.MonthlyTokenLimit
	}
	if req.AllowedBackendIDs != nil {
		user.AllowedBackendIDs = *req.AllowedBackendIDs
	}
	if req.AllowedModelIDs != nil {
		user.AllowedModelIDs = *req.AllowedModelIDs
	}
	if req.AllowedPipelineIDs != nil {
		user.AllowedPipelineIDs = *req.AllowedPipelineIDs
	}
	if req.CanAddOwnBackends != nil {
		user.CanAddOwnBackends = *req.CanAddOwnBackends
	}
	if req.CanAddOwnPipelines != nil {
		user.CanAddOwnPipelines = *req.CanAddOwnPipelines
	}
	if req.CanChangeDefaultPipeline != nil {
		user.CanChangeDefaultPipeline = *req.CanChangeDefaultPipeline
	}

	if err := db.UserStore().Update(ctx, user); err != nil {
		logger.Errorf("update user %d: %v", id, err)
		RespondInternalError(c, "failed to update user")
		return
	}

	RespondSuccess(c, toUserResponse(user))
}

// ── Admin: DELETE /api/v1/admin/users/:id ───────────────────────────────────

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		RespondBadRequest(c, "invalid user id")
		return
	}

	// Prevent self-deletion.
	selfID, _ := auth.GetUserID(c)
	if id == selfID {
		RespondBadRequest(c, "cannot delete your own account")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()

	// Clean up the user's tenant if it exists (cascade delete tenant data).
	tenant, err := db.TenantStore().GetTenantByUserID(ctx, id)
	if err == nil && tenant != nil {
		if err := db.TenantStore().DeleteTenant(ctx, tenant.ID); err != nil {
			logger.Warnf("Failed to delete tenant %s for user %d: %v", tenant.ID, id, err)
		} else {
			logger.Info("Tenant deleted along with user",
				logger.GetField("tenant_id", tenant.ID),
				logger.GetField("user_id", id))
		}
	}

	if err := db.UserStore().Delete(ctx, id); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "user not found")
		} else {
			logger.Errorf("delete user %d: %v", id, err)
			RespondInternalError(c, "failed to delete user")
		}
		return
	}

	RespondSuccessWithMessage(c, "user deleted")
}

// ── Admin: PUT /api/v1/admin/users/:id/password ──────────────────────────────

type adminResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

func (h *UserHandler) AdminResetPassword(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		RespondBadRequest(c, "invalid user id")
		return
	}

	var req adminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if len(req.NewPassword) < 6 {
		RespondBadRequest(c, "password must be at least 6 characters")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()
	user, err := db.UserStore().GetByID(ctx, id)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		RespondInternalError(c, "failed to hash password")
		return
	}
	user.Password = hash
	if err := db.UserStore().Update(ctx, user); err != nil {
		RespondInternalError(c, "failed to update password")
		return
	}
	// Revoke all refresh tokens so the user must re-login.
	_ = db.RefreshTokenStore().RevokeAllForUser(ctx, id)
	RespondSuccessWithMessage(c, "password reset")
}

// ── Self: GET /api/v1/user/profile ──────────────────────────────────────────

func (h *UserHandler) GetProfile(c *gin.Context) {
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

// ── Self: PUT /api/v1/user/profile ──────────────────────────────────────────

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	db := database.Get()
	ctx := c.Request.Context()
	user, err := db.UserStore().GetByID(ctx, userID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	user.DisplayName = req.DisplayName
	user.Email = req.Email
	if err := db.UserStore().Update(ctx, user); err != nil {
		RespondInternalError(c, "failed to update profile")
		return
	}
	RespondSuccess(c, toUserResponse(user))
}

// ── Self: PUT /api/v1/user/password ─────────────────────────────────────────

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if len(req.NewPassword) < 6 {
		RespondBadRequest(c, "new password must be at least 6 characters")
		return
	}

	db := database.Get()
	ctx := c.Request.Context()
	user, err := db.UserStore().GetByID(ctx, userID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	if !auth.CheckPassword(req.OldPassword, user.Password) {
		RespondError(c, http.StatusUnauthorized, "incorrect current password")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		RespondInternalError(c, "failed to hash password")
		return
	}
	user.Password = hash
	if err := db.UserStore().Update(ctx, user); err != nil {
		RespondInternalError(c, "failed to update password")
		return
	}
	// Revoke all refresh tokens; client must re-login.
	_ = db.RefreshTokenStore().RevokeAllForUser(ctx, userID)
	RespondSuccessWithMessage(c, "password changed")
}

func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}
