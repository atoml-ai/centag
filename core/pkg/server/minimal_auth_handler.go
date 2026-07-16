package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

const (
	minimalAdminUsername = "admin"
	minimalAdminUserID   = int64(1)
)

// MinimalAuthHandler provides single-password admin auth for the minimal edition.
type MinimalAuthHandler struct {
	dataDir string
	mu      sync.Mutex
	// refreshTokens maps refresh token -> expiry unix
	refreshTokens map[string]int64
	apiKeys       *minimalAPIKeyStore
}

func NewMinimalAuthHandler(dataDir string) *MinimalAuthHandler {
	return &MinimalAuthHandler{
		dataDir:       dataDir,
		refreshTokens: make(map[string]int64),
		apiKeys:       newMinimalAPIKeyStore(dataDir),
	}
}

func (h *MinimalAuthHandler) passwordPath() string {
	return filepath.Join(h.dataDir, "admin.password.hash")
}

func (h *MinimalAuthHandler) hasPassword() bool {
	data, err := os.ReadFile(h.passwordPath())
	return err == nil && strings.TrimSpace(string(data)) != ""
}

func (h *MinimalAuthHandler) readHash() (string, error) {
	data, err := os.ReadFile(h.passwordPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (h *MinimalAuthHandler) writeHash(hash string) error {
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(h.passwordPath(), []byte(hash+"\n"), 0o600)
}

func (h *MinimalAuthHandler) adminUser() gin.H {
	return gin.H{
		"id":           minimalAdminUserID,
		"username":     minimalAdminUsername,
		"role":         "admin",
		"display_name": "管理员",
		"email":        "",
		"enabled":      true,
		"created_at":   "",
	}
}

func (h *MinimalAuthHandler) issueTokens() (access, refresh string, expiresIn int, err error) {
	access, err = auth.IssueAccessToken(minimalAdminUserID, minimalAdminUsername, "admin", "")
	if err != nil {
		return "", "", 0, err
	}
	refresh = randomToken()
	h.mu.Lock()
	h.refreshTokens[refresh] = time.Now().Add(auth.RefreshTokenTTL).Unix()
	h.mu.Unlock()
	expiresIn = int(auth.AccessTokenTTL.Seconds())
	return access, refresh, expiresIn, nil
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterRoutes mounts public auth routes under /api/auth and /api/v1/auth.
func (h *MinimalAuthHandler) RegisterRoutes(r *gin.Engine) {
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", h.Login)
		authGroup.POST("/logout", h.Logout)
		authGroup.POST("/refresh", h.Refresh)
		authGroup.GET("/me", auth.JWTMiddleware(), h.Me)
	}

	v1Auth := r.Group("/api/v1/auth")
	{
		v1Auth.GET("/bootstrap-status", h.BootstrapStatus)
		v1Auth.POST("/setup", h.Setup)
	}

	settings := r.Group("/api/v1/settings")
	settings.Use(auth.JWTMiddleware())
	{
		settings.POST("/password", h.ChangePassword)
		h.registerAPIKeyRoutes(settings)
	}
}

func (h *MinimalAuthHandler) BootstrapStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"data": gin.H{
			"initialized": h.hasPassword(),
			"username":    minimalAdminUsername,
			"edition":     "minimal",
		},
	})
}

func (h *MinimalAuthHandler) Setup(c *gin.Context) {
	if h.hasPassword() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "管理密码已设置"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请提供密码"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "密码至少 6 位"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "无法保存密码"})
		return
	}
	if err := h.writeHash(hash); err != nil {
		logger.Errorf("[MinimalAuth] write password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "无法保存密码"})
		return
	}
	access, refresh, expiresIn, err := h.issueTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    expiresIn,
			"user":          h.adminUser(),
		},
	})
}

func (h *MinimalAuthHandler) Login(c *gin.Context) {
	if !h.hasPassword() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请先设置管理密码", "code": "SETUP_REQUIRED"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式错误"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = minimalAdminUsername
	}
	if username != minimalAdminUsername {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户名或密码错误"})
		return
	}
	hash, err := h.readHash()
	if err != nil || !auth.CheckPassword(req.Password, hash) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户名或密码错误"})
		return
	}
	access, refresh, expiresIn, err := h.issueTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    expiresIn,
			"user":          h.adminUser(),
		},
	})
}

func (h *MinimalAuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少 refresh_token"})
		return
	}
	h.mu.Lock()
	exp, ok := h.refreshTokens[req.RefreshToken]
	if ok {
		delete(h.refreshTokens, req.RefreshToken)
	}
	h.mu.Unlock()
	if !ok || time.Now().Unix() > exp {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "refresh token 无效或已过期"})
		return
	}
	access, refresh, expiresIn, err := h.issueTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    expiresIn,
			"user":          h.adminUser(),
		},
	})
}

func (h *MinimalAuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.RefreshToken != "" {
		h.mu.Lock()
		delete(h.refreshTokens, req.RefreshToken)
		h.mu.Unlock()
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *MinimalAuthHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.adminUser()})
}

func (h *MinimalAuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式错误"})
		return
	}
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "新密码至少 6 位"})
		return
	}
	hash, err := h.readHash()
	if err != nil || !auth.CheckPassword(req.OldPassword, hash) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "原密码错误"})
		return
	}
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "无法保存密码"})
		return
	}
	if err := h.writeHash(newHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "无法保存密码"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码已更新"})
}
