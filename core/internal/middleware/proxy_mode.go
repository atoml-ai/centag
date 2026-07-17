package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"centag/core/pkg/config"
	"centag/core/pkg/proxymode"
	"centag/core/internal/proxy"
	"centag/core/internal/session"
)

// ProxyModeMiddleware 代理模式解析中间件（net/http 版本）
func ProxyModeMiddleware(modeMgr *proxymode.ModeManager, sessionStore *session.ProxyModeStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := resolveAndRewriteProxyMode(r, modeMgr, sessionStore); err != nil {
				http.Error(w, `{"success":false,"error":"invalid or disabled proxy mode"}`, http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ProxyModeMiddlewareGin Gin 版本的代理模式解析中间件
func ProxyModeMiddlewareGin(modeMgr *proxymode.ModeManager, sessionStore *session.ProxyModeStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := resolveAndRewriteProxyMode(c.Request, modeMgr, sessionStore); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid or disabled proxy mode"})
			return
		}
		c.Next()
	}
}

func resolveAndRewriteProxyMode(r *http.Request, modeMgr *proxymode.ModeManager, sessionStore *session.ProxyModeStore) error {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return nil
	}
	r.Body.Close()

	var body map[string]interface{}
	hasBody := json.Unmarshal(bodyBytes, &body) == nil

	// 1. 消息内容快捷码（#d、#s 等）— 最常用的显式指定方式
	if hasBody {
		if messages, ok := body["messages"].([]interface{}); ok {
			mode, backend, pipelineID, model, deptTag, found, modified := ExtractModeFromChatContent(messages)
			if found && modified {
				if err := applyResolvedShortcut(r, modeMgr, mode, backend, pipelineID, model, deptTag); err != nil {
					return err
				}
				newBody, _ := json.Marshal(body)
				r.Body = io.NopCloser(bytes.NewReader(newBody))
				return nil
			}
		}
	}

	// 2. model 字段流水线前缀（pipeline.direct-backend glm-4-flash）
	if hasBody {
		if pipelineID, applied := proxy.ApplyModelPipelinePrefixToBody(body); applied {
			modelHdr := ""
			if m, ok := body["model"].(string); ok && !proxymode.IsPipelineModel(m) {
				modelHdr = m
			}
			if err := applyResolvedShortcut(r, modeMgr, pipelineID, "", "", modelHdr, ""); err != nil {
				return err
			}
			newBody, _ := json.Marshal(body)
			r.Body = io.NopCloser(bytes.NewReader(newBody))
			return nil
		}
	}

	// 3. centag 扩展字段（非标准 OpenAI，但保留兼容）
	if hasBody {
		if mode, backend, model, found := extractCentagField(body); found {
			if err := applyResolvedShortcut(r, modeMgr, mode, backend, "", model, ""); err != nil {
				return err
			}
			newBody, _ := json.Marshal(body)
			r.Body = io.NopCloser(bytes.NewReader(newBody))
			return nil
		}
	}

	// 4. X-Proxy-Mode 请求头 — 可定制渠道，默认关闭，需 allow_header_override=true
	if allowHeaderOverride() {
		if mode, backend, model, found := ParseProxyModeFromHeader(r); found {
			if err := applyResolvedShortcut(r, modeMgr, mode, backend, "", model, ""); err != nil {
				return err
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return nil
		}
	}

	// 5. 会话粘性模式 — 仅当客户端显式要求（避免影响标准 Agent 的默认流水线）
	if shouldApplySessionMode(r) && sessionStore != nil {
		clientID := GetClientIP(r)
		if sessionMode, exists := sessionStore.Get(clientID); exists {
			if err := applyResolvedShortcut(r, modeMgr, sessionMode.ModeKey, sessionMode.BackendID, "", sessionMode.ModelName, ""); err != nil {
				return err
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return nil
		}
	}

	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return nil
}

func allowHeaderOverride() bool {
	cfg := config.Get()
	return cfg != nil && cfg.Proxy.AllowHeaderOverride
}

func shouldApplySessionMode(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Centag-Use-Session")))
	return v == "1" || v == "true" || v == "yes"
}

func applyResolvedShortcut(
	r *http.Request,
	modeMgr *proxymode.ModeManager,
	mode, backend, pipelineID, model, deptTag string,
) error {
	mode = canonProxyModeKey(mode)
	// 仅校验 # 前缀快捷码；流水线 ID（direct-backend 等）由 ModeDispatcher 解析。
	if strings.HasPrefix(mode, "#") {
		validMode, exists := modeMgr.GetMode(mode)
		if !exists || !validMode.Enabled {
			return errInvalidProxyMode
		}
		if pipelineID == "" {
			pipelineID = modeMgr.PipelineIDForShortcut(mode)
		}
	}
	r.Header.Set(proxy.HeaderCentagResolvedMode, mode)
	if backend != "" {
		r.Header.Set("X-Backend-ID", backend)
	}
	if pipelineID != "" {
		r.Header.Set("X-Pipeline-ID", pipelineID)
	}
	if model != "" {
		r.Header.Set("X-Model", model)
	}
	if deptTag != "" {
		r.Header.Set("X-Dept-Tag", deptTag)
	}
	return nil
}

var errInvalidProxyMode = &proxyModeError{"invalid or disabled proxy mode"}

type proxyModeError struct{ msg string }

func (e *proxyModeError) Error() string { return e.msg }

func extractCentagField(body map[string]interface{}) (mode, backend, model string, found bool) {
	centag, ok := body["centag"].(map[string]interface{})
	if !ok {
		return "", "", "", false
	}
	mode, ok = centag["mode"].(string)
	if !ok || strings.TrimSpace(mode) == "" {
		return "", "", "", false
	}
	backend, _ = centag["backend"].(string)
	model, _ = centag["model"].(string)
	delete(body, "centag")
	return mode, backend, model, true
}

// ParseProxyModeFromHeader 从请求头解析代理模式
func ParseProxyModeFromHeader(r *http.Request) (mode, backend, model string, found bool) {
	mode = r.Header.Get("X-Proxy-Mode")
	if mode == "" {
		mode = r.Header.Get("X-Centag-Mode")
	}
	if mode == "" {
		return "", "", "", false
	}

	backend = r.Header.Get("X-Backend-ID")
	if backend == "" {
		backend = r.Header.Get("X-Centag-Backend")
	}
	model = r.Header.Get("X-Model")
	if model == "" {
		model = r.Header.Get("X-Centag-Model")
	}
	return mode, backend, model, true
}

// ParseProxyModeFromBody 从请求体解析代理模式（会移除 centag 字段）
func ParseProxyModeFromBody(r *http.Request) (mode, backend, model string, found bool) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", "", false
	}
	r.Body.Close()

	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return "", "", "", false
	}

	mode, backend, model, found = extractCentagField(body)
	if !found {
		return "", "", "", false
	}

	newBody, _ := json.Marshal(body)
	r.Body = io.NopCloser(bytes.NewReader(newBody))
	return mode, backend, model, true
}

