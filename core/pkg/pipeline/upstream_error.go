package pipeline

import (
	"encoding/json"
	"net/http"
	"strings"
)

// upstreamErrorEnvelope 匹配 OpenAI 风格错误体：{"error": {...}} 或 {"error": "msg"}。
type upstreamErrorEnvelope struct {
	Error json.RawMessage `json:"error"`
}

type upstreamErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// DetectUpstreamErrorPayload 识别「HTTP 2xx 但 body 是错误对象」的上游响应
// （P1-T3：部分上游以 200 返回业务错误，透传会让客户端误判成功）。
// 返回映射后的 HTTP 状态码与是否命中；非 JSON / 无 error 字段时返回 false。
func DetectUpstreamErrorPayload(contentType string, body []byte) (int, bool) {
	if !strings.Contains(strings.ToLower(contentType), "json") {
		return 0, false
	}
	trimmed := strings.TrimSpace(string(body))
	if !strings.HasPrefix(trimmed, "{") {
		return 0, false
	}
	var env upstreamErrorEnvelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil || len(env.Error) == 0 {
		return 0, false
	}
	// "error": null 是合法成功响应（OpenAI SDK 常见），不算错误
	if string(env.Error) == "null" {
		return 0, false
	}
	// error 为字符串（如 {"error":"not found"}）
	if env.Error[0] != '{' {
		return http.StatusBadGateway, true
	}
	var detail upstreamErrorDetail
	if err := json.Unmarshal(env.Error, &detail); err != nil {
		return http.StatusBadGateway, true
	}
	return mapUpstreamErrorStatus(detail), true
}

// mapUpstreamErrorStatus 按错误 type/code 粗映射到规范 HTTP 状态。
func mapUpstreamErrorStatus(d upstreamErrorDetail) int {
	joined := strings.ToLower(d.Type + " " + d.Code)
	switch {
	case strings.Contains(joined, "invalid_request") || strings.Contains(joined, "invalid_prompt"):
		return http.StatusBadRequest
	case strings.Contains(joined, "auth") || strings.Contains(joined, "api_key") || strings.Contains(joined, "unauthorized"):
		return http.StatusUnauthorized
	case strings.Contains(joined, "permission") || strings.Contains(joined, "forbidden"):
		return http.StatusForbidden
	case strings.Contains(joined, "not_found"):
		return http.StatusNotFound
	case strings.Contains(joined, "rate_limit") || strings.Contains(joined, "rate limit"):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}
