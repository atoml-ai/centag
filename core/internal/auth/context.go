package auth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context keys for Gin context values.
const (
	CtxKeyUserID   = "auth_user_id"
	CtxKeyUsername = "auth_username"
	CtxKeyRole     = "auth_role"
	CtxKeyTenantID = "auth_tenant_id" // 多租户：当前请求的租户 ID
)

var ErrUnauthenticated = errors.New("unauthenticated")

// SetUserContext stores the authenticated user's claims into the Gin context.
func SetUserContext(c *gin.Context, claims *Claims) {
	c.Set(CtxKeyUserID, claims.UserID)
	c.Set(CtxKeyUsername, claims.Username)
	c.Set(CtxKeyRole, claims.Role)
	if claims.TenantID != "" {
		c.Set(CtxKeyTenantID, claims.TenantID)
	}
}

// GetUserID extracts the authenticated user's ID from the Gin context.
// Returns 0 and ErrUnauthenticated if not set.
func GetUserID(c *gin.Context) (int64, error) {
	v, exists := c.Get(CtxKeyUserID)
	if !exists {
		return 0, ErrUnauthenticated
	}
	id, ok := v.(int64)
	if !ok {
		return 0, ErrUnauthenticated
	}
	return id, nil
}

// GetRole extracts the authenticated user's role from the Gin context.
func GetRole(c *gin.Context) string {
	v, _ := c.Get(CtxKeyRole)
	role, _ := v.(string)
	return role
}

// GetTenantID extracts the tenant ID from the Gin context.
// Returns empty string if not set (single-user mode or unauthenticated).
func GetTenantID(c *gin.Context) string {
	v, exists := c.Get(CtxKeyTenantID)
	if !exists {
		return ""
	}
	tid, _ := v.(string)
	return tid
}

// IsAdmin reports whether the request is from an admin user.
func IsAdmin(c *gin.Context) bool {
	r := strings.ToLower(strings.TrimSpace(GetRole(c)))
	return r == string(RoleAdmin) || r == "administrator"
}

// ScopedAccess describes what data scope an authenticated request should use.
type ScopedAccess int

const (
	// AccessGlobal means the request sees all data (admin-only).
	AccessGlobal ScopedAccess = iota
	// AccessTenant means the request is scoped to its own tenant (normal user).
	AccessTenant
	// AccessDenied means the request has no valid scope (e.g. missing tenant in multi-tenant mode).
	AccessDenied
)

// GetScopedAccess determines the data access scope for the current request.
// Rules:
//   - Admin → always global access (can see/manage all tenants' data).
//   - Normal user with tenant_id → tenant-scoped access.
//   - Normal user without tenant_id (single-user mode) → global access for backward-compat.
//   - Unauthenticated → denied.
func GetScopedAccess(c *gin.Context) ScopedAccess {
	if IsAdmin(c) {
		return AccessGlobal
	}
	tid := GetTenantID(c)
	if tid != "" {
		return AccessTenant
	}
	// Single-user mode: user has role but no tenant_id yet.
	// Treat as global to avoid breaking existing deployments.
	if GetRole(c) != "" {
		return AccessGlobal
	}
	return AccessDenied
}

// RoleAdmin / RoleNormal mirror the database constants to avoid import cycles.
const (
	RoleAdmin  = "admin"
	RoleNormal = "normal"
)
