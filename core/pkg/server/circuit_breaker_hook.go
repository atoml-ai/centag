package server

import (
	"time"

	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/scheduler"
)

func buildCircuitBreakerManager() *scheduler.CircuitBreakerManager {
	cfg := scheduler.DefaultCircuitBreakerConfig()
	// 从 ProxyConfig.CircuitBreaker 读取用户配置（热生效）
	if c := config.Get(); c != nil && c.Proxy.CircuitBreaker != nil {
		cb := c.Proxy.CircuitBreaker
		if cb.FailureThreshold > 0 {
			cfg.FailureThreshold = cb.FailureThreshold
		}
		if cb.SuccessThreshold > 0 {
			cfg.SuccessThreshold = cb.SuccessThreshold
		}
		if cb.TimeoutSec > 0 {
			cfg.Timeout = time.Duration(cb.TimeoutSec) * time.Second
		}
		if cb.WindowSec > 0 {
			cfg.WindowDuration = time.Duration(cb.WindowSec) * time.Second
		}
		logger.Infof("[CircuitBreaker] Using custom config: failure_threshold=%d, success_threshold=%d, timeout=%ds, window=%ds",
			cfg.FailureThreshold, cfg.SuccessThreshold, int(cfg.Timeout.Seconds()), int(cfg.WindowDuration.Seconds()))
	}
	// 初始化全局单例（供 backend plugin 等外部包使用）
	circuitbreaker.Init(cfg)
	return circuitbreaker.Get()
}

// hotReloadCircuitBreaker 从 ProxyConfig 热更新熔断器配置。
func hotReloadCircuitBreaker() {
	c := config.Get()
	if c == nil || c.Proxy.CircuitBreaker == nil {
		return
	}
	cb := c.Proxy.CircuitBreaker
	schedCfg := scheduler.CircuitBreakerConfig{
		FailureThreshold: cb.FailureThreshold,
		SuccessThreshold: cb.SuccessThreshold,
		Timeout:          time.Duration(cb.TimeoutSec) * time.Second,
		WindowDuration:   time.Duration(cb.WindowSec) * time.Second,
	}
	weight := cb.RateLimitWeight
	if weight <= 0 {
		weight = 2
	}
	circuitbreaker.UpdateConfig(schedCfg, weight)
	logger.Infof("[CircuitBreaker] Config hot-reloaded: failure_threshold=%d, rate_limit_weight=%d",
		schedCfg.FailureThreshold, weight)
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