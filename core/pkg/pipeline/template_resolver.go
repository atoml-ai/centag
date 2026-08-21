package pipeline

import (
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/config"
)

// TemplateVarResolver 解析 NodeConfig.TemplateVars 中的路径表达式为实际值。
//
// 支持的路径语法：
//
//	input.content                  → 原始用户输入内容
//	input.metadata.<key>           → 输入元数据字段
//	node.<nodeID>.content          → 指定上游节点的输出内容
//	node.<nodeID>.metadata.<key>   → 指定上游节点的元数据字段
//	node.<nodeID>.score            → 指定上游节点的评分（审核节点输出）
//	node.<nodeID>.passed           → 指定上游节点的是否通过（审核节点输出）
//	node.<nodeID>.feedback         → 指定上游节点的反馈文本
//	context.timestamp              → 当前 RFC3339 时间戳
//	context.user_id                → 执行上下文中的用户 ID
//	context.session_id             → 执行上下文中的会话 ID
//	context.pipeline_id            → 当前流水线 ID
//	pipeline.<key>                 → 流水线元数据字段（从 pipeline.metadata 读取）
//	system.default_backend         → 系统默认后端 ID
//	system.default_model           → 系统默认模型
//	system.fallback_backend        → 系统降级后端 ID
//	system.fallback_model          → 系统降级模型
//	literal:<任意字符串>            → 字面量值（冒号后原样返回）
type TemplateVarResolver struct {
	input   *NodeInput
	execCtx *ExecutionContext
}

// NewTemplateVarResolver 创建解析器。execCtx 可为 nil（降级时仅解析 input 相关路径）。
func NewTemplateVarResolver(input *NodeInput, execCtx *ExecutionContext) *TemplateVarResolver {
	return &TemplateVarResolver{input: input, execCtx: execCtx}
}

// Resolve 解析单条路径表达式，返回对应的值。
// 支持管道语法：pipeline.xxx | default: 'value'
func (r *TemplateVarResolver) Resolve(path string) (interface{}, error) {
	if strings.HasPrefix(path, "literal:") {
		return path[len("literal:"):], nil
	}

	// 处理管道语法：path | default: 'value'
	var defaultValue string
	actualPath := path
	if idx := strings.Index(path, "|"); idx != -1 {
		actualPath = strings.TrimSpace(path[:idx])
		rest := strings.TrimSpace(path[idx+1:])
		if strings.HasPrefix(rest, "default:") {
			defaultValue = strings.TrimSpace(rest[len("default:"):])
			// 移除引号
			defaultValue = strings.Trim(defaultValue, "'\"")
		}
	}

	parts := strings.SplitN(actualPath, ".", 3)
	switch parts[0] {
	case "input":
		return r.resolveInput(parts[1:])
	case "node":
		return r.resolveNode(parts[1:])
	case "context":
		return r.resolveContext(parts[1:])
	case "pipeline":
		result, err := r.resolvePipeline(parts[1:])
		if err != nil && defaultValue != "" {
			return defaultValue, nil
		}
		return result, err
	case "system":
		return r.resolveSystem(parts[1:])
	default:
		if defaultValue != "" {
			return defaultValue, nil
		}
		return nil, fmt.Errorf("unknown path prefix %q (支持: input / node / context / pipeline / system / literal:)", parts[0])
	}
}

func (r *TemplateVarResolver) resolveInput(parts []string) (interface{}, error) {
	if len(parts) == 0 || parts[0] == "content" {
		return r.input.Content, nil
	}
	if parts[0] == "metadata" {
		if len(parts) < 2 {
			return r.input.Metadata, nil
		}
		return r.input.Metadata[parts[1]], nil
	}
	return nil, fmt.Errorf("unknown input field: %s", parts[0])
}

func (r *TemplateVarResolver) resolveNode(parts []string) (interface{}, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("node path 格式: node.<nodeID>.<field>")
	}
	nodeID := parts[0]

	// 优先从 UpstreamResults 中查找
	var result *NodeOutput
	if r.input.UpstreamResults != nil {
		result = r.input.UpstreamResults[nodeID]
	}
	if result == nil {
		return nil, fmt.Errorf("上游节点 %q 的输出不存在（确认该节点在 depends_on 中且已执行）", nodeID)
	}

	field := parts[1]
	switch field {
	case "content":
		return result.Content, nil
	case "feedback":
		return result.Feedback, nil
	case "score":
		if result.Score != nil {
			return *result.Score, nil
		}
		return nil, nil
	case "passed":
		if result.Passed != nil {
			return *result.Passed, nil
		}
		return nil, nil
	case "metadata":
		if len(parts) < 3 {
			return result.Metadata, nil
		}
		if result.Metadata == nil {
			return nil, nil
		}
		return result.Metadata[parts[2]], nil
	default:
		return nil, fmt.Errorf("unknown node field: %s (支持: content / feedback / score / passed / metadata.<key>)", field)
	}
}

