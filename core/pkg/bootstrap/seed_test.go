package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"centag/core/pkg/database"
	_ "centag/plugins/database/sqlite"
)

// 验证新环境：迁移(含 010/017) + 首轮 Seed 后租户表与管理员租户记录齐全。
func TestFreshInitCreatesAdminTenant(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fresh.db")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := database.Init(ctx, "sqlite", map[string]interface{}{"path": dbPath}); err != nil {
		t.Fatalf("database init: %v", err)
	}
	t.Cleanup(func() { _ = database.Get().Close() })

	if err := Seed(ctx); err != nil {
		t.Fatalf("bootstrap seed: %v", err)
	}

	db := database.Get()
	admin, err := db.UserStore().GetByUsername(ctx, AdminUsername())
	if err != nil {
		t.Fatalf("get admin user: %v", err)
	}
	if admin.TenantID == nil || *admin.TenantID == "" {
		t.Fatal("admin user missing tenant_id after seed")
	}

	tenant, err := db.TenantStore().GetTenantByID(ctx, *admin.TenantID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if tenant.UserID != admin.ID {
		t.Fatalf("tenant user_id = %d, want %d", tenant.UserID, admin.ID)
	}

	quota, err := db.TenantStore().GetTenantQuota(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("get tenant quota: %v", err)
	}
	if quota.DailyTokenLimit == 0 {
		t.Fatal("expected default daily token limit")
	}

	// 二次启动应跳过 seed，不重复创建
	if err := Seed(ctx); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	tenants, err := db.TenantStore().ListTenants(ctx)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("tenant count = %d, want 1", len(tenants))
	}
}