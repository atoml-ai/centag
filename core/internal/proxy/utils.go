package proxy

import (
	"net/http"
	"strings"

	"centag/core/pkg/config"
)

// DetectProxyMode 定义见 mode_detection.go。

// normalizeProxyMode 规范化代理模式关键字
// 将前端的关键字（如 #c, #m, #s 等）映射到后端的模式常量
func normalizeProxyMode(mode string) ProxyMode {
	trimmed := strings.TrimSpace(strings.ToLower(mode))
	if trimmed == "" {
		return ModeSmartScheduling
	}
	switch trimmed {
	case "#c", "classify", "intent-classification":
		return ModeIntentClassification
	case "#s", "schedule", "smart", "smart-scheduling":
		return ModeSmartScheduling
	case "#d", "direct", "direct-backend":
		return ModeDirectBackend
	case "#t", "transparent", "transparent-proxy":
		return ModeTransparentProxy
	case "#tf", "transparent-fast":
		return ModeTransparentFast
	case "#raw", "raw", "raw-forward":
		return ModeRawForward
	case "#m", "match", "model-matching":
		return ModeModelMatching
	case "#f", "fallback":
		return ModeFallback
	case "#a", "audit", "audit-mode":
		return ModeAuditMode
	case "#o", "optimize", "optimize-mode":
		return ModeOptimizeMode
	case "#p", "pipeline", "pipeline-mode":
		return ModePipeline
	case "#ag", "aggregator", "aggregator-mode":
		return ModeAggregator
	case "#agent", "agent", "agent-mode":
		return ModeAgent
	case "#rag", "rag", "rag-mode":
		return ModeRAG
	case "#r", "router", "router-mode":
		return ModeRouter
	case "#l", "translate", "translation", "translate-mode":
		return ModeTranslate
	case "#mem0", "mem0", "mem0-memory":
		return ModeMem0
	case "#ch", "cache-hit":
		return ModeCacheHit
	case "#cm", "cache-mode":
		return ModeCacheMode
	case "#sec", "sec", "security", "security-mode":
		return ModeSecurity
	default:
		return ProxyMode(mode)
	}
}

// extractPipelineFromModel 保留供测试兼容；优先使用 parseModelPipelinePrefix。
func extractPipelineFromModel(r *http.Request) string {
	bodyBytes := peekRequestBody(r)
	pipelineID, _, ok := parseModelPipelinePrefixBytes(bodyBytes)
	if !ok {
		return ""
	}
	return pipelineID
}

// extractPipelineFromContent 保留供测试兼容。
func extractPipelineFromContent(r *http.Request) string {
	return extractPipelineFromContentBytes(peekRequestBody(r))
}

// DetectCacheControl 从请求中检测缓存控制
//
// 读/写是否参与缓存流程以 Cache 中的 EnableCacheRead / EnableCacheWrite 为准
//（与 WebUI「启用缓存命中/写入」一致）。历史库里的 cache_control.default_read、
// default_write 不再覆盖上述主开关，避免界面已打开读缓存但 DB 里 default_read=false
// 导致流式/非流式整段跳过缓存查询。
//
// CacheControl.DefaultQASplit 仍在 Enabled 时作为「默认是否允许 QA 拆分」相关响应头
// 的基线；实际是否拆分仍由 QASplit 服务配置与 ShouldSplitQA 等逻辑决定。
func DetectCacheControl(r *http.Request) CacheControl {
	cfg := config.Get()

	defaultRead := true
	defaultWrite := true
	defaultQASplit := true
	defaultSaveOnly := false

	if cfg != nil {
		defaultRead = cfg.Cache.EnableCacheRead
		defaultWrite = cfg.Cache.EnableCacheWrite
		if cfg.Cache.SaveOnlyMode {
			defaultSaveOnly = true
			defaultQASplit = false
		}
		if cfg.CacheControl.Enabled {
			defaultQASplit = cfg.CacheControl.DefaultQASplit
			if cfg.Cache.SaveOnlyMode {
				defaultQASplit = false
			}
		}
	}

	return CacheControl{
		Read:     parseBoolHeader(r, "X-Cache-Read", defaultRead),
		Write:    parseBoolHeader(r, "X-Cache-Write", defaultWrite),
		QASplit:  parseBoolHeader(r, "X-QA-Split", defaultQASplit),
		SaveOnly: defaultSaveOnly,
	}
}

// parseBoolHeader 解析布尔类型的请求头
func parseBoolHeader(r *http.Request, header string, defaultValue bool) bool {
	value := r.Header.Get(header)
	if value == "" {
		return defaultValue
	}

	lowerValue := strings.ToLower(value)
	return lowerValue == "enable" ||
		lowerValue == "true" ||
		lowerValue == "1" ||
		lowerValue == "yes"
}

// IsValidProxyMode 检查代理模式是否有效。
// 用户自定义流水线 id / 快捷码解析后的模式同样合法，不在此枚举限制。
func IsValidProxyMode(mode ProxyMode) bool {
	return strings.TrimSpace(string(mode)) != ""
}

// StripProxyModePrefix 去除消息开头的代理模式关键字前缀
// 支持任意 #xxx 前缀，例如 "#c python是干嘛的" -> "python是干嘛的"
func StripProxyModePrefix(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "#") {
		return trimmed
	}
	// 找到第一个空白字符的位置，其前的部分就是 #xxx 前缀
	idx := -1
	for i, r := range trimmed {
		if i == 0 {
			continue // 跳过开头的 #
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			idx = i
			break
		}
	}
	if idx > 0 {
		return strings.TrimSpace(trimmed[idx:])
	}
	// 没有空白字符，整个内容就是前缀，返回空
	return ""
}
