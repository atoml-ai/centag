package server

import (
	"net/http"

	"centag/core/internal/auth"
	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
)

// UserHandler serves self-profile endpoints shared by personal and team.
// Team admin user CRUD + tenant provisioning live in centag-pro (E2.1+).
type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
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
