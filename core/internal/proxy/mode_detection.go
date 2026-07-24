package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/proxymode"
)

// HeaderCentagResolvedMode 由中间件在解析快捷码后设置，供 Handler 识别。
// 与用户显式传入的 X-Proxy-Mode 区分：后者仅在 AllowHeaderOverride 时生效。
const HeaderCentagResolvedMode = "X-Centag-Resolved-Mode"

// DetectProxyMode 从请求中检测代理模式。
//
// 优先级（符合标准 OpenAI Agent 使用习惯）：
//  1. 中间件/请求体快捷码（消息 #d、model pipeline.xxx、centag 扩展字段）
//  2. X-Proxy-Mode / proxy_mode 查询参数（仅当 allow_header_override=true）
//  3. ModeDefault — 由 Handler 解析为系统默认流水线
func DetectProxyMode(r *http.Request) (ProxyMode, string) {
	return detectProxyModeWithConfig(r, config.Get())
}

func detectProxyModeWithConfig(r *http.Request, cfg *config.Config) (ProxyMode, string) {
	if resolved := strings.TrimSpace(r.Header.Get(HeaderCentagResolvedMode)); resolved != "" {
		return normalizeProxyMode(resolved), "shortcut"
	}

	bodyBytes := peekRequestBody(r)

	if mode := extractPipelineFromContentBytes(bodyBytes); mode != "" {
		return normalizeProxyMode(mode), "content-prefix"
	}

	if mode, _, ok := parseModelPipelinePrefixBytes(bodyBytes); ok {
		return normalizeProxyMode(mode), "model-prefix"
	}

	if mode := extractCentagModeBytes(bodyBytes); mode != "" {
		return normalizeProxyMode(mode), "centag-field"
	}

	if allowHeaderOverride(cfg) {
		if mode := r.Header.Get("X-Proxy-Mode"); mode != "" {
			return normalizeProxyMode(mode), "proxy-mode-header"
		}
		if mode := r.URL.Query().Get("proxy_mode"); mode != "" {
			return normalizeProxyMode(mode), "query-param"
		}
	}

	return ModeDefault, "default"
}

func allowHeaderOverride(cfg *config.Config) bool {
	return cfg != nil && cfg.Proxy.AllowHeaderOverride
}

func peekRequestBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes
}

func findShortcutTokenInContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	// 与中间件 parseShortcutLine 一致：首行以 # 开头时只校验首行，避免把正文里的 #ch 当作快捷码。
	if strings.HasPrefix(trimmed, "#") {
		if modeCode := shortcutTokenFromLine(strings.SplitN(content, "\n", 2)[0]); modeCode != "" {
			return modeCode
		}
		return ""
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		if modeCode := shortcutTokenFromLine(line); modeCode != "" {
			return modeCode
		}
	}
	return ""
}

func shortcutTokenFromLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return ""
	}
	parts := strings.SplitN(trimmed, " ", 2)
	modeCode := parts[0]
	if len(modeCode) < 2 {
		return ""
	}
	if err := proxymode.ValidateModeKey(modeCode); err != nil {
		return ""
	}
	return modeCode
}

func extractPipelineFromContentBytes(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		return ""
	}
	messages, ok := reqBody["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}
		content, ok := ExtractMessageText(msg["content"])
		if !ok || content == "" {
			continue
		}
		if modeCode := findShortcutTokenInContent(content); modeCode != "" {
			return modeCode
		}
		break
	}
	return ""
}

