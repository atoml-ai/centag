package backend

import (
	"net/url"
	"strings"
)

// NormalizeOllamaAPIBase 将误写成 OpenAI 兼容形式的 BaseURL（末尾 /v1）还原为 Ollama 原生根地址，
// 以便正确拼接 /api/chat、/api/tags 等路径。
func NormalizeOllamaAPIBase(raw string) string {
	s := strings.TrimSpace(raw)
	for {
		s = strings.TrimRight(s, "/")
		if len(s) < 3 {
			return s
		}
		low := strings.ToLower(s)
		if strings.HasSuffix(low, "/v1") {
			s = s[:len(s)-3]
			continue
		}
		break
	}
	return strings.TrimRight(s, "/")
}

// NormalizeOpenAIAPIBase 保证 OpenAI 兼容 API 的根路径以 /v1 结尾（无路径或仅有 / 时补上 /v1），
// 便于拼接 /models、/chat/completions。
func NormalizeOpenAIAPIBase(raw string) string {
	s := strings.TrimSpace(raw)
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimSuffix(s, "/")
	}
	path := strings.TrimSuffix(u.Path, "/")
	if path == "" || path == "/" {
		u.Path = "/v1"
	} else {
		u.Path = path
	}
	out := u.String()
	return strings.TrimSuffix(out, "/")
}

// CandidateOpenAIAPIRoots 返回用于探测 OpenAI 兼容接口的候选根 URL（去重、有序）。
// 许多厂商的 BaseURL 写成 …/openai 而真实接口在 …/openai/v1；或仅有 chat 而无 GET /models（如部分 DashScope 接入）。
func CandidateOpenAIAPIRoots(raw string) []string {
	b := strings.TrimSpace(strings.TrimSuffix(raw, "/"))
	if b == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var roots []string
	add := func(s string) {
		s = strings.TrimSuffix(strings.TrimSpace(s), "/")
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		roots = append(roots, s)
	}
	// 优先尝试 …/v1（网关常见）
	if !strings.HasSuffix(strings.ToLower(b), "/v1") {
		add(b + "/v1")
	}
	add(NormalizeOpenAIAPIBase(b))
	add(b)
	return roots
}