// canonProxyModeKey 将长名/别名规范为 ModeManager 中注册的短键（如 #p、#d）。
func canonProxyModeKey(mode string) string {
	mode = strings.TrimSpace(mode)
	em, err := proxymode.FromString(mode)
	if err != nil {
		return mode
	}
	if k := em.GetKey(); k != "" {
		return k
	}
	return mode
}

// ParseProxyModeFromContent 从内容中解析代理模式（#x）及可选令牌 /backend:、/pipeline:、/model:、/cost:。
// 支持首行快捷码，也支持 Agent 在正文前注入 memory/session 后单独一行写 #ch 问题。
func ParseProxyModeFromContent(content string) (mode, backend, pipelineID, model, deptTag, newContent string, found, modified bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", "", "", "", content, false, false
	}

	if strings.HasPrefix(trimmed, "#") {
		lines := strings.SplitN(content, "\n", 2)
		m, b, p, md, dt, question, ok := parseShortcutLine(lines[0])
		if !ok {
			return "", "", "", "", "", content, false, false
		}
		if len(lines) > 1 {
			if strings.TrimSpace(question) != "" {
				question = question + "\n" + lines[1]
			} else {
				question = lines[1]
			}
		}
		return m, b, p, md, dt, strings.TrimSpace(question), true, true
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		m, b, p, md, dt, question, ok := parseShortcutLine(line)
		if !ok {
			continue
		}
		return m, b, p, md, dt, rebuildContentWithoutShortcutLine(lines, i, question), true, true
	}

	return "", "", "", "", "", content, false, false
}

