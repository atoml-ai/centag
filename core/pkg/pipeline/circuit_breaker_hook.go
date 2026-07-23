package pipeline

import (
	"strings"

	"centag/core/pkg/config"
)

// IsCircuitOpen reports whether a backend's circuit breaker is open.
// Wired from server startup to avoid an import cycle with the scheduler package.
var IsCircuitOpen func(backendID string) bool

// RecordCircuitOutcome records a backend request success or failure for circuit breaker stats.
var RecordCircuitOutcome func(backendID string, success bool)

func nodeBackendID(config PipelineNodeConfig, nodeConfig NodeConfig) string {
	// 正常路径使用归一化后的 Config.Backend；兼容未归一化输入时回退顶层 backend。
	backend := strings.TrimSpace(nodeConfig.Backend)
	if backend == "" {
		backend = strings.TrimSpace(config.Backend)
	}
	if backend == "" {
		return ""
	}
	// 熔断键必须用解析后的真实 backend ID，否则 {{system.default_backend}} 会与实际上游统计错位。
	resolved, _ := ResolveVirtualVars(backend, "")
	return strings.TrimSpace(resolved)
}

func isCircuitOpenForBackend(backendID string) bool {
	if backendID == "" || IsCircuitOpen == nil {
		return false
	}
	return IsCircuitOpen(backendID)
}

func isCircuitBreakerSkipError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "circuit breaker open")
}

// isBillingQuotaError 余额/额度失败：应触发降级，但不计入熔断（否则同后端免费模型也被挡住）。
func isBillingQuotaError(err error) bool {
	return err != nil && config.IsBillingOrQuotaFailure(0, err.Error())
}

// isModelNotFoundNodeError 模型不存在：同样触发降级且不计入熔断。
func isModelNotFoundNodeError(err error) bool {
	return err != nil && isUpstreamModelOrPlaceholderError(err.Error())
}

func isFixedEgressNodeConfig(cfg NodeConfig) bool {
	if cfg.CustomConfig == nil {
		return false
	}
	if s, ok := cfg.CustomConfig["route_policy"].(string); ok {
		switch strings.TrimSpace(strings.ToLower(s)) {
		case "fixed", "fixed_egress", "direct":
			return true
		case "match_model", "match-model", "loose":
			return false
		}
	}
	if v, ok := cfg.CustomConfig["fixed_egress"].(bool); ok {
		return v
	}
	return false
}

func recordNodeCircuitOutcome(backendID string, success bool, skippedDueToCircuit bool) {
	if backendID == "" || RecordCircuitOutcome == nil || skippedDueToCircuit {
		return
	}
	RecordCircuitOutcome(backendID, success)
}
