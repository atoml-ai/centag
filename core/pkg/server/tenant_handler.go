// Package server 提供租户管理 REST API
// Phase 5: 管理员可查看/管理所有租户，用户可查看自己的租户信息
package server

import (
	"net/http"
	"strconv"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// TenantHandler 租户管理处理器
type TenantHandler struct {
	db *database.Manager
}

// NewTenantHandler 创建租户处理器
func NewTenantHandler(db *database.Manager) *TenantHandler {
	return &TenantHandler{db: db}
}

// ── 响应结构 ────────────────────────────────────────────────────────────────

type tenantResponse struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type tenantQuotaResponse struct {
	TenantID            string    `json:"tenant_id"`
	DailyTokenLimit     int64     `json:"daily_token_limit"`
	MonthlyTokenLimit   int64     `json:"monthly_token_limit"`
	DailyRequestLimit   int64     `json:"daily_request_limit"`
	MonthlyRequestLimit int64     `json:"monthly_request_limit"`
	MaxBackends         int       `json:"max_backends"`
	MaxAPIKeys          int       `json:"max_api_keys"`
	UsedTodayTokens     int64     `json:"used_today_tokens"`
	UsedTodayRequests   int64     `json:"used_today_requests"`
	UsedMonthTokens     int64     `json:"used_month_tokens"`
	UsedMonthRequests   int64     `json:"used_month_requests"`
	ResetDate           time.Time `json:"reset_date"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type tenantDetailResponse struct {
	Tenant *tenantResponse      `json:"tenant"`
	Quota  *tenantQuotaResponse `json:"quota,omitempty"`
}

// toTenantResponse 转换数据库模型为响应
func toTenantResponse(t *database.Tenant) *tenantResponse {
	if t == nil {
		return nil
	}
	return &tenantResponse{
		ID:          t.ID,
		UserID:      t.UserID,
		Name:        t.Name,
		Description: t.Description,
		Status:      t.Status,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// toTenantQuotaResponse 转换配额模型为响应
func toTenantQuotaResponse(q *database.TenantQuota) *tenantQuotaResponse {
	if q == nil {
		return nil
	}
	return &tenantQuotaResponse{
		TenantID:            q.TenantID,
		DailyTokenLimit:     q.DailyTokenLimit,
		MonthlyTokenLimit:   q.MonthlyTokenLimit,
		DailyRequestLimit:   q.DailyRequestLimit,
		MonthlyRequestLimit: q.MonthlyRequestLimit,
		MaxBackends:         q.MaxBackends,
		MaxAPIKeys:          q.MaxAPIKeys,
		UsedTodayTokens:     q.UsedTodayTokens,
		UsedTodayRequests:   q.UsedTodayRequests,
		UsedMonthTokens:     q.UsedMonthTokens,
		UsedMonthRequests:   q.UsedMonthRequests,
		ResetDate:           q.ResetDate,
		UpdatedAt:           q.UpdatedAt,
	}
}

// ── 管理员: GET /api/v1/admin/tenants ───────────────────────────────────────
// 列出所有租户（仅管理员）

func (h *TenantHandler) ListTenants(c *gin.Context) {
	ctx := c.Request.Context()
	tenants, err := h.db.TenantStore().ListTenants(ctx)
	if err != nil {
		logger.Errorf("list tenants: %v", err)
		RespondInternalError(c, "failed to list tenants")
		return
	}

	resp := make([]*tenantResponse, 0, len(tenants))
	for _, t := range tenants {
		resp = append(resp, toTenantResponse(t))
	}
	RespondSuccess(c, resp)
}

// ── 管理员: GET /api/v1/admin/tenants/:id ───────────────────────────────────
// 获取单个租户详情（含配额）

func (h *TenantHandler) GetTenant(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	// 获取租户信息
	tenant, err := h.db.TenantStore().GetTenantByID(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "tenant not found")
			return
		}
		logger.Errorf("get tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to get tenant")
		return
	}

	// 获取配额信息
	quota, _ := h.db.TenantStore().GetTenantQuota(ctx, tenantID)
	// 配额不存在不报错，返回空配额

	RespondSuccess(c, tenantDetailResponse{
		Tenant: toTenantResponse(tenant),
		Quota:  toTenantQuotaResponse(quota),
	})
}

// ── 管理员: PUT /api/v1/admin/tenants/:id ───────────────────────────────────
// 更新租户信息（名称、描述、状态）

type updateTenantRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"omitempty,oneof=active suspended"`
}

func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// 获取现有租户
	tenant, err := h.db.TenantStore().GetTenantByID(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "tenant not found")
			return
		}
		logger.Errorf("get tenant %s for update: %v", tenantID, err)
		RespondInternalError(c, "failed to get tenant")
		return
	}

	// 更新字段
	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Description != "" {
		tenant.Description = req.Description
	}
	if req.Status != "" {
		tenant.Status = req.Status
	}
	tenant.UpdatedAt = time.Now().UTC()

	if err := h.db.TenantStore().UpdateTenant(ctx, tenant); err != nil {
		logger.Errorf("update tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to update tenant")
		return
	}

	RespondSuccess(c, toTenantResponse(tenant))
}

// ── 管理员: DELETE /api/v1/admin/tenants/:id ────────────────────────────────
// 删除租户（危险操作，需确认）

