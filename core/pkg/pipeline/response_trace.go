package pipeline

import (
	"strings"

	"centag/core/pkg/config"
)

// ResponseTraceBannerEnabled 全局「响应追踪」开关是否打开。
func ResponseTraceBannerEnabled() bool {
	cfg := config.Get()
	return cfg != nil && cfg.Proxy.ResponseTraceBanner
}

// ApplyResponseTraceBanner 在开关打开时，于返回正文最前附加简洁请求→响应流程追踪。
func ApplyResponseTraceBanner(out *PipelineOutput, pipelineID string) {
	if out == nil || !ResponseTraceBannerEnabled() {
		return
	}
	if out.Metadata == nil {
		out.Metadata = make(map[string]interface{})
	}
	if out.Metadata["response_trace_applied"] == true {
		return
	}

	if pipelineID == "" && out.ExecutionLog != nil {
		pipelineID = out.ExecutionLog.PipelineID
	}
	if pipelineID == "" {
		pipelineID = firstMetaString(out.Metadata, "pipeline_id")
	}
	if pipelineID != "" && firstMetaString(out.Metadata, "pipeline_id") == "" {
		out.Metadata["pipeline_id"] = pipelineID
	}

	backendID := firstMetaString(out.Metadata, "backend_id", "executor_backend", "billing_fallback_backend")
	model := firstMetaString(out.Metadata, "executor_model", "model", "billing_fallback_to_model", "fallback_to_model")
	requested := firstMetaString(out.Metadata, "requested_model", "fallback_from_model", "billing_fallback_from_model")
	from, to := fallbackModelPair(out.Metadata)
	if from == "" {
		from = requested
	}
	if to == "" {
		to = model
	}

	nodePath := nodePathFromExecLog(out.ExecutionLog)
	banner := buildResponseTraceBanner(pipelineID, nodePath, backendID, model, from, to, isFallbackOutput(out.Metadata))
	out.Metadata["response_trace_banner"] = strings.TrimSpace(banner)
	out.Metadata["response_trace_applied"] = true

	// 若节点层曾打过旧版降级前缀，先剥掉再写统一横幅，避免重复
	out.Content = stripLegacyFallbackNoticePrefix(out.Content)

	if strings.TrimSpace(out.Content) == "" {
		out.Content = banner
		return
	}
	// responses→chat 桥接后 Content 是纯文本，必须走 FormatChunk；勿注入 chat.completion SSE。
	if out.Metadata["responses_to_chat"] == true {
		out.Content = banner + out.Content
		return
	}
	// /v1/responses 客户端不能收到 chat.completion.chunk（含 centag-fallback-notice）。
	if isResponsesAPIPath(firstMetaString(out.Metadata, "request_path")) {
		out.Content = banner + out.Content
		return
	}
	if looksLikeOpenAISSEContent(out.Content) {
		out.Content = sseFallbackNoticePrefix(strings.TrimSpace(banner)) + out.Content
		return
	}
	out.Content = banner + out.Content
}

// buildResponseTraceBanner 单行流程：req → 流水线:节点链 → 后端/模型 → resp
// 例：
//
//	[Centag] req → transparent-proxy:generate → opencode-zen/deepseek-v4 → resp
//	[Centag] req → translate-mode:generate→translate → openai/gpt-4o → resp
//	[Centag] req → transparent-proxy:generate → opencode-zen/gpt-5.6→deepseek-v4 → resp
func buildResponseTraceBanner(pipelineID, nodePath, backendID, model, from, to string, fallback bool) string {
	var b strings.Builder
	b.WriteString("[Centag] req → ")

	pipe := strings.TrimSpace(pipelineID)
	if pipe == "" {
		pipe = "?"
	}
	b.WriteString(pipe)
	if np := strings.TrimSpace(nodePath); np != "" {
		b.WriteByte(':')
		b.WriteString(np)
	}

	b.WriteString(" → ")
	b.WriteString(formatTraceEgress(backendID, model, from, to, fallback))
	b.WriteString(" → resp\n")
	b.WriteString("--------------------\n")
	return b.String()
}

func formatTraceEgress(backendID, model, from, to string, fallback bool) string {
	be := strings.TrimSpace(backendID)
	m := strings.TrimSpace(model)
	f := strings.TrimSpace(from)
	t := strings.TrimSpace(to)
	if t == "" {
		t = m
	}

	var right string
	switch {
	case fallback && f != "" && t != "" && !strings.EqualFold(f, t):
		right = f + "→" + t
	case t != "":
		right = t
	case m != "":
		right = m
	}

	switch {
	case be != "" && right != "":
		return be + "/" + right
	case be != "":
		return be
	case right != "":
		return right
	default:
		return "?"
	}
}

// nodePathFromExecLog 从执行日志提取节点流程（按时间序，合并连续同名节点）。
func nodePathFromExecLog(log *ExecutionLog) string {
	if log == nil || len(log.NodeLogs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(log.NodeLogs))
	for _, nl := range log.NodeLogs {
		id := strings.TrimSpace(nl.NodeID)
		if id == "" {
			continue
		}
		if len(parts) > 0 && parts[len(parts)-1] == id {
			continue
		}
		parts = append(parts, id)
	}
	return strings.Join(parts, "→")
}

func stripLegacyFallbackNoticePrefix(content string) string {
	trim := content
	for {
		if strings.HasPrefix(trim, "⚠️ 模型已降级") {
			if idx := strings.Index(trim, "\n\n"); idx >= 0 {
				trim = trim[idx+2:]
				continue
			}
		}
		if strings.HasPrefix(strings.TrimSpace(trim), "data:") && strings.Contains(trim, "centag-fallback-notice") {
			rest := trim
			if i := strings.Index(rest, "\n\n"); i >= 0 {
				trim = rest[i+2:]
				continue
			}
		}
		break
	}
	return trim
}
