package proxy

// 以下映射仅用于配置诊断/管理 API（config_checker），不参与请求分发。
// 运行时流水线解析请使用 PipelineResolver（注册表 + 存储 + X-Pipeline-ID）。

// ModeToPipelineMapping 模式到流水线的映射（诊断用）
type ModeToPipelineMapping struct {
	Mode        ProxyMode
	PipelineID  string
	Description string
	Enabled     bool
}

// defaultModeMappings 内置模板参考列表（诊断用，非运行时真源）
var defaultModeMappings = []ModeToPipelineMapping{
	{Mode: ModeAuditMode, PipelineID: "audit-mode", Description: "审核模式流水线", Enabled: true},
	{Mode: ModeOptimizeMode, PipelineID: "optimize-mode", Description: "优化模式流水线", Enabled: true},
	{Mode: ModeDirectBackend, PipelineID: "direct-backend", Description: "直连后端（注入 system prompt）", Enabled: true},
	{Mode: ModeTransparentProxy, PipelineID: "transparent-proxy", Description: "透明模式（不注入 system prompt）", Enabled: true},
	{Mode: ModeTransparentFast, PipelineID: "transparent-fast", Description: "透明模式快（不注入 system prompt）", Enabled: true},
	{Mode: ModeRawForward, PipelineID: "raw-forward", Description: "原始 HTTP 转发（需 Target-URL/hostproxy）", Enabled: true},
	{Mode: ModeFallback, PipelineID: "fallback-mode", Description: "降级容错流水线", Enabled: true},
	{Mode: ModeModelMatching, PipelineID: "model-matching", Description: "模型匹配流水线", Enabled: true},
	{Mode: ModeIntentClassification, PipelineID: "router-mode", Description: "意图分类流水线", Enabled: true},
	{Mode: ModeSmartScheduling, PipelineID: "smart-scheduling", Description: "智能调度流水线", Enabled: true},
	{Mode: ModePipeline, PipelineID: "pipeline-mode", Description: "通用流水线模式", Enabled: true},
	{Mode: ModeAggregator, PipelineID: "aggregator-mode", Description: "聚合模式流水线", Enabled: true},
	{Mode: ModeRouter, PipelineID: "router-mode", Description: "路由模式流水线", Enabled: true},
	{Mode: ModeTranslate, PipelineID: "translate-mode", Description: "翻译模式流水线", Enabled: true},
	{Mode: ModeMem0, PipelineID: "mem0-memory", Description: "Mem0 记忆存储流水线", Enabled: true},
	{Mode: ModeCacheHit, PipelineID: "cache-hit", Description: "缓存优先流水线", Enabled: true},
	{Mode: ModeCacheMode, PipelineID: "cache-mode", Description: "缓存写入流水线", Enabled: true},
	{Mode: ModeSecurity, PipelineID: "security-mode", Description: "安全审核防火墙流水线", Enabled: true},
	{Mode: ProxyMode("multilingual-support"), PipelineID: "multilingual-support", Description: "多语言客服流水线", Enabled: true},
	{Mode: ProxyMode("geo-routing-mode"), PipelineID: "geo-routing-mode", Description: "地理路由流水线", Enabled: true},
}