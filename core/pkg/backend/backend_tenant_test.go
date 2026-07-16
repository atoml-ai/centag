package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"centag/core/pkg/logger"
)

func init() {
	// 初始化 logger 避免测试中的 nil pointer panic
	_ = logger.Init(logger.Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
}

// TestManager_GetByTenant_Isolation 验证租户隔离：只能访问自己的后端和系统默认后端
func TestManager_GetByTenant_Isolation(t *testing.T) {
	m := NewManager()

	// 注册系统默认后端
	sysBackend := &BackendConfig{
		ID:       "sys-backend",
		Name:     "System Backend",
		Type:     "openai",
		BaseURL:  "https://api.openai.com",
		APIKey:   "sys-key",
		Enabled:  true,
		TenantID: "", // 系统默认
	}
	m.Add(sysBackend)

	// 注册租户A专属后端
	tenantABackend := &BackendConfig{
		ID:       "tenant-a-backend",
		Name:     "Tenant A Backend",
		Type:     "openai",
		BaseURL:  "https://tenant-a.example.com",
		APIKey:   "tenant-a-key",
		Enabled:  true,
		TenantID: "tenant-a",
	}
	m.Add(tenantABackend)

	// 注册租户B专属后端
	tenantBBackend := &BackendConfig{
		ID:       "tenant-b-backend",
		Name:     "Tenant B Backend",
		Type:     "anthropic",
		BaseURL:  "https://tenant-b.example.com",
		APIKey:   "tenant-b-key",
		Enabled:  true,
		TenantID: "tenant-b",
	}
	m.Add(tenantBBackend)

	// 测试1: 系统模式（tenantID=""）只能访问系统默认后端
	cfg, err := m.GetByTenant("", "sys-backend")
	assert.NoError(t, err)
	assert.Equal(t, "sys-backend", cfg.ID)

	_, err = m.GetByTenant("", "tenant-a-backend")
	assert.Error(t, err) // 系统模式不能访问租户后端

	// 测试2: 租户A可以访问系统默认和租户A后端
	cfg, err = m.GetByTenant("tenant-a", "sys-backend")
	assert.NoError(t, err)
	assert.Equal(t, "sys-backend", cfg.ID)

	cfg, err = m.GetByTenant("tenant-a", "tenant-a-backend")
	assert.NoError(t, err)
	assert.Equal(t, "tenant-a-backend", cfg.ID)

	// 测试3: 租户A不能访问租户B后端
	_, err = m.GetByTenant("tenant-a", "tenant-b-backend")
	assert.Error(t, err)

	// 测试4: 租户B可以访问系统默认和租户B后端
	cfg, err = m.GetByTenant("tenant-b", "sys-backend")
	assert.NoError(t, err)

	cfg, err = m.GetByTenant("tenant-b", "tenant-b-backend")
	assert.NoError(t, err)

	_, err = m.GetByTenant("tenant-b", "tenant-a-backend")
	assert.Error(t, err)
}

// TestManager_ListByTenant_Isolation 验证 ListByTenant 的隔离性
func TestManager_ListByTenant_Isolation(t *testing.T) {
	m := NewManager()

	m.Add(&BackendConfig{ID: "sys-1", Name: "System 1", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "sys-2", Name: "System 2", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "a-1", Name: "Tenant A 1", Enabled: true, TenantID: "tenant-a"})
	m.Add(&BackendConfig{ID: "b-1", Name: "Tenant B 1", Enabled: true, TenantID: "tenant-b"})

	// 系统模式：只能看到系统后端
	sysList := m.ListByTenant("")
	assert.Len(t, sysList, 2)
	for _, cfg := range sysList {
		assert.Equal(t, "", cfg.TenantID)
	}

	// 租户A：系统后端 + 租户A后端
	aList := m.ListByTenant("tenant-a")
	assert.Len(t, aList, 3)
	ids := make(map[string]bool)
	for _, cfg := range aList {
		ids[cfg.ID] = true
		assert.True(t, cfg.TenantID == "" || cfg.TenantID == "tenant-a")
	}
	assert.True(t, ids["sys-1"])
	assert.True(t, ids["sys-2"])
	assert.True(t, ids["a-1"])
	assert.False(t, ids["b-1"])

	// 租户B：系统后端 + 租户B后端
	bList := m.ListByTenant("tenant-b")
	assert.Len(t, bList, 3)
}

// TestManager_GetEnabledByTenant 验证 GetEnabledByTenant 过滤
func TestManager_GetEnabledByTenant(t *testing.T) {
	m := NewManager()

	m.Add(&BackendConfig{ID: "sys-enabled", Name: "Sys Enabled", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "sys-disabled", Name: "Sys Disabled", Enabled: false, TenantID: ""})
	m.Add(&BackendConfig{ID: "a-enabled", Name: "A Enabled", Enabled: true, TenantID: "tenant-a"})
	m.Add(&BackendConfig{ID: "a-disabled", Name: "A Disabled", Enabled: false, TenantID: "tenant-a"})

	list := m.GetEnabledByTenant("tenant-a")
	assert.Len(t, list, 2) // sys-enabled + a-enabled
	for _, cfg := range list {
		assert.True(t, cfg.Enabled)
		assert.True(t, cfg.TenantID == "" || cfg.TenantID == "tenant-a")
	}
}

// TestManager_SelectDefaultBackendByTenant 验证默认后端选择
func TestManager_SelectDefaultBackendByTenant(t *testing.T) {
	m := NewManager()

	// 无后端时应报错
	_, err := m.SelectDefaultBackendByTenant("tenant-x")
	assert.Error(t, err)

	// 注册系统后端和租户后端
	m.Add(&BackendConfig{ID: "sys-backend", Name: "System", Enabled: true, TenantID: "", Weight: 5})
	m.Add(&BackendConfig{ID: "a-backend", Name: "Tenant A", Enabled: true, TenantID: "tenant-a", Weight: 10})

	// 租户A应选择权重最高的（a-backend）
	selected, err := m.SelectDefaultBackendByTenant("tenant-a")
	assert.NoError(t, err)
	assert.Equal(t, "a-backend", selected.ID)

	// 系统模式只能选择系统后端
	selected, err = m.SelectDefaultBackendByTenant("")
	assert.NoError(t, err)
	assert.Equal(t, "sys-backend", selected.ID)
}

// TestManager_GetByTypeAndTenant 验证按类型和租户过滤
func TestManager_GetByTypeAndTenant(t *testing.T) {
	m := NewManager()

	m.Add(&BackendConfig{ID: "sys-openai", Type: "openai", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "sys-anthropic", Type: "anthropic", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "a-openai", Type: "openai", Enabled: true, TenantID: "tenant-a"})
	m.Add(&BackendConfig{ID: "a-ollama", Type: "ollama", Enabled: true, TenantID: "tenant-a"})
	m.Add(&BackendConfig{ID: "b-openai", Type: "openai", Enabled: true, TenantID: "tenant-b"})

	// 租户A的 openai 后端：sys-openai + a-openai
	list := m.GetByTypeAndTenant("openai", "tenant-a")
	assert.Len(t, list, 2)
	ids := make(map[string]bool)
	for _, cfg := range list {
		ids[cfg.ID] = true
		assert.Equal(t, "openai", cfg.Type)
		assert.True(t, cfg.TenantID == "" || cfg.TenantID == "tenant-a")
	}
	assert.True(t, ids["sys-openai"])
	assert.True(t, ids["a-openai"])

	// 租户A的 anthropic 后端：只有 sys-anthropic
	list = m.GetByTypeAndTenant("anthropic", "tenant-a")
	assert.Len(t, list, 1)
	assert.Equal(t, "sys-anthropic", list[0].ID)
}

// TestManager_SelectBackendByTenant 验证负载均衡选择
func TestManager_SelectBackendByTenant(t *testing.T) {
	m := NewManager()

	m.Add(&BackendConfig{ID: "sys-openai", Type: "openai", Enabled: true, TenantID: "", Weight: 1})
	m.Add(&BackendConfig{ID: "a-openai", Type: "openai", Enabled: true, TenantID: "tenant-a", Weight: 100})

	selected, err := m.SelectBackendByTenant("openai", "tenant-a")
	assert.NoError(t, err)
	assert.Equal(t, "a-openai", selected.ID) // 权重最高
}

// TestManager_TenantIsolation_BackwardCompatible 验证向后兼容：空 tenantID 时行为与旧版一致
func TestManager_TenantIsolation_BackwardCompatible(t *testing.T) {
	m := NewManager()

	// 只注册系统后端（无 TenantID）
	m.Add(&BackendConfig{ID: "backend-1", Name: "Backend 1", Enabled: true, TenantID: ""})
	m.Add(&BackendConfig{ID: "backend-2", Name: "Backend 2", Enabled: true, TenantID: ""})

	// 旧版 Get 方法（无租户隔离）应返回所有后端
	all := m.GetAll()
	assert.Len(t, all, 2)

	// 新版 GetByTenant("", id) 在单用户模式下应等价于 Get(id)
	cfg1, err := m.GetByTenant("", "backend-1")
	assert.NoError(t, err)
	assert.Equal(t, "backend-1", cfg1.ID)

	// ListByTenant("") 在单用户模式下应返回所有系统后端
	list := m.ListByTenant("")
	assert.Len(t, list, 2)
}
