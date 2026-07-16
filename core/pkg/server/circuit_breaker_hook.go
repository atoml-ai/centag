package server

import (
	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

func buildCircuitBreakerManager() *scheduler.CircuitBreakerManager {
	cfg := scheduler.DefaultCircuitBreakerConfig()
	// 初始化全局单例（供 backend plugin 等外部包使用）
	circuitbreaker.Init(cfg)
	return circuitbreaker.Get()
}

func wireCircuitBreaker(cbManager *scheduler.CircuitBreakerManager) {
	if cbManager == nil {
		return
	}

	pipeline.IsCircuitOpen = func(backendID string) bool {
		if backendID == "" {
			return false
		}
		return cbManager.IsOpen(backendID)
	}

	pipeline.RecordCircuitOutcome = func(backendID string, success bool) {
		if backendID == "" {
			return
		}
		if success {
			cbManager.RecordSuccess(backendID)
		} else {
			cbManager.RecordFailure(backendID)
		}
	}

	logger.Info("[CircuitBreaker] Manager initialized and wired to pipeline + global singleton")
}