func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	// 获取租户确认存在
	tenant, err := h.db.TenantStore().GetTenantByID(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "tenant not found")
			return
		}
		logger.Errorf("get tenant %s for delete: %v", tenantID, err)
		RespondInternalError(c, "failed to get tenant")
		return
	}

	// 删除租户（级联删除相关数据）
	if err := h.db.TenantStore().DeleteTenant(ctx, tenantID); err != nil {
		logger.Errorf("delete tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to delete tenant")
		return
	}

	logger.Info("Tenant deleted",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("user_id", tenant.UserID))

	RespondSuccess(c, gin.H{"message": "tenant deleted", "tenant_id": tenantID})
}

// ── 管理员: GET /api/v1/admin/tenants/:id/quota ─────────────────────────────
// 获取租户配额

func (h *TenantHandler) GetTenantQuota(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	quota, err := h.db.TenantStore().GetTenantQuota(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "quota not found for tenant")
			return
		}
		logger.Errorf("get quota for tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to get quota")
		return
	}

	RespondSuccess(c, toTenantQuotaResponse(quota))
}

// ── 管理员: PUT /api/v1/admin/tenants/:id/quota ─────────────────────────────
// 更新租户配额

type updateQuotaRequest struct {
	DailyTokenLimit     int64 `json:"daily_token_limit"`
	MonthlyTokenLimit   int64 `json:"monthly_token_limit"`
	DailyRequestLimit   int64 `json:"daily_request_limit"`
	MonthlyRequestLimit int64 `json:"monthly_request_limit"`
	MaxBackends         int   `json:"max_backends"`
	MaxAPIKeys          int   `json:"max_api_keys"`
}

func (h *TenantHandler) UpdateTenantQuota(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	var req updateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// 验证租户存在
	_, err := h.db.TenantStore().GetTenantByID(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "tenant not found")
			return
		}
		logger.Errorf("get tenant %s for quota update: %v", tenantID, err)
		RespondInternalError(c, "failed to get tenant")
		return
	}

	quota := &database.TenantQuota{
		TenantID:            tenantID,
		DailyTokenLimit:     req.DailyTokenLimit,
		MonthlyTokenLimit:   req.MonthlyTokenLimit,
		DailyRequestLimit:   req.DailyRequestLimit,
		MonthlyRequestLimit: req.MonthlyRequestLimit,
		MaxBackends:         req.MaxBackends,
		MaxAPIKeys:          req.MaxAPIKeys,
		UpdatedAt:           time.Now().UTC(),
	}

	if err := h.db.TenantStore().SetTenantQuota(ctx, quota); err != nil {
		logger.Errorf("set quota for tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to update quota")
		return
	}

	RespondSuccess(c, toTenantQuotaResponse(quota))
}

// ── 管理员: PUT /api/v1/admin/tenants/:id/quota/reset ───────────────────────
// 重置租户用量计数

func (h *TenantHandler) ResetTenantQuota(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("id")
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant id is required")
		return
	}

	// 获取现有配额
	quota, err := h.db.TenantStore().GetTenantQuota(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "quota not found for tenant")
			return
		}
		logger.Errorf("get quota for reset %s: %v", tenantID, err)
		RespondInternalError(c, "failed to get quota")
		return
	}

	// 重置用量计数
	quota.UsedTodayTokens = 0
	quota.UsedTodayRequests = 0
	quota.UsedMonthTokens = 0
	quota.UsedMonthRequests = 0
	quota.ResetDate = time.Now().UTC().Truncate(24 * time.Hour)
	quota.UpdatedAt = time.Now().UTC()

	if err := h.db.TenantStore().SetTenantQuota(ctx, quota); err != nil {
		logger.Errorf("reset quota for tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to reset quota")
		return
	}

	RespondSuccess(c, toTenantQuotaResponse(quota))
}

// ── 用户: GET /api/v1/user/tenant ───────────────────────────────────────────
// 获取当前用户的租户信息

func (h *TenantHandler) GetMyTenant(c *gin.Context) {
	ctx := c.Request.Context()

	// 从上下文获取用户 ID
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 获取用户对应的租户
	tenant, err := h.db.TenantStore().GetTenantByUserID(ctx, userID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "tenant not found for user")
			return
		}
		logger.Errorf("get tenant by user %d: %v", userID, err)
		RespondInternalError(c, "failed to get tenant")
		return
	}

	// 获取配额
	quota, _ := h.db.TenantStore().GetTenantQuota(ctx, tenant.ID)

	RespondSuccess(c, tenantDetailResponse{
		Tenant: toTenantResponse(tenant),
		Quota:  toTenantQuotaResponse(quota),
	})
}

// ── 用户: GET /api/v1/user/tenant/quota ─────────────────────────────────────
// 获取当前用户的配额使用情况

func (h *TenantHandler) GetMyQuota(c *gin.Context) {
	ctx := c.Request.Context()

	// 从上下文获取租户 ID
	tenantID := auth.GetTenantID(c)
	if tenantID == "" {
		RespondError(c, http.StatusBadRequest, "tenant not found in context")
		return
	}

	quota, err := h.db.TenantStore().GetTenantQuota(ctx, tenantID)
	if err != nil {
		if err == database.ErrNotFound {
			RespondError(c, http.StatusNotFound, "quota not found")
			return
		}
		logger.Errorf("get quota for tenant %s: %v", tenantID, err)
		RespondInternalError(c, "failed to get quota")
		return
	}

	RespondSuccess(c, toTenantQuotaResponse(quota))
}

// ── 辅助函数 ────────────────────────────────────────────────────────────────

// GetUserID 从 gin context 获取用户 ID（兼容 int64 和 string）
func getUserID(c *gin.Context) int64 {
	v, exists := c.Get(auth.CtxKeyUserID)
	if !exists {
		return 0
	}
	switch id := v.(type) {
	case int64:
		return id
	case int:
		return int64(id)
	case string:
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			return n
		}
	}
	return 0
}
