package pipeline

import (
	"errors"
	"fmt"
	"strings"
)

// UpstreamError 携带上游 HTTP 状态码与提供方错误码的可解包错误。
//
// 使 classifyNodeError（重试/降级决策）与代理层 handler（HTTP 状态透传、错误脱敏）
// 能通过 errors.As 拿到真实状态码，而不是依赖脆弱的文本解析。
type UpstreamError struct {
	StatusCode   int    // 上游返回的 HTTP 状态码（0 表示未知）
	ProviderCode string // 上游 provider error code（如 CreditsError / rate_limit_error），可能为空
	BackendID    string
	Model        string
	URL          string
	Message      string // 原始错误文案（含上游响应片段；对客户端输出前需脱敏）
	// PoolExhausted 标记账户池已耗尽（所有 Key 均已尝试且失败）。
	// 用于 classifyNodeError 将此错误分类为不可重试，避免 executeWithRetry
	// 重复执行已耗尽的账户池轮换（否则 N 个 Key × M 次重试 = N×M 倍请求放大）。
	PoolExhausted bool
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("upstream error status=%d backend=%s model=%s", e.StatusCode, e.BackendID, e.Model)
}

// newTransparentUpstreamError 构造 transparent_forward 节点的上游错误。
// 保持原有文案格式（兼容按字符串匹配的旧逻辑），但额外携带结构化状态码。
func newTransparentUpstreamError(nodeID, backendID, model, targetURL string, statusCode int, body string, poolExhausted ...bool) error {
	pe := len(poolExhausted) > 0 && poolExhausted[0]
	return &UpstreamError{
		StatusCode:    statusCode,
		BackendID:     backendID,
		Model:         model,
		URL:           targetURL,
		PoolExhausted: pe,
		Message: fmt.Sprintf("transparent_forward node %q: backend=%s model=%s url=%s upstream returned %d: %s",
			nodeID, backendID, model, targetURL, statusCode, truncateBody([]byte(body), 512)),
	}
}

// upstreamStatusCodeOf 从错误链中提取上游 HTTP 状态码；提取不到返回 0。
func upstreamStatusCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ue *UpstreamError
	if errors.As(err, &ue) && ue != nil {
		return ue.StatusCode
	}
	return 0
}

// extractUpstreamStatusCode 从错误文案中兜底提取 "upstream returned %d" 的状态码。
// 供未使用 UpstreamError 类型的旧路径（如插件层）使用。
func extractUpstreamStatusCode(msg string) (int, bool) {
	const marker = "upstream returned "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0, false
	}
	rest := msg[idx+len(marker):]
	// 跳过可选空格，解析连续数字
	start := 0
	for start < len(rest) && (rest[start] == ' ' || rest[start] == ':') {
		start++
	}
	end := start
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	var code int
	for i := start; i < end; i++ {
		code = code*10 + int(rest[i]-'0')
	}
	if code <= 0 || code < 100 || code >= 600 {
		return 0, false
	}
	return code, true
}

// fallbackGroupError 聚合降级组全败时的错误。
// Error() 只返回脱敏后的聚合文案；通过 Unwrap() []error 暴露原因链，
// 使 errors.As 仍可取回上游状态码，同时避免原始上游响应体泄漏给客户端。
type fallbackGroupError struct {
	msg    string
	causes []error
}

func (e *fallbackGroupError) Error() string {
	if e == nil {
		return ""
	}
	return e.msg
}

func (e *fallbackGroupError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.causes
}

// buildFallbackGroupError 聚合降级组全败时主节点与末次降级节点的错误，保留根因与状态码链。
// 面向客户端的文案做脱敏；通过 Unwrap() []error 暴露原因链，便于 errors.As 取状态码。
func buildFallbackGroupError(primaryID string, primaryErr, lastFallbackErr error) error {
	msg := fmt.Sprintf("all fallback attempts failed for primary node %s", primaryID)
	var detail []string
	if primaryErr != nil {
		detail = append(detail, "primary: "+MaskSensitiveData(primaryErr.Error()))
	}
	if lastFallbackErr != nil {
		detail = append(detail, "last fallback: "+MaskSensitiveData(lastFallbackErr.Error()))
	}
	if len(detail) > 0 {
		msg += " (" + strings.Join(detail, "; ") + ")"
	}

	agg := &fallbackGroupError{msg: msg}
	if primaryErr != nil {
		agg.causes = append(agg.causes, primaryErr)
	}
	if lastFallbackErr != nil {
		agg.causes = append(agg.causes, lastFallbackErr)
	}
	return agg
}
