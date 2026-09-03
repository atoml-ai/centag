package pipeline

import (
	"strings"
	"testing"
	"time"
)

// TestUpstreamVerdict 表驱动覆盖技术方案 §3.1 判定矩阵（测试用例.md TC-UV-001~015）。
func TestUpstreamVerdict(t *testing.T) {
	const caseBody = `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`

	tests := []struct {
		id         string
		statusCode int
		body       string
		want       UpstreamVerdict
	}{
		{id: "TC-UV-001", statusCode: 400, body: caseBody, want: VerdictUpstreamError},
		{id: "TC-UV-002", statusCode: 200, body: `{"error":{"message":"boom"}}`, want: VerdictUpstreamError},
		{id: "TC-UV-003", statusCode: 400, body: `{"type":"error","error":{"type":"api_error","message":"x"}}`, want: VerdictUpstreamError},
		{id: "TC-UV-004", statusCode: 200, body: `{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}`, want: VerdictSuccess},
		{id: "TC-UV-005", statusCode: 200, body: `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]}}]}`, want: VerdictSuccess},
		// 正文内嵌错误形文本：parse-then-check 顶层键，不得子串误报。
		{id: "TC-UV-006", statusCode: 200, body: `{"choices":[{"message":{"content":"示例：{\"error\":{\"type\":\"x\"}} 用法"}}]}`, want: VerdictSuccess},
		{id: "TC-UV-007", statusCode: 200, body: `{"object":"list","data":[{"id":"m1","object":"model"}]}`, want: VerdictSuccess},
		{id: "TC-UV-008", statusCode: 200, body: "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n", want: VerdictSuccess},
		{id: "TC-UV-009", statusCode: 200, body: "data: {\"error\":{\"type\":\"overloaded\",\"message\":\"x\"}}\n\ndata: [DONE]\n\n", want: VerdictUpstreamError},
		{id: "TC-UV-010", statusCode: 200, body: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\"}}\n\n", want: VerdictUpstreamError},
		// SSE delta 正文内嵌错误形文本：顶层键为 choices，不误报。
		{id: "TC-UV-011", statusCode: 200, body: "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"error\\\":...}\"}}]}\n\ndata: [DONE]\n\n", want: VerdictSuccess},
		{id: "TC-UV-012a", statusCode: 502, body: "<html>Bad Gateway</html>", want: VerdictIndeterminate},
		{id: "TC-UV-012b", statusCode: 200, body: "plain text response", want: VerdictIndeterminate},
		// TC-UV-012c：JSON 前缀但解析失败（畸形 JSON）。
		{id: "TC-UV-012c", statusCode: 200, body: `{"a":`, want: VerdictIndeterminate},
		{id: "TC-UV-013a", statusCode: 200, body: `{"foo":1}`, want: VerdictIndeterminate},
		{id: "TC-UV-013b", statusCode: 200, body: `[]`, want: VerdictIndeterminate},
		// TC-UV-013c：type 非字符串形 → 无法识别（不误判 Anthropic 形）。
		{id: "TC-UV-013c", statusCode: 200, body: `{"type":123}`, want: VerdictIndeterminate},
		{id: "TC-UV-014", statusCode: 400, body: `{"detail":"context too long"}`, want: VerdictIndeterminate},
		// TC-UV-016：空白 body（判定矩阵第 1 行）。
		{id: "TC-UV-016", statusCode: 200, body: "   \n  ", want: VerdictIndeterminate},
		// TC-UV-017：正常 chunk 后接终止 error 事件——「错误优先」语义；event 行大小写不敏感。
		{id: "TC-UV-017", statusCode: 200, body: "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\nevent: Error\ndata: {\"error\":{\"type\":\"overloaded\"}}\n\n", want: VerdictUpstreamError},
		// TC-UV-018：content/data 字符串形不计入成功形状（防普通 KV JSON 误判）。
		{id: "TC-UV-018a", statusCode: 200, body: `{"content":"plain"}`, want: VerdictIndeterminate},
		{id: "TC-UV-018b", statusCode: 200, body: `{"data":"ok"}`, want: VerdictIndeterminate},
		{id: "TC-UV-018c", statusCode: 200, body: `{"content":[]}`, want: VerdictSuccess},
		// TC-UV-020：SSE 健壮性——注释行、非 error event 行、畸形 data 行均跳过，
		// 正常 chunk 仍判成功（不含 error 时畸形行不触发解析）。
		{id: "TC-UV-020", statusCode: 200, body: ": keepalive\n\nevent: message\ndata: {oops not json\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n", want: VerdictSuccess},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			got := classifyUpstreamResponse(tc.statusCode, tc.body)
			if got != tc.want {
				t.Fatalf("classifyUpstreamResponse(%d, %.60q...) = %v, want %v", tc.statusCode, tc.body, got, tc.want)
			}
		})
	}
}

