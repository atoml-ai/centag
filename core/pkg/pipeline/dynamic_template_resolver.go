package pipeline

import (
	"fmt"
	"strings"
)

// DynamicTemplateResolver 支持动态配置的模板解析器
// 支持从请求头、查询参数、上下文变量解析模板
type DynamicTemplateResolver struct {
	*TemplateVarResolver
	RequestHeaders map[string]string
	QueryParams    map[string]string
	ContextVars    map[string]interface{}
}

// NewDynamicTemplateResolver 创建动态模板解析器
func NewDynamicTemplateResolver(
	input *NodeInput,
	execCtx *ExecutionContext,
	headers map[string]string,
	queryParams map[string]string,
) *DynamicTemplateResolver {
	return &DynamicTemplateResolver{
		TemplateVarResolver: NewTemplateVarResolver(input, execCtx),
		RequestHeaders:      headers,
		QueryParams:         queryParams,
		ContextVars:         make(map[string]interface{}),
	}
}

// SetContextVar 设置上下文变量
func (r *DynamicTemplateResolver) SetContextVar(key string, value interface{}) {
	r.ContextVars[key] = value
}

// Resolve 解析模板变量
// 支持新的路径语法：
//   header.<name>           → 请求头值
//   param.<name>            → 查询参数值
//   context.<name>          → 上下文变量
//   input.content           → 原始输入
//   node.<id>.content       → 上游节点输出
//   metadata.<key>          → 元数据
func (r *DynamicTemplateResolver) Resolve(path string) (interface{}, error) {
	// 首先尝试原有的解析逻辑
	if value, err := r.TemplateVarResolver.Resolve(path); err == nil && value != nil {
		return value, nil
	}

	// 尝试新的动态路径
	parts := strings.SplitN(path, ".", 2)
	if len(parts) < 2 {
		return r.TemplateVarResolver.Resolve(path)
	}

	prefix := parts[0]
	key := parts[1]

	switch prefix {
	case "header":
		return r.resolveHeader(key)
	case "param":
		return r.resolveQueryParam(key)
	case "metadata":
		return r.resolveMetadata(key)
	default:
		// 回退到原有解析器
		return r.TemplateVarResolver.Resolve(path)
	}
}

func (r *DynamicTemplateResolver) resolveHeader(key string) (interface{}, error) {
	// 支持大小写不敏感匹配
	for k, v := range r.RequestHeaders {
		if strings.EqualFold(k, key) {
			return v, nil
		}
	}
	return nil, fmt.Errorf("header %q not found", key)
}

func (r *DynamicTemplateResolver) resolveQueryParam(key string) (interface{}, error) {
	if value, ok := r.QueryParams[key]; ok {
		return value, nil
	}
	return nil, fmt.Errorf("query param %q not found", key)
}

func (r *DynamicTemplateResolver) resolveMetadata(key string) (interface{}, error) {
	if r.input != nil && r.input.Metadata != nil {
		if value, ok := r.input.Metadata[key]; ok {
			return value, nil
		}
	}
	return nil, fmt.Errorf("metadata key %q not found", key)
}

// BuildDynamicTemplateData 构建包含动态配置的模板数据
func (r *DynamicTemplateResolver) BuildDynamicTemplateData(
	builtinVars map[string]interface{},
	templateVarsConfig map[string]string,
) (map[string]interface{}, []string) {
	// 首先构建基础模板数据
	data, resolveErrors := r.TemplateVarResolver.BuildTemplateData(builtinVars, templateVarsConfig)

	// 添加动态变量
	// 1. 请求头（前缀为 header_）
	for k, v := range r.RequestHeaders {
		key := "header_" + strings.ReplaceAll(strings.ToLower(k), "-", "_")
		data[key] = v
	}

	// 2. 查询参数（前缀为 param_）
	for k, v := range r.QueryParams {
		key := "param_" + strings.ToLower(k)
		data[key] = v
	}

	// 3. 上下文变量（前缀为 context_）
	for k, v := range r.ContextVars {
		key := "context_" + strings.ToLower(k)
		data[key] = v
	}

	return data, resolveErrors
}

// ExtractConfigFromHeaders 从标准请求头中提取代理模式配置
// 支持以下请求头：
//   X-Executor-Backend-ID    → executor_backend
//   X-Executor-Model         → executor_model
//   X-Auditor-Backend-ID     → auditor_backend
//   X-Auditor-Model          → auditor_model
//   X-Optimizer-Backend-ID   → optimizer_backend
//   X-Optimizer-Model        → optimizer_model
//   X-Backend-ID             → backend_id
//   X-Model                  → model
//   X-Target-URL             → target_url
func ExtractConfigFromHeaders(headers map[string]string) map[string]interface{} {
	config := make(map[string]interface{})

	// 请求头到配置键的映射
	headerMapping := map[string]string{
		"X-Executor-Backend-ID":  "executor_backend",
		"X-Executor-Model":       "executor_model",
		"X-Auditor-Backend-ID":   "auditor_backend",
		"X-Auditor-Model":        "auditor_model",
		"X-Optimizer-Backend-ID": "optimizer_backend",
		"X-Optimizer-Model":      "optimizer_model",
		"X-Backend-ID":           "backend_id",
		"X-Model":                "model",
		"X-Target-URL":           "target_url",
		"X-Original-Host":        "original_host",
		"X-Original-Path":        "original_path",
		"X-Pipeline-ID":          "pipeline_id",
	}

	for headerKey, configKey := range headerMapping {
		if value, ok := headers[headerKey]; ok && value != "" {
			config[configKey] = value
		}
	}

	return config
}

// MergeConfigWithDefaults 将提取的配置与默认值合并
func MergeConfigWithDefaults(extractedConfig map[string]interface{}, defaults map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// 先设置默认值
	for k, v := range defaults {
		merged[k] = v
	}

	// 再用提取的配置覆盖
	for k, v := range extractedConfig {
		merged[k] = v
	}

	return merged
}
