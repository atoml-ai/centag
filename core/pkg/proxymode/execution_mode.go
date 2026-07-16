package proxymode

import (
	"errors"
	"strings"
)

// ExecutionMode 执行模式枚举
// 统一 PresetMode 和 ProxyMode 的单一模式定义
type ExecutionMode string

const (
	// === 核心执行模式 ===

	// ModeDirectBackend 指定后端模式
	// 对应: PresetMode=direct-backend, ProxyMode=#d (type=direct)
	ModeDirectBackend ExecutionMode = "direct-backend"

	// ModeTransparentProxy 透明模式：generator 直连后端，不注入网关 system prompt
	// 对应: PresetMode=transparent-proxy, ProxyMode=#t (type=transparent)
	ModeTransparentProxy ExecutionMode = "transparent-proxy"

	// ModeTransparentFast 透明模式（快）：与 ModeTransparentProxy 同语义
	// 对应: PresetMode=transparent-fast, ProxyMode=#tf (type=transparent-fast)
	ModeTransparentFast ExecutionMode = "transparent-fast"

	// ModeRawForward 原始 HTTP 转发（高级）：X-Target-URL / hostproxy
	// 对应: PresetMode=raw-forward, ProxyMode=#raw
	ModeRawForward ExecutionMode = "raw-forward"

	// ModeSystemScheduling 系统调度模式
	// 对应: PresetMode=system-scheduling, ProxyMode=#s (type=schedule)
	ModeSystemScheduling ExecutionMode = "system-scheduling"

	// ModeModelMatching 模型匹配模式
	// 对应: PresetMode=model-matching, ProxyMode=#m (type=match)
	ModeModelMatching ExecutionMode = "model-matching"

	// ModeIntentClassification 意图分类模式
	// 对应: PresetMode=intent-classification, ProxyMode=#c (type=classify)
	ModeIntentClassification ExecutionMode = "intent-classification"

	// ModePipeline 流水线编排模式
	// 对应: ProxyMode=#p (type=pipeline)，与 X-Pipeline-ID 联用
	ModePipeline ExecutionMode = "pipeline-mode"

	// === 扩展模式 ===

	// ModeFallback 降级模式
	// 对应: ProxyMode=#f (type=fallback)
	ModeFallback ExecutionMode = "fallback-mode"

	// ModeAuditMode 审核模式
	// 对应: ProxyMode=#a (type=audit)
	ModeAuditMode ExecutionMode = "audit-mode"

	// ModeOptimizeMode 优化模式
	// 对应: ProxyMode=#o (type=optimize)
	ModeOptimizeMode ExecutionMode = "optimize-mode"

	// ModeAggregator 聚合模式
	// 对应: ProxyMode=#ag (type=aggregator)
	ModeAggregator ExecutionMode = "aggregator-mode"

	// ModeRouter 路由模式
	// 对应: ProxyMode=#r (type=router)
	ModeRouter ExecutionMode = "router-mode"

	// ModeTranslate 翻译模式
	// 对应: ProxyMode=#l (type=translate)
	ModeTranslate ExecutionMode = "translate-mode"

	// ModeCacheHit 缓存命中模式
	// 对应: ProxyMode=cache-hit
	ModeCacheHit ExecutionMode = "cache-hit"

	// ModeCacheMode 缓存模式
	// 对应: ProxyMode=cache-mode
	ModeCacheMode ExecutionMode = "cache-mode"

	// ModeMem0 Mem0记忆存储模式
	// 对应: ProxyMode=#mem0 (type=mem0-memory)
	ModeMem0 ExecutionMode = "mem0-memory"

	// ModeRAG RAG 知识库网关
	// 对应: ProxyMode=#rag (type=rag-mode)
	ModeRAG ExecutionMode = "rag-mode"

	// ModeAgent 智能 Agent 分流（Pi + Mem0）
	// 对应: ProxyMode=#agent (type=agent-mode)
	ModeAgent ExecutionMode = "agent-mode"

	// ModeSecurity 安全审核防火墙
	// 对应: ProxyMode=#sec (type=security-mode)
	ModeSecurity ExecutionMode = "security-mode"

	// ModeMultilingualSupport 多语言客服（缓存 + Mem0 + 翻译）
	// 对应: ProxyMode=#cs (type=multilingual-support)
	ModeMultilingualSupport ExecutionMode = "multilingual-support"

	// ModeGeoRouting 地理 IP 路由
	// 对应: ProxyMode=#geo (type=geo-routing-mode)
	ModeGeoRouting ExecutionMode = "geo-routing-mode"

	// ModeCustom 自定义模式
	// 对应: ProxyMode=#custom (type=custom)
	ModeCustom ExecutionMode = "custom"

	// ModeCodingAgent 编程Agent模式
	// 对应: ProxyMode=#code (type=coding-agent)，使用独立编程助手流水线
	ModeCodingAgent ExecutionMode = "coding-agent"
)

