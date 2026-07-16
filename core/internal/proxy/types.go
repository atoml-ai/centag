package proxy

// ProxyMode 代理模式类型
type ProxyMode string

const (
	// ModeDefault 默认模式标记（由 DefaultPipelineResolver 解析实际流水线）
	ModeDefault ProxyMode = "__default__"

	// ModeSmartScheduling 智能缓存调度模式 (使用缓存匹配策略)
	ModeSmartScheduling ProxyMode = "smart-scheduling"

	// ModeDirectBackend 指定后端模式 (直接使用指定后端)
	ModeDirectBackend ProxyMode = "direct-backend"

	// ModeTransparentProxy 透明模式：generator 直连后端，不注入网关 system prompt
	ModeTransparentProxy ProxyMode = "transparent-proxy"

	// ModeTransparentFast 透明模式（快）：与 ModeTransparentProxy 同语义
	ModeTransparentFast ProxyMode = "transparent-fast"

	// ModeRawForward 原始 HTTP 转发（高级）：依赖 X-Target-URL / hostproxy
	ModeRawForward ProxyMode = "raw-forward"

	// ModeModelMatching 模型匹配调度模式 (使用小模型分析 + 路由决策)
	ModeModelMatching ProxyMode = "model-matching"

	// ModeIntentClassification 意图分类模式 (使用小模型分类 + 任务策略路由)
	ModeIntentClassification ProxyMode = "intent-classification"

	// ModeAuditMode 审核模式 (由执行模型完成请求，审核模型对结果进行审核)
	ModeAuditMode ProxyMode = "audit-mode"

	// ModeOptimizeMode 优化模式 (由执行模型完成请求，优化模型对结果进行优化)
	ModeOptimizeMode ProxyMode = "optimize-mode"

	// ModePipeline 流水线模式 (使用可配置的流水线编排多阶段AI处理)
	ModePipeline ProxyMode = "pipeline-mode"

	// ModeFallback 降级模式 (主后端失败后自动降级到备用后端)
	ModeFallback ProxyMode = "fallback-mode"

	// ModeAggregator 聚合模式 (多模型并行生成，聚合器综合输出)
	ModeAggregator ProxyMode = "aggregator-mode"

	// ModeRouter 路由模式 (按关键词/规则路由到不同生成器)
	ModeRouter ProxyMode = "router-mode"

	// ModeTranslate 翻译模式 (生成 → 翻译)
	ModeTranslate ProxyMode = "translate-mode"

	// ModeMem0 Mem0记忆存储模式 (自动保存对话到Mem0)
	ModeMem0 ProxyMode = "mem0-memory"

	// ModeCacheHit 缓存命中模式 (仅读取缓存)
	ModeCacheHit ProxyMode = "cache-hit"

	// ModeCacheMode 缓存模式 (生成内容并保存到缓存)
	ModeCacheMode ProxyMode = "cache-mode"

	// ModeRAG RAG 知识库网关
	ModeRAG ProxyMode = "rag-mode"

	// ModeAgent Agent 智能分流
	ModeAgent ProxyMode = "agent-mode"

	// ModeSecurity 安全审核防火墙
	ModeSecurity ProxyMode = "security-mode"
)

// TransparentProxyTarget 透明代理目标配置
// 透明代理模式下只改变访问地址，不改变请求内容
type TransparentProxyTarget struct {
	BaseURL string // 目标 API 的基础 URL（必需）
	APIKey  string // 目标 API 的密钥（可选，用于日志记录）
	Model   string // 目标模型名称（可选，仅用于日志）
}

// CacheControl 缓存控制
type CacheControl struct {
	Read        bool // 是否读取缓存
	Write       bool // 是否写入缓存
	QASplit     bool // 是否进行问答拆分
	SaveOnly    bool // 是否是仅保存模式
}

// ProxyContext 代理上下文
type ProxyContext struct {
	Mode         ProxyMode
	CacheControl CacheControl
	BackendID    string // 仅用于 ModeDirectBackend
	TargetURL    string // 仅用于 ModeTransparentProxy (已废弃，保留兼容性)
}
