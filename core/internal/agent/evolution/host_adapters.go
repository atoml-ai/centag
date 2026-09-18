package evolution

import (
	"context"
	"fmt"
	"sync"
)

// TargetApplier 将提案参数真实应用到宿主组件。
// 在 Apply 成功时由 Runtime 调用；失败返回 error 以阻止状态迁移到 applied。
type TargetApplier interface {
	// Target 返回适配的目标名称，需与 proposal.go 白名单一致。
	Target() string
	// Apply 将参数写入真实运行组件。
	Apply(ctx context.Context, params TargetKeyState) error
	// Rollback 恢复到前一次快照参数。
	Rollback(ctx context.Context, before TargetKeyState) error
}

// AdapterRegistry 管理所有 TargetApplier（并发安全，可在运行期注册）。
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]TargetApplier
}

// NewAdapterRegistry 创建空注册表。
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{adapters: make(map[string]TargetApplier)}
}

// Register 注册一个 TargetApplier（同名覆盖）。
func (r *AdapterRegistry) Register(a TargetApplier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[a.Target()] = a
}

// Get 获取指定目标的适配器，不存在返回 nil。
func (r *AdapterRegistry) Get(target string) TargetApplier {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[target]
}

// Len 已注册适配器数（测试/观测用）。
func (r *AdapterRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.adapters)
}

// HostAppliers 宿主应用面注入点：生产由 wiring 注册真实适配器；
// 未注册适配器的目标在 Apply/Rollback 时显式失败（fail-closed，P0-1）。
var HostAppliers = NewAdapterRegistry()

// noApplierError 未注册适配器时的失败语义。
func noApplierError(target string) error {
	return fmt.Errorf("target %q 无宿主适配器注册，拒绝应用（P0-1 fail-closed）", target)
}
