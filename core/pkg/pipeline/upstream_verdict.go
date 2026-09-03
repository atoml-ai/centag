package pipeline

import (
	"encoding/json"
	"net/http"
	"strings"
)

// UpstreamVerdict 结构判定器对上游响应体的三态判定结果。
// 判定原则（技术方案 §2.2）：body 结构 > 状态码；parse-then-check 顶层键，禁止子串匹配；
// 仅显式错误结构判失败，无法识别时由调用方维持既有行为（零误报优先）。
type UpstreamVerdict int

const (
	// VerdictSuccess 显式成功结构（chat.completion / Responses / Anthropic / Gemini 形状）。
	VerdictSuccess UpstreamVerdict = iota
	// VerdictUpstreamError 显式错误结构（顶层 error 键 / type:"error" / SSE error 事件）。
	VerdictUpstreamError
	// VerdictIndeterminate 无法识别（调用方维持既有行为，如 5xx 走 IsRetryableStatusCode、其余透传）。
	VerdictIndeterminate
)

// classifyUpstreamResponse 按响应体结构判定上游响应真伪（技术方案 §3.1 判定矩阵）。
// 纯函数：不读配置、无状态；statusCode 仅保留调用方上下文，矩阵真源是 body
// ——状态码由网关逐跳给出、可能标错（如上游故障标 400），body 才是端到端最终事实。
// 判定顺序（按序短路）：
//  1. body 为空 → Indeterminate（空响应由 transparentOutputIsEmpty 单独负责）
//  2. JSON 顶层存在 "error" 键 → UpstreamError（OpenAI 形）
//  3. JSON 顶层 "type" == "error" → UpstreamError（Anthropic 形）
//  4. JSON 顶层存在 choices / output / candidates，或 content / data（数组形）→ Success
//  5. SSE：任一 data 行 payload 顶层有 "error"，或存在 event: error 行 → UpstreamError
//  6. 其余 → Indeterminate
func classifyUpstreamResponse(statusCode int, body string) UpstreamVerdict {
	_ = statusCode
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return VerdictIndeterminate
	}
	if strings.HasPrefix(trimmed, "{") {
		return classifyUpstreamJSON(trimmed)
	}
	if strings.Contains(trimmed, "\n") || strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, "event:") {
		return sseClassify(trimmed)
	}
	return VerdictIndeterminate
}

// classifyUpstreamJSON 对顶层 JSON 对象按判定矩阵 2/3/4 行短路判定。
func classifyUpstreamJSON(trimmed string) UpstreamVerdict {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &top); err != nil {
		return VerdictIndeterminate
	}
	if _, ok := top["error"]; ok {
		return VerdictUpstreamError
	}
	if raw, ok := top["type"]; ok {
		var t string
		if err := json.Unmarshal(raw, &t); err == nil && strings.EqualFold(t, "error") {
			return VerdictUpstreamError
		}
	}
	for _, key := range []string{"choices", "output", "candidates"} {
		if _, ok := top[key]; ok {
			return VerdictSuccess
		}
	}
	// content / data 仅数组形计入（Anthropic content[]、列表端点 data[]）；
	// 字符串值不计入（防普通 KV JSON 误判）。
	for _, key := range []string{"content", "data"} {
		if raw, ok := top[key]; ok {
			var arr []json.RawMessage
			if err := json.Unmarshal(raw, &arr); err == nil {
				return VerdictSuccess
			}
		}
	}
	return VerdictIndeterminate
}

// sseClassify 扫描 SSE 文本（技术方案 §3.1 判定矩阵第 5 行，含成功形状回退）：
//   - "event: error" 行，或任一 data 行 payload 顶层含 "error"/type:"error" → UpstreamError（错误优先，即使流中已有正常 chunk）；
//   - 否则首个可解析 data 行具备成功形状 → Success；
//   - 其余 → Indeterminate。
//
// 性能：仅对「首个 data 行」与「含 "error" 子串的行」做 JSON 解析，其余行字符串跳过。
func sseClassify(body string) UpstreamVerdict {
	successSeen := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, ":") {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "event:"):
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "event:")), "error") {
				return VerdictUpstreamError
			}
		case strings.HasPrefix(trimmed, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			// 首个 data 行必须解析（建立成功/失败基线）；其余仅错误候选行解析。
			if successSeen && !strings.Contains(payload, "error") {
				continue
			}
			var top map[string]json.RawMessage
			if err := json.Unmarshal([]byte(payload), &top); err != nil {
				continue
			}
			if _, ok := top["error"]; ok {
				return VerdictUpstreamError
			}
			if raw, ok := top["type"]; ok {
				var t string
				if err := json.Unmarshal(raw, &t); err == nil && strings.EqualFold(t, "error") {
					return VerdictUpstreamError
				}
			}
			for _, key := range []string{"choices", "output", "candidates", "content", "data"} {
				if _, ok := top[key]; ok {
					successSeen = true
					break
				}
			}
		}
	}
	if successSeen {
		return VerdictSuccess
	}
	return VerdictIndeterminate
}

// isFakeSuccessUpstreamOutput 判定节点输出是否为「应转为失败的假成功」：
// body 为显式错误结构，且状态码非纯鉴权类。401/403 是客户端侧鉴权问题，
// 降级救不了，且既有契约是「池已穷尽的纯鉴权 401 透传给客户端」
// （transparent_forward_node.go 错误处理注释 + Plain401AuthStillPassthrough 契约测试），
// 结构判定兜底不接管这两个状态码。
func isFakeSuccessUpstreamOutput(statusCode int, content string) bool {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return false
	}
	return classifyUpstreamResponse(statusCode, content) == VerdictUpstreamError
}

// rawErrorBodyPassthrough 调用方是否显式豁免假成功兜底（R08）。
// /execute 调试入口（server pipeline_handler）注入该标记：其既有契约是把上游
// 原始错误体作为数据返回（PRESERVE_STATUS 开关映射 HTTP 状态），依赖节点
// 「成功携带 status_code 标记」语义；代理主路径不注入，兜底正常生效。
func rawErrorBodyPassthrough(meta map[string]interface{}) bool {
	if meta == nil {
		return false
	}
	v, ok := meta["raw_error_body_passthrough"].(bool)
	return ok && v
}

// upstreamErrorKind 提取上游错误类型枚举值（OpenAI 形 error.type/error.code，
// Anthropic 形内层 error.type，error 为字符串时取该字符串），供日志与观测；
// 提取不到返回 ""。仅返回类型枚举，不返回 message 原文（避免敏感内容入日志）。
func upstreamErrorKind(body string) string {
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, "{") {
		return ""
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &top); err != nil {
		return ""
	}
	raw, ok := top["error"]
	if !ok {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		// error 为字符串形（如 {"error":"rate_limit"}）。
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	for _, key := range []string{"type", "code"} {
		if v, ok := obj[key]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
