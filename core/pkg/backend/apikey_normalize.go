package backend

import "strings"

// NormalizeOpenAICompatibleAPIKey trims whitespace and removes one leading "Bearer "
// prefix (case-insensitive) so callers never send Authorization: Bearer Bearer <token>
// to OpenAI-compatible APIs (DashScope / Coding Plan, OpenAI, etc.).
func NormalizeOpenAICompatibleAPIKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "bearer ") {
		s = strings.TrimSpace(s[7:])
	}
	return s
}
