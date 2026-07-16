package auth

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetAndGetUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Role:     RoleAdmin,
	}

	SetUserContext(c, claims)

	userID, err := GetUserID(c)
	if err != nil {
		t.Errorf("GetUserID failed: %v", err)
	}
	if userID != 123 {
		t.Errorf("GetUserID = %d, want 123", userID)
	}

	role := GetRole(c)
	if role != RoleAdmin {
		t.Errorf("GetRole = %s, want %s", role, RoleAdmin)
	}

	if !IsAdmin(c) {
		t.Error("IsAdmin should return true for admin user")
	}
}

func TestGetUserID_Unauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	_, err := GetUserID(c)
	if err != ErrUnauthenticated {
		t.Errorf("GetUserID should return ErrUnauthenticated, got: %v", err)
	}
}

func TestGetUserID_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	c.Set(CtxKeyUserID, "not_an_int64")

	_, err := GetUserID(c)
	if err != ErrUnauthenticated {
		t.Errorf("GetUserID should return ErrUnauthenticated for invalid type, got: %v", err)
	}
}

func TestGetRole_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	role := GetRole(c)
	if role != "" {
		t.Errorf("GetRole should return empty string, got: %s", role)
	}
}

func TestIsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		role     string
		expected bool
	}{
		{"admin", RoleAdmin, true},
		{"administrator", "administrator", true},
		{"ADMIN", "ADMIN", true},
		{"Admin", "Admin", true},
		{"normal", RoleNormal, false},
		{"", "", false},
		{"user", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			c.Set(CtxKeyRole, tt.role)

			result := IsAdmin(c)
			if result != tt.expected {
				t.Errorf("IsAdmin with role %q = %v, want %v", tt.role, result, tt.expected)
			}
		})
	}
}
