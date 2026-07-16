package pipeline

import (
	"fmt"
	"strings"

	"centag/core/pkg/utils"
)

const defaultMessagesPreviewMax = 4000

// FormatMessagesPreview 将对话消息格式化为可读预览（截断脱敏由调用方日志层处理）。
func FormatMessagesPreview(messages []Message, maxLen int) string {
	if maxLen <= 0 {
		maxLen = defaultMessagesPreviewMax
	}
	if len(messages) == 0 {
		return ""
	}
	perMsg := maxLen / len(messages)
	if perMsg < 256 {
		perMsg = 256
	}
	var b strings.Builder
	for i, m := range messages {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "unknown"
		}
		content := strings.TrimSpace(m.Content)
		fmt.Fprintf(&b, "[%s] %s", role, utils.TruncateString(content, perMsg))
	}
	return utils.TruncateString(b.String(), maxLen)
}
