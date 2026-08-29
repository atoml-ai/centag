package pipeline

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyNodeError_UpstreamErrorStatusCode(t *testing.T) {
	// 结构化 UpstreamError：应解析出真实状态码（此前 %*q Sscanf 恒失败，status_code 恒为 0）。
	// 正文避免 FreeUsageLimitError 等计费关键词（那些会优先归为 billing）。
	err := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash",
		"https://opencode.ai/zen/v1/chat/completions", 503,
		`{"error":{"message":"bad gateway"}}`)
	typ, code, _ := classifyNodeError(err)
	if typ != "http_status" || code != 503 {
		t.Fatalf("classifyNodeError(UpstreamError 503) = (%s, %d), want (http_status, 503)", typ, code)
	}
}

func TestClassifyNodeError_FreeUsageLimitIsBilling(t *testing.T) {
	err := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash-free",
		"https://opencode.ai/zen/v1/chat/completions", 429,
		`{"type":"error","error":{"type":"FreeUsageLimitError","message":"Rate limit exceeded."}}`)
	typ, _, _ := classifyNodeError(err)
	if typ != "billing" {
		t.Fatalf("classifyNodeError(FreeUsageLimitError) = %s, want billing", typ)
	}
}

func TestClassifyNodeError_TextStatusFallback(t *testing.T) {
	// 非结构化文本（旧格式 / 插件路径）：仍能兜底解析状态码
	msg := `transparent_forward node "forward": backend=zen model=m url=https://x upstream returned 503: bad gateway`
	typ, code, _ := classifyNodeError(errors.New(msg))
	if typ != "http_status" || code != 503 {
		t.Fatalf("classifyNodeError(text 503) = (%s, %d), want (http_status, 503)", typ, code)
	}
}

func TestClassifyNodeError_BillingPriority(t *testing.T) {
	// 计费类失败优先归为 billing（保证 skipCircuitRecord 语义不变）
	err := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash",
		"https://opencode.ai/zen/v1/chat/completions", 401,
		`{"type":"error","error":{"type":"CreditsError","message":"Insufficient balance. Manage your billing here: https://opencode.ai/billing"}}`)
	typ, _, _ := classifyNodeError(err)
	if typ != "billing" {
		t.Fatalf("classifyNodeError(CreditsError) = %s, want billing", typ)
	}
}

func TestClassifyNodeError_OpenAIFormat(t *testing.T) {
	typ, code, _ := classifyNodeError(errors.New(`API error (status 429): rate limit`))
	if typ != "http_status" || code != 429 {
		t.Fatalf("classifyNodeError(API error 429) = (%s, %d), want (http_status, 429)", typ, code)
	}
}

func TestBuildFallbackGroupError_AggregatesRootCause(t *testing.T) {
	primary := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash",
		"https://opencode.ai/zen/v1/chat/completions", 401,
		`{"type":"error","error":{"type":"CreditsError","message":"Insufficient balance"}}`)
	fallback := newTransparentUpstreamError("forward_fallback", "zen", "deepseek-v4-flash-free",
		"https://opencode.ai/zen/v1/chat/completions", 429,
		`{"type":"error","error":{"type":"FreeUsageLimitError","message":"Rate limit exceeded."}}`)

	got := buildFallbackGroupError("forward", primary, fallback)

	// 保留原错误前缀（外部按字符串匹配的依赖仍生效）
	if !strings.Contains(got.Error(), "all fallback attempts failed for primary node forward") {
		t.Fatalf("aggregated error lost prefix: %s", got.Error())
	}
	// 聚合主节点 + 末次降级根因
	if !strings.Contains(got.Error(), "primary: ") || !strings.Contains(got.Error(), "last fallback: ") {
		t.Fatalf("aggregated error missing root causes: %s", got.Error())
	}
	if !strings.Contains(got.Error(), "Insufficient balance") || !strings.Contains(got.Error(), "Rate limit exceeded") {
		t.Fatalf("aggregated error missing upstream detail: %s", got.Error())
	}
	// 通过 errors.As 可取回主节点状态码（供 handler 透传）
	if code := upstreamStatusCodeOf(got); code != 401 {
		t.Fatalf("upstreamStatusCodeOf(aggregated) = %d, want 401", code)
	}
}

func TestBuildFallbackGroupError_SanitizesKeys(t *testing.T) {
	primary := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash",
		"https://x", 401, `api key sk-live-ABCDEF0123456789 leaked`)
	got := buildFallbackGroupError("forward", primary, nil)
	if strings.Contains(got.Error(), "sk-live-ABCDEF0123456789") {
		t.Fatalf("aggregated error leaked API key: %s", got.Error())
	}
	if code := upstreamStatusCodeOf(got); code != 401 {
		t.Fatalf("upstreamStatusCodeOf(typed 401) = %d, want 401", code)
	}
}

func TestExtractUpstreamStatusCode(t *testing.T) {
	cases := []struct {
		msg  string
		code int
		ok   bool
	}{
		{`upstream returned 429: {...}`, 429, true},
		{`upstream returned 503: bad`, 503, true},
		{`no marker here`, 0, false},
		{`upstream returned 99`, 0, false}, // 非法状态码
	}
	for _, c := range cases {
		code, ok := extractUpstreamStatusCode(c.msg)
		if code != c.code || ok != c.ok {
			t.Errorf("extractUpstreamStatusCode(%q) = (%d,%v), want (%d,%v)", c.msg, code, ok, c.code, c.ok)
		}
	}
}

func TestIsTransientRateLimitError(t *testing.T) {
	free429 := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash-free",
		"https://x", 429, `Rate limit exceeded.`)
	paid429 := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash",
		"https://x", 429, `Rate limit exceeded.`)
	free503 := newTransparentUpstreamError("forward", "zen", "deepseek-v4-flash-free",
		"https://x", 503, `bad gateway`)
	// 永久性额度耗尽：FreeUsageLimitError 文案虽含 "rate limit"，但必须计入熔断
	quotaFree := newTransparentUpstreamError("forward", "zen", "hy3-free",
		"https://x", 429, `{"error":{"code":"FreeUsageLimitError","message":"Rate limit exceeded. You have used all your free usage."}}`)
	// 临时限流：任何档位都豁免
	temp429 := newTransparentUpstreamError("forward", "zen", "glm-4.7",
		"https://x", 429, `Too many requests, please try again later.`)

	if !isTransientRateLimitError(free429, "deepseek-v4-flash-free") {
		t.Fatal("free-tier transient 429 should be exempt from circuit recording")
	}
	if isTransientRateLimitError(paid429, "deepseek-v4-flash") {
		t.Fatal("paid-tier 429 should still count toward circuit")
	}
	if isTransientRateLimitError(free503, "deepseek-v4-flash-free") {
		t.Fatal("free-tier non-429 should not use free-tier rate-limit exemption")
	}
	if isTransientRateLimitError(quotaFree, "hy3-free") {
		t.Fatal("permanent free-tier quota exhaustion (FreeUsageLimitError) must count toward circuit")
	}
	if !isTransientRateLimitError(temp429, "glm-4.7") {
		t.Fatal("temporary rate limit (429 + try again later) should be exempt on any tier")
	}
}
