package config

import (
	"strings"
)

// IsBillingOrQuotaFailure 判断是否为余额/额度类失败（应触发降级，且默认不计入熔断）。
// statusCode 可为 0（仅按消息判断）。401/403 仅在正文/消息命中计费关键词时成立，避免把鉴权错误当成可降级。
func IsBillingOrQuotaFailure(statusCode int, bodyOrMsg string) bool {
	if statusCode == 402 {
		return true
	}
	if !looksLikeBillingOrQuotaMessage(bodyOrMsg) {
		return false
	}
	// 无状态码时仅凭消息；有状态码时限在常见计费/鉴权相关响应
	switch statusCode {
	case 0, 401, 402, 403, 429:
		return true
	default:
		return statusCode >= 500
	}
}

func looksLikeBillingOrQuotaMessage(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if lower == "" {
		return false
	}
	keywords := []string{
		"creditserror",
		"credit_balance",
		"insufficient_quota",
		"insufficient_credits",
		"insufficient balance",
		"not_enough_balance",
		"not enough balance",
		"quota exceeded",
		"quota_exceeded",
		"payment_required",
		"payment required",
		"余额不足",
		"额度不足",
		"积分不足",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	// 宽松匹配：credit + (error|insufficient|balance|exhaust)
	if strings.Contains(lower, "credit") &&
		(strings.Contains(lower, "error") ||
			strings.Contains(lower, "insufficient") ||
			strings.Contains(lower, "balance") ||
			strings.Contains(lower, "exhaust")) {
		return true
	}
	return false
}

// IsRetryableStatusCode 判断 HTTP 状态码是否应触发重试/降级（热生效：每次调用读取最新配置）。
func IsRetryableStatusCode(statusCode int) bool {
	cfg := Get()
	if cfg == nil {
		return isDefaultRetryableStatusCode(statusCode)
	}
	codes := cfg.Proxy.RetryableStatusCodes
	if len(codes) == 0 {
		codes = DefaultRetryableStatusCodes()
	}
	for _, c := range codes {
		if c == statusCode {
			return true
		}
	}
	return false
}

// IsRetryableErrorCode 判断提供方错误码是否应触发重试/降级（热生效）。
func IsRetryableErrorCode(code string) bool {
	cfg := Get()
	if cfg == nil {
		return isDefaultRetryableErrorCode(code)
	}
	codes := cfg.Proxy.RetryableErrorCodes
	if len(codes) == 0 {
		codes = DefaultRetryableErrorCodes()
	}
	lower := strings.ToLower(code)
	for _, c := range codes {
		if strings.ToLower(c) == lower {
			return true
		}
	}
	return false
}

// IsTimeoutRetryable 判断超时是否触发重试/降级（热生效）。
func IsTimeoutRetryable() bool {
	cfg := Get()
	if cfg == nil || cfg.Proxy.TimeoutRetryable == nil {
		return true // 默认允许
	}
	return *cfg.Proxy.TimeoutRetryable
}

// IsNetworkRetryable 判断网络错误是否触发重试/降级（热生效）。
func IsNetworkRetryable() bool {
	cfg := Get()
	if cfg == nil || cfg.Proxy.NetworkRetryable == nil {
		return true // 默认允许
	}
	return *cfg.Proxy.NetworkRetryable
}

// IsRetryableError 综合判断一个错误是否应触发重试/降级。
// errorType 来自 pipeline 层分类："http_status" | "timeout" | "network" | "provider_error" | "billing" | "unknown"。
func IsRetryableError(errorType string, statusCode int, providerErrorCode string) bool {
	switch errorType {
	case "http_status":
		if IsRetryableStatusCode(statusCode) {
			return true
		}
		// 401 + CreditsError 等：按计费失败允许上层降级（同节点重试通常无意义，由 MaxAttempts=0 控制）
		return IsBillingOrQuotaFailure(statusCode, providerErrorCode)
	case "timeout":
		return IsTimeoutRetryable()
	case "network":
		return IsNetworkRetryable()
	case "provider_error":
		return IsRetryableErrorCode(providerErrorCode) || IsBillingOrQuotaFailure(0, providerErrorCode)
	case "billing":
		return true
	default:
		return false // 未知错误不重试（如纯鉴权 401/403）
	}
}

// ── 纯函数版本（用于无 DB 的场景如 seeder） ────────────────────────────────

func isDefaultRetryableStatusCode(statusCode int) bool {
	for _, c := range DefaultRetryableStatusCodes() {
		if c == statusCode {
			return true
		}
	}
	return false
}

func isDefaultRetryableErrorCode(code string) bool {
	lower := strings.ToLower(code)
	for _, c := range DefaultRetryableErrorCodes() {
		if strings.ToLower(c) == lower {
			return true
		}
	}
	return false
}
