package database

import (
	"context"
	"fmt"
	"time"

	"centag/core/pkg/logger"
)

// NewTenantID 生成租户 ID（格式 t_<userID>_<unix>）
func NewTenantID(userID int64) string {
	return fmt.Sprintf("t_%d_%d", userID, time.Now().Unix())
}

// DefaultTenantQuota 返回新租户的默认配额
func DefaultTenantQuota(tenantID string) *TenantQuota {
	now := time.Now().UTC()
	return &TenantQuota{
		TenantID:            tenantID,
		DailyTokenLimit:     1_000_000,
		MonthlyTokenLimit:   10_000_000,
		DailyRequestLimit:   10_000,
		MonthlyRequestLimit: 100_000,
		MaxBackends:         10,
		MaxAPIKeys:          5,
		ResetDate:           now.Truncate(24 * time.Hour),
		UpdatedAt:           now,
	}
}

// ProvisionUserTenant 为用户创建租户记录、默认配额，并回写 user.tenant_id。
// 首轮 bootstrap 与运行期创建用户均应调用。
func ProvisionUserTenant(ctx context.Context, mgr *Manager, user *User) (*Tenant, error) {
	if mgr == nil || user == nil || user.ID == 0 {
		return nil, fmt.Errorf("invalid user or database manager")
	}

	ts := mgr.TenantStore()
	if ts == nil {
		return nil, fmt.Errorf("tenant store not available")
	}

	if user.TenantID != nil && *user.TenantID != "" {
		tenant, err := ts.GetTenantByID(ctx, *user.TenantID)
		if err == nil {
			return tenant, nil
		}
		if err != ErrNotFound {
			return nil, err
		}
	}

	tenantID := NewTenantID(user.ID)
	displayName := user.Username
	if displayName == "" {
		displayName = fmt.Sprintf("User %d", user.ID)
	}

	tenant := &Tenant{
		ID:          tenantID,
		UserID:      user.ID,
		Name:        fmt.Sprintf("%s's workspace", displayName),
		Description: fmt.Sprintf("Tenant for user %s", displayName),
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := ts.CreateTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	if err := ts.SetTenantQuota(ctx, DefaultTenantQuota(tenantID)); err != nil {
		logger.Warnf("set default quota for tenant %s: %v", tenantID, err)
	}

	user.TenantID = &tenantID
	if err := mgr.UserStore().Update(ctx, user); err != nil {
		return nil, fmt.Errorf("link user to tenant: %w", err)
	}

	logger.Infof("provisioned tenant %s for user %d (%s)", tenantID, user.ID, user.Username)
	return tenant, nil
}