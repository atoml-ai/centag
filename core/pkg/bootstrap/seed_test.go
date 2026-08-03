package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"centag/core/pkg/database"
	_ "centag/plugins/database/sqlite"
)

// 验证新环境：迁移 + 首轮 Seed 后管理员创建成功，且组模型（036）下不再
// 预建租户记录、不再回写 admin.tenant_id。
func TestFreshInitCreatesAdminNoTenant(t *testing.T) {
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
	if admin.TenantID != nil {
		t.Fatalf("admin user should have no tenant_id after seed (group model), got %q", *admin.TenantID)
	}

	// 二次启动应跳过 seed，不重复创建
	if err := Seed(ctx); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	tenants, err := db.TenantStore().ListTenants(ctx)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(tenants) != 0 {
		t.Fatalf("tenant count = %d, want 0 (no tenant provisioning in group model)", len(tenants))
	}
}