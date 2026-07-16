package proxymode

// 虚拟模型名 ID
// 用于区分流水线模式和直接请求，避免与真实模型名冲突
// 格式: pipeline.<mode-name>.auto
const (
	// PipelineModelFallback 降级模式虚拟模型
	PipelineModelFallback = "pipeline.fallback.auto"
	
	// PipelineModelOptimize 优化模式虚拟模型
	PipelineModelOptimize = "pipeline.optimize.auto"
	
	// PipelineModelAudit 审核模式虚拟模型
	PipelineModelAudit = "pipeline.audit.auto"
	
	// PipelineModelAggregate 聚合模式虚拟模型
	PipelineModelAggregate = "pipeline.aggregate.auto"
	
	// PipelineModelTranslate 翻译模式虚拟模型
	PipelineModelTranslate = "pipeline.translate.auto"
	
	// PipelineModelSmartScheduling 智能调度虚拟模型
	PipelineModelSmartScheduling = "pipeline.smart-scheduling.auto"
	
	// PipelineModelModelMatching 模型匹配虚拟模型
	PipelineModelModelMatching = "pipeline.model-matching.auto"
	
	// PipelineModelRouter 路由模式虚拟模型
	PipelineModelRouter = "pipeline.router.auto"
	
	// PipelineModelDirectBackend 直连后端虚拟模型（使用请求中的 model）
	PipelineModelDirectBackend = "pipeline.direct-backend.auto"
	
	// PipelineModelTransparentProxy 透明代理虚拟模型
	PipelineModelTransparentProxy = "pipeline.transparent-proxy.auto"

	// PipelineModelTransparentFast 透明模式（快）虚拟模型
	PipelineModelTransparentFast = "pipeline.transparent-fast.auto"

	// PipelineModelRawForward 原始 HTTP 转发虚拟模型
	PipelineModelRawForward = "pipeline.raw-forward.auto"
	
	// PipelineModelPipeline 通用流水线虚拟模型
	PipelineModelPipeline = "pipeline.custom.auto"

	// PipelineModelCodingAgent 编程Agent虚拟模型
	// 通过 /model pipeline.coding-agent.auto 切换到独立编程助手流水线
	PipelineModelCodingAgent = "pipeline.coding-agent.auto"
)

// GetPipelineModel 根据模式获取虚拟模型名
func GetPipelineModel(mode ExecutionMode) string {
	switch mode {
	case ModeFallback:
		return PipelineModelFallback
	case ModeOptimizeMode:
		return PipelineModelOptimize
	case ModeAuditMode:
		return PipelineModelAudit
	case ModeAggregator:
		return PipelineModelAggregate
	case ModeTranslate:
		return PipelineModelTranslate
	case ModeSystemScheduling:
		return PipelineModelSmartScheduling
	case ModeModelMatching:
		return PipelineModelModelMatching
	case ModeRouter:
		return PipelineModelRouter
	case ModeDirectBackend:
		return PipelineModelDirectBackend
	case ModeTransparentProxy:
		return PipelineModelTransparentProxy
	case ModeTransparentFast:
		return PipelineModelTransparentFast
	case ModeRawForward:
		return PipelineModelRawForward
	case ModePipeline:
		return PipelineModelPipeline
	case ModeCodingAgent:
		return PipelineModelCodingAgent
	default:
		return ""
	}
}

// IsPipelineModel 检查是否为虚拟模型名
func IsPipelineModel(model string) bool {
	return len(model) > 9 && model[:9] == "pipeline."
}
