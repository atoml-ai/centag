package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestToTenantResponse 测试租户响应转换
func TestToTenantResponse(t *testing.T) {
	now := time.Now().UTC()
	tenant := &database.Tenant{
		ID:          "t_42_1704067200",
		UserID:      42,
		Name:        "test's workspace",
		Description: "test tenant",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resp := toTenantResponse(tenant)
	assert.NotNil(t, resp)
	assert.Equal(t, tenant.ID, resp.ID)
	assert.Equal(t, tenant.UserID, resp.UserID)
	assert.Equal(t, tenant.Name, resp.Name)
	assert.Equal(t, tenant.Status, resp.Status)
}

// TestToTenantResponse_Nil 测试 nil 输入
func TestToTenantResponse_Nil(t *testing.T) {
	resp := toTenantResponse(nil)
	assert.Nil(t, resp)
}

// TestToTenantQuotaResponse 测试配额响应转换
func TestToTenantQuotaResponse(t *testing.T) {
	quota := &database.TenantQuota{
		TenantID:            "t_42",
		DailyTokenLimit:     1000000,
		MonthlyTokenLimit:   10000000,
		DailyRequestLimit:   10000,
		MonthlyRequestLimit: 100000,
		MaxBackends:         10,
		MaxAPIKeys:          5,
		UsedTodayTokens:     100,
		UsedTodayRequests:   10,
		UsedMonthTokens:     1000,
		UsedMonthRequests:   100,
		ResetDate:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	resp := toTenantQuotaResponse(quota)
	assert.NotNil(t, resp)
	assert.Equal(t, quota.TenantID, resp.TenantID)
	assert.Equal(t, quota.DailyTokenLimit, resp.DailyTokenLimit)
	assert.Equal(t, quota.MaxBackends, resp.MaxBackends)
}

// TestTenantHandler_GetMyTenant_NoAuth 测试未认证访问
func TestTenantHandler_GetMyTenant_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 创建 handler（db 为 nil，但会在认证阶段失败）
	handler := &TenantHandler{}

	router.GET("/tenant", handler.GetMyTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenant", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestUpdateTenantRequest_Validation 测试更新请求结构
func TestUpdateTenantRequest_Validation(t *testing.T) {
	// 有效状态
	req1 := updateTenantRequest{Name: "Test", Status: "active"}
	assert.Equal(t, "active", req1.Status)

	// 空状态（允许）
	req2 := updateTenantRequest{Name: "Test"}
	assert.Equal(t, "", req2.Status)
}

// TestUpdateQuotaRequest 测试配额更新请求
func TestUpdateQuotaRequest(t *testing.T) {
	req := updateQuotaRequest{
		DailyTokenLimit:     500000,
		MonthlyTokenLimit:   5000000,
		DailyRequestLimit:   5000,
		MonthlyRequestLimit: 50000,
		MaxBackends:         5,
		MaxAPIKeys:          3,
	}

	assert.Equal(t, int64(500000), req.DailyTokenLimit)
	assert.Equal(t, 5, req.MaxBackends)
}