// === 向后兼容别名 ===

// PresetMode 预设模式别名（向后兼容）
type PresetMode = ExecutionMode

// PresetMode 常量（向后兼容）
const (
	DirectBackendMode        = ModeDirectBackend
	TransparentProxyMode     = ModeTransparentProxy
	SystemSchedulingMode     = ModeSystemScheduling
	ModelMatchingMode        = ModeModelMatching
	IntentClassificationMode = ModeIntentClassification
)

// ModeKey 模式关键字映射
// 用于 ProxyMode 的 #d, #s, #m 等快捷方式
var ModeKey = map[string]ExecutionMode{
	"#d":       ModeDirectBackend,
	"#s":       ModeSystemScheduling,
	"#m":       ModeModelMatching,
	"#c":       ModeIntentClassification,
	"#p":       ModePipeline,
	"#t":       ModeTransparentProxy,
	"#tf":      ModeTransparentFast,
	"#raw":     ModeRawForward,
	"#f":       ModeFallback,
	"#a":       ModeAuditMode,
	"#o":       ModeOptimizeMode,
	"#ag":      ModeAggregator,
	"#r":       ModeRouter,
	"#l":       ModeTranslate,
	"#custom":  ModeCustom,
	"direct":   ModeDirectBackend,
	"schedule": ModeSystemScheduling,
	"match":    ModeModelMatching,
	"classify": ModeIntentClassification,
	"intent":   ModeIntentClassification,
	"#mem0":    ModeMem0,
	"mem0":     ModeMem0,
	"#rag":     ModeRAG,
	"rag":      ModeRAG,
	"#agent":   ModeAgent,
	"agent":    ModeAgent,
	"#sec":     ModeSecurity,
	"sec":      ModeSecurity,
	"security": ModeSecurity,
	"#cs":      ModeMultilingualSupport,
	"cs":       ModeMultilingualSupport,
	"#geo":     ModeGeoRouting,
	"geo":      ModeGeoRouting,
	"#ch":      ModeCacheHit,
	"#cm":      ModeCacheMode,
	"#code":    ModeCodingAgent,
	"coding-agent": ModeCodingAgent,
}

