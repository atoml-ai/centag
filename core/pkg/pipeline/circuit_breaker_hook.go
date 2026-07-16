package pipeline

import "strings"

// IsCircuitOpen reports whether a backend's circuit breaker is open.
// Wired from server startup to avoid an import cycle with the scheduler package.
var IsCircuitOpen func(backendID string) bool

// RecordCircuitOutcome records a backend request success or failure for circuit breaker stats.
var RecordCircuitOutcome func(backendID string, success bool)

func nodeBackendID(config PipelineNodeConfig, nodeConfig NodeConfig) string {
	// 正常路径使用归一化后的 Config.Backend；兼容未归一化输入时回退顶层 backend。
	if backend := strings.TrimSpace(nodeConfig.Backend); backend != "" {
		return backend
	}
	return strings.TrimSpace(config.Backend)
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

func recordNodeCircuitOutcome(backendID string, success bool, skippedDueToCircuit bool) {
	if backendID == "" || RecordCircuitOutcome == nil || skippedDueToCircuit {
		return
	}
	RecordCircuitOutcome(backendID, success)
}