package utils

// TruncateString 截断字符串，按 rune 计数，支持 Unicode（中文等）。
// 如果长度未超过 maxLen，返回原字符串；否则截断并在末尾添加 "..."。
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