// ModeType 模式类型（用于 ProxyMode 的 type 字段）
var ModeType = map[string]ExecutionMode{
	"direct":                ModeDirectBackend,
	"direct-backend":        ModeDirectBackend,
	"schedule":              ModeSystemScheduling,
	"smart-scheduling":      ModeSystemScheduling,
	"system-scheduling":     ModeSystemScheduling,
	"match":                 ModeModelMatching,
	"model-matching":        ModeModelMatching,
	"classify":              ModeIntentClassification,
	"intent-classification": ModeIntentClassification,
	"pipeline":              ModePipeline,
	"pipeline-mode":         ModePipeline,
	"transparent":           ModeTransparentProxy,
	"transparent-proxy":     ModeTransparentProxy,
	"transparent-fast":      ModeTransparentFast,
	"raw-forward":           ModeRawForward,
	"raw":                   ModeRawForward,
	"fallback":              ModeFallback,
	"fallback-mode":         ModeFallback,
	"audit":                 ModeAuditMode,
	"audit-mode":            ModeAuditMode,
	"optimize":              ModeOptimizeMode,
	"optimize-mode":         ModeOptimizeMode,
	"aggregator":            ModeAggregator,
	"aggregator-mode":       ModeAggregator,
	"router":                ModeRouter,
	"router-mode":           ModeRouter,
	"translate":             ModeTranslate,
	"translate-mode":        ModeTranslate,
	"translation":           ModeTranslate,
	"cache-hit":             ModeCacheHit,
	"cache-mode":            ModeCacheMode,
	"mem0":                  ModeMem0,
	"mem0-memory":           ModeMem0,
	"agent-mode":            ModeAgent,
	"agent":                 ModeAgent,
	"security-mode":         ModeSecurity,
	"security":              ModeSecurity,
	"multilingual-support":  ModeMultilingualSupport,
	"customer-support":      ModeMultilingualSupport,
	"geo-routing-mode":      ModeGeoRouting,
	"geo-routing":           ModeGeoRouting,
	"rag-mode":              ModeRAG,
	"coding-agent":          ModeCodingAgent,
	"custom":                ModeCustom,
}

// String 返回模式的中文描述
func (m ExecutionMode) String() string {
	switch m {
	case ModeDirectBackend:
		return "指定后端"
	case ModeTransparentProxy:
		return "透明模式"
	case ModeTransparentFast:
		return "透明模式（快）"
	case ModeRawForward:
		return "原始HTTP转发"
	case ModeSystemScheduling:
		return "系统调度"
	case ModeModelMatching:
		return "模型匹配"
	case ModeIntentClassification:
		return "意图分类"
	case ModePipeline:
		return "流水线编排"
	case ModeFallback:
		return "降级"
	case ModeAuditMode:
		return "审核模式"
	case ModeOptimizeMode:
		return "优化模式"
	case ModeAggregator:
		return "聚合模式"
	case ModeRouter:
		return "路由模式"
	case ModeTranslate:
		return "翻译模式"
	case ModeCacheHit:
		return "缓存命中"
	case ModeCacheMode:
		return "缓存模式"
	case ModeMem0:
		return "Mem0记忆存储"
	case ModeRAG:
		return "RAG知识库"
	case ModeAgent:
		return "Agent智能分流"
	case ModeSecurity:
		return "安全审核防火墙"
	case ModeMultilingualSupport:
		return "多语言客服"
	case ModeGeoRouting:
		return "地理路由"
	case ModeCustom:
		return "自定义"
	case ModeCodingAgent:
		return "编程Agent"
	default:
		return string(m)
	}
}

// Description 返回模式的详细描述
func (m ExecutionMode) Description() string {
	switch m {
	case ModeDirectBackend:
		return "直连已配置后端，并注入网关 system prompt"
	case ModeTransparentProxy:
		return "直连已配置后端，不注入网关 system prompt，原样保留客户端 messages"
	case ModeTransparentFast:
		return "与透明模式相同：不注入 system prompt 的 generator 直连"
	case ModeRawForward:
		return "高级：HTTP 原样转发到 X-Target-URL 或 hostproxy 上游"
	case ModeSystemScheduling:
		return "根据负载和权重自动选择后端"
	case ModeModelMatching:
		return "根据模型名称匹配最佳后端"
	case ModeIntentClassification:
		return "使用小模型分类后路由到合适后端"
	case ModePipeline:
		return "按已注册流水线多阶段编排执行（X-Pipeline-ID）"
	case ModeFallback:
		return "主后端失败时自动降级到备用后端"
	case ModeAuditMode:
		return "由执行模型完成请求，审核模型对结果进行审核"
	case ModeOptimizeMode:
		return "由执行模型完成请求，优化模型对结果进行优化后返回"
	case ModeAggregator:
		return "多模型并行生成，聚合器综合输出"
	case ModeRouter:
		return "按关键词/规则路由到不同生成器"
	case ModeTranslate:
		return "生成后翻译为目标语言"
	case ModeCacheHit:
		return "仅读取缓存，不生成新内容"
	case ModeCacheMode:
		return "生成内容并保存到缓存"
	case ModeMem0:
		return "自动将对话保存到Mem0记忆服务"
	case ModeCustom:
		return "使用自定义路由逻辑"
	case ModeCodingAgent:
		return "使用独立编程助手流水线，通过 function calling 完成编程任务"
	default:
		return ""
	}
}

