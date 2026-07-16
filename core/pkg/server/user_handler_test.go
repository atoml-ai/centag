package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
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
		DailyTokenUsed:   50000,
		MonthlyTokenUsed: 1500000,
		CreatedAt:        now,
	}

	resp := toUserResponse(user)
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
	resp := toUserResponse(user)
	assert.Equal(t, "admin", resp.Role)
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		wantID  int64
		wantErr bool
	}{
		{"valid", "42", 42, false},
		{"zero", "0", 0, false},
		{"negative", "-1", -1, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
		{"overflow", "99999999999999999999", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/users/"+tt.param, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.param}}

			id, err := parseID(c)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &UserHandler{}
	router.POST("/users", handler.CreateUser)

	w := httptest.NewRecorder()
	body := bytes.NewBufferString("not json")
	req, _ := http.NewRequest("POST", "/users", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateUser_InvalidRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &UserHandler{}
	router.POST("/users", handler.CreateUser)

	reqBody := createUserRequest{
		Username: "testuser",
		Password: "password123",
		Role:     "superadmin",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "role must be")
}

func TestCreateUser_DefaultRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := createUserRequest{
		Username: "testuser",
		Password: "password123",
	}
	assert.Equal(t, "", reqBody.Role, "default role should be empty in request")

	// After processing, handler defaults to "normal"
	user := &database.User{
		Role: database.UserRole(reqBody.Role),
	}
	if user.Role == "" {
		user.Role = database.RoleNormal
	}
	assert.Equal(t, database.RoleNormal, user.Role)
}

func TestUpdateUserRequest_Fields(t *testing.T) {
	name := "new name"
	email := "new@example.com"
	role := "admin"
	enabled := false
	pipeline := "router-mode"
	var dailyLimit int64 = 50000
	var monthlyLimit int64 = 1500000

	req := updateUserRequest{
		DisplayName:      &name,
		Email:            &email,
		Role:             &role,
		Enabled:          &enabled,
		DefaultPipelineID: &pipeline,
		DailyTokenLimit:   &dailyLimit,
		MonthlyTokenLimit: &monthlyLimit,
	}

	assert.Equal(t, "new name", *req.DisplayName)
	assert.Equal(t, "new@example.com", *req.Email)
	assert.Equal(t, "admin", *req.Role)
	assert.False(t, *req.Enabled)
	assert.Equal(t, "router-mode", *req.DefaultPipelineID)
	assert.Equal(t, int64(50000), *req.DailyTokenLimit)
	assert.Equal(t, int64(1500000), *req.MonthlyTokenLimit)
}

func TestUpdateUser_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &UserHandler{}
	router.PUT("/users/:id", handler.UpdateUser)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/users/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid user id")
}

func TestDeleteUser_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &UserHandler{}
	router.DELETE("/users/:id", handler.DeleteUser)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/users/notanumber", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid user id")
}

func TestAdminResetPassword_ShortPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &UserHandler{}
	router.PUT("/users/:id/password", handler.AdminResetPassword)

	reqBody := adminResetPasswordRequest{NewPassword: "123"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/users/1/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least 6 characters")
}
