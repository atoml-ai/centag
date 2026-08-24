package circuitbreaker

import (
	"sync"

	"centag/core/pkg/scheduler"
)

// global 全局熔断器管理器实例
var (
	global     *scheduler.CircuitBreakerManager
	globalOnce sync.Once
)

// Init 初始化全局熔断器管理器（由 server 启动时调用）
func Init(config scheduler.CircuitBreakerConfig) {
	globalOnce.Do(func() {
		global = scheduler.NewCircuitBreakerManager(config)
	})
}

// Get 获取全局熔断器管理器
func Get() *scheduler.CircuitBreakerManager {
	return global
}

// Allow 检查是否允许请求（便捷函数）
func Allow(backendID string) bool {
	if global == nil {
		return true
	}
	return global.Allow(backendID)
}

// RecordSuccess 记录成功（便捷函数）
func RecordSuccess(backendID string) {
	if global == nil {
		return
	}
	global.RecordSuccess(backendID)
}

// RecordFailure 记录失败（便捷函数）
func RecordFailure(backendID string) {
	if global == nil {
		return
	}
	global.RecordFailure(backendID)
}

// IsOpen 检查熔断器是否打开（便捷函数）
func IsOpen(backendID string) bool {
	if global == nil {
		return false
	}
	return global.IsOpen(backendID)
}

// GetHealthyBackends 过滤出健康的后端 ID 列表
func GetHealthyBackends(backendIDs []string) []string {
	if global == nil {
		return backendIDs
	}
	return global.GetHealthyBackends(backendIDs)
}

// GetAllStates 获取所有后端熔断器状态
func GetAllStates() map[string]scheduler.CircuitState {
	if global == nil {
		return nil
	}
	return global.GetAllStates()
}

// GetAllDetailedStates 获取所有后端熔断器的实时快照（供 WebUI 展示）。
func GetAllDetailedStates() []scheduler.CircuitBreakerSnapshot {
	if global == nil {
		return nil
	}
	return global.GetAllDetailedStates()
}

// Reset 重置指定后端的熔断器
func Reset(backendID string) {
	if global == nil {
		return
	}
	cb := global.Get(backendID)
	if cb != nil {
		cb.Reset()
	}
}

// UpdateConfig 热更新全局熔断器配置（无需重启）。
func UpdateConfig(config scheduler.CircuitBreakerConfig, rateLimitWeight int) {
	if global == nil {
		return
	}
	global.UpdateConfig(config, rateLimitWeight)
}

// RecordRateLimitFailure 记录429限流失败（自动应用加重系数）。
func RecordRateLimitFailure(backendID string) {
	if global == nil {
		return
	}
	global.RecordRateLimitFailure(backendID)
}
