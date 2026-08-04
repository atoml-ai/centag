package pipeline

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPipelineRegistry_GetByTenant_Priority 验证租户专属优先于系统预设
func TestPipelineRegistry_GetByTenant_Priority(t *testing.T) {
	r := NewPipelineRegistry()

	// 注册系统预设流水线
	sysPipeline := &AgentPatternPipeline{
		ID:      "pipeline-1",
		Name:    "System Pipeline",
		Version: "1.0",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}
	err := r.Register(sysPipeline)
	assert.NoError(t, err)

	// 注册租户A专属流水线（同名覆盖）
	tenantAPipeline := &AgentPatternPipeline{
		ID:      "pipeline-1",
		Name:    "Tenant A Custom Pipeline",
		Version: "2.0",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
			{ID: "node2", Type: NodeTypeProcessor, Backend: "b", Model: "m"},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}
	err = r.RegisterForTenant("tenant-a", tenantAPipeline)
	assert.NoError(t, err)

	// 租户A获取 pipeline-1 应返回租户专属版本
	p := r.GetByTenant("tenant-a", "pipeline-1")
	assert.NotNil(t, p)
	assert.Equal(t, "Tenant A Custom Pipeline", p.Name)
	assert.Equal(t, "2.0", p.Version)

	// 租户B获取 pipeline-1 应返回系统预设版本（无覆盖）
	p = r.GetByTenant("tenant-b", "pipeline-1")
	assert.NotNil(t, p)
	assert.Equal(t, "System Pipeline", p.Name)
	assert.Equal(t, "1.0", p.Version)

	// 系统模式获取应返回系统预设
	p = r.GetByTenant("", "pipeline-1")
	assert.NotNil(t, p)
	assert.Equal(t, "System Pipeline", p.Name)
}

// TestPipelineRegistry_GetByTenant_NotFound 验证不存在的流水线
func TestPipelineRegistry_GetByTenant_NotFound(t *testing.T) {
	r := NewPipelineRegistry()

	p := r.GetByTenant("tenant-a", "nonexistent")
	assert.Nil(t, p)
}

