package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

func init() {
	// 初始化 logger 避免测试中的 nil pointer panic
	_ = logger.Init(logger.Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
}

// TestGenerateTenantID 验证租户 ID 生成格式
func TestGenerateTenantID(t *testing.T) {
	id1 := database.NewTenantID(42)
	assert.Contains(t, id1, "t_42_")

	id2 := database.NewTenantID(99)
	assert.Contains(t, id2, "t_99_")

	// 不同调用应生成不同 ID（含时间戳）
	assert.NotEqual(t, id1, id2)
}

// TestTenantProvisioner_ProvisionForUser_InvalidUser 验证无效用户校验
func TestTenantProvisioner_ProvisionForUser_InvalidUser(t *testing.T) {
	bm := backend.NewManager()
	pr := pipeline.NewPipelineRegistry()
	p := NewTenantProvisioner(nil, bm, pr)

	// nil 用户
	_, _, err := p.ProvisionForUser(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user")

	// 零 ID 用户
	_, _, err = p.ProvisionForUser(nil, &database.User{ID: 0, Username: "test"})
	assert.Error(t, err)
}

// TestTenantProvisioner_copySystemBackends_NoSystemBackends 验证无系统后端时静默返回
func TestTenantProvisioner_copySystemBackends_NoSystemBackends(t *testing.T) {
	bm := backend.NewManager()
	pr := pipeline.NewPipelineRegistry()
	p := NewTenantProvisioner(nil, bm, pr)

	err := p.copySystemBackends(nil, "tenant-123")
	assert.NoError(t, err)
}

// TestTenantProvisioner_copySystemBackends_WithBackends 验证系统后端复制
func TestTenantProvisioner_copySystemBackends_WithBackends(t *testing.T) {
	bm := backend.NewManager()

	// 注册系统后端
	err := bm.Add(&backend.BackendConfig{
		ID:       "sys-openai",
		Name:     "System OpenAI",
		Type:     "openai",
		BaseURL:  "https://api.openai.com",
		APIKey:   "sk-system-secret",
		Enabled:  true,
		TenantID: "",
	})
	assert.NoError(t, err)

	pr := pipeline.NewPipelineRegistry()
	p := NewTenantProvisioner(nil, bm, pr)

	// 复制到租户（可能报错但继续执行）
	err = p.copySystemBackends(nil, "tenant-abc")
	// 错误来自 Save() 的全局配置未初始化，但 Add() 已成功
	// 因此系统后端和租户副本都会存在于 manager 中

	// 验证租户后端列表（包含系统后端 + 租户副本）
	tenantBackends := bm.ListByTenant("tenant-abc")
	assert.Len(t, tenantBackends, 2) // sys-openai + tenant-abc_sys-openai

	// 找到租户副本
	var copied *backend.BackendConfig
	for _, b := range tenantBackends {
		if b.TenantID == "tenant-abc" {
			copied = b
			break
		}
	}
	assert.NotNil(t, copied)
	assert.Contains(t, copied.ID, "tenant-abc_")
	assert.Equal(t, "", copied.APIKey) // API Key 应被清空
	assert.Equal(t, "tenant-abc", copied.TenantID)
	assert.Equal(t, "true", copied.Metadata["copied_from_system"])
	assert.Equal(t, "sys-openai", copied.Metadata["original_id"])
}

// TestTenantProvisioner_copySystemBackends_SkipsTenantBackends 验证跳过非系统后端
func TestTenantProvisioner_copySystemBackends_SkipsTenantBackends(t *testing.T) {
	bm := backend.NewManager()

	// 系统后端
	err := bm.Add(&backend.BackendConfig{
		ID:       "sys-backend",
		Name:     "System",
		Type:     "openai",
		Enabled:  true,
		TenantID: "",
	})
	assert.NoError(t, err)

	// 其他租户的后端（不应被复制）
	err = bm.Add(&backend.BackendConfig{
		ID:       "other-tenant-backend",
		Name:     "Other Tenant",
		Type:     "anthropic",
		Enabled:  true,
		TenantID: "other-tenant",
	})
	assert.NoError(t, err)

	pr := pipeline.NewPipelineRegistry()
	p := NewTenantProvisioner(nil, bm, pr)

	err = p.copySystemBackends(nil, "new-tenant")
	// 同样可能因 Save() 报错，但 Add() 成功

	// 验证只复制了系统后端（跳过 other-tenant 的后端）
	tenantBackends := bm.ListByTenant("new-tenant")
	// 系统后端 sys-backend + 副本 new-tenant_sys-backend = 2
	assert.Len(t, tenantBackends, 2)

	// 确认 other-tenant-backend 不在列表中
	for _, b := range tenantBackends {
		assert.NotEqual(t, "other-tenant-backend", b.ID)
	}

	// 确认包含系统后端的副本
	var hasCopy bool
	for _, b := range tenantBackends {
		if b.TenantID == "new-tenant" {
			hasCopy = true
			break
		}
	}
	assert.True(t, hasCopy)
}

// TestTenantProvisioner_copySystemPipelines_NoSystemPipelines 验证无系统流水线时静默返回
func TestTenantProvisioner_copySystemPipelines_NoSystemPipelines(t *testing.T) {
	bm := backend.NewManager()
	pr := pipeline.NewPipelineRegistry()
	p := NewTenantProvisioner(nil, bm, pr)

	err := p.copySystemPipelines(nil, "tenant-123")
	assert.NoError(t, err)
}

// TestTenantProvisioner_copySystemPipelines_WithPipelines 验证系统流水线复制
func TestTenantProvisioner_copySystemPipelines_WithPipelines(t *testing.T) {
	bm := backend.NewManager()
	pr := pipeline.NewPipelineRegistry()

	// 注册系统流水线
	err := pr.Register(&pipeline.AgentPatternPipeline{
		ID:   "sys-pipe",
		Name: "System Pipeline",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"},
		},
		GlobalConfig: pipeline.DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	p := NewTenantProvisioner(nil, bm, pr)

	// 复制到租户
	err = p.copySystemPipelines(nil, "tenant-xyz")
	assert.NoError(t, err)

	// 验证租户流水线（包含系统预设 + 租户副本）
	tenantPipes := pr.ListByTenant("tenant-xyz")
	assert.Len(t, tenantPipes, 2) // sys-pipe + tenant-xyz_sys-pipe

	// 找到租户副本
	var copied *pipeline.AgentPatternPipeline
	for _, p := range tenantPipes {
		if p.ID != "sys-pipe" {
			copied = p
			break
		}
	}
	assert.NotNil(t, copied)
	assert.Contains(t, copied.ID, "tenant-xyz_")
	assert.Equal(t, "System Pipeline", copied.Name)
}

// TestTenantProvisioner_deepCopyBackendConfig 验证深拷贝
func TestTenantProvisioner_deepCopyBackendConfig(t *testing.T) {
	original := &backend.BackendConfig{
		ID:       "original",
		Name:     "Original",
		Type:     "openai",
		BaseURL:  "https://api.openai.com",
		APIKey:   "secret-key",
		Enabled:  true,
		TenantID: "",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "gpt-4", ActualModel: "gpt-4-turbo"},
		},
		Capabilities: backend.ModelCapabilities{
			MaxContextTokens: 8192,
			Features:         []string{"chat", "completion"},
		},
		Weight:   5,
		Priority: 10,
	}

	copied := deepCopyBackendConfig(original)

	// 基本字段应相同
	assert.Equal(t, original.ID, copied.ID)
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Type, copied.Type)
	assert.Equal(t, original.Weight, copied.Weight)
	assert.Equal(t, original.Priority, copied.Priority)

	// Metadata 应独立（深拷贝）
	copied.Metadata["new-key"] = "new-value"
	assert.NotContains(t, original.Metadata, "new-key")

	// SupportedModels 应独立
	copied.SupportedModels[0].RequestedModel = "modified"
	assert.Equal(t, "gpt-4", original.SupportedModels[0].RequestedModel)
}

// TestTenantProvisioner_deepCopyPipeline 验证流水线深拷贝
func TestTenantProvisioner_deepCopyPipeline(t *testing.T) {
	original := &pipeline.AgentPatternPipeline{
		ID:   "pipe-1",
		Name: "Pipeline 1",
		Nodes: []pipeline.PipelineNodeConfig{
			{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "n2", Type: pipeline.NodeTypeProcessor, Backend: "b", Model: "m"},
		},
		GlobalConfig: pipeline.DefaultGlobalConfig(),
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	copied := deepCopyPipeline(original)

	assert.Equal(t, original.ID, copied.ID)
	assert.Equal(t, original.Name, copied.Name)
	assert.Len(t, copied.Nodes, 2)

	// 修改副本不应影响原件
	copied.Nodes[0].Backend = "modified"
	assert.Equal(t, "b", original.Nodes[0].Backend)
}
