package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnnotateFallbackNotice 记录降级元数据。正文前缀由全局「响应追踪」开关统一注入（见 ApplyResponseTraceBanner）。
func AnnotateFallbackNotice(out *NodeOutput) {
	if out == nil {
		return
	}
	if out.Metadata == nil {
		out.Metadata = make(map[string]interface{})
	}
	if out.Metadata["fallback_notice_applied"] == true {
		return
	}
	if !isFallbackOutput(out.Metadata) {
		return
	}

	from, to := fallbackModelPair(out.Metadata)
	notice := formatFallbackNotice(from, to)
	out.Metadata["fallback_notice"] = strings.TrimSpace(notice)
	out.Metadata["fallback_notice_applied"] = true
	out.Metadata["fallback_used"] = true
}

func isFallbackOutput(meta map[string]interface{}) bool {
	if meta == nil {
		return false
	}
	if meta["billing_fallback_used"] == true || meta["fallback_used"] == true {
		return true
	}
	if _, ok := meta["fallback_policy_id"]; ok {
		return true
	}
	if _, ok := meta["billing_fallback_to_model"]; ok {
		return true
	}
	return false
}

func fallbackModelPair(meta map[string]interface{}) (from, to string) {
	from = firstMetaString(meta,
		"billing_fallback_from_model",
		"fallback_from_model",
		"requested_model",
	)
	to = firstMetaString(meta,
		"billing_fallback_to_model",
		"fallback_to_model",
		"executor_model",
		"model",
	)
	return from, to
}

func firstMetaString(meta map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func formatFallbackNotice(from, to string) string {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	switch {
	case from != "" && to != "" && !strings.EqualFold(from, to):
		return fmt.Sprintf("⚠️ 模型已降级：%s → %s\n\n", from, to)
	case to != "":
		return fmt.Sprintf("⚠️ 模型已降级，当前使用：%s\n\n", to)
	default:
		return "⚠️ 模型已降级（上游原模型不可用）\n\n"
	}
}

func looksLikeOpenAISSEContent(body string) bool {
	trim := strings.TrimSpace(body)
	return strings.HasPrefix(trim, "data:") || strings.Contains(body, "\ndata:")
}

func sseFallbackNoticePrefix(notice string) string {
	notice = strings.TrimSuffix(notice, "\n\n")
	notice = strings.TrimSpace(notice)
	payload := map[string]interface{}{
		"id":     "centag-fallback-notice",
		"object": "chat.completion.chunk",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"role":    "assistant",
					"content": notice + "\n\n",
				},
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "data: {\"choices\":[{\"delta\":{\"content\":\"" + escapeSSEText(notice) + "\\n\\n\"}}]}\n\n"
	}
	return "data: " + string(b) + "\n\n"
}

func escapeSSEText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// markFallbackGroupOutput 标记 FallbackGroup 成功输出并加前缀提示。
func markFallbackGroupOutput(fbNode *ExecutionNode, primaryNodeID string) {
	if fbNode == nil || fbNode.Output == nil {
		return
	}
	if fbNode.Output.Metadata == nil {
		fbNode.Output.Metadata = make(map[string]interface{})
	}
	fbNode.Output.Metadata["fallback_used"] = true
	fbNode.Output.Metadata["fallback_from_node"] = primaryNodeID
	if to := firstMetaString(fbNode.Output.Metadata, "executor_model", "model"); to != "" {
		fbNode.Output.Metadata["fallback_to_model"] = to
	}
	AnnotateFallbackNotice(fbNode.Output)
}
