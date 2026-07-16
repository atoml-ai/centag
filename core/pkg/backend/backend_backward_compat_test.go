package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"centag/core/pkg/logger"
)

func init() {
	_ = logger.Init(logger.Config{Level: "info", Format: "console", Output: "stdout"})
}

// TestBackwardCompatibility_SingleUserMode 验证单用户模式：所有后端无 TenantID
func TestBackwardCompatibility_SingleUserMode(t *testing.T) {
	m := NewManager()

	// 注册无 TenantID 的后端（模拟旧数据）
	err := m.Add(&BackendConfig{
		ID:       "legacy-backend",
		Name:     "Legacy Backend",
		Type:     "openai",
		BaseURL:  "https://api.openai.com",
		APIKey:   "sk-legacy",
		Enabled:  true,
		TenantID: "", // 空 = 系统默认 = 旧版行为
	})
	assert.NoError(t, err)

	// 旧版 Get 方法应正常工作
	cfg, err := m.Get("legacy-backend")
	assert.NoError(t, err)
	assert.Equal(t, "legacy-backend", cfg.ID)

	// 旧版 List 方法应返回所有后端
	all := m.List()
	assert.Len(t, all, 1)

	// 旧版 GetAll 方法应返回所有后端
	all = m.GetAll()
	assert.Len(t, all, 1)

	// 新版 GetByTenant("", id) 应等价于 Get(id)
	cfg2, err := m.GetByTenant("", "legacy-backend")
	assert.NoError(t, err)
	assert.Equal(t, cfg.ID, cfg2.ID)

	// 新版 ListByTenant("") 应返回所有后端（因为都是系统默认）
	list := m.ListByTenant("")
	assert.Len(t, list, 1)

	// 新版 GetEnabledByTenant("") 应返回所有启用的后端
	enabled := m.GetEnabledByTenant("")
	assert.Len(t, enabled, 1)

	// 新版 SelectDefaultBackendByTenant("") 应等价于 SelectDefaultBackend()
	default1, err := m.SelectDefaultBackend()
	assert.NoError(t, err)
	default2, err := m.SelectDefaultBackendByTenant("")
	assert.NoError(t, err)
	assert.Equal(t, default1.ID, default2.ID)
}

// TestBackwardCompatibility_MixedMode 验证混合模式：系统后端 + 租户后端共存
func TestBackwardCompatibility_MixedMode(t *testing.T) {
	m := NewManager()

	// 旧版后端（无 TenantID）
	m.Add(&BackendConfig{
		ID:       "legacy-1", Name: "Legacy 1", Type: "openai",
		BaseURL: "https://api.openai.com", Enabled: true, TenantID: "",
	})

	// 新版租户后端
	m.Add(&BackendConfig{
		ID:       "tenant-1", Name: "Tenant 1", Type: "anthropic",
		BaseURL: "https://tenant.example.com", Enabled: true, TenantID: "tenant-a",
	})

	// 旧版 List 仍返回所有后端（向后兼容）
	all := m.List()
	assert.Len(t, all, 2)

	// 新版 ListByTenant("tenant-a") 只返回租户可见的
	tenantList := m.ListByTenant("tenant-a")
	assert.Len(t, tenantList, 2) // legacy-1 (system) + tenant-1 (tenant-a)

	// 新版 ListByTenant("") 只返回系统后端
	sysList := m.ListByTenant("")
	assert.Len(t, sysList, 1)
	assert.Equal(t, "legacy-1", sysList[0].ID)
}

// TestBackwardCompatibility_EmptyTenantIDEqualsSystem 验证空 TenantID = 系统预设
func TestBackwardCompatibility_EmptyTenantIDEqualsSystem(t *testing.T) {
	m := NewManager()

	m.Add(&BackendConfig{
		ID: "sys-backend", Name: "System", Enabled: true,
		TenantID: "", // 空
	})

	// 空 tenantID 应能访问空 TenantID 的后端
	cfg, err := m.GetByTenant("", "sys-backend")
	assert.NoError(t, err)
	assert.Equal(t, "sys-backend", cfg.ID)

	// 任意租户也应能访问系统后端
	cfg, err = m.GetByTenant("any-tenant", "sys-backend")
	assert.NoError(t, err)
	assert.Equal(t, "sys-backend", cfg.ID)
}
