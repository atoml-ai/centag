package config

import (
	"strings"
)

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
// errorType 来自 pipeline 层分类："http_status" | "timeout" | "network" | "provider_error" | "unknown"。
func IsRetryableError(errorType string, statusCode int, providerErrorCode string) bool {
	switch errorType {
	case "http_status":
		return IsRetryableStatusCode(statusCode)
	case "timeout":
		return IsTimeoutRetryable()
	case "network":
		return IsNetworkRetryable()
	case "provider_error":
		return IsRetryableErrorCode(providerErrorCode)
	default:
		return false // 未知错误不重试（如 401/403 配置错误）
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
