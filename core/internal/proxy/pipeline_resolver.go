package proxy

import (
	"strings"

	"centag/core/pkg/pipeline"
)

// PipelineResolver 根据用户已加载的流水线配置动态解析 pipeline ID。
// 不依赖内置流水线名称硬编码；策略管理列表 / 注册表 / 存储为唯一数据源。
type PipelineResolver struct {
	engine   PipelineEngineInterface
	registry *pipeline.PipelineRegistry
	store    pipeline.PipelineStore
}

// NewPipelineResolver 创建流水线解析器。
func NewPipelineResolver(
	engine PipelineEngineInterface,
	registry *pipeline.PipelineRegistry,
	store pipeline.PipelineStore,
) *PipelineResolver {
	return &PipelineResolver{
		engine:   engine,
		registry: registry,
		store:    store,
	}
}

// Resolve 解析代理模式或快捷码为流水线 ID。
// 优先级：X-Pipeline-ID 请求头 > 模式字符串作为 pipeline id > 注册表快捷码 > 存储快捷码。
func (r *PipelineResolver) Resolve(mode ProxyMode, headerPipelineID string) string {
	if pid := strings.TrimSpace(headerPipelineID); pid != "" && r.hasPipeline(pid) {
		return pid
	}

	modeStr := strings.TrimSpace(string(mode))
	if modeStr == "" || modeStr == string(ModeDefault) {
		return ""
	}

	if r.hasPipeline(modeStr) {
		return modeStr
	}

	if pid := resolvePipelineModeAlias(modeStr); pid != "" && r.hasPipeline(pid) {
		return pid
	}

	if pid := r.resolveShortcutFromRegistry(modeStr); pid != "" {
		return pid
	}

	if pid := r.resolveShortcutFromStore(modeStr); pid != "" {
		return pid
	}

	return ""
}

func (r *PipelineResolver) hasPipeline(pipelineID string) bool {
	if r.engine == nil {
		return false
	}
	return r.engine.HasPipeline(pipelineID)
}

func (r *PipelineResolver) resolveShortcutFromRegistry(modeStr string) string {
	if r.registry == nil {
		return ""
	}
	for _, code := range shortcutCodeCandidates(modeStr) {
		for _, p := range r.registry.ListAll() {
			if p == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(p.ShortcutCode), code) {
				return p.ID
			}
		}
	}
	return ""
}

func (r *PipelineResolver) resolveShortcutFromStore(modeStr string) string {
	if r.store == nil {
		return ""
	}
	for _, code := range shortcutCodeCandidates(modeStr) {
		p, err := r.store.GetByShortcutCode(code)
		if err == nil && p != nil {
			return p.ID
		}
	}
	return ""
}

// resolvePipelineModeAlias maps legacy / shorthand mode keys to canonical pipeline IDs.
func resolvePipelineModeAlias(modeStr string) string {
	switch strings.ToLower(strings.TrimSpace(modeStr)) {
	case "#c", "c", string(ModeIntentClassification):
		return "router-mode"
	case "#edu", "edu", "education-scene":
		return "education-scene"
	default:
		return ""
	}
}

func shortcutCodeCandidates(modeStr string) []string {
	modeStr = strings.TrimSpace(modeStr)
	if modeStr == "" {
		return nil
	}
	candidates := []string{modeStr}
	if strings.HasPrefix(modeStr, "#") {
		trimmed := strings.TrimPrefix(modeStr, "#")
		if trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	} else {
		candidates = append(candidates, "#"+modeStr)
	}
	return candidates
}