// resolvePipeline 解析流水线元数据字段
// pipeline.<key> → 从流水线 metadata 中读取
func (r *TemplateVarResolver) resolvePipeline(parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("pipeline path 需要字段名")
	}

	if r.execCtx == nil || r.execCtx.pipeline == nil {
		return nil, fmt.Errorf("pipeline context not available")
	}

	// 从流水线 metadata 中读取
	key := parts[0]
	if r.execCtx.pipeline.Metadata != nil {
		if v, ok := r.execCtx.pipeline.Metadata[key]; ok {
			return v, nil
		}
	}

	return nil, fmt.Errorf("pipeline metadata field %q not found", key)
}

func (r *TemplateVarResolver) resolveContext(parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("context path 需要字段名")
	}
	switch parts[0] {
	case "timestamp":
		return time.Now().Format(time.RFC3339), nil
	case "user_id":
		if r.execCtx != nil {
			if v, ok := r.execCtx.GetVariable("user_id"); ok {
				return fmt.Sprintf("%v", v), nil
			}
		}
		return "", nil
	case "session_id":
		if r.execCtx != nil {
			if v, ok := r.execCtx.GetVariable("session_id"); ok {
				return fmt.Sprintf("%v", v), nil
			}
		}
		return "", nil
	case "pipeline_id":
		if r.execCtx != nil {
			return r.execCtx.pipeline.ID, nil
		}
		return "", nil
	default:
		// 尝试从执行上下文变量中兜底查找
		if r.execCtx != nil {
			if v, ok := r.execCtx.GetVariable(parts[0]); ok {
				return fmt.Sprintf("%v", v), nil
			}
		}
		return nil, fmt.Errorf("unknown context field: %s", parts[0])
	}
}

// resolveSystem 解析系统级配置路径
func (r *TemplateVarResolver) resolveSystem(parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("system path 需要字段名")
	}

	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("system config not initialized")
	}

	switch parts[0] {
	case "default_backend":
		return cfg.Proxy.DefaultBackendID, nil
	case "default_model":
		return cfg.Proxy.DefaultModel, nil
	case "fallback_backend":
		// 优先从 ModelVariables 读取（Model Config 页面配置的值）
		if v, ok := cfg.ModelVariables.SystemVariables["system.fallback_backend"]; ok && v != "" {
			return v, nil
		}
		return cfg.Proxy.FallbackBackendID, nil
	case "fallback_model":
		// 优先从 ModelVariables 读取（Model Config 页面配置的值）
		if v, ok := cfg.ModelVariables.SystemVariables["system.fallback_model"]; ok && v != "" {
			return v, nil
		}
		return cfg.Proxy.FallbackModel, nil
	case "embedding_backend":
		return cfg.Embedding.BackendID, nil
	case "embedding_model":
		return cfg.Embedding.Model, nil
	case "rerank_backend":
		if v, ok := cfg.ModelVariables.SystemVariables["system.rerank_backend"]; ok {
			return v, nil
		}
		return "", nil
	case "rerank_model":
		if v, ok := cfg.ModelVariables.SystemVariables["system.rerank_model"]; ok {
			return v, nil
		}
		return "", nil
	case "classify_backend":
		if v, ok := cfg.ModelVariables.SystemVariables["system.classify_backend"]; ok {
			return v, nil
		}
		return "", nil
	case "classify_model":
		if v, ok := cfg.ModelVariables.SystemVariables["system.classify_model"]; ok {
			return v, nil
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unknown system field: %s (支持: default_backend / default_model / fallback_backend / fallback_model / embedding_backend / embedding_model / rerank_backend / rerank_model / classify_backend / classify_model)", parts[0])
	}
}

// BuildTemplateData 构建最终传入 Go template 的 data map。
//
// 优先级（高优先级覆盖低优先级）：
//  3. 显式 template_vars 绑定（最高）
//  2. input.Metadata 自动展开（中）
//  1. 内置变量 builtinVars（最低）
//
// 解析失败的 template_vars 条目会被跳过（降级），同时通过第二返回值暴露失败信息，
// 调用方可选择记录日志便于排查配置问题。
func (r *TemplateVarResolver) BuildTemplateData(
	builtinVars map[string]interface{},
	templateVarsConfig map[string]string,
) (map[string]interface{}, []string) {
	data := make(map[string]interface{}, len(builtinVars)+len(r.input.Metadata)+len(templateVarsConfig))
	var resolveErrors []string

	// 1. 内置变量
	for k, v := range builtinVars {
		data[k] = v
	}

	// 2. 自动展开 input.Metadata（不覆盖内置变量）
	for k, v := range r.input.Metadata {
		if _, exists := data[k]; !exists {
			data[k] = v
		}
	}

	// 3. 显式 template_vars（最高优先级，可覆盖前两层）
	for varName, path := range templateVarsConfig {
		if varName == "" || path == "" {
			continue
		}
		value, err := r.Resolve(path)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("%s=%s: %v", varName, path, err))
			continue
		}
		if value != nil {
			data[varName] = value
		}
	}

	return data, resolveErrors
}
