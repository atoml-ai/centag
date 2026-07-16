package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	headerAgentType      = "X-Agent-Type"
	headerDetectedBy     = "X-ProxyClaw-Agent-Detected-By"
	contextKeyAgentType  = "agent_type"
	detectByExplicit     = "explicit-header"
	detectByPathFallback = "path-fallback"
	detectByUserAgent    = "user-agent"
)

// AgentClientDetectMiddlewareGin 识别客户端类型并注入 gin context，供后续路由/供应商选择使用。
func AgentClientDetectMiddlewareGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		agentType, detectedBy := DetectAgentType(c.Request.URL.Path, c.GetHeader("User-Agent"), c.GetHeader(headerAgentType))
		if agentType != "" {
			c.Set(contextKeyAgentType, agentType)
			c.Request.Header.Set(headerAgentType, agentType)
			c.Writer.Header().Set(headerAgentType, agentType)
			c.Writer.Header().Set(headerDetectedBy, detectedBy)
		}
		c.Next()
	}
}

// DetectAgentType 根据显式头、路径和 UA 识别 agent 类型。
func DetectAgentType(path, userAgent, explicitAgentType string) (agentType, detectedBy string) {
	explicitAgentType = normalizeAgentType(explicitAgentType)
	if explicitAgentType != "" {
		return explicitAgentType, detectByExplicit
	}

	path = strings.ToLower(strings.TrimSpace(path))
	ua := strings.ToLower(strings.TrimSpace(userAgent))

	// /v1/messages 默认优先识别为 Claude 类客户端。
	if strings.HasSuffix(path, "/v1/messages") || strings.HasSuffix(path, "/messages") {
		switch {
		case strings.Contains(ua, "gemini"):
			return "gemini-cli", detectByPathFallback
		case strings.Contains(ua, "codex"):
			return "codex", detectByPathFallback
		default:
			return "claude-code", detectByPathFallback
		}
	}

	switch {
	case strings.Contains(ua, "claude"):
		return "claude-code", detectByUserAgent
	case strings.Contains(ua, "codex"):
		return "codex", detectByUserAgent
	case strings.Contains(ua, "gemini"):
		return "gemini-cli", detectByUserAgent
	case strings.Contains(ua, "opencode"):
		return "opencode", detectByUserAgent
	case strings.Contains(ua, "openclaw"):
		return "openclaw", detectByUserAgent
	case strings.Contains(ua, "hermes"):
		return "hermes", detectByUserAgent
	default:
		return "", ""
	}
}

func normalizeAgentType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "claude", "claude-code", "claude_desktop", "claude-desktop":
		return "claude-code"
	case "codex", "codex-cli":
		return "codex"
	case "gemini", "gemini-cli":
		return "gemini-cli"
	case "opencode":
		return "opencode"
	case "openclaw":
		return "openclaw"
	case "hermes":
		return "hermes"
	default:
		return ""
	}
}