// parseModelPipelinePrefix 解析 model 字段中的流水线前缀。
// 支持 centag/direct-backend、pipeline.direct-backend、pipeline_direct-backend、
// 以及带实际模型后缀的写法（如 centag/direct-backend glm-4-flash）。
func parseModelPipelinePrefix(model string) (pipelineID, actualModel string, ok bool) {
	model = strings.TrimSpace(model)
	// 方式1：model 直接写流水线 ID（如 smart-scheduling）
	// 仅识别内置已知流水线名，避免误把普通模型名当作流水线。
	if pid := normalizeModelAsPipelineID(model); pid != "" {
		return pid, "", true
	}

	prefixed := false
	switch {
	case strings.HasPrefix(model, "centag/"):
		for strings.HasPrefix(model, "centag/") {
			model = strings.TrimPrefix(model, "centag/")
		}
		prefixed = true
	case strings.HasPrefix(model, "pipeline."):
		for strings.HasPrefix(model, "pipeline.") {
			model = strings.TrimPrefix(model, "pipeline.")
		}
		prefixed = true
	case strings.HasPrefix(model, "pipeline_"):
		for strings.HasPrefix(model, "pipeline_") {
			model = strings.TrimPrefix(model, "pipeline_")
		}
		prefixed = true
	}
	if !prefixed {
		return "", "", false
	}
	parts := strings.SplitN(model, " ", 2)
	pipelineID = strings.TrimSpace(parts[0])
	// 兼容虚拟模型名 pipeline.<id>.auto / centag/<id>.auto
	pipelineID = strings.TrimSuffix(pipelineID, ".auto")
	if pipelineID == "" {
		return "", "", false
	}
	if len(parts) > 1 {
		actualModel = strings.TrimSpace(parts[1])
	}
	return pipelineID, actualModel, true
}

func normalizeModelAsPipelineID(model string) string {
	switch strings.TrimSpace(strings.ToLower(model)) {
	case "smart-scheduling", "direct-backend", "fallback-mode", "router-mode",
		"optimize-mode", "audit-mode", "aggregator-mode", "translate-mode",
		"model-matching", "transparent-proxy", "transparent-fast", "pipeline-mode", "cache-hit",
		"cache-mode", "mem0-memory", "rag-mode", "agent-mode", "pi-agent",
		"security-mode", "multilingual-support", "geo-routing-mode":
		return strings.TrimSpace(strings.ToLower(model))
	default:
		return ""
	}
}

func parseModelPipelinePrefixBytes(bodyBytes []byte) (pipelineID, actualModel string, ok bool) {
	if len(bodyBytes) == 0 {
		return "", "", false
	}
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		return "", "", false
	}
	model, ok := reqBody["model"].(string)
	if !ok || model == "" {
		return "", "", false
	}
	return parseModelPipelinePrefix(model)
}

func extractCentagModeBytes(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}
	centag, ok := body["centag"].(map[string]interface{})
	if !ok {
		return ""
	}
	mode, ok := centag["mode"].(string)
	if !ok || strings.TrimSpace(mode) == "" {
		return ""
	}
	return mode
}

// extractCentagSceneBytes 从请求体 centag 扩展字段提取 scene（教育等场景路由参数）。
func extractCentagSceneBytes(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}
	centag, ok := body["centag"].(map[string]interface{})
	if !ok {
		return ""
	}
	scene, ok := centag["scene"].(string)
	if !ok || strings.TrimSpace(scene) == "" {
		return ""
	}
	return scene
}

// ApplyModelPipelinePrefixToBody 若 model 含 pipeline 前缀，则剥离前缀并写回请求体。
// 仅有流水线 ID、无真实模型时，回退为系统 default_model（必要时取默认后端首选模型），
// 避免 transparent_forward 把虚拟 model 名原样打到上游。
func ApplyModelPipelinePrefixToBody(body map[string]interface{}) (pipelineID string, applied bool) {
	model, ok := body["model"].(string)
	if !ok || model == "" {
		return "", false
	}
	pipelineID, actualModel, ok := parseModelPipelinePrefix(model)
	if !ok {
		return "", false
	}
	if actualModel != "" {
		body["model"] = actualModel
	} else if fallback := resolvePipelineOnlyDefaultModel(); fallback != "" {
		body["model"] = fallback
	}
	return pipelineID, true
}

// resolvePipelineOnlyDefaultModel 解析「仅 pipeline.<id>」时的默认上游模型。
func resolvePipelineOnlyDefaultModel() string {
	cfg := config.Get()
	if cfg == nil {
		return ""
	}
	if m := strings.TrimSpace(cfg.Proxy.DefaultModel); m != "" {
		return m
	}
	backendID := strings.TrimSpace(cfg.Proxy.DefaultBackendID)
	if backendID == "" {
		return ""
	}
	if mgr := backend.GetManager(); mgr != nil {
		if b, err := mgr.Get(backendID); err == nil {
			return strings.TrimSpace(backend.PreferredDefaultModel(b))
		}
	}
	return ""
}