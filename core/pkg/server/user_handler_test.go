package server

import (
	"testing"
	"time"

	"centag/core/pkg/database"

	"github.com/stretchr/testify/assert"
)

func TestToUserResponse(t *testing.T) {
	now := time.Now()
	user := &database.User{
		ID:                42,
		Username:          "testuser",
		Role:              database.RoleNormal,
		DisplayName:       "Test User",
		Email:             "test@example.com",
		Enabled:           true,
		DefaultPipelineID: "audit-mode",
		DailyTokenLimit:   100000,
		MonthlyTokenLimit: 3000000,
		DailyTokenUsed:    50000,
		MonthlyTokenUsed:  1500000,
		CreatedAt:         now,
	}

	resp := ToUserResponse(user)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(42), resp.ID)
	assert.Equal(t, "testuser", resp.Username)
	assert.Equal(t, "normal", resp.Role)
	assert.Equal(t, "Test User", resp.DisplayName)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.True(t, resp.Enabled)
	assert.Equal(t, "audit-mode", resp.DefaultPipelineID)
	assert.Equal(t, int64(100000), resp.DailyTokenLimit)
	assert.Equal(t, int64(3000000), resp.MonthlyTokenLimit)
	assert.Equal(t, int64(50000), resp.DailyTokenUsed)
	assert.Equal(t, int64(1500000), resp.MonthlyTokenUsed)
}

func TestToUserResponse_AdminRole(t *testing.T) {
	user := &database.User{
		ID:       1,
		Username: "admin",
		Role:     database.RoleAdmin,
	}
	resp := ToUserResponse(user)
	assert.Equal(t, "admin", resp.Role)
}

func TestNewUserHandler(t *testing.T) {
	h := NewUserHandler()
	assert.NotNil(t, h)
}
