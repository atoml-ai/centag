package pipeline

import (
	"fmt"
	"net/url"
	"strings"
)

// ResolveTransparentTargetURL picks the upstream URL for transparent forwarding.
// Priority:
//  1. explicit target_url
//  2. configured Centag backend (system-proxy / transparent-proxy 主路径：勿打回原厂 Host)
//  3. original_host + path（#raw / Host 劫持真透传）
//
// MITM 会带 X-Original-Host=opencode.ai 等；若优先 original_host，会用 Centag Key 打回 Zen
// 导致 AuthError: Invalid API key，并绕过「走 Centag 后端」的出口策略。
func ResolveTransparentTargetURL(meta map[string]interface{}, backendID, requestPath, defaultScheme string) (string, error) {
	if meta != nil {
		if u := strings.TrimSpace(stringMeta(meta, "target_url")); u != "" {
			return normalizeTargetURL(u, requestPath, defaultScheme)
		}
	}

	bid := strings.TrimSpace(backendID)
	if bid == "" && meta != nil {
		bid = strings.TrimSpace(stringMeta(meta, "backend_id"))
	}
	if bid != "" && ResolveBackendEndpoint != nil {
		ep, err := ResolveBackendEndpoint(bid)
		if err != nil {
			return "", fmt.Errorf("resolve backend %q: %w", bid, err)
		}
		if ep != nil && strings.TrimSpace(ep.BaseURL) != "" {
			base := strings.TrimRight(strings.TrimSpace(ep.BaseURL), "/")
			// 客户端 requestPath 是到 Centag 的路径（如 /v1/chat/completions），
			// 不能直接拼到后端 BaseURL 上（会导致 /v4/v1/chat/completions 这类错误）。
			// OpenAI 兼容 API 的聊天补全端点统一为 /chat/completions，由后端 BaseURL 提供版本前缀。
			return base + "/chat/completions", nil
		}
	}

	if meta != nil {
		if host := strings.TrimSpace(stringMeta(meta, "original_host")); host != "" {
			path := requestPath
			if p := strings.TrimSpace(stringMeta(meta, "original_path")); p != "" {
				path = p
			}
			if path == "" {
				path = "/v1/chat/completions"
			}
			scheme := defaultScheme
			if s := strings.TrimSpace(stringMeta(meta, "target_scheme")); s != "" {
				scheme = s
			}
			return fmt.Sprintf("%s://%s%s", scheme, host, ensureLeadingSlash(path)), nil
		}
	}

	return "", fmt.Errorf("transparent forward: no target_url, original_host, or backend_id")
}

func normalizeTargetURL(raw, requestPath, defaultScheme string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty target url")
	}
	if !strings.Contains(raw, "://") {
		scheme := defaultScheme
		if scheme == "" {
			scheme = "https"
		}
		raw = scheme + "://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Path == "" || u.Path == "/" {
		path := requestPath
		if path == "" {
			path = "/v1/chat/completions"
		}
		u.Path = ensureLeadingSlash(path)
	}
	return u.String(), nil
}

func stringMeta(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}