func parseShortcutLine(line string) (mode, backend, pipelineID, model, deptTag, question string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", "", "", "", "", "", false
	}
	parts := strings.SplitN(trimmed, " ", 2)
	modeToken := parts[0]
	if len(modeToken) < 2 {
		return "", "", "", "", "", "", false
	}
	if err := proxymode.ValidateModeKey(modeToken); err != nil {
		return "", "", "", "", "", "", false
	}
	remaining := ""
	if len(parts) > 1 {
		remaining = parts[1]
	}
	backend, pipelineID, model, deptTag, remaining = consumeProxyModeOptionTokens(remaining)
	return modeToken, backend, pipelineID, model, deptTag, remaining, true
}

func rebuildContentWithoutShortcutLine(lines []string, shortcutLine int, question string) string {
	var rebuilt []string
	if shortcutLine > 0 {
		rebuilt = append(rebuilt, lines[:shortcutLine]...)
	}
	if strings.TrimSpace(question) != "" {
		rebuilt = append(rebuilt, question)
	}
	if shortcutLine+1 < len(lines) {
		rebuilt = append(rebuilt, lines[shortcutLine+1:]...)
	}
	return strings.TrimSpace(strings.Join(rebuilt, "\n"))
}

// consumeProxyModeOptionTokens 从行首连续解析 /backend: /pipeline: /model: /cost: 令牌，返回剩余正文。
func consumeProxyModeOptionTokens(s string) (backend, pipelineID, model, deptTag, rest string) {
	remaining := strings.TrimSpace(s)
	for remaining != "" {
		tok, after := nextSpaceToken(remaining)
		switch {
		case strings.HasPrefix(tok, "/backend:"):
			backend = strings.TrimPrefix(tok, "/backend:")
			remaining = strings.TrimSpace(after)
		case strings.HasPrefix(tok, "/pipeline:"):
			pipelineID = strings.TrimPrefix(tok, "/pipeline:")
			remaining = strings.TrimSpace(after)
		case strings.HasPrefix(tok, "/p:"):
			pipelineID = strings.TrimPrefix(tok, "/p:")
			remaining = strings.TrimSpace(after)
		case strings.HasPrefix(tok, "/model:"):
			model = strings.TrimPrefix(tok, "/model:")
			remaining = strings.TrimSpace(after)
		case strings.HasPrefix(tok, "/cost:"):
			deptTag = sanitizeDeptTag(strings.TrimPrefix(tok, "/cost:"))
			remaining = strings.TrimSpace(after)
		default:
			return backend, pipelineID, model, deptTag, remaining
		}
	}
	return backend, pipelineID, model, deptTag, ""
}

func sanitizeDeptTag(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 64 {
		s = s[:64]
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func nextSpaceToken(s string) (token, after string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	idx := strings.IndexByte(s, ' ')
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

// ExtractModeFromChatContent 从聊天消息中提取模式。
// 仅检查当前轮（最后一条 user 消息）的快捷码；历史轮次中的 #a 等不应影响后续无关键码的请求。
func ExtractModeFromChatContent(messages []interface{}) (mode, backend, pipelineID, model, deptTag string, found, modified bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}

		content, ok := proxy.ExtractMessageText(msg["content"])
		if !ok {
			return "", "", "", "", "", false, false
		}

		mode, backend, pipelineID, model, deptTag, newContent, found, modified := ParseProxyModeFromContent(content)
		if found && modified {
			proxy.SetMessageText(msg, newContent)
			messages[i] = msg
			return mode, backend, pipelineID, model, deptTag, true, true
		}
		// 已定位到当前轮 user 消息且无快捷码，不再回溯更早轮次
		return "", "", "", "", "", false, false
	}

	return "", "", "", "", "", false, false
}

// GetClientIP 获取客户端 IP 地址
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// StripProxyModeFromBody 从请求体中移除 centag 字段
func StripProxyModeFromBody(body map[string]interface{}) (mode, backend string, found bool) {
	centag, ok := body["centag"].(map[string]interface{})
	if !ok {
		return "", "", false
	}

	mode, ok = centag["mode"].(string)
	if !ok {
		return "", "", false
	}

	backend, _ = centag["backend"].(string)

	delete(body, "centag")
	return mode, backend, true
}