// GetKey 返回模式的关键字（用于快捷切换）
func (m ExecutionMode) GetKey() string {
	for key, mode := range ModeKey {
		if mode == m && strings.HasPrefix(key, "#") {
			return key
		}
	}
	return ""
}

// GetType 返回模式的类型字符串（用于 ProxyMode type 字段）
func (m ExecutionMode) GetType() string {
	switch m {
	case ModeDirectBackend:
		return "direct"
	case ModeSystemScheduling:
		return "schedule"
	case ModeModelMatching:
		return "match"
	case ModeIntentClassification:
		return "classify"
	case ModePipeline:
		return "pipeline"
	case ModeTransparentProxy:
		return "transparent"
	case ModeTransparentFast:
		return "transparent-fast"
	case ModeRawForward:
		return "raw-forward"
	case ModeFallback:
		return "fallback"
	case ModeAuditMode:
		return "audit"
	case ModeOptimizeMode:
		return "optimize"
	case ModeAggregator:
		return "aggregator"
	case ModeRouter:
		return "router"
	case ModeTranslate:
		return "translate"
	case ModeCacheHit:
		return "cache-hit"
	case ModeCacheMode:
		return "cache-mode"
	case ModeMem0:
		return "mem0"
	case ModeCustom:
		return "custom"
	case ModeCodingAgent:
		return "coding-agent"
	default:
		return string(m)
	}
}

// IsValid 检查模式是否有效
func (m ExecutionMode) IsValid() bool {
	switch m {
	case ModeDirectBackend, ModeTransparentProxy, ModeTransparentFast, ModeRawForward, ModeSystemScheduling,
		ModeModelMatching, ModeIntentClassification, ModePipeline, ModeFallback,
		ModeAuditMode, ModeOptimizeMode, ModeAggregator, ModeRouter, ModeTranslate,
		ModeCacheHit, ModeCacheMode, ModeCustom, ModeMem0, ModeCodingAgent:
		return true
	default:
		return false
	}
}

// FromString 从字符串解析模式
// 支持: 模式名、关键字 (#d)、类型 (direct)
func FromString(s string) (ExecutionMode, error) {
	if s == "" {
		return "", errors.New("mode string is empty")
	}

	s = strings.TrimSpace(s)

	// 尝试直接匹配
	if m := ExecutionMode(s); m.IsValid() {
		return m, nil
	}

	// 尝试关键字匹配 (#d, #s, etc.)
	if mode, ok := ModeKey[s]; ok {
		return mode, nil
	}

	// 尝试类型匹配 (direct, schedule, etc.)
	if mode, ok := ModeType[s]; ok {
		return mode, nil
	}

	return "", errors.New("unknown mode: " + s)
}

// AllModes 返回所有可用模式
func AllModes() []ExecutionMode {
	return []ExecutionMode{
		ModeDirectBackend,
		ModeTransparentProxy,
		ModeTransparentFast,
		ModeRawForward,
		ModeSystemScheduling,
		ModeModelMatching,
		ModeIntentClassification,
		ModePipeline,
		ModeFallback,
		ModeAuditMode,
		ModeOptimizeMode,
		ModeAggregator,
		ModeRouter,
		ModeTranslate,
		ModeCacheHit,
		ModeCacheMode,
		ModeMem0,
		ModeCustom,
		ModeCodingAgent,
	}
}

// CoreModes 返回核心模式列表
func CoreModes() []ExecutionMode {
	return []ExecutionMode{
		ModeDirectBackend,
		ModeTransparentProxy,
		ModeSystemScheduling,
		ModeModelMatching,
		ModeIntentClassification,
	}
}
