package pipeline

import (
	"fmt"
	"strings"
	"time"
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
func (r *TemplateVarResolver) Resolve(path string) (interface{}, error) {
	if strings.HasPrefix(path, "literal:") {
		return path[len("literal:"):], nil
	}

	parts := strings.SplitN(path, ".", 3)
	switch parts[0] {
	case "input":
		return r.resolveInput(parts[1:])
	case "node":
		return r.resolveNode(parts[1:])
	case "context":
		return r.resolveContext(parts[1:])
	default:
		return nil, fmt.Errorf("unknown path prefix %q (支持: input / node / context / literal:)", parts[0])
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
