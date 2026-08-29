package pipeline

import (
	"context"
	"net/http"
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
)

// IsCircuitOpen reports whether a backend's circuit breaker is open.
// Wired from server startup to avoid an import cycle with the scheduler package.
var IsCircuitOpen func(backendID string) bool

// RecordCircuitOutcome records a backend request success or failure for circuit breaker stats.
var RecordCircuitOutcome func(backendID string, success bool)

func nodeBackendID(config PipelineNodeConfig, nodeConfig NodeConfig) string {
	return nodeBackendIDContext(context.Background(), config, nodeConfig)
}

func nodeBackendIDContext(ctx context.Context, config PipelineNodeConfig, nodeConfig NodeConfig) string {
	// 正常路径使用归一化后的 Config.Backend；兼容未归一化输入时回退顶层 backend。
	backend := strings.TrimSpace(nodeConfig.Backend)
	if backend == "" {
		backend = strings.TrimSpace(config.Backend)
	}
	if backend == "" {
		return ""
	}
	// 熔断键必须用解析后的真实 backend ID，否则 {{system.default_backend}} 会与实际上游统计错位。
	resolved, _ := ResolveVirtualVarsContext(ctx, backend, "")
	return strings.TrimSpace(resolved)
}

// nodeModel 解析节点配置中的真实模型名（与 nodeBackendID 对称，供熔断豁免判定使用）。
func nodeModel(config PipelineNodeConfig, nodeConfig NodeConfig) string {
	return nodeModelContext(context.Background(), config, nodeConfig)
}

func nodeModelContext(ctx context.Context, config PipelineNodeConfig, nodeConfig NodeConfig) string {
	model := strings.TrimSpace(nodeConfig.Model)
	if model == "" {
		model = strings.TrimSpace(config.Model)
	}
	if model == "" {
		return ""
	}
	if strings.Contains(model, "{{") {
		_, resolved := ResolveVirtualVarsContext(ctx, "", model)
		return strings.TrimSpace(resolved)
	}
	return model
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

// isModelNotFoundNodeError 模型不存在：同样触发降级且不计入熔断。
func isModelNotFoundNodeError(err error) bool {
	return err != nil && isUpstreamModelOrPlaceholderError(err.Error())
}

// isTransientRateLimitError 瞬时性限流豁免：不计入熔断。
//   - 临时限流（429 + "try again later" 等）：等待后重试同 Key 即可恢复，任何档位都豁免；
//   - 免费档限流高峰（429 / "rate limit" 文案）：避免免费档流量把整个后端打成 open，
//     误伤同后端付费模型。
//
// 永久性额度耗尽（FreeUsageLimitError / CreditsError 等 billing 失败）不属于瞬时限流，
// 必须如实计数——额度不会自愈，达到阈值熔断、靠半开探测恢复，否则限额后端
// 在熔断面板上永远显示正常。
func isTransientRateLimitError(err error, model string) bool {
	if err == nil {
		return false
	}
	typ, code, _ := classifyNodeError(err)
	if typ == "rate_limit" {
		return true
	}
	if config.IsBillingOrQuotaFailure(0, err.Error()) {
		return false
	}
	if backend.ModelHasFreeTier(model) {
		if typ == "http_status" && code == http.StatusTooManyRequests {
			return true
		}
		return strings.Contains(strings.ToLower(err.Error()), "rate limit")
	}
	return false
}

// circuitFallbackRescue 检测节点内兜底救回（transparent_forward 的 billing/model fallback）。
// 救回时节点返回 err == nil，若照常记账会把「主后端失败」记成「主后端成功」。
// 返回值：(主后端ID, 触发类型, 是否救回)。触发类型为 "billing"（计费/额度）或
// "model_not_found"（模型不存在）；旧元数据无 fallback_trigger 时按 fallback_reason 分类。
func circuitFallbackRescue(out *NodeOutput) (fromBackend, trigger string, rescued bool) {
	if out == nil || out.Metadata == nil || out.Metadata["billing_fallback_used"] != true {
		return "", "", false
	}
	fromBackend = firstMetaString(out.Metadata, "billing_fallback_from_backend", "fallback_from_backend")
	trigger = firstMetaString(out.Metadata, "fallback_trigger")
	if trigger == "" {
		if reason := firstMetaString(out.Metadata, "fallback_reason"); reason != "" && !config.IsBillingOrQuotaFailure(0, reason) {
			trigger = "model_not_found"
		} else {
			trigger = "billing"
		}
	}
	return fromBackend, trigger, fromBackend != ""
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

func recordNodeCircuitOutcome(backendID, model string, success bool, skippedDueToCircuit bool, err error) {
	if backendID == "" || RecordCircuitOutcome == nil || skippedDueToCircuit {
		return
	}
	if !success && isTransientRateLimitError(err, model) {
		return
	}
	RecordCircuitOutcome(backendID, success)
}
