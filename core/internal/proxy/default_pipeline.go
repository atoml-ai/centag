package proxy

import (
	"context"
	"fmt"

	"centag/core/pkg/config"
)

// DefaultPipelineResolver 默认流水线解析器
// 负责在无模式指定时解析应该使用的流水线
type DefaultPipelineResolver struct {
	config         *config.Config
	userQuotaService UserQuotaService
}

// UserQuotaService 用户额度服务接口（用于获取用户默认流水线）
type UserQuotaService interface {
	GetUserDefaultPipelineID(ctx context.Context, userID int64) (string, error)
}

// NewDefaultPipelineResolver 创建默认流水线解析器
func NewDefaultPipelineResolver(cfg *config.Config) *DefaultPipelineResolver {
	return &DefaultPipelineResolver{
		config: cfg,
	}
}

// SetUserQuotaService 设置用户额度服务
func (r *DefaultPipelineResolver) SetUserQuotaService(service UserQuotaService) {
	r.userQuotaService = service
}

// ResolveProxyMode 解析请求应该使用的流水线模式
// 返回值：(proxyMode, 来源, 错误)
// v2.1: 优先使用用户默认流水线，其次使用系统默认
func (r *DefaultPipelineResolver) ResolveProxyMode(
	ctx context.Context,
	model string,
	userID *int64,
	tenantID *string,
) (ProxyMode, string, error) {
	if r.config == nil {
		return ProxyMode(config.DefaultSystemPipelineID), "fallback", nil
	}

	// v2.1: 如果有用户ID，先检查用户默认流水线
	if userID != nil && r.userQuotaService != nil {
		if userPipelineID, err := r.userQuotaService.GetUserDefaultPipelineID(ctx, *userID); err == nil && userPipelineID != "" {
			return ProxyMode(userPipelineID), "user-default", nil
		}
	}

	// 使用系统默认流水线
	pipelineID := r.config.Proxy.EffectiveDefaultPipeline()
	source := defaultPipelineSource(r.config.Proxy)
	// 默认流水线以用户配置的 pipeline id 为准，不依赖内置模式枚举映射。
	return ProxyMode(pipelineID), source, nil
}

// PipelineIDToMode 将流水线 ID 转换为 ProxyMode
func PipelineIDToMode(pipelineID string) ProxyMode {
	switch pipelineID {
	case "smart-scheduling":
		return ModeSmartScheduling
	case "direct-backend":
		return ModeDirectBackend
	case "transparent-proxy":
		return ModeTransparentProxy
	case "transparent-fast":
		return ModeTransparentFast
	case "fixed-egress":
		return ModeFixedEgress
	case "model-matching":
		return ModeModelMatching
	case "audit-mode":
		return ModeAuditMode
	case "optimize-mode":
		return ModeOptimizeMode
	case "fallback-mode":
		return ModeFallback
	case "router-mode":
		return ModeRouter
	case "translate-mode":
		return ModeTranslate
	case "aggregator-mode":
		return ModeAggregator
	case "mem0-memory":
		return ModeMem0
	case "cache-hit":
		return ModeCacheHit
	case "cache-mode":
		return ModeCacheMode
	case "pipeline-mode":
		return ModePipeline
	default:
		return ""
	}
}

// getPipelineConfig 获取 PipelineConfig，支持 nil 安全
func (r *DefaultPipelineResolver) getPipelineConfig() *config.PipelineConfig {
	if r.config != nil && r.config.Proxy.PipelineConfig != nil {
		return r.config.Proxy.PipelineConfig
	}
	return nil
}

// Resolve 解析请求应该使用的流水线（返回流水线 ID）
// 返回值：(pipelineID, 来源, 错误)
func (r *DefaultPipelineResolver) Resolve(
	ctx context.Context,
) (string, string, error) {
	if r.config == nil {
		return config.DefaultSystemPipelineID, "fallback", nil
	}
	return r.config.Proxy.EffectiveDefaultPipeline(), defaultPipelineSource(r.config.Proxy), nil
}

// FallbackMode 在解析失败时返回已配置的默认流水线（与 EffectiveDefaultPipeline 一致）。
func (r *DefaultPipelineResolver) FallbackMode() (ProxyMode, string) {
	if r.config == nil {
		return ProxyMode(config.DefaultSystemPipelineID), "fallback"
	}
	pipelineID := r.config.Proxy.EffectiveDefaultPipeline()
	source := defaultPipelineSource(r.config.Proxy)
	return ProxyMode(pipelineID), source
}

func defaultPipelineSource(proxy config.ProxyConfig) string {
	if proxy.PipelineConfig != nil && proxy.PipelineConfig.DefaultPipeline != "" {
		return "system-default"
	}
	if proxy.DefaultMode != "" {
		return "system-default"
	}
	return "fallback"
}

// GetPipelineIDByMode 根据 ProxyMode 获取对应的流水线 ID
func GetPipelineIDByMode(mode ProxyMode) string {
	switch mode {
	case ModeSmartScheduling:
		return "smart-scheduling"
	case ModeDirectBackend:
		return "direct-backend"
	case ModeTransparentProxy:
		return "transparent-proxy"
	case ModeTransparentFast:
		return "transparent-fast"
	case ModeFixedEgress:
		return "fixed-egress"
	case ModeModelMatching:
		return "model-matching"
	case ModeAuditMode:
		return "audit-mode"
	case ModeOptimizeMode:
		return "optimize-mode"
	case ModeFallback:
		return "fallback-mode"
	case ModeRouter:
		return "router-mode"
	case ModeTranslate:
		return "translate-mode"
	case ModeAggregator:
		return "aggregator-mode"
	case ModeMem0:
		return "mem0-memory"
	case ModePipeline:
		return "pipeline-mode"
	default:
		return string(mode)
	}
}

// String 返回 ProxyMode 的字符串表示
func (m ProxyMode) String() string {
	return string(m)
}

// fmt 使用
var _ = fmt.Sprintf
