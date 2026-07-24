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

// ApplyResponseTraceBanner 在开关打开时，于返回正文最前附加流水线/后端/模型/降级信息。
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

	banner := buildResponseTraceBanner(pipelineID, backendID, model, from, to, isFallbackOutput(out.Metadata))
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

func buildResponseTraceBanner(pipelineID, backendID, model, from, to string, fallback bool) string {
	var b strings.Builder
	b.WriteString("[Centag 响应追踪]\n")
	if pipelineID != "" {
		b.WriteString("流水线: ")
		b.WriteString(pipelineID)
		b.WriteByte('\n')
	}
	if backendID != "" {
		b.WriteString("后端: ")
		b.WriteString(backendID)
		b.WriteByte('\n')
	}
	if model != "" {
		b.WriteString("模型: ")
		b.WriteString(model)
		b.WriteByte('\n')
	}
	if fallback {
		switch {
		case from != "" && to != "" && !strings.EqualFold(from, to):
			b.WriteString("降级: ")
			b.WriteString(from)
			b.WriteString(" → ")
			b.WriteString(to)
			b.WriteByte('\n')
		case to != "":
			b.WriteString("降级: 已切换至 ")
			b.WriteString(to)
			b.WriteByte('\n')
		default:
			b.WriteString("降级: 是\n")
		}
	}
	b.WriteByte('\n')
	return b.String()
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