func TestPipelineRegistry_ExistsAnywhere_IncludesTenant(t *testing.T) {
	r := NewPipelineRegistry()
	err := r.RegisterForTenant("user:2", &AgentPatternPipeline{
		ID: "transparent-proxy-copy-1", Name: "Mine", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	assert.True(t, r.ExistsAnywhere("transparent-proxy-copy-1"))
	assert.False(t, r.ExistsAnywhere("missing"))
	// Get (system-only) must not see tenant-only copy
	assert.Nil(t, r.Get("transparent-proxy-copy-1"))
}

// TestPipelineRegistry_ListByTenant_Merge 验证列表合并逻辑
func TestPipelineRegistry_ListByTenant_Merge(t *testing.T) {
	r := NewPipelineRegistry()

	// 系统预设
	err := r.Register(&AgentPatternPipeline{
		ID: "sys-1", Name: "System 1", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	err = r.Register(&AgentPatternPipeline{
		ID: "sys-2", Name: "System 2", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 租户A专属（覆盖 sys-1，新增 a-1）
	err = r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "sys-1", Name: "Tenant A System 1", Version: "2.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	err = r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "a-1", Name: "Tenant A Only", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 租户A列表：sys-1(覆盖版) + sys-2 + a-1 = 3个
	aList := r.ListByTenant("tenant-a")
	assert.Len(t, aList, 3)

	names := make(map[string]string)
	for _, p := range aList {
		names[p.ID] = p.Name
	}
	assert.Equal(t, "Tenant A System 1", names["sys-1"]) // 覆盖版
	assert.Equal(t, "System 2", names["sys-2"])          // 系统预设
	assert.Equal(t, "Tenant A Only", names["a-1"])       // 租户专属

	// 租户B列表：只有系统预设 = 2个
	bList := r.ListByTenant("tenant-b")
	assert.Len(t, bList, 2)

	// 系统模式列表：只有系统预设 = 2个
	sysList := r.ListByTenant("")
	assert.Len(t, sysList, 2)
}

// TestPipelineRegistry_ExistsInTenant 验证存在性检查
func TestPipelineRegistry_ExistsInTenant(t *testing.T) {
	r := NewPipelineRegistry()

	err := r.Register(&AgentPatternPipeline{
		ID: "sys-pipe", Name: "System", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	err = r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "a-pipe", Name: "Tenant A", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 系统预设对所有租户可见
	assert.True(t, r.ExistsInTenant("tenant-a", "sys-pipe"))
	assert.True(t, r.ExistsInTenant("tenant-b", "sys-pipe"))

	// 租户专属只对特定租户可见
	assert.True(t, r.ExistsInTenant("tenant-a", "a-pipe"))
	assert.False(t, r.ExistsInTenant("tenant-b", "a-pipe"))

	// 不存在的流水线
	assert.False(t, r.ExistsInTenant("tenant-a", "nonexistent"))
}

// TestPipelineRegistry_DeleteScoped_ClearsStaleGlobal 删除租户流水线后不应残留全局内存副本
func TestPipelineRegistry_DeleteScoped_ClearsStaleGlobal(t *testing.T) {
	r := NewPipelineRegistry()

	// 模拟系统预设已加载到全局 map
	err := r.Register(&AgentPatternPipeline{
		ID: "my-pipe", Name: "System Copy", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 租户创建/接管同 ID 流水线（RegisterForTenant 会清除全局副本）
	err = r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "my-pipe", Name: "Tenant Copy", Version: "2.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 删除租户流水线后列表应立即为空（含清除陈旧全局内存副本）
	err = r.DeleteScoped("tenant-a", "my-pipe")
	assert.NoError(t, err)
	assert.Nil(t, r.GetByTenant("tenant-a", "my-pipe"))
	assert.Len(t, r.ListByTenant("tenant-a"), 0)
}

// TestPipelineRegistry_RemoveFromTenant 验证租户级删除
func TestPipelineRegistry_RemoveFromTenant(t *testing.T) {
	r := NewPipelineRegistry()

	err := r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "a-pipe", Name: "Tenant A", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 删除前存在
	assert.True(t, r.ExistsInTenant("tenant-a", "a-pipe"))

	// 删除
	r.RemoveFromTenant("tenant-a", "a-pipe")

	// 删除后不存在
	assert.False(t, r.ExistsInTenant("tenant-a", "a-pipe"))
}

// TestPipelineRegistry_Remove_Global 验证全局删除会级联删除所有租户的副本
func TestPipelineRegistry_Remove_Global(t *testing.T) {
	r := NewPipelineRegistry()

	// 注册系统预设
	err := r.Register(&AgentPatternPipeline{
		ID: "shared-pipe", Name: "Shared", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 注册租户副本
	err = r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID: "shared-pipe", Name: "Tenant A Shared", Version: "2.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 全局删除
	r.Remove("shared-pipe")

	// 系统预设和租户副本都应被删除
	assert.False(t, r.Exists("shared-pipe"))
	assert.False(t, r.ExistsInTenant("tenant-a", "shared-pipe"))
}

// TestPipelineRegistry_RegisterForTenant_NilPipeline 验证空流水线校验
func TestPipelineRegistry_RegisterForTenant_NilPipeline(t *testing.T) {
	r := NewPipelineRegistry()
	err := r.RegisterForTenant("tenant-a", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline cannot be nil")
}

// TestPipelineRegistry_RegisterForTenant_InvalidPipeline 验证无效流水线校验
func TestPipelineRegistry_RegisterForTenant_InvalidPipeline(t *testing.T) {
	r := NewPipelineRegistry()
	err := r.RegisterForTenant("tenant-a", &AgentPatternPipeline{
		ID:   "", // 无效：缺少 ID
		Name: "Invalid",
	})
	assert.Error(t, err)
}

// TestPipelineRegistry_ConcurrentAccess 验证并发安全
func TestPipelineRegistry_ConcurrentAccess(t *testing.T) {
	r := NewPipelineRegistry()

	// 先注册系统预设
	err := r.Register(&AgentPatternPipeline{
		ID: "sys-pipe", Name: "System", Version: "1.0",
		Nodes:        []PipelineNodeConfig{{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"}},
		GlobalConfig: DefaultGlobalConfig(),
	})
	assert.NoError(t, err)

	// 并发注册租户流水线
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			pipe := &AgentPatternPipeline{
				ID:   fmt.Sprintf("pipe-%d", idx),
				Name: fmt.Sprintf("Pipe %d", idx),
				Nodes: []PipelineNodeConfig{
					{ID: "n1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				},
				GlobalConfig: DefaultGlobalConfig(),
			}
			_ = r.RegisterForTenant("tenant-concurrent", pipe)
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	// 验证所有流水线都注册成功
	list := r.ListByTenant("tenant-concurrent")
	assert.Len(t, list, 51) // 50 租户专属 + 1 系统预设
}