// TestUpstreamErrorKind 覆盖 upstreamErrorKind 的类型提取（TC-UV-001/003/009 断言点）。
func TestUpstreamErrorKind(t *testing.T) {
	tests := []struct {
		id   string
		body string
		want string
	}{
		{id: "TC-UV-001-kind", body: `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`, want: "server_error"},
		{id: "TC-UV-003-kind", body: `{"type":"error","error":{"type":"api_error","message":"x"}}`, want: "api_error"},
		{id: "TC-UV-009-kind", body: `{"error":{"type":"overloaded"}}`, want: "overloaded"},
		{id: "kind-by-code", body: `{"error":{"code":"capacity_exceeded"}}`, want: "capacity_exceeded"},
		{id: "kind-string-error", body: `{"error":"rate_limit"}`, want: "rate_limit"},
		{id: "kind-missing", body: `{"error":{"message":"no type"}}`, want: ""},
		{id: "kind-not-error", body: `{"choices":[]}`, want: ""},
		{id: "kind-not-json", body: "<html>x</html>", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			if got := upstreamErrorKind(tc.body); got != tc.want {
				t.Fatalf("upstreamErrorKind(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

// TestUpstreamVerdictLargeSSE 覆盖 TC-UV-015：1MB 正常 SSE 流（含 "error" 干扰行）不误报且限时。
func TestUpstreamVerdictLargeSSE(t *testing.T) {
	var b strings.Builder
	b.WriteString("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
	chunk := strings.Repeat("x", 128)
	for i := 0; i < 6000; i++ {
		// 大量含 "error" 字样的 delta 正文：考察 parse-then-check 过滤不误报。
		b.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"")
		b.WriteString(chunk)
		b.WriteString(" error mention ")
		b.WriteString("\"}}]}\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	body := b.String()

	start := time.Now()
	got := classifyUpstreamResponse(200, body)
	elapsed := time.Since(start)

	if got != VerdictSuccess {
		t.Fatalf("large SSE verdict = %v, want VerdictSuccess", got)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("large SSE classify took %v, want < 50ms", elapsed)
	}
}

// TestRawErrorBodyPassthrough 豁免标记 helper 的直接单元验证
// （TC-TF-009 / TC-EN-006 / TC-EN-007 背后的开关语义，R08）。
func TestRawErrorBodyPassthrough(t *testing.T) {
	tests := []struct {
		name string
		meta map[string]interface{}
		want bool
	}{
		{name: "nil-meta", meta: nil, want: false},
		{name: "missing", meta: map[string]interface{}{}, want: false},
		{name: "wrong-type", meta: map[string]interface{}{"raw_error_body_passthrough": "true"}, want: false},
		{name: "false", meta: map[string]interface{}{"raw_error_body_passthrough": false}, want: false},
		{name: "true", meta: map[string]interface{}{"raw_error_body_passthrough": true}, want: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := rawErrorBodyPassthrough(tc.meta); got != tc.want {
				t.Fatalf("rawErrorBodyPassthrough = %v, want %v", got, tc.want)
			}
		})
	}
}
