package pipeline

import (
	"bytes"
	"centag/core/internal/cache"
	evalplugin "centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/embedding"
	"centag/core/pkg/plugin"
	"centag/core/pkg/processor"
	"centag/core/pkg/storage"
	"centag/core/pkg/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"centag/core/pkg/pipeline/promptstrategy"
)

// renderGoTemplate 是所有节点共用的 Go template 渲染辅助函数。
// name 用于报错信息定位，tmpl 是模板字符串，data 是传入模板的数据 map。
func renderGoTemplate(name, tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GeneratorNode 生成节点 - 调用LLM生成初始响应
type GeneratorNode struct {
	BaseNode
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
	// SystemPromptStrategy system prompt 策略（passthrough/append/replace）
	SystemPromptStrategy promptstrategy.SystemMode
	// AppendPosition append 模式下的插入位置
	AppendPosition promptstrategy.AppendPosition
}

func NewGeneratorNode(config NodeConfig) (PipelineNode, error) {
	node := &GeneratorNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     60,
			retryConfig: DefaultRetryConfig(),
		},
		Temperature: 0.7,
		MaxTokens:   2048,
	}

	if config.Temperature != nil {
		node.Temperature = *config.Temperature
	}
	if config.MaxTokens != nil {
		node.MaxTokens = *config.MaxTokens
	}
	if config.SystemPrompt != "" {
		node.SystemPrompt = config.SystemPrompt
	}

	// 解析 system_prompt_strategy（从 custom_config）
	if config.CustomConfig != nil {
		if s, ok := config.CustomConfig["system_prompt_strategy"].(string); ok {
			node.SystemPromptStrategy = promptstrategy.ResolveSystemMode(s, nil)
		}
		if s, ok := config.CustomConfig["append_position"].(string); ok {
			node.AppendPosition = promptstrategy.AppendPosition(strings.TrimSpace(s))
		}
	}

	return node, nil
}

func (n *GeneratorNode) Type() NodeType {
	return NodeTypeGenerator
}

func (n *GeneratorNode) Validate() error {
	if n.config.Backend == "" {
		return fmt.Errorf("generator node requires backend")
	}
	if n.config.Model == "" {
		return fmt.Errorf("generator node requires model")
	}
	return nil
}

func (n *GeneratorNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	// 构建模板数据（与 Processor/Reviewer 对称，用于渲染 system_prompt / prompt_template）
	question := input.Content
	if execCtx != nil {
		if q, ok := execCtx.GetVariable("input"); ok {
			if qs, ok := q.(string); ok {
				question = qs
			}
		}
	}
	resolver := NewTemplateVarResolver(input, execCtx)
	builtinVars := map[string]interface{}{
		"input":     input.Content,
		"question":  question,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for nodeID, result := range input.UpstreamResults {
		if result != nil {
			builtinVars[nodeID+"_content"] = result.Content
		}
	}
	tmplData, resolveErrs := resolver.BuildTemplateData(builtinVars, n.config.TemplateVars)
	if logger != nil && len(resolveErrs) > 0 {
		logger.Warn("[generator] template_vars 中存在解析失败的路径，对应变量将不可用",
			"node_id", n.id,
			"errors", resolveErrs,
		)
	}

	// 渲染 system_prompt（支持 {{.question}} 等模板变量）
	// 优先级：节点级 system_prompt > pipeline 级 global_config.system_prompt
	systemPromptSrc := n.SystemPrompt
	if systemPromptSrc == "" && execCtx != nil && execCtx.pipeline != nil {
		systemPromptSrc = execCtx.pipeline.GlobalConfig.SystemPrompt
	}
	renderedSystemPrompt := systemPromptSrc
	if systemPromptSrc != "" {
		if rendered, err := renderGoTemplate("system_prompt", systemPromptSrc, tmplData); err == nil {
			renderedSystemPrompt = rendered
		} else if logger != nil {
			logger.Warn("[generator] system_prompt 模板渲染失败，将使用原始文本",
				"node_id", n.id,
				"error", err,
			)
		}
	}

	// 构建消息列表
	messages := input.Messages
	if len(messages) == 0 && input.Content != "" {
		messages = []Message{
			{Role: "user", Content: input.Content},
		}
	}

	// 应用 system prompt 策略
	// 未配置 strategy 且有 system_prompt → 默认 replace（兼容旧行为）
	// 显式 passthrough → 保留客户端 system，不强制改写
	if renderedSystemPrompt != "" {
		strategy := n.SystemPromptStrategy
		if strategy == "" {
			strategy = promptstrategy.SystemModeReplace
		}
		if strategy != promptstrategy.SystemModePassthrough {
			psMessages := make([]promptstrategy.Message, len(messages))
			for i, msg := range messages {
				psMessages[i] = promptstrategy.Message{
					Role:    msg.Role,
					Content: msg.Content,
				}
			}
			result, err := promptstrategy.ApplySystemStrategy(promptstrategy.SystemApplyInput{
				Mode:           strategy,
				GatewayPrompt:  renderedSystemPrompt,
				AppendPosition: n.AppendPosition,
				Messages:       psMessages,
			})
			if err == nil && result.Applied {
				messages = make([]Message, len(result.Messages))
				for i, msg := range result.Messages {
					messages[i] = Message{
						Role:    msg.Role,
						Content: msg.Content,
					}
				}
			} else if err == nil && !result.Applied {
				// passthrough / 空 gateway：保持 messages
			} else {
				// 降级到原有逻辑：直接替换
				filtered := make([]Message, 0, len(messages))
				for _, msg := range messages {
					if msg.Role != "system" {
						filtered = append(filtered, msg)
					}
				}
				messages = append([]Message{{Role: "system", Content: renderedSystemPrompt}}, filtered...)
			}
		}
	}

	if logger != nil {
		sendFields := AppendRequestIDFields(ctx,
			"node_id", n.id,
			"backend_id", n.config.Backend,
			"model", n.config.Model,
			"message_count", len(messages),
			"system_prompt_set", renderedSystemPrompt != "",
			"messages_preview", MaskSensitiveData(FormatMessagesPreview(messages, defaultMessagesPreviewMax)),
			"user_input_preview", MaskSensitiveData(utils.TruncateString(input.Content, 2000)),
		)
		logger.Info("[generator] 发送请求", sendFields...)
	}

	// 构建请求
	req := &LLMRequest{
		Model:       n.config.Model,
		Messages:    messages,
		Temperature: n.Temperature,
		MaxTokens:   n.MaxTokens,
		Tools:       input.Tools,
		ToolChoice:  input.ToolChoice,
	}

	// 调用后端获取响应
	if logger != nil {
		logger.Debug("[generator] calling LLM via CapabilityBroker",
			"node_id", n.id,
			"permissions", n.resolveLLMPermissions(),
		)
	}
	resp, err := n.CallLLM(ctx, "generator", req)
	if err != nil {
		return nil, fmt.Errorf("generator failed via capability broker: %w", err)
	}

	if logger != nil {
		recvFields := AppendRequestIDFields(ctx,
			"node_id", n.id,
			"backend_id", n.config.Backend,
			"model", resp.Model,
			"tokens", resp.TokenUsage,
			"response_preview", MaskSensitiveData(utils.TruncateString(resp.Content, defaultMessagesPreviewMax)),
		)
		logger.Info("[generator] 收到响应", recvFields...)
	}

	return &NodeOutput{
		Content:          resp.Content,
		ToolCalls:        resp.ToolCalls,
		FinishReason:     resp.FinishReason,
		ReasoningContent: resp.ReasoningContent,
		Messages: append(messages, Message{
			Role:             "assistant",
			Content:          resp.Content,
			ToolCalls:        resp.ToolCalls,
			ReasoningContent: resp.ReasoningContent,
		}),
		Metadata: map[string]interface{}{
			"model":         resp.Model,
			"backend_id":    n.config.Backend,
			"tokens":        resp.TokenUsage,
			"prompt_tokens": len(input.Content) / 4,
			"temperature":   n.Temperature,
			"max_tokens":    n.MaxTokens,
			"finish_reason": resp.FinishReason,
		},
	}, nil
}

// ExecuteStream 流式执行生成节点。返回 chunk channel，调用方负责消费直到关闭。
func (n *GeneratorNode) ExecuteStream(ctx context.Context, input *NodeInput) (<-chan plugin.StreamChunk, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	question := input.Content
	if execCtx != nil {
		if q, ok := execCtx.GetVariable("input"); ok {
			if qs, ok := q.(string); ok {
				question = qs
			}
		}
	}
	resolver := NewTemplateVarResolver(input, execCtx)
	builtinVars := map[string]interface{}{
		"input":     input.Content,
		"question":  question,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for nodeID, result := range input.UpstreamResults {
		if result != nil {
			builtinVars[nodeID+"_content"] = result.Content
		}
	}
	tmplData, _ := resolver.BuildTemplateData(builtinVars, n.config.TemplateVars)

	// 优先级：节点级 system_prompt > pipeline 级 global_config.system_prompt
	systemPromptSrc := n.SystemPrompt
	if systemPromptSrc == "" && execCtx != nil && execCtx.pipeline != nil {
		systemPromptSrc = execCtx.pipeline.GlobalConfig.SystemPrompt
	}
	renderedSystemPrompt := systemPromptSrc
	if systemPromptSrc != "" {
		if rendered, err := renderGoTemplate("system_prompt", systemPromptSrc, tmplData); err == nil {
			renderedSystemPrompt = rendered
		} else if logger != nil {
			logger.Warn("[generator-stream] system_prompt 模板渲染失败，使用原始文本", "node_id", n.id, "error", err)
		}
	}

	messages := input.Messages
	if len(messages) == 0 && input.Content != "" {
		messages = []Message{
			{Role: "user", Content: input.Content},
		}
	}

	// 应用 system prompt 策略（流式路径与非流式路径对齐）
	if renderedSystemPrompt != "" {
		strategy := n.SystemPromptStrategy
		if strategy == "" {
			strategy = promptstrategy.SystemModeReplace
		}
		if strategy != promptstrategy.SystemModePassthrough {
			psMessages := make([]promptstrategy.Message, len(messages))
			for i, msg := range messages {
				psMessages[i] = promptstrategy.Message{
					Role:    msg.Role,
					Content: msg.Content,
				}
			}
			result, err := promptstrategy.ApplySystemStrategy(promptstrategy.SystemApplyInput{
				Mode:           strategy,
				GatewayPrompt:  renderedSystemPrompt,
				AppendPosition: n.AppendPosition,
				Messages:       psMessages,
			})
			if err == nil && result.Applied {
				messages = make([]Message, len(result.Messages))
				for i, msg := range result.Messages {
					messages[i] = Message{
						Role:    msg.Role,
						Content: msg.Content,
					}
				}
			} else if err != nil {
				filtered := make([]Message, 0, len(messages))
				for _, msg := range messages {
					if msg.Role != "system" {
						filtered = append(filtered, msg)
					}
				}
				messages = append([]Message{{Role: "system", Content: renderedSystemPrompt}}, filtered...)
			}
		}
	}

	if logger != nil {
		streamFields := AppendRequestIDFields(ctx,
			"node_id", n.id,
			"backend_id", n.config.Backend,
			"model", n.config.Model,
			"message_count", len(messages),
			"messages_preview", MaskSensitiveData(FormatMessagesPreview(messages, defaultMessagesPreviewMax)),
			"user_input_preview", MaskSensitiveData(utils.TruncateString(input.Content, 2000)),
		)
		logger.Info("[generator-stream] 发送流式请求", streamFields...)
	}

	req := &LLMRequest{
		Model:       n.config.Model,
		Messages:    messages,
		Temperature: n.Temperature,
		MaxTokens:   n.MaxTokens,
		Stream:      true,
		Tools:       input.Tools,
		ToolChoice:  input.ToolChoice,
	}

	if logger != nil {
		logger.Debug("[generator-stream] calling LLM stream via CapabilityBroker", "node_id", n.id)
	}

	chunkCh, err := n.CallLLMStream(ctx, "generator-stream", req)
	if err != nil {
		return nil, fmt.Errorf("generator stream call failed: %w", err)
	}

	return chunkCh, nil
}

// ProcessorNode 处理节点 - 处理/转换内容(优化、翻译、摘要等)
// Deprecated: 请使用业务插件 business.optimizer / business.summarizer / business.translator。
// 保留内置节点用于向后兼容，新增场景建议直接使用 plugins/business/ 下的业务插件。
type ProcessorNode struct {
	BaseNode
	Operation      string // optimize, translate, summarize, etc.
	TargetLang     string // for translate operation
	PromptTemplate string
}

func NewProcessorNode(config NodeConfig) (PipelineNode, error) {
	node := &ProcessorNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}

	// 从自定义配置中读取参数
	if customConfig, ok := config.CustomConfig["operation"]; ok {
		node.Operation = customConfig.(string)
	}
	if customConfig, ok := config.CustomConfig["target_lang"]; ok {
		node.TargetLang = customConfig.(string)
	}
	if config.PromptTemplate != "" {
		node.PromptTemplate = config.PromptTemplate
	} else {
		// 使用默认模板
		node.PromptTemplate = node.defaultPromptTemplate()
	}

	return node, nil
}

func (n *ProcessorNode) defaultPromptTemplate() string {
	switch n.Operation {
	case "optimize":
		return `请优化以下回答，使其更清晰、准确、完整:

## 原始回答
{{.input}}

请直接返回优化后的回答。`
	case "translate":
		return `请将以下内容翻译成{{.target_lang}}:

{{.input}}

请直接返回翻译结果。`
	case "summarize":
		return `请对以下内容进行摘要:

{{.input}}

请返回简洁的摘要。`
	default:
		return `请处理以下内容:

{{.input}}`
	}
}

func (n *ProcessorNode) Type() NodeType {
	return NodeTypeProcessor
}

func (n *ProcessorNode) Validate() error {
	if n.config.Backend == "" {
		return fmt.Errorf("processor node requires backend")
	}
	if n.config.Model == "" {
		return fmt.Errorf("processor node requires model")
	}
	return nil
}

func (n *ProcessorNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// 构建模板数据：内置变量 + 自动展开 metadata + 显式 template_vars
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	resolver := NewTemplateVarResolver(input, execCtx)

	// 原始用户问题（execCtx 中的 "input" 变量始终是用户原始输入）
	question := input.Content
	if execCtx != nil {
		if q, ok := execCtx.GetVariable("input"); ok {
			if qs, ok := q.(string); ok {
				question = qs
			}
		}
	}

	builtinVars := map[string]interface{}{
		"input":       input.Content,
		"question":    question,
		"target_lang": n.TargetLang,
		"metadata":    input.Metadata,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	// 自动将所有上游节点的输出以 {nodeID}_content 注入，无需 template_vars 绑定
	for nodeID, result := range input.UpstreamResults {
		if result != nil {
			builtinVars[nodeID+"_content"] = result.Content
		}
	}
	data, resolveErrs := resolver.BuildTemplateData(builtinVars, n.config.TemplateVars)
	if len(resolveErrs) > 0 {
		// ProcessorNode.Execute 无法直接访问 logger，通过 execCtx 间接记录
		if l, _ := ctx.Value(loggerContextKey{}).(Logger); l != nil {
			l.Warn("[processor] template_vars 中存在解析失败的路径，对应变量将不可用",
				"node_id", n.id,
				"errors", resolveErrs,
			)
		}
	}

	prompt, err := n.renderTemplate(n.PromptTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt template: %w", err)
	}

	// 构建请求
	req := &LLMRequest{
		Model: n.config.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	// 调用后端获取响应
	resp, err := n.CallLLM(ctx, "processor", req)
	if err != nil {
		return nil, fmt.Errorf("processor failed via capability broker: %w", err)
	}

	return &NodeOutput{
		Content: resp.Content,
		Metadata: map[string]interface{}{
			"operation":     n.Operation,
			"original":      input.Content,
			"model":         resp.Model,
			"tokens":        resp.TokenUsage,
			"prompt_tokens": len(prompt) / 4,
			"target_lang":   n.TargetLang,
		},
	}, nil
}

func (n *ProcessorNode) renderTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ReviewerNode 审核节点 - 审核内容质量
// Deprecated: 请使用业务插件 business.reviewer。
// 保留内置节点用于向后兼容，新增场景建议直接使用 plugins/business/reviewer/ 下的业务插件。
type ReviewerNode struct {
	BaseNode
	Criteria       []string
	MinScore       float64
	PromptTemplate string
}

type ReviewResult struct {
	Passed      bool     `json:"passed"`
	Score       float64  `json:"score"`
	Feedback    string   `json:"feedback"`
	Suggestions []string `json:"suggestions"`
}

func NewReviewerNode(config NodeConfig) (PipelineNode, error) {
	node := &ReviewerNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
		MinScore: 0.8,
	}

	// 从自定义配置中读取参数
	if customConfig, ok := config.CustomConfig["criteria"]; ok {
		if criteria, ok := customConfig.([]interface{}); ok {
			for _, c := range criteria {
				node.Criteria = append(node.Criteria, c.(string))
			}
		}
	}
	if customConfig, ok := config.CustomConfig["min_score"]; ok {
		node.MinScore = customConfig.(float64)
	}
	if config.PromptTemplate != "" {
		node.PromptTemplate = config.PromptTemplate
	} else {
		node.PromptTemplate = node.defaultReviewPrompt()
	}

	return node, nil
}

func (n *ReviewerNode) defaultReviewPrompt() string {
	return `你是一名专业的内容审核员。请审核以下AI助手的回答质量。

## 用户问题
{{.question}}

## AI回答
{{.answer}}

## 审核维度
{{range .criteria}}
- {{.}}
{{end}}

## 输出格式
请严格返回JSON格式：
{
    "passed": true/false,
    "score": 0.0-1.0,
    "feedback": "审核反馈说明",
    "suggestions": ["改进建议1", "改进建议2"]
}`
}

func (n *ReviewerNode) Type() NodeType {
	return NodeTypeReviewer
}

func (n *ReviewerNode) Validate() error {
	if n.config.Backend == "" {
		return fmt.Errorf("reviewer node requires backend")
	}
	if n.config.Model == "" {
		return fmt.Errorf("reviewer node requires model")
	}
	return nil
}

func (n *ReviewerNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	// 原始用户输入：优先从 execCtx 取（与 ProcessorNode 逻辑完全对称），
	// 这样不论是 generator→reviewer 还是任意多级链路，都能正确拿到原始输入。
	question := input.Content
	if execCtx != nil {
		if q, ok := execCtx.GetVariable("input"); ok {
			if qs, ok := q.(string); ok {
				question = qs
			}
		}
	}

	// 构建模板数据：内置变量 + 自动注入上游结果 + 显式 template_vars
	resolver := NewTemplateVarResolver(input, execCtx)
	builtinVars := map[string]interface{}{
		// 语义命名（审核场景惯例，但 prompt 可完全自定义）
		"question":  question,
		"answer":    input.Content,
		"criteria":  n.Criteria,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	// 自动将所有上游节点的输出以 {nodeID}_content 注入，无需 template_vars 绑定
	// 这使得任意流水线拓扑都可通过 {{.someNode_content}} 直接引用上游节点
	for nodeID, result := range input.UpstreamResults {
		if result != nil {
			builtinVars[nodeID+"_content"] = result.Content
		}
	}
	data, resolveErrs := resolver.BuildTemplateData(builtinVars, n.config.TemplateVars)
	if logger != nil && len(resolveErrs) > 0 {
		logger.Warn("[reviewer] template_vars 中存在解析失败的路径，对应变量将不可用",
			"node_id", n.id,
			"errors", resolveErrs,
		)
	}

	prompt, err := n.renderTemplate(n.PromptTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render review prompt: %w", err)
	}

	if logger != nil {
		logger.Info("[reviewer] 发送审核请求",
			"node_id", n.id,
			"model", n.config.Model,
			"question_preview", utils.TruncateString(question, 100),
			"answer_preview", utils.TruncateString(input.Content, 100),
			"prompt_length", len(prompt),
			"prompt_preview", utils.TruncateString(prompt, 500),
		)
	}

	// 构建请求
	req := &LLMRequest{
		Model: n.config.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	// 调用后端获取响应
	resp, err := n.CallLLM(ctx, "reviewer", req)
	if err != nil {
		return nil, fmt.Errorf("reviewer node %q: %w", n.id, err)
	}
	if resp == nil || resp.Content == "" {
		return nil, fmt.Errorf("reviewer node %q: no backend client available (backend %q not resolved, CapabilityBroker required)", n.id, n.config.Backend)
	}
	rawContent := resp.Content

	if logger != nil {
		logger.Info("[reviewer] 收到原始响应",
			"node_id", n.id,
			"model", resp.Model,
			"tokens", resp.TokenUsage,
			"raw_response", utils.TruncateString(rawContent, 1000),
		)
	}

	// 解析审核结果
	result, parseErr := n.parseReviewResult(rawContent)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse review result: %w", parseErr)
	}

	if logger != nil {
		if result.Feedback == "无法解析审核结果" {
			logger.Warn("[reviewer] JSON 解析失败，使用兜底值",
				"node_id", n.id,
				"raw_response_full", rawContent,
			)
		} else {
			logger.Info("[reviewer] 审核完成",
				"node_id", n.id,
				"passed", result.Passed,
				"score", result.Score,
				"feedback", result.Feedback,
			)
		}
	}

	return &NodeOutput{
		Content: input.Content, // 审核节点不改变内容，仅在顶层字段暴露审核结论
		Metadata: map[string]interface{}{
			// 仅保留执行统计信息；审核结论（passed/score/feedback/suggestions）
			// 已作为 NodeOutput 顶层字段存在，PipelineOutput 会将其提升到响应顶层，
			// 不在 Metadata 中重复以避免 API 响应出现冗余字段。
			"model":         resp.Model,
			"tokens":        resp.TokenUsage,
			"prompt_tokens": len(prompt) / 4,
		},
		Passed:      &result.Passed,
		Score:       &result.Score,
		Feedback:    result.Feedback,
		Suggestions: result.Suggestions,
	}, nil
}

func (n *ReviewerNode) renderTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("review").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (n *ReviewerNode) parseReviewResult(content string) (*ReviewResult, error) {
	// 清理可能被格式标记包裹的JSON
	cleaned := cleanJSONContent(content)

	var result ReviewResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		// 如果解析失败，返回默认结果
		return &ReviewResult{
			Passed:   true,
			Score:    0.5,
			Feedback: "无法解析审核结果",
		}, nil
	}
	return &result, nil
}

// cleanJSONContent 从 LLM 响应中提取 JSON 内容。
// 按优先级依次尝试以下策略：
//  1. 整体就是一个合法 JSON
//  2. markdown 代码块 ```json ... ```
//  3. 正文中第一个完整 { ... } 对象（处理模型在 JSON 前后添加说明文字的情况）
func cleanJSONContent(content string) string {
	content = strings.TrimSpace(content)

	// 策略 1：整体直接是 JSON
	if strings.HasPrefix(content, "{") {
		return content
	}

	// 策略 2：markdown 代码块 ```json ... ``` 或 ``` ... ```
	codeBlockRe := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")
	if matches := codeBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 策略 3：从正文中提取第一个完整 JSON 对象（括号计数法）
	if extracted := extractFirstJSONObject(content); extracted != "" {
		return extracted
	}

	// 兜底：去掉所有 ``` 标记后原样返回，让上层 json.Unmarshal 报错
	content = regexp.MustCompile("(?m)^```.*$").ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

// extractFirstJSONObject 用括号计数从字符串中提取第一个完整的 JSON 对象 { ... }
func extractFirstJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// pipelineRouterRule 流水线 Router 节点的一条可排序规则。
type pipelineRouterRule struct {
	kind     string // contains | prefix | regex
	pattern  string
	re       *regexp.Regexp
	target   string
	priority int
}

// RouterNode 路由节点 - 条件分支（支持多种匹配策略与显式默认分支）
//
// 策略枚举（routing_strategy）：
//   - keyword_contains：关键词子串包含匹配（默认，向后兼容）
//   - keyword_prefix：关键词前缀匹配
//   - ordered：按 route_rules 顺序匹配
//   - regex_only：按 route_rules 正则匹配
//   - llm_classify：通过 LLM 对用户输入做意图分类，类别名与 routes 的 key 精确匹配
//   - keyword_then_intent：先关键字/规则，未命中再轻量意图分类（IntentResolver），可选小模型
type RouterNode struct {
	BaseNode
	strategy       string // keyword_contains | keyword_prefix | ordered | regex_only | llm_classify | keyword_then_intent
	defaultRoute   string
	legacyRoutes   map[string]string // keyword -> next_node_id（兼容旧配置；llm_classify 下为 category -> target）
	rules          []pipelineRouterRule
	classifyPrompt string // llm_classify 自定义分类 Prompt（空则使用内置默认）

	// classifyBackend / classifyModel：LLM 分类专用后端/模型（custom_config 显式指定，
	// 可为 {{system.*}} 模板或字面量；为空时按 resolveClassifyBackendModel 逐级回退）。
	// 适用于「用低成本/快速模型做意图分类」的典型场景。
	classifyBackend string
	classifyModel   string

	// keyword_then_intent options
	enableLLMClassifier  bool
	confidenceThreshold  float64
	enableIntentResolver bool
}

func NewRouterNode(config NodeConfig) (PipelineNode, error) {
	node := &RouterNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     10,
			retryConfig: DefaultRetryConfig(),
		},
		strategy:     "keyword_contains",
		legacyRoutes: make(map[string]string),
	}

	if cc := config.CustomConfig; cc != nil {
		if s, ok := cc["routing_strategy"].(string); ok && strings.TrimSpace(s) != "" {
			node.strategy = strings.TrimSpace(s)
		}
		if d, ok := cc["default_route"].(string); ok {
			node.defaultRoute = strings.TrimSpace(d)
		}
		if raw, ok := cc["route_rules"]; ok {
			node.rules = append(node.rules, parsePipelineRouteRules(raw)...)
		}
		if customRoutes, ok := cc["routes"].(map[string]interface{}); ok {
			for condition, nextNode := range customRoutes {
				kw := strings.TrimSpace(condition)
				tgt, _ := nextNode.(string)
				if kw == "" || tgt == "" {
					continue
				}
				node.legacyRoutes[kw] = tgt
			}
		}
		if p, ok := cc["classify_prompt"].(string); ok {
			node.classifyPrompt = p
		}
		if b, ok := cc["classify_backend"].(string); ok {
			node.classifyBackend = strings.TrimSpace(b)
		}
		if m, ok := cc["classify_model"].(string); ok {
			node.classifyModel = strings.TrimSpace(m)
		}
		node.enableIntentResolver = true
		node.confidenceThreshold = 0.55
		if intentRaw, ok := cc["intent"].(map[string]interface{}); ok {
			if v, ok := intentRaw["enable_fast_matcher"].(bool); ok {
				node.enableIntentResolver = v
			}
			if v, ok := intentRaw["enable_llm_classifier"].(bool); ok {
				node.enableLLMClassifier = v
			}
			switch v := intentRaw["confidence_threshold"].(type) {
			case float64:
				if v > 0 {
					node.confidenceThreshold = v
				}
			case int:
				if v > 0 {
					node.confidenceThreshold = float64(v)
				}
			}
		}
	}

	// llm_classify / keyword_then_intent 下 legacyRoutes 也作为 category -> target
	if node.strategy == "llm_classify" {
		return node, nil
	}
	if node.strategy == "keyword_then_intent" {
		// still compile keyword rules from legacyRoutes for first-pass matching
	}

	if len(node.rules) == 0 && len(node.legacyRoutes) > 0 {
		keys := make([]string, 0, len(node.legacyRoutes))
		for k := range node.legacyRoutes {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) != len(keys[j]) {
				return len(keys[i]) > len(keys[j])
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			kind := "contains"
			if node.strategy == "keyword_prefix" {
				kind = "prefix"
			}
			node.rules = append(node.rules, pipelineRouterRule{
				kind:     kind,
				pattern:  k,
				target:   node.legacyRoutes[k],
				priority: len(k),
			})
		}
	}

	return node, nil
}

func parsePipelineRouteRules(raw interface{}) []pipelineRouterRule {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var out []pipelineRouterRule
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		match, _ := m["match"].(string)
		target, _ := m["target"].(string)
		if strings.TrimSpace(target) == "" {
			continue
		}
		prio := 0
		switch v := m["priority"].(type) {
		case float64:
			prio = int(v)
		case int:
			prio = v
		case string:
			prio, _ = strconv.Atoi(strings.TrimSpace(v))
		}
		kind, pat := splitPipelineRouteMatch(match)
		rule := pipelineRouterRule{kind: kind, pattern: pat, target: target, priority: prio}
		if kind == "regex" {
			re, err := regexp.Compile(pat)
			if err != nil {
				continue
			}
			rule.re = re
		}
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].priority != out[j].priority {
			return out[i].priority > out[j].priority
		}
		return len(out[i].pattern) > len(out[j].pattern)
	})
	return out
}

func splitPipelineRouteMatch(s string) (kind, pattern string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "contains", ""
	}
	low := strings.ToLower(s)
	switch {
	case strings.HasPrefix(low, "contains:"):
		return "contains", strings.TrimSpace(s[len("contains:"):])
	case strings.HasPrefix(low, "prefix:"):
		return "prefix", strings.TrimSpace(s[len("prefix:"):])
	case strings.HasPrefix(low, "regex:"):
		return "regex", strings.TrimSpace(s[len("regex:"):])
	default:
		return "contains", s
	}
}

func (n *RouterNode) Type() NodeType {
	return NodeTypeRouter
}

func (n *RouterNode) Validate() error {
	if n.strategy == "llm_classify" {
		if n.config.Backend == "" {
			return fmt.Errorf("router node with llm_classify strategy requires backend")
		}
		if n.config.Model == "" {
			return fmt.Errorf("router node with llm_classify strategy requires model")
		}
		if len(n.legacyRoutes) == 0 {
			return fmt.Errorf("router node with llm_classify strategy requires routes (category -> target)")
		}
		return nil
	}
	if n.strategy == "keyword_then_intent" {
		if len(n.legacyRoutes) == 0 && len(n.rules) == 0 && n.defaultRoute == "" {
			return fmt.Errorf("router node with keyword_then_intent requires routes/route_rules or default_route")
		}
		if n.enableLLMClassifier {
			if n.config.Backend == "" || n.config.Model == "" {
				return fmt.Errorf("keyword_then_intent with enable_llm_classifier requires backend and model")
			}
		}
		return nil
	}
	if len(n.rules) == 0 && n.defaultRoute == "" {
		return fmt.Errorf("router node requires route_rules/routes or default_route")
	}
	return nil
}

func (n *RouterNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	forcedRoute := ""
	if input != nil && input.Metadata != nil {
		if v, ok := input.Metadata["forced_route"].(string); ok {
			forcedRoute = strings.TrimSpace(v)
		}
	}
	selectedRoute, matched, llmRaw := n.selectRoute(ctx, input.Content, forcedRoute)
	meta := map[string]interface{}{
		"routing_strategy": n.strategy,
		"selected_route":   selectedRoute,
		"matched":          matched,
		"category":         matched,
		"route_category":   matched,
	}
	if len(n.legacyRoutes) > 0 {
		meta["routes"] = n.legacyRoutes
	}
	if llmRaw != "" {
		meta["llm_raw_response"] = llmRaw
	}
	if forcedRoute != "" {
		meta["forced_route"] = forcedRoute
	}
	return &NodeOutput{
		Content:  input.Content,
		Metadata: meta,
	}, nil
}

func (n *RouterNode) selectRoute(ctx context.Context, content, forcedRoute string) (targetID, matched, llmRaw string) {
	// 强制路由优先（如显式指定 skill 时跳过 LLM 分类）。forced_route 可为
	// 类别名（routes 的 key，如 skill 名）或目标节点 ID（routes 的 value）。
	if forcedRoute != "" {
		if target, ok := n.legacyRoutes[forcedRoute]; ok && target != "" {
			return target, forcedRoute, ""
		}
		for k, target := range n.legacyRoutes {
			if strings.EqualFold(k, forcedRoute) && target != "" {
				return target, k, ""
			}
		}
		for _, target := range n.legacyRoutes {
			if target == forcedRoute {
				return target, forcedRoute, ""
			}
		}
		if n.defaultRoute != "" {
			return n.defaultRoute, "__forced_route__", ""
		}
		return "", "__forced_route_no_match__", ""
	}

	// llm_classify：通过 LLM 语义分类
	if n.strategy == "llm_classify" {
		category, raw, err := n.classifyWithLLM(ctx, content)
		llmRaw = raw
		if err != nil {
			if logger, _ := ctx.Value(loggerContextKey{}).(Logger); logger != nil {
				logger.Warn("[router] llm_classify failed, falling back to default_route",
					"node_id", n.id,
					"error", err,
				)
			}
			if n.defaultRoute != "" {
				return n.defaultRoute, "__llm_error_fallback__", llmRaw
			}
			return "", "__llm_error_fallback__", llmRaw
		}
		if target, ok := n.legacyRoutes[category]; ok && target != "" {
			return target, category, llmRaw
		}
		if n.defaultRoute != "" {
			return n.defaultRoute, "__default__", llmRaw
		}
		return "", "__no_match__", llmRaw
	}

	// keyword_then_intent：关键字优先 → 轻量意图 → 可选 LLM → default
	if n.strategy == "keyword_then_intent" {
		contentLower := strings.ToLower(strings.TrimSpace(content))
		for _, r := range n.rules {
			if n.ruleMatches(r, content, contentLower) {
				return r.target, r.pattern, ""
			}
		}
		categories := n.legacyRouteKeys()
		if n.enableIntentResolver {
			resolver := GetIntentResolver()
			if resolver != nil {
				cat, conf, err := resolver.ResolveCategory(ctx, content, categories)
				if err == nil && cat != "" && conf >= n.confidenceThreshold {
					if target, ok := n.legacyRoutes[cat]; ok && target != "" {
						return target, cat, ""
					}
					// case-insensitive key match
					for k, target := range n.legacyRoutes {
						if strings.EqualFold(k, cat) && target != "" {
							return target, k, ""
						}
					}
				}
			}
		}
		if n.enableLLMClassifier {
			category, raw, err := n.classifyWithLLM(ctx, content)
			llmRaw = raw
			if err == nil && category != "" {
				if target, ok := n.legacyRoutes[category]; ok && target != "" {
					return target, category, llmRaw
				}
			}
		}
		if n.defaultRoute != "" {
			return n.defaultRoute, "__default__", llmRaw
		}
		return "", "__no_match__", llmRaw
	}

	// 关键词 / 规则匹配（保留原行为）
	contentLower := strings.ToLower(strings.TrimSpace(content))
	for _, r := range n.rules {
		if n.ruleMatches(r, content, contentLower) {
			return r.target, r.pattern, ""
		}
	}

	if n.defaultRoute != "" {
		return n.defaultRoute, "__default__", ""
	}
	if len(n.rules) > 0 {
		return n.rules[0].target, "__fallback_first_rule__", ""
	}
	return "", "", ""
}

// resolveClassifyBackendModel 解析 LLM 分类调用应使用的 backend 与 model。
// 优先级：节点 custom_config.classify_backend/classify_model >
// 系统 system.classify_backend/system.classify_model（默认快速分类后端/模型）>
// 节点自身 config.Backend/config.Model。
// 返回的 backend/model 可能为 {{system.*}} 模板，交由 CreateClient 解析。
func (n *RouterNode) resolveClassifyBackendModel() (backendID, model string) {
	backendID = n.classifyBackend
	model = n.classifyModel
	if backendID == "" || model == "" {
		if cfg := config.Get(); cfg != nil {
			mv := cfg.ModelVariables.SystemVariables
			if backendID == "" {
				if v := mv["system.classify_backend"]; v != "" {
					backendID = "{{system.classify_backend}}"
				}
			}
			if model == "" {
				if v := mv["system.classify_model"]; v != "" {
					model = "{{system.classify_model}}"
				}
			}
		}
	}
	if backendID == "" {
		backendID = n.config.Backend
	}
	if model == "" {
		model = n.config.Model
	}
	return backendID, model
}

// classifyWithLLM 通过 LLM 对用户输入做意图分类
// 返回: (category, raw_response, error)
//   - category: 清洗后小写、与 routes 的 key 完全匹配的类别名；未命中时为空字符串
//   - raw_response: LLM 原始响应（用于 metadata 追溯）
//   - error: LLM 调用 / 渲染 / 解析错误
func (n *RouterNode) classifyWithLLM(ctx context.Context, content string) (string, string, error) {
	if strings.TrimSpace(content) == "" {
		return "", "", fmt.Errorf("empty input for llm_classify")
	}

	// 渲染分类 Prompt
	tmpl := n.classifyPrompt
	if strings.TrimSpace(tmpl) == "" {
		tmpl = n.defaultClassifyPrompt()
	}
	prompt, err := renderGoTemplate("classify_prompt", tmpl, map[string]interface{}{
		"input":      content,
		"categories": n.legacyRouteKeys(),
	})
	if err != nil {
		return "", "", fmt.Errorf("render classify prompt: %w", err)
	}

	backendID, model := n.resolveClassifyBackendModel()
	req := &LLMRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
		MaxTokens:   16,
	}
	resp, err := n.CallLLMWithBackend(ctx, "router", req, backendID, model)
	if err != nil {
		return "", "", err
	}
	if resp == nil {
		return "", "", fmt.Errorf("llm returned nil response")
	}

	raw := resp.Content
	category := cleanClassifyResponse(raw)
	if category == "" {
		return "", raw, nil
	}
	return category, raw, nil
}

// defaultClassifyPrompt 内置默认分类 Prompt
func (n *RouterNode) defaultClassifyPrompt() string {
	categories := n.legacyRouteKeys()
	catList := strings.Join(categories, " / ")
	if catList == "" {
		catList = "code / translate / summary / chat"
	}
	return fmt.Sprintf(`你是意图分类助手。请判断用户输入的意图类别，只返回类别名（英文小写），不要输出任何解释。

可选类别及判断标准（用户输入意图越接近某类别，就返回该类别名）：
%v

用户输入：{{.input}}

类别：`, catList)
}

// legacyRouteKeys 返回 routes 的 key 列表（保留顺序不可靠：map 遍历；分类用途足够）
func (n *RouterNode) legacyRouteKeys() []string {
	keys := make([]string, 0, len(n.legacyRoutes))
	for k := range n.legacyRoutes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// cleanClassifyResponse 清洗 LLM 分类响应：
//   - 去除 markdown 代码块包裹
//   - 去除首尾空白、引号
//   - 统一小写
//   - 仅保留首行（防止模型输出多解释）
//
// 返回: 清洗后的小写字符串；为空时返回 ""
func cleanClassifyResponse(raw string) string {
	if raw == "" {
		return ""
	}
	// 仅看首行，丢掉可能的多余说明
	firstLine := raw
	if idx := strings.IndexAny(raw, "\n\r"); idx >= 0 {
		firstLine = raw[:idx]
	}
	s := strings.TrimSpace(firstLine)
	// 去除 markdown 代码块围栏
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	// 去除语言标签（如 ```text）
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	// 去除首尾引号（包括成对的双/单引号和中英文引号）
	s = strings.Trim(s, "\"'`“”‘’")
	// 去除类别前缀（"类别：code" / "category: code" / "类别:code"）
	// 同时处理中英文冒号
	stripPrefix := func(sep string) {
		if idx := strings.LastIndex(s, sep); idx >= 0 && idx < len(s)-1 {
			tail := strings.TrimSpace(s[idx+len(sep):])
			if tail != "" {
				s = tail
			}
		}
	}
	stripPrefix(":")
	stripPrefix("：")
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	return s
}

func (n *RouterNode) ruleMatches(r pipelineRouterRule, content, contentLower string) bool {
	patLower := strings.ToLower(r.pattern)
	switch r.kind {
	case "prefix":
		return strings.HasPrefix(contentLower, patLower)
	case "regex":
		if r.re != nil {
			return r.re.MatchString(content)
		}
		return false
	default: // contains
		return strings.Contains(contentLower, patLower)
	}
}

// AggregatorNode 聚合节点 - 合并多个上游节点的输出
type AggregatorNode struct {
	BaseNode
	Strategy       string // concat, summarize, vote, best
	PromptTemplate string
}

func NewAggregatorNode(config NodeConfig) (PipelineNode, error) {
	node := &AggregatorNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
		Strategy: "concat",
	}

	if customConfig, ok := config.CustomConfig["strategy"]; ok {
		node.Strategy = customConfig.(string)
	}
	if config.PromptTemplate != "" {
		node.PromptTemplate = config.PromptTemplate
	}

	return node, nil
}

func (n *AggregatorNode) Type() NodeType {
	return NodeTypeAggregator
}

func (n *AggregatorNode) Validate() error {
	validStrategies := map[string]bool{
		"concat":    true,
		"merge":     true,
		"summarize": true,
		"vote":      true,
		"best":      true,
		"score":     true,
	}
	if !validStrategies[n.Strategy] {
		return fmt.Errorf("invalid aggregator strategy: %s", n.Strategy)
	}
	// summarize / score 策略需要调用 LLM，必须配置 backend 和 model
	if n.Strategy == "summarize" || n.Strategy == "score" {
		if n.config.Backend == "" {
			return fmt.Errorf("aggregator node with summarize strategy requires backend")
		}
		if n.config.Model == "" {
			return fmt.Errorf("aggregator node with summarize strategy requires model")
		}
	}
	return nil
}

func (n *AggregatorNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// AggregatorNode 的多输入通过 input.Metadata 传递
	upstreamOutputs := n.extractUpstreamOutputs(input)
	if len(upstreamOutputs) == 0 {
		return &NodeOutput{
			Content: input.Content,
			Metadata: map[string]interface{}{
				"strategy": n.Strategy,
			},
		}, nil
	}

	var result string
	var metadata map[string]interface{}

	switch n.Strategy {
	case "concat", "merge":
		result, metadata = n.aggregateConcat(upstreamOutputs)
	case "summarize":
		result, metadata = n.aggregateSummarize(ctx, upstreamOutputs)
	case "vote":
		result, metadata = n.aggregateVote(upstreamOutputs)
	case "best":
		result, metadata = n.aggregateBest(upstreamOutputs)
	case "score":
		result, metadata = n.aggregateScore(ctx, input, upstreamOutputs)
	}

	return &NodeOutput{
		Content:  result,
		Metadata: metadata,
	}, nil
}

// extractUpstreamOutputs 从输入的 Metadata 中提取上游节点输出
func (n *AggregatorNode) extractUpstreamOutputs(input *NodeInput) []upstreamOutput {
	var outputs []upstreamOutput
	for key, value := range input.Metadata {
		if meta, ok := value.(map[string]interface{}); ok {
			if content, ok := meta["content"].(string); ok {
				outputs = append(outputs, upstreamOutput{
					nodeID:  key,
					content: content,
					meta:    meta,
				})
			}
		}
	}
	return outputs
}

type upstreamOutput struct {
	nodeID  string
	content string
	meta    map[string]interface{}
}

func (n *AggregatorNode) aggregateConcat(outputs []upstreamOutput) (string, map[string]interface{}) {
	var parts []string
	for _, out := range outputs {
		parts = append(parts, fmt.Sprintf("[%s]\n%s", out.nodeID, out.content))
	}
	result := strings.Join(parts, "\n\n---\n\n")
	return result, map[string]interface{}{
		"strategy":       n.Strategy,
		"upstream_count": len(outputs),
		"upstream_nodes": n.extractNodeIDs(outputs),
	}
}

func (n *AggregatorNode) aggregateSummarize(ctx context.Context, outputs []upstreamOutput) (string, map[string]interface{}) {
	combined := n.buildCombinedContent(outputs)
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	if logger != nil {
		logger.Info("[aggregator] summarize: preparing to call LLM",
			"node_id", n.ID(),
			"backend", n.config.Backend,
			"model", n.config.Model,
			"upstream_count", len(outputs),
			"combined_length", len(combined),
		)
	}

	// 构建 prompt（支持用户自定义模板或默认模板）
	prompt := n.buildSummarizePrompt(combined, outputs)

	// 构建 LLM 请求
	req := &LLMRequest{
		Model: n.config.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	// 通过 CapabilityBroker 调用 LLM
	resp, err := n.CallLLM(ctx, "aggregator", req)
	if err != nil {
		if logger != nil {
			logger.Warn("[aggregator] summarize: LLM call failed, falling back to concat",
				"node_id", n.ID(),
				"error", err,
			)
		}
		return combined, map[string]interface{}{
			"strategy":        n.Strategy,
			"upstream_count":  len(outputs),
			"fallback":        true,
			"fallback_reason": "llm_error",
			"combined_length": len(combined),
			"error":           err.Error(),
		}
	}

	if logger != nil {
		logger.Info("[aggregator] summarize: received LLM response",
			"node_id", n.ID(),
			"model", resp.Model,
			"tokens", resp.TokenUsage,
			"response_preview", utils.TruncateString(resp.Content, 2000),
		)
	}

	return resp.Content, map[string]interface{}{
		"strategy":        n.Strategy,
		"upstream_count":  len(outputs),
		"model":           resp.Model,
		"tokens":          resp.TokenUsage,
		"combined_length": len(combined),
	}
}

// buildSummarizePrompt 构建聚合摘要的 prompt。
// 若用户配置了 PromptTemplate，则使用模板渲染（支持 {{.combined_content}} 变量）；
// 否则使用内置默认 prompt。
func (n *AggregatorNode) buildSummarizePrompt(combined string, outputs []upstreamOutput) string {
	if n.PromptTemplate != "" {
		data := map[string]interface{}{
			"combined_content": combined,
			"upstream_count":   len(outputs),
		}
		if rendered, err := renderGoTemplate("aggregator_prompt", n.PromptTemplate, data); err == nil {
			return rendered
		}
	}

	// 默认 prompt：将各上游输出编号列出，要求 LLM 综合生成最终答案
	var parts []string
	for i, out := range outputs {
		parts = append(parts, fmt.Sprintf("## 回答 %d (%s)\n%s", i+1, out.nodeID, out.content))
	}

	return fmt.Sprintf(`请综合以下多个回答，生成一个全面、结构清晰、无冗余的最终答案：

%s

请整合以上回答的优点，去除重复内容，生成一个完整的最终答案：`, strings.Join(parts, "\n\n"))
}

func (n *AggregatorNode) buildCombinedContent(outputs []upstreamOutput) string {
	var parts []string
	for _, out := range outputs {
		parts = append(parts, out.content)
	}
	return strings.Join(parts, "\n\n")
}

func (n *AggregatorNode) aggregateVote(outputs []upstreamOutput) (string, map[string]interface{}) {
	// 简单投票：选择最长的输出作为"最佳"
	best := outputs[0]
	for _, out := range outputs {
		if len(out.content) > len(best.content) {
			best = out
		}
	}
	return best.content, map[string]interface{}{
		"strategy":       n.Strategy,
		"upstream_count": len(outputs),
		"selected_node":  best.nodeID,
		"vote_method":    "longest_content",
	}
}

func (n *AggregatorNode) aggregateScore(ctx context.Context, input *NodeInput, outputs []upstreamOutput) (string, map[string]interface{}) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	question := ""
	if input != nil && input.Metadata != nil {
		if q, ok := input.Metadata["question"].(string); ok {
			question = q
		}
	}
	if question == "" && input != nil {
		question = input.Content
	}

	type scored struct {
		out   upstreamOutput
		score float64
	}
	scoredOutputs := make([]scored, 0, len(outputs))
	for _, out := range outputs {
		score := 0.0
		if ReviewContent != nil {
			review, err := ReviewContent(ctx, ContentReviewRequest{
				Question: question,
				Answer:   out.content,
				Backend:  n.config.Backend,
				Model:    n.config.Model,
			})
			if err != nil {
				if logger != nil {
					logger.Warn("[aggregator] score: reviewer failed, using fallback score",
						"node_id", n.id,
						"upstream_node", out.nodeID,
						"error", err,
					)
				}
				score = 0.5
			} else if review != nil {
				score = review.Score
			}
		} else if s, ok := out.meta["score"].(float64); ok {
			score = s
		}
		scoredOutputs = append(scoredOutputs, scored{out: out, score: score})
	}

	if len(scoredOutputs) == 0 {
		return "", map[string]interface{}{"strategy": n.Strategy, "upstream_count": 0}
	}

	best := scoredOutputs[0]
	for _, item := range scoredOutputs[1:] {
		if item.score > best.score {
			best = item
		}
	}

	scoresByNode := make(map[string]float64, len(scoredOutputs))
	for _, item := range scoredOutputs {
		scoresByNode[item.out.nodeID] = item.score
	}

	metadata := map[string]interface{}{
		"strategy":       n.Strategy,
		"upstream_count": len(outputs),
		"selected_node":  best.out.nodeID,
		"best_score":     best.score,
		"scores":         scoresByNode,
		"review_method":  "business.reviewer",
	}

	if len(scoredOutputs) >= 2 {
		a, b := scoredOutputs[0], scoredOutputs[1]
		persistABEvalFromScore(ctx, input, ABEvalPersistRequest{
			Strategy:       "score",
			Question:       question,
			WinnerNode:     best.out.nodeID,
			CandidateANode: a.out.nodeID,
			CandidateBNode: b.out.nodeID,
			ModelA:         metaString(a.out.meta, "model"),
			ModelB:         metaString(b.out.meta, "model"),
			ScoreA:         a.score,
			ScoreB:         b.score,
			LatencyAMs:     metaInt64(a.out.meta, "latency_ms"),
			LatencyBMs:     metaInt64(b.out.meta, "latency_ms"),
			CostAUSD:       estimateMetaCostUSD(a.out.meta),
			CostBUSD:       estimateMetaCostUSD(b.out.meta),
		})
	}

	return best.out.content, metadata
}

func metaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key].(string); ok {
		return v
	}
	return ""
}

func metaInt64(meta map[string]interface{}, key string) int64 {
	if meta == nil {
		return 0
	}
	switch v := meta[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func estimateMetaCostUSD(meta map[string]interface{}) float64 {
	if meta == nil {
		return 0
	}
	if v, ok := meta["cost_usd"].(float64); ok {
		return v
	}
	return 0
}

func (n *AggregatorNode) aggregateBest(outputs []upstreamOutput) (string, map[string]interface{}) {
	// 择优：优先选择带有评分且分数最高的输出
	var best *upstreamOutput
	var bestScore float64
	for i := range outputs {
		if score, ok := outputs[i].meta["score"]; ok {
			if s, ok := score.(float64); ok && s > bestScore {
				bestScore = s
				best = &outputs[i]
			}
		}
	}
	if best == nil {
		// 没有评分则选择最长的
		best = &outputs[0]
		for i := range outputs {
			if len(outputs[i].content) > len(best.content) {
				best = &outputs[i]
			}
		}
	}
	return best.content, map[string]interface{}{
		"strategy":       n.Strategy,
		"upstream_count": len(outputs),
		"selected_node":  best.nodeID,
		"best_score":     bestScore,
	}
}

func (n *AggregatorNode) extractNodeIDs(outputs []upstreamOutput) []string {
	ids := make([]string, len(outputs))
	for i, out := range outputs {
		ids[i] = out.nodeID
	}
	return ids
}

// SchedulerNode 智能调度节点 — 根据意图与多维评分选择最优 backend/model
type SchedulerNode struct {
	BaseNode
	Strategy string
}

func NewSchedulerNode(config NodeConfig) (PipelineNode, error) {
	node := &SchedulerNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     15,
			retryConfig: DefaultRetryConfig(),
		},
		Strategy: "balance",
	}
	if config.CustomConfig != nil {
		if strategy, ok := config.CustomConfig["strategy"].(string); ok && strategy != "" {
			node.Strategy = strategy
		}
	}
	return node, nil
}

func (n *SchedulerNode) Type() NodeType {
	return NodeTypeScheduler
}

func (n *SchedulerNode) Validate() error {
	return nil
}

func (n *SchedulerNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	question := input.Content
	if question == "" && input.Metadata != nil {
		if q, ok := input.Metadata["question"].(string); ok {
			question = q
		}
	}
	if question == "" && execCtx != nil {
		if q, ok := execCtx.GetVariable("input"); ok {
			if qs, ok := q.(string); ok {
				question = qs
			}
		}
	}

	requestedModel := n.config.Model
	if input.Metadata != nil {
		if m, ok := input.Metadata["model"].(string); ok && m != "" {
			requestedModel = m
		}
	}

	if ScheduleBackend == nil {
		return nil, fmt.Errorf("scheduler node %q: ScheduleBackend hook not wired", n.id)
	}

	// 从 CustomConfig 读取分类器配置
	classifyBackend := ""
	classifyModel := ""
	classifyPrompt := ""
	if n.config.CustomConfig != nil {
		if v, ok := n.config.CustomConfig["classify_backend"].(string); ok {
			classifyBackend = v
		}
		if v, ok := n.config.CustomConfig["classify_model"].(string); ok {
			classifyModel = v
		}
		if v, ok := n.config.CustomConfig["classify_prompt"].(string); ok {
			classifyPrompt = v
		}
	}

	result, err := ScheduleBackend(ScheduleRequest{
		Question:        question,
		RequestedModel:  requestedModel,
		Strategy:        n.Strategy,
		ClassifyBackend: classifyBackend,
		ClassifyModel:   classifyModel,
		ClassifyPrompt:  classifyPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("scheduler node %q: schedule failed: %w", n.id, err)
	}
	if result == nil {
		return nil, fmt.Errorf("scheduler node %q: schedule returned nil result", n.id)
	}

	if execCtx != nil {
		if result.BackendID != "" {
			execCtx.SetVariable("backend_id", result.BackendID)
		}
		if result.Model != "" {
			execCtx.SetVariable("scheduled_model", result.Model)
		}
	}

	if logger != nil {
		logger.Info("[scheduler] backend selected",
			"node_id", n.id,
			"backend_id", result.BackendID,
			"model", result.Model,
			"task_type", result.TaskType,
			"reason", result.Reason,
		)
	}

	return &NodeOutput{
		Content: "",
		Metadata: map[string]interface{}{
			"scheduler_decision":   true,
			"backend_id":           result.BackendID,
			"model":                result.Model,
			"reason":               result.Reason,
			"task_type":            result.TaskType,
			"strategy":             n.Strategy,
			"estimated_cost":       result.EstimatedCost,
			"estimated_latency_ms": result.EstimatedLatencyMs,
		},
	}, nil
}

// RegisterBuiltinNodes 注册所有内置节点到注册表
func RegisterBuiltinNodes(registry *NodeRegistry) error {
	if registry == nil {
		return fmt.Errorf("node registry cannot be nil")
	}

	generatorFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewGeneratorNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeGenerator, generatorFactory, "内置生成节点", "调用已配置 LLM 后端生成内容", []string{"llm.call"}, true); err != nil {
		return err
	}

	// Deprecated: 建议使用 plugins/business/ 下的 optimizer / summarizer / translator 业务插件
	processorFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewProcessorNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeProcessor, processorFactory, "内置处理节点", "基于提示词对上游内容进行转换、优化、翻译或摘要", []string{"llm.call"}); err != nil {
		return err
	}

	// Deprecated: 建议使用 plugins/business/reviewer/ 的 business.reviewer 业务插件
	reviewerFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewReviewerNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeReviewer, reviewerFactory, "内置审核节点", "调用 LLM 对上游回答评分并返回审核结论", []string{"llm.call"}); err != nil {
		return err
	}

	routerFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewRouterNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeRouter, routerFactory, "内置路由节点", "按关键字、前缀或正则规则选择下游路径", nil); err != nil {
		return err
	}

	aggregatorFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewAggregatorNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeAggregator, aggregatorFactory, "内置聚合节点", "合并、摘要或择优多个上游节点输出", []string{"llm.call"}); err != nil {
		return err
	}

	memoryFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewMemoryNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeMemory, memoryFactory, "内置记忆查询节点", "查询用户/会话/全局记忆", []string{"memory.read"}); err != nil {
		return err
	}

	auditFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewAuditNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeAudit, auditFactory, "内置审核节点", "内容安全与合规审核", []string{"llm.call"}); err != nil {
		return err
	}

	optimizeFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewOptimizeNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeOptimize, optimizeFactory, "内置优化节点", "内容优化：清晰度、结构、完整性", []string{"llm.call"}); err != nil {
		return err
	}

	// 注册 LoopController 节点
	loopControllerFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewLoopControllerNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeLoopController, loopControllerFactory, "内置循环控制器节点", "支持重复执行子图直到条件满足或达到最大迭代次数", nil); err != nil {
		return err
	}

	// Phase 4: 注册缓存节点
	cacheFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewCacheNode(config)
	}
	// supportsStream=true 使 CreateFromConfig 返回原始 *CacheNode（而非 PluginBackedNode 包装），
	// 这样 engine 才能直接注入 cacheFacade / CacheManager / 策略插件（engine.go executeNode 中的类型断言依赖具体类型）。
	if err := registerBuiltinNodePlugin(registry, NodeTypeCache, cacheFactory, "内置缓存节点", "提供缓存读写功能，支持 exact/semantic/hybrid 策略和 memory/redis/sqlite 存储", nil, true); err != nil {
		return err
	}

	// Phase 4: 注册 Token 计量节点
	tokenUsageFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewTokenUsageNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeTokenUsage, tokenUsageFactory, "内置Token计量节点", "记录和统计 Token 使用，支持 memory/sqlite/postgresql 存储", nil); err != nil {
		return err
	}

	schedulerFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewSchedulerNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeScheduler, schedulerFactory, "内置调度节点", "根据意图分类与多维评分选择最优 backend/model", nil); err != nil {
		return err
	}

	transparentForwardFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewTransparentForwardNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeTransparentForward, transparentForwardFactory, "透明转发节点", "将原始 HTTP 请求原样转发到目标 API（#t / hostproxy）", []string{"network.outbound"}); err != nil {
		return err
	}

	// 注册工具调用注入节点
	toolCallInjectorFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewToolCallInjectorNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeToolCallInjector, toolCallInjectorFactory, "工具调用注入节点", "在Pipeline中注入工具调用指令，支持条件触发和模板变量解析", nil); err != nil {
		return err
	}

	// 注册用户 Prompt 操作节点
	userPromptOpsFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewUserPromptOpsNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeUserPromptOps, userPromptOpsFactory, "用户Prompt操作节点", "入站检查与优化：敏感词检测、密钥启发式、截断、空白折叠", nil); err != nil {
		return err
	}

	// 注册输出后处理节点
	outputPostOpsFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewOutputPostOpsNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeOutputPostOps, outputPostOpsFactory, "输出后处理节点", "字符串级输出规范化：trim、strip fence、extract JSON、compact JSON", nil); err != nil {
		return err
	}

	// 注册问题拆分节点（内置 fallback，当 business.question_splitter 未注册时使用）
	questionSplitterFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewQuestionSplitterNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeQuestionSplitter, questionSplitterFactory, "内置问题拆分节点", "问题拆分 fallback：直接透传原始问题，不做拆分（当 business.question_splitter 未注册时使用）", nil); err != nil {
		return err
	}

	// 注册答案合成节点（内置 fallback，当 business.answer_synthesizer 未注册时使用）
	answerSynthesizerFactory := func(config NodeConfig) (PipelineNode, error) {
		return NewAnswerSynthesizerNode(config)
	}
	if err := registerBuiltinNodePlugin(registry, NodeTypeAnswerSynthesizer, answerSynthesizerFactory, "内置答案合成节点", "答案合成 fallback：直接返回第一个子答案或原始内容（当 business.answer_synthesizer 未注册时使用）", nil); err != nil {
		return err
	}

	return nil
}

func registerBuiltinNodePlugin(registry *NodeRegistry, nodeType NodeType, factory NodeFactory, name, description string, permissions []string, supportsStream ...bool) error {
	if err := registry.Register(nodeType, factory); err != nil {
		return err
	}
	schemas := GetBuiltinSchemas(nodeType)
	stream := len(supportsStream) > 0 && supportsStream[0]
	plugin, err := NewBuiltinNodePlugin(nodeType, factory, NodePluginDescriptor{
		Name:           name,
		Implementation: BuiltinImplementationForType(nodeType),
		Kind:           KindForBuiltinType(nodeType),
		Version:        "1.0.0",
		Description:    description,
		ConfigSchema:   schemas.ConfigSchema,
		InputSchema:    schemas.InputSchema,
		OutputSchema:   schemas.OutputSchema,
		Permissions:    permissions,
		SupportsStream: stream,
	})
	if err != nil {
		return err
	}
	return registry.RegisterPlugin(plugin)
}

type MemoryNode struct {
	BaseNode
	QueryType string
	TopK      int
	Filter    map[string]interface{}
}

func NewMemoryNode(config NodeConfig) (PipelineNode, error) {
	node := &MemoryNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
		QueryType: "user",
		TopK:      5,
	}

	if config.CustomConfig != nil {
		if qt, ok := config.CustomConfig["query_type"].(string); ok {
			node.QueryType = qt
		}
		if tk, ok := config.CustomConfig["top_k"].(float64); ok {
			node.TopK = int(tk)
		}
		if f, ok := config.CustomConfig["filter"].(map[string]interface{}); ok {
			node.Filter = f
		}
	}

	return node, nil
}

func (n *MemoryNode) Type() NodeType {
	return NodeTypeMemory
}

func (n *MemoryNode) Validate() error {
	validQueryTypes := map[string]bool{
		"user":    true,
		"session": true,
		"global":  true,
	}
	if !validQueryTypes[n.QueryType] {
		return fmt.Errorf("invalid query_type: %s, must be user/session/global", n.QueryType)
	}
	if n.TopK <= 0 {
		return fmt.Errorf("top_k must be positive")
	}
	return nil
}

func (n *MemoryNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	userID := "0"
	sessionID := ""
	if input.Context != nil {
		if uid, ok := input.Context["user_id"].(string); ok {
			userID = uid
		}
		if sid, ok := input.Context["session_id"].(string); ok {
			sessionID = sid
		}
	}

	namespace := n.getNamespace(sessionID)
	query := input.Content
	if query == "" && input.Metadata != nil {
		if q, ok := input.Metadata["query"].(string); ok {
			query = q
		}
	}

	results, err := n.performSearch(ctx, namespace, query, n.TopK)
	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	memoryContent := n.formatResults(results)

	return &NodeOutput{
		Content: memoryContent,
		Metadata: map[string]interface{}{
			"query_type":   n.QueryType,
			"top_k":        n.TopK,
			"result_count": len(results),
			"namespace":    namespace,
			"user_id":      userID,
		},
	}, nil
}

func (n *MemoryNode) getNamespace(sessionID string) string {
	switch n.QueryType {
	case "user":
		return "user:" + n.BaseNode.ID()
	case "session":
		if sessionID != "" {
			return "session:" + sessionID
		}
		return "session:default"
	case "global":
		return "global"
	default:
		return "main"
	}
}

func (n *MemoryNode) performSearch(ctx context.Context, namespace, query string, topK int) ([]MemoryResult, error) {
	broker := n.GetCapabilityBroker()
	if broker == nil {
		return n.mockSearch(query, topK), nil
	}

	perms := n.GetPermissions()
	if len(perms) == 0 {
		perms = []string{"memory.read"}
	}

	memory, err := broker.GetMemory(ctx, perms)
	if err != nil || memory == nil {
		return n.mockSearch(query, topK), nil
	}

	return memory.Search(ctx, query, topK)
}

func (n *MemoryNode) mockSearch(query string, topK int) []MemoryResult {
	results := make([]MemoryResult, 0, topK)
	for i := 0; i < topK; i++ {
		results = append(results, MemoryResult{
			Key:   fmt.Sprintf("memory-%d", i+1),
			Score: 1.0 - float64(i)*0.1,
			Data:  []byte(fmt.Sprintf("Related memory %d for query: %s", i+1, query)),
		})
	}
	return results
}

func (n *MemoryNode) formatResults(results []MemoryResult) string {
	if len(results) == 0 {
		return ""
	}

	var content string
	for _, r := range results {
		content += string(r.Data) + "\n\n"
	}
	return content[:len(content)-2]
}

type AuditNode struct {
	BaseNode
	Model          string
	Rules          []string
	Threshold      float64
	PromptTemplate string
}

func NewAuditNode(config NodeConfig) (PipelineNode, error) {
	node := &AuditNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     45,
			retryConfig: DefaultRetryConfig(),
		},
		Model:     config.Model,
		Threshold: 0.8,
	}

	if config.CustomConfig != nil {
		if r, ok := config.CustomConfig["rules"].([]interface{}); ok {
			for _, rule := range r {
				if ruleStr, ok := rule.(string); ok {
					node.Rules = append(node.Rules, ruleStr)
				}
			}
		}
		if t, ok := config.CustomConfig["threshold"].(float64); ok {
			node.Threshold = t
		}
	}

	if len(node.Rules) == 0 {
		node.Rules = []string{"准确性", "完整性", "安全性", "专业性"}
	}

	if config.PromptTemplate != "" {
		node.PromptTemplate = config.PromptTemplate
	} else {
		node.PromptTemplate = node.defaultAuditPrompt()
	}

	return node, nil
}

func (n *AuditNode) defaultAuditPrompt() string {
	return `你是一名专业的内容审核员。请根据以下规则审核AI助手的回答。

## 待审核内容
{{.input}}

## 审核规则
{{range .rules}}- {{.}}
{{end}}

## 输出要求
请严格返回JSON格式：
{
    "passed": true/false,
    "score": 0.0-1.0,
    "feedback": "审核反馈说明",
    "violations": ["违规项1", "违规项2"]
}`
}

func (n *AuditNode) Type() NodeType {
	return NodeTypeAudit
}

func (n *AuditNode) Validate() error {
	if n.config.Backend == "" && n.Model == "" {
		return fmt.Errorf("audit node requires backend or model")
	}
	if n.Threshold < 0 || n.Threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1")
	}
	return nil
}

func (n *AuditNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	content := input.Content
	if content == "" && input.Metadata != nil {
		if c, ok := input.Metadata["content"].(string); ok {
			content = c
		}
	}

	prompt := n.buildPrompt(content)
	req := &LLMRequest{
		Model: n.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := n.CallLLM(ctx, "audit", req)
	if err != nil {
		return nil, fmt.Errorf("audit node %q: %w", n.id, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("audit node %q: no LLM client available (no capability broker or backend resolved)", n.id)
	}

	result := n.parseAuditResult(resp.Content)

	passed := result.Score >= n.Threshold

	return &NodeOutput{
		Content: content,
		Metadata: map[string]interface{}{
			"model":     n.Model,
			"rules":     n.Rules,
			"threshold": n.Threshold,
		},
		Passed:   &passed,
		Score:    &result.Score,
		Feedback: result.Feedback,
	}, nil
}

func (n *AuditNode) buildPrompt(content string) string {
	data := map[string]interface{}{
		"input": content,
		"rules": n.Rules,
	}

	tmpl, err := parseTemplate(n.PromptTemplate, data)
	if err != nil {
		return fmt.Sprintf("请审核以下内容：\n\n%s", content)
	}
	return tmpl
}

func parseTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("audit").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (n *AuditNode) parseAuditResult(content string) AuditResult {
	cleaned := cleanJSONContent(content)

	var result AuditResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return AuditResult{
			Passed:   true,
			Score:    0.5,
			Feedback: "无法解析审核结果",
		}
	}
	return result
}

type AuditResult struct {
	Passed     bool     `json:"passed"`
	Score      float64  `json:"score"`
	Feedback   string   `json:"feedback"`
	Violations []string `json:"violations"`
}

type OptimizeNode struct {
	BaseNode
	Model          string
	Strategy       string
	PromptTemplate string
}

func NewOptimizeNode(config NodeConfig) (PipelineNode, error) {
	node := &OptimizeNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     60,
			retryConfig: DefaultRetryConfig(),
		},
		Model:    config.Model,
		Strategy: "clarity",
	}

	if config.CustomConfig != nil {
		if s, ok := config.CustomConfig["strategy"].(string); ok {
			node.Strategy = s
		}
	}

	if config.PromptTemplate != "" {
		node.PromptTemplate = config.PromptTemplate
	} else {
		node.PromptTemplate = node.defaultOptimizePrompt()
	}

	return node, nil
}

func (n *OptimizeNode) defaultOptimizePrompt() string {
	switch n.Strategy {
	case "clarity":
		return `请优化以下内容，使其表达更清晰、结构更好：

## 原始内容
{{.input}}

## 要求
1. 使表达更清晰易懂
2. 保持原意不变
3. 优化结构

请直接返回优化后的内容。`
	case "structure":
		return `请优化以下内容，使其结构更清晰：

## 原始内容
{{.input}}

## 要求
1. 重新组织结构
2. 添加适当的标题和小结
3. 保持内容完整性

请直接返回优化后的内容。`
	case "completeness":
		return `请补全以下内容，补充缺失的关键信息：

## 原始内容
{{.input}}

## 要求
1. 补充缺失但关键的信息
2. 不添加无关内容
3. 保持准确性

请直接返回优化后的内容。`
	default:
		return `请优化以下内容：

## 原始内容
{{.input}}

请直接返回优化后的内容。`
	}
}

func (n *OptimizeNode) Type() NodeType {
	return NodeTypeOptimize
}

func (n *OptimizeNode) Validate() error {
	validStrategies := map[string]bool{
		"clarity":      true,
		"structure":    true,
		"completeness": true,
	}
	if !validStrategies[n.Strategy] {
		return fmt.Errorf("invalid strategy: %s, must be clarity/structure/completeness", n.Strategy)
	}
	return nil
}

func (n *OptimizeNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	content := input.Content
	if content == "" && input.Metadata != nil {
		if c, ok := input.Metadata["content"].(string); ok {
			content = c
		}
	}

	prompt := n.buildPrompt(content)
	req := &LLMRequest{
		Model: n.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := n.CallLLM(ctx, "optimize", req)
	if err != nil {
		return nil, fmt.Errorf("optimize node %q: %w", n.id, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("optimize node %q: no LLM client available (no capability broker or backend resolved)", n.id)
	}

	return &NodeOutput{
		Content: resp.Content,
		Metadata: map[string]interface{}{
			"model":    n.Model,
			"strategy": n.Strategy,
			"original": content,
		},
	}, nil
}

func (n *OptimizeNode) buildPrompt(content string) string {
	data := map[string]interface{}{
		"input": content,
	}

	tmpl, err := parseTemplate(n.PromptTemplate, data)
	if err != nil {
		return fmt.Sprintf("请优化以下内容：\n\n%s", content)
	}
	return tmpl
}

// ============================================
// LoopControllerNode - 循环控制器节点
// ============================================

// LoopControllerNode 循环控制器节点
// 支持重复执行子图直到条件满足或达到最大迭代次数
type LoopControllerNode struct {
	BaseNode
	MaxIterations int                  // 最大迭代次数
	Condition     string               // 循环条件表达式
	LoopVariable  string               // 循环计数变量名
	Subgraph      []PipelineNodeConfig // 子图节点配置
}

// LoopControllerConfig 循环控制器配置
type LoopControllerConfig struct {
	MaxIterations int                  `json:"max_iterations" yaml:"max_iterations"`
	Condition     string               `json:"condition" yaml:"condition"`
	LoopVariable  string               `json:"loop_variable" yaml:"loop_variable"`
	Subgraph      []PipelineNodeConfig `json:"subgraph" yaml:"subgraph"`
}

// NewLoopControllerNode 创建循环控制器节点
func NewLoopControllerNode(config NodeConfig) (PipelineNode, error) {
	node := &LoopControllerNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     300, // 循环节点默认超时更长
			retryConfig: DefaultRetryConfig(),
		},
		MaxIterations: 3,
		LoopVariable:  "loop_count",
	}

	// 从 CustomConfig 解析配置
	if config.CustomConfig != nil {
		if mi, ok := config.CustomConfig["max_iterations"]; ok {
			switch v := mi.(type) {
			case int:
				node.MaxIterations = v
			case float64:
				node.MaxIterations = int(v)
			}
		}
		if cond, ok := config.CustomConfig["condition"].(string); ok {
			node.Condition = cond
		}
		if lv, ok := config.CustomConfig["loop_variable"].(string); ok {
			node.LoopVariable = lv
		}
		if sg, ok := config.CustomConfig["subgraph"].([]interface{}); ok {
			// 解析子图配置
			for _, item := range sg {
				if nodeConfig, ok := item.(map[string]interface{}); ok {
					parsedConfig := parseNodeConfigFromMap(nodeConfig)
					node.Subgraph = append(node.Subgraph, parsedConfig)
				}
			}
		}
	}

	return node, nil
}

func (n *LoopControllerNode) Type() NodeType {
	return NodeTypeLoopController
}

func (n *LoopControllerNode) Validate() error {
	if n.MaxIterations <= 0 {
		return fmt.Errorf("loop_controller node requires max_iterations > 0")
	}
	if n.MaxIterations > 100 {
		return fmt.Errorf("loop_controller node max_iterations cannot exceed 100")
	}
	if n.Condition == "" {
		return fmt.Errorf("loop_controller node requires condition")
	}
	if len(n.Subgraph) == 0 {
		return fmt.Errorf("loop_controller node requires subgraph with at least one node")
	}
	return nil
}

// Execute 执行循环控制器节点
func (n *LoopControllerNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	if logger != nil {
		logger.Info(fmt.Sprintf("[LoopController] Starting loop execution - max_iterations: %d, condition: %s",
			n.MaxIterations, n.Condition))
	}

	var lastOutput *NodeOutput
	var lastErr error

	for i := 0; i < n.MaxIterations; i++ {
		// 设置循环变量
		if execCtx != nil {
			execCtx.SetVariable(n.LoopVariable, i)
			execCtx.SetVariable(fmt.Sprintf("%s_iteration", n.LoopVariable), i+1)
		}

		if logger != nil {
			logger.Info(fmt.Sprintf("[LoopController] Iteration %d/%d", i+1, n.MaxIterations))
		}

		// 执行子图（简化版本：顺序执行子图中的所有节点）
		// 注意：实际实现应该使用 PipelineEngine 来执行子图
		// 这里返回一个占位结果，实际逻辑在 PipelineEngine.executeLoopNode 中实现
		lastOutput = &NodeOutput{
			Content: fmt.Sprintf("Loop iteration %d", i+1),
			Metadata: map[string]interface{}{
				n.LoopVariable:        i,
				"iteration":           i + 1,
				"max_iterations":      n.MaxIterations,
				"condition":           n.Condition,
				"subgraph_node_count": len(n.Subgraph),
			},
		}

		// 评估循环条件
		if execCtx != nil && n.Condition != "" {
			evaluator := NewConditionEvaluator(execCtx)
			shouldContinue := evaluator.Evaluate(n.Condition)

			if logger != nil {
				logger.Info(fmt.Sprintf("[LoopController] Condition evaluated: %v", shouldContinue))
			}

			if !shouldContinue {
				if logger != nil {
					logger.Info(fmt.Sprintf("[LoopController] Condition met, exiting loop at iteration %d", i+1))
				}
				break
			}
		}
	}

	if lastOutput == nil {
		lastOutput = &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				n.LoopVariable:   n.MaxIterations,
				"iteration":      n.MaxIterations,
				"max_iterations": n.MaxIterations,
				"condition":      n.Condition,
			},
		}
	}

	// 添加循环完成标记
	lastOutput.Metadata["loop_completed"] = true
	lastOutput.Metadata["final_iteration"] = lastOutput.Metadata["iteration"]

	return lastOutput, lastErr
}

// ========== Phase 4: Cache Node ==========

// CacheNode 缓存节点 - 提供缓存读写功能，支持多种存储策略
type CacheNode struct {
	BaseNode
	Operation    string                 // 操作类型: read, write, delete
	Strategy     string                 // 策略: exact, semantic, hybrid
	StorageType  string                 // 存储类型: memory, redis, sqlite (兼容旧字段)
	TTL          int                    // 缓存过期时间(秒)
	KeyTemplate  string                 // 缓存键模板
	Config       map[string]interface{} // 存储特定配置
	CacheManager cache.CacheManager     // 缓存管理器（可选注入）

	// 可配置的存储后端（支持读写分离）
	ReadStorageName  string          // 读取存储名称（KV存储）
	WriteStorageName string          // 写入存储名称（KV存储）
	ReadStorage      storage.KVStore // 读取存储实例
	WriteStorage     storage.KVStore // 写入存储实例

	// 向量存储配置（语义缓存专用，可与KV存储分离）
	VectorStorageName string              // 向量存储名称（如 "pgvector"）
	ReadVectorStore   storage.VectorStore // 读取向量存储实例（用于语义缓存）
	WriteVectorStore  storage.VectorStore // 写入向量存储实例（用于语义缓存）

	// Embedding 服务配置（语义缓存需要向量化）
	EmbeddingBackendID string                     // Embedding 后端 ID（在后端管理页面配置）
	EmbeddingModel     string                     // Embedding 模型名称
	embeddingService   embedding.EmbeddingService // Embedding 服务实例（运行时注入）

	// 新增：评估配置（任务B）
	EnableEvaluation bool                   // 是否启用评估
	EvalThreshold    float64                // 评估通过阈值（默认70）
	EvalConfig       map[string]interface{} // 评估插件配置

	// 新增：QA拆分配置（任务C）
	EnableQASplit  bool   // 是否启用QA拆分
	QASplitPrompt  string // QA拆分提示词
	QASplitService string // QA拆分使用的模型服务名称

	// 新增：策略插件支持（P2）
	strategyPlugin    CacheStrategyCapability // 策略插件实例（通过 CapabilityBroker 获取）
	useStrategyPlugin bool                    // 是否使用策略插件模式
	semanticThreshold float32                 // 语义搜索阈值
	semanticTopK      int                     // 语义搜索 TopK

	// v0.3.3: 与 ProxyCache 共用的召回门面（优先于裸 CacheManager.Get）
	cacheFacade *cache.Facade
}

// NewCacheNode 创建缓存节点
func NewCacheNode(config NodeConfig) (PipelineNode, error) {
	node := &CacheNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     10,
			retryConfig: DefaultRetryConfig(),
		},
		Operation:   "read",               // 默认读操作
		Strategy:    "exact",              // 默认精确匹配
		StorageType: "memory",             // 默认内存存储
		TTL:         3600,                 // 默认1小时
		KeyTemplate: "{{model}}:{{hash}}", // 默认键模板
		Config:      make(map[string]interface{}),
	}

	if config.CustomConfig != nil {
		if op, ok := config.CustomConfig["operation"].(string); ok {
			node.Operation = op
		}
		if strategy, ok := config.CustomConfig["strategy"].(string); ok {
			node.Strategy = strategy
		}
		if storageType, ok := config.CustomConfig["storage_type"].(string); ok {
			node.StorageType = storageType
		}
		if ttl, ok := config.CustomConfig["ttl"].(float64); ok {
			node.TTL = int(ttl)
		}
		if keyTpl, ok := config.CustomConfig["key_template"].(string); ok {
			node.KeyTemplate = keyTpl
		}
		if cfg, ok := config.CustomConfig["config"].(map[string]interface{}); ok {
			node.Config = cfg
		}

		// 新增：读取存储后端配置
		if readStorage, ok := config.CustomConfig["read_storage_name"].(string); ok {
			node.ReadStorageName = readStorage
		}
		if writeStorage, ok := config.CustomConfig["write_storage_name"].(string); ok {
			node.WriteStorageName = writeStorage
		}

		// 新增：向量存储配置（语义缓存专用）
		if vectorStorage, ok := config.CustomConfig["vector_storage_name"].(string); ok {
			node.VectorStorageName = vectorStorage
		}

		// 新增：Embedding 服务配置（语义缓存需要向量化）
		if embeddingBackendID, ok := config.CustomConfig["embedding_backend_id"].(string); ok {
			node.EmbeddingBackendID = embeddingBackendID
		}
		if embeddingModel, ok := config.CustomConfig["embedding_model"].(string); ok {
			node.EmbeddingModel = embeddingModel
		}

		// 新增：评估配置（任务B）
		if enableEval, ok := config.CustomConfig["enable_evaluation"].(bool); ok {
			node.EnableEvaluation = enableEval
		}
		if evalThreshold, ok := config.CustomConfig["eval_threshold"].(float64); ok {
			node.EvalThreshold = evalThreshold
		} else {
			node.EvalThreshold = 70.0 // 默认阈值70分
		}
		if evalConfig, ok := config.CustomConfig["eval_config"].(map[string]interface{}); ok {
			node.EvalConfig = evalConfig
		}

		// 新增：QA拆分配置（任务C）
		if enableQASplit, ok := config.CustomConfig["enable_qa_split"].(bool); ok {
			node.EnableQASplit = enableQASplit
		}
		if qaSplitPrompt, ok := config.CustomConfig["qa_split_prompt"].(string); ok {
			node.QASplitPrompt = qaSplitPrompt
		}
		if qaSplitService, ok := config.CustomConfig["qa_split_service"].(string); ok {
			node.QASplitService = qaSplitService
		}

		// 新增：策略插件配置（P2）
		if useStrategyPlugin, ok := config.CustomConfig["use_strategy_plugin"].(bool); ok {
			node.useStrategyPlugin = useStrategyPlugin
		}
		if semanticThreshold, ok := config.CustomConfig["semantic_threshold"].(float64); ok {
			node.semanticThreshold = float32(semanticThreshold)
		} else {
			node.semanticThreshold = 0.85 // 默认阈值
		}
		if semanticTopK, ok := config.CustomConfig["semantic_top_k"].(float64); ok {
			node.semanticTopK = int(semanticTopK)
		} else {
			node.semanticTopK = 5 // 默认 TopK
		}
	}

	return node, nil
}

func (n *CacheNode) Type() NodeType {
	return NodeTypeCache
}

// SetCacheManager 设置缓存管理器（由外部注入）
func (n *CacheNode) SetCacheManager(cm cache.CacheManager) {
	n.CacheManager = cm
}

// SetCacheFacade 设置统一缓存门面（与 middleware ProxyCache 共用）
func (n *CacheNode) SetCacheFacade(f *cache.Facade) {
	n.cacheFacade = f
}

// globalCacheAllowsSemantic reports whether global cache.backend permits semantic recall/write.
func globalCacheAllowsSemantic() bool {
	cfg := config.Get()
	if cfg == nil {
		return false
	}
	c := cfg.Cache
	config.NormalizeCacheConfig(&c)
	return c.Backend == config.CacheBackendSemantic ||
		(c.AllowBackendStacking && c.Backend == config.CacheBackendExact)
}

func facadeLookupParams(n *CacheNode) (threshold float32, topK int) {
	threshold = n.semanticThreshold
	topK = n.semanticTopK
	if cfg := config.Get(); cfg != nil {
		c := cfg.Cache
		config.NormalizeCacheConfig(&c)
		if threshold <= 0 {
			threshold = c.Semantic.Threshold
		}
		if topK <= 0 {
			topK = c.Semantic.TopK
		}
	}
	if threshold <= 0 {
		threshold = 0.8
	}
	if topK <= 0 {
		topK = 5
	}
	return threshold, topK
}

func sessionIDFromCacheInput(input *NodeInput, execCtx *ExecutionContext) string {
	if input != nil {
		if input.Context != nil {
			if s, ok := input.Context["session_id"].(string); ok && s != "" {
				return s
			}
		}
		if input.Metadata != nil {
			if s, ok := input.Metadata["session_id"].(string); ok && s != "" {
				return s
			}
		}
	}
	if execCtx != nil {
		if v, ok := execCtx.GetVariable("session_id"); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// SetStrategyPlugin 设置缓存策略插件（P2新增）
func (n *CacheNode) SetStrategyPlugin(plugin CacheStrategyCapability) {
	n.strategyPlugin = plugin
	n.useStrategyPlugin = true
}

// GetStrategyPlugin 获取缓存策略插件
func (n *CacheNode) GetStrategyPlugin() CacheStrategyCapability {
	return n.strategyPlugin
}

// IsUsingStrategyPlugin 是否使用策略插件模式
func (n *CacheNode) IsUsingStrategyPlugin() bool {
	return n.useStrategyPlugin && n.strategyPlugin != nil
}

// storageManagerKey 用于在 context 中传递 storage.Manager
type storageManagerKey struct{}

// InitializeStorages 初始化存储后端（从 context 或全局获取存储管理器）
func (n *CacheNode) InitializeStorages(ctx context.Context) error {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	// 1. 尝试从 context 获取 storage.Manager
	var storageMgr *storage.Manager
	if ctx != nil {
		if mgr, ok := ctx.Value(storageManagerKey{}).(*storage.Manager); ok && mgr != nil {
			storageMgr = mgr
		}
	}

	// 2. 回退：使用全局存储管理器
	if storageMgr == nil {
		storageMgr = storage.GetGlobalManager()
	}

	if storageMgr == nil {
		if logger != nil {
			logger.Error("[CacheNode] InitializeStorages: storage manager not initialized, will use fallback cache behavior")
		}
		return fmt.Errorf("storage manager not initialized")
	}

	if logger != nil {
		logger.Info("[CacheNode] InitializeStorages: initializing storages for node",
			"read_storage_name", n.ReadStorageName,
			"write_storage_name", n.WriteStorageName)
	}

	listStorageNames := func() []string {
		storages := storageMgr.ListStorages()
		names := make([]string, len(storages))
		for i, s := range storages {
			names[i] = s.Name
		}
		return names
	}

	// 初始化读取存储
	if n.ReadStorageName != "" && n.ReadStorage == nil {
		kvStore, err := storageMgr.GetKVStore(n.ReadStorageName)
		if err != nil {
			if logger != nil {
				logger.Error("[CacheNode] InitializeStorages: failed to get read KVStore",
					"storage", n.ReadStorageName,
					"available_storages", listStorageNames(),
					"error", err.Error())
			}
			return fmt.Errorf("failed to get read KVStore '%s': %w", n.ReadStorageName, err)
		}
		n.ReadStorage = kvStore
		if logger != nil {
			logger.Debug("[CacheNode] InitializeStorages: read KVStore initialized", "storage", n.ReadStorageName)
		}

		// 如果策略是 semantic 或 hybrid，还需要向量存储
		if n.Strategy == "semantic" || n.Strategy == "hybrid" {
			// 优先使用 VectorStorageName，否则回退到 ReadStorageName
			vectorStorageName := n.VectorStorageName
			if vectorStorageName == "" {
				vectorStorageName = n.ReadStorageName
			}

			vectorStore, err := storageMgr.GetVectorStore(vectorStorageName)
			if err != nil {
				// 向量存储可选，记录警告但不失败
				if logger != nil {
					logger.Warn("[CacheNode] InitializeStorages: failed to get read VectorStore",
						"vector_storage", vectorStorageName,
						"error", err.Error())
				}
			} else {
				n.ReadVectorStore = vectorStore
				if logger != nil {
					logger.Debug("[CacheNode] InitializeStorages: read VectorStore initialized", "vector_storage", vectorStorageName)
				}
			}
		}
	}

	// 初始化写入存储
	if n.WriteStorageName != "" && n.WriteStorage == nil {
		kvStore, err := storageMgr.GetKVStore(n.WriteStorageName)
		if err != nil {
			if logger != nil {
				logger.Error("[CacheNode] InitializeStorages: failed to get write KVStore",
					"storage", n.WriteStorageName,
					"available_storages", listStorageNames(),
					"error", err.Error())
			}
			return fmt.Errorf("failed to get write KVStore '%s': %w", n.WriteStorageName, err)
		}
		n.WriteStorage = kvStore
		if logger != nil {
			logger.Debug("[CacheNode] InitializeStorages: write KVStore initialized", "storage", n.WriteStorageName)
		}

		// 如果策略是 semantic 或 hybrid，还需要向量存储
		if n.Strategy == "semantic" || n.Strategy == "hybrid" {
			// 优先使用 VectorStorageName，否则回退到 WriteStorageName
			vectorStorageName := n.VectorStorageName
			if vectorStorageName == "" {
				vectorStorageName = n.WriteStorageName
			}

			vectorStore, err := storageMgr.GetVectorStore(vectorStorageName)
			if err != nil {
				if logger != nil {
					logger.Warn("[CacheNode] InitializeStorages: failed to get write VectorStore",
						"vector_storage", vectorStorageName,
						"error", err.Error())
				}
			} else {
				n.WriteVectorStore = vectorStore
				if logger != nil {
					logger.Debug("[CacheNode] InitializeStorages: write VectorStore initialized", "vector_storage", vectorStorageName)
				}
			}
		}
	}

	// 初始化 Embedding 服务（语义缓存需要向量化）
	if (n.Strategy == "semantic" || n.Strategy == "hybrid") && n.embeddingService == nil {
		if n.EmbeddingBackendID != "" {
			embService, err := n.createEmbeddingService(ctx)
			if err != nil {
				if logger != nil {
					logger.Warn("[CacheNode] InitializeStorages: failed to create embedding service, degrading to exact match",
						"backend_id", n.EmbeddingBackendID,
						"model", n.EmbeddingModel,
						"error", err.Error())
				}
				// 不返回错误，允许降级到精确匹配
			} else {
				n.embeddingService = embService
				if logger != nil {
					logger.Debug("[CacheNode] InitializeStorages: embedding service initialized",
						"backend_id", n.EmbeddingBackendID,
						"model", n.EmbeddingModel)
				}
			}
		} else if logger != nil {
			logger.Warn("[CacheNode] InitializeStorages: semantic strategy selected but no embedding_backend_id configured")
		}
	}

	if logger != nil {
		logger.Debug("[CacheNode] InitializeStorages: completed")
	}
	return nil
}

// SetReadStorage 设置读取存储（用于测试或手动注入）
func (n *CacheNode) SetReadStorage(kvStore storage.KVStore, vectorStore storage.VectorStore) {
	n.ReadStorage = kvStore
	n.ReadVectorStore = vectorStore
}

// SetWriteStorage 设置写入存储（用于测试或手动注入）
func (n *CacheNode) SetWriteStorage(kvStore storage.KVStore, vectorStore storage.VectorStore) {
	n.WriteStorage = kvStore
	n.WriteVectorStore = vectorStore
}

// SetEmbeddingService 设置 Embedding 服务（由外部注入，用于测试或手动配置）
func (n *CacheNode) SetEmbeddingService(svc embedding.EmbeddingService) {
	n.embeddingService = svc
}

// GetEmbeddingService 获取 Embedding 服务
func (n *CacheNode) GetEmbeddingService() embedding.EmbeddingService {
	return n.embeddingService
}

// createEmbeddingService 根据配置创建 Embedding 服务
func (n *CacheNode) createEmbeddingService(ctx context.Context) (embedding.EmbeddingService, error) {
	if n.EmbeddingBackendID == "" {
		return nil, fmt.Errorf("embedding_backend_id not configured")
	}

	// 从全局后端管理器加载配置
	var embConfig *embedding.EmbeddingConfig

	mgr := backend.GetManager()
	if mgr != nil {
		if bCfg, err := mgr.Get(n.EmbeddingBackendID); err == nil {
			embConfig = &embedding.EmbeddingConfig{
				Provider: bCfg.Type,
				Model:    n.EmbeddingModel,
				BaseURL:  bCfg.BaseURL,
				APIKey:   bCfg.APIKey,
				Timeout:  bCfg.Timeout,
			}
		}
	}

	// 回退：如果无法从管理器加载配置，尝试从配置上下文中获取 base_url
	if embConfig == nil || embConfig.BaseURL == "" {
		if backendURL, ok := n.Config["embedding_base_url"].(string); ok && backendURL != "" {
			embConfig = &embedding.EmbeddingConfig{
				Provider: "ollama",
				Model:    n.EmbeddingModel,
				BaseURL:  backendURL,
				Timeout:  30,
			}
		}
	}

	if embConfig == nil {
		return nil, fmt.Errorf("cannot resolve backend config for embedding_backend_id=%q: not found in backend manager and no fallback URL provided", n.EmbeddingBackendID)
	}

	if embConfig.Provider == "" {
		// 根据 BaseURL 推断 provider
		if strings.Contains(strings.ToLower(embConfig.BaseURL), "ollama") || strings.Contains(embConfig.BaseURL, "21434") {
			embConfig.Provider = "ollama"
		} else {
			embConfig.Provider = "openai"
		}
	}

	return embedding.NewEmbeddingService(embConfig)
}

func (n *CacheNode) Validate() error {
	validOperations := map[string]bool{"read": true, "write": true, "delete": true}
	if !validOperations[n.Operation] {
		return fmt.Errorf("invalid operation: %s, must be read/write/delete", n.Operation)
	}

	validStrategies := map[string]bool{"exact": true, "semantic": true, "hybrid": true}
	if !validStrategies[n.Strategy] {
		return fmt.Errorf("invalid strategy: %s, must be exact/semantic/hybrid", n.Strategy)
	}

	validStorages := map[string]bool{"memory": true, "redis": true, "sqlite": true, "postgresql": true}
	if !validStorages[n.StorageType] {
		return fmt.Errorf("invalid storage_type: %s, must be memory/redis/sqlite/postgresql", n.StorageType)
	}

	return nil
}

func (n *CacheNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	// 获取执行上下文
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	// 解析模板变量（在请求级覆盖之前）
	if execCtx != nil {
		resolver := NewTemplateVarResolver(input, execCtx)

		// 解析 ReadStorageName
		if strings.Contains(n.ReadStorageName, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.ReadStorageName, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.ReadStorageName = s
				}
			}
		}

		// 解析 WriteStorageName
		if strings.Contains(n.WriteStorageName, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.WriteStorageName, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.WriteStorageName = s
				}
			}
		}

		// 解析 Strategy
		if strings.Contains(n.Strategy, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.Strategy, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.Strategy = s
				}
			}
		}

		// 解析 VectorStorageName
		if strings.Contains(n.VectorStorageName, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.VectorStorageName, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.VectorStorageName = s
				}
			}
		}

		// 解析 EmbeddingModel
		if strings.Contains(n.EmbeddingModel, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.EmbeddingModel, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.EmbeddingModel = s
				}
			}
		}

		// 解析 EmbeddingBackendID
		if strings.Contains(n.EmbeddingBackendID, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.EmbeddingBackendID, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.EmbeddingBackendID = s
				}
			}
		}
	}

	// 请求级参数覆盖：从执行上下文读取动态配置（优先级高于模板默认值）
	if execCtx != nil {
		if strategy, ok := execCtx.GetVariable("cache_strategy"); ok {
			if s, ok := strategy.(string); ok && s != "" {
				n.Strategy = s
				if logger != nil {
					logger.Info("[CacheNode] Request-level override: strategy=" + s)
				}
			}
		}
		if vectorStorage, ok := execCtx.GetVariable("vector_storage"); ok {
			if vs, ok := vectorStorage.(string); ok && vs != "" {
				n.ReadStorageName = vs
				n.WriteStorageName = vs
				n.ReadVectorStore = nil  // 需要重新初始化
				n.WriteVectorStore = nil // 需要重新初始化
				if logger != nil {
					logger.Info("[CacheNode] Request-level override: vector_storage=" + vs)
				}
			}
		}
		if embeddingModel, ok := execCtx.GetVariable("embedding_model"); ok {
			if em, ok := embeddingModel.(string); ok && em != "" {
				// 记录 embedding 模型配置，供后续使用
				if n.Config == nil {
					n.Config = make(map[string]interface{})
				}
				n.Config["embedding_model"] = em
				if logger != nil {
					logger.Info("[CacheNode] Request-level override: embedding_model=" + em)
				}
			}
		}
	}

	if skipped, ok := n.cacheOperationSkipped(execCtx, input); ok {
		return skipped, nil
	}

	// P2新增：如果启用了策略插件模式，使用策略插件执行
	if n.IsUsingStrategyPlugin() {
		return n.executeWithStrategyPlugin(ctx, input, logger)
	}

	// 在执行前初始化存储（如果尚未初始化）
	if n.ReadStorage == nil && n.ReadStorageName != "" || n.WriteStorage == nil && n.WriteStorageName != "" {
		if logger != nil {
			logger.Info("[CacheNode] Execute: initializing storages before execution")
		}
		if err := n.InitializeStorages(ctx); err != nil { // 传递 ctx
			if logger != nil {
				logger.Warn("[CacheNode] Execute: failed to initialize storages: " + err.Error())
			}
			// 不返回错误，允许降级到内存存储
		} else {
			if logger != nil {
				logger.Info("[CacheNode] Execute: storages initialized successfully")
			}
		}
	}

	if logger != nil {
		logger.Info("[CacheNode] executing operation",
			"operation", n.Operation,
			"strategy", n.Strategy,
			"read_storage", n.ReadStorageName,
			"write_storage", n.WriteStorageName,
			"read_storage_ready", n.ReadStorage != nil,
			"write_storage_ready", n.WriteStorage != nil,
			"facade_ready", n.cacheFacade != nil,
		)
	}

	// 构建缓存键
	// 对于 write 操作，需要基于用户输入（而不是助手响应）构建缓存键
	var cacheKey string
	if n.Operation == "write" {
		// 尝试获取用户输入
		userInput := extractUserInput(input)
		if logger != nil {
			logger.Debug("[CacheNode] executeWrite: extractUserInput result",
				"userInput", userInput,
				"inputContent", input.Content)
		}
		if userInput == "" {
			// 如果 input.Content 就是用户输入（没有 Messages），直接使用
			userInput = input.Content
		}

		// 如果仍然为空，尝试从执行上下文获取
		if userInput == "" {
			if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
				if originalInput, ok := execCtx.GetVariable("input"); ok {
					if s, ok := originalInput.(string); ok {
						userInput = s
						if logger != nil {
							logger.Debug("[CacheNode] executeWrite: got userInput from execCtx",
								"userInput", userInput)
						}
					}
				}
			}
		}

		// 使用用户输入构建缓存键
		// 键模型回退：扁平 input.Metadata 可能缺失 model（上游业务节点丢弃元数据），
		// 从 generator 结果补齐，保证与 cache_read 侧的键一致
		keyMetadata := input.Metadata
		if keyMetadata == nil {
			keyMetadata = map[string]interface{}{}
		}
		if m, _ := keyMetadata["model"].(string); m == "" {
			if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
				if genResult, ok := execCtx.GetResult("generator"); ok && genResult != nil && genResult.Metadata != nil {
					if gm, ok := genResult.Metadata["model"].(string); ok && gm != "" {
						merged := map[string]interface{}{"model": gm}
						for k, v := range keyMetadata {
							merged[k] = v
						}
						keyMetadata = merged
					}
				}
			}
		}
		cacheKey = n.buildCacheKey(&NodeInput{
			Content:  userInput,
			Metadata: keyMetadata,
		})
		if logger != nil {
			logger.Info("[CacheNode] executeWrite: built cache key for write operation",
				"cacheKey", cacheKey,
				"userInput", userInput)
		}
	} else {
		// read/delete 操作使用当前 input
		cacheKey = n.buildCacheKey(input)
	}

	// 根据操作类型执行
	switch n.Operation {
	case "read":
		return n.executeRead(ctx, cacheKey, input)
	case "write":
		return n.executeWrite(ctx, cacheKey, input)
	case "delete":
		return n.executeDelete(ctx, cacheKey)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", n.Operation)
	}
}

// cacheOperationSkipped returns a no-op output when per-request cache switches disable this operation.
func (n *CacheNode) cacheOperationSkipped(execCtx *ExecutionContext, input *NodeInput) (*NodeOutput, bool) {
	switch n.Operation {
	case "read":
		if BoolFromExecCtx(execCtx, "cache_read", true) {
			return nil, false
		}
		return &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"cache_hit":          false,
				"cache_read_skipped": true,
			},
		}, true
	case "write":
		if BoolFromExecCtx(execCtx, "cache_write", true) {
			return nil, false
		}
		content := ""
		if input != nil {
			content = input.Content
		}
		return &NodeOutput{
			Content: content,
			Metadata: map[string]interface{}{
				"write_success":       false,
				"cache_write_skipped": true,
			},
		}, true
	default:
		return nil, false
	}
}

// executeWithStrategyPlugin 使用策略插件执行缓存操作（P2新增）
func (n *CacheNode) executeWithStrategyPlugin(ctx context.Context, input *NodeInput, logger Logger) (*NodeOutput, error) {
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if skipped, ok := n.cacheOperationSkipped(execCtx, input); ok {
		return skipped, nil
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] Using strategy plugin: %s", n.strategyPlugin.StrategyName()))
	}

	// 提取用户输入
	userInput := extractUserInput(input)
	if userInput == "" {
		userInput = input.Content
	}

	// 构建缓存键
	cacheKey := n.buildCacheKey(input)

	switch n.Operation {
	case "read":
		// 使用策略插件读取
		result, err := n.strategyPlugin.Read(ctx, userInput, n.semanticThreshold, n.semanticTopK)
		if err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] Strategy plugin read failed: " + err.Error())
			}
			return &NodeOutput{
				Content:  "",
				Metadata: map[string]interface{}{"cache_hit": false, "error": err.Error()},
			}, nil
		}

		return &NodeOutput{
			Content: result.Content,
			Metadata: map[string]interface{}{
				"cache_hit":   result.Hit,
				"cache_key":   result.Key,
				"cache_score": result.Score,
				"strategy":    n.strategyPlugin.StrategyName(),
			},
		}, nil

	case "write":
		// 使用策略插件写入（request=用户问题，content=响应）
		ttl := time.Duration(n.TTL) * time.Second
		if err := n.strategyPlugin.Write(ctx, cacheKey, userInput, input.Content, ttl); err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] Strategy plugin write failed: " + err.Error())
			}
			return &NodeOutput{
				Content:  input.Content,
				Metadata: map[string]interface{}{"cache_write_success": false, "error": err.Error()},
			}, nil
		}

		return &NodeOutput{
			Content:  input.Content,
			Metadata: map[string]interface{}{"cache_write_success": true, "cache_key": cacheKey},
		}, nil

	case "delete":
		// 使用策略插件删除
		if err := n.strategyPlugin.Delete(ctx, cacheKey); err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] Strategy plugin delete failed: " + err.Error())
			}
			return &NodeOutput{
				Content:  "",
				Metadata: map[string]interface{}{"cache_delete_success": false, "error": err.Error()},
			}, nil
		}

		return &NodeOutput{
			Content:  "",
			Metadata: map[string]interface{}{"cache_delete_success": true, "cache_key": cacheKey},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported operation: %s", n.Operation)
	}
}

// buildCacheKey 构建缓存键
func (n *CacheNode) buildCacheKey(input *NodeInput) string {
	// 使用简单的模板替换
	key := n.KeyTemplate

	// 替换 {{model}} - 从多个可能的来源获取模型名称
	modelName := ""

	// 1. 优先从 input.Metadata 获取
	if input.Metadata != nil {
		if model, ok := input.Metadata["model"].(string); ok && model != "" {
			modelName = model
		}
	}

	// 2. 回退：从执行上下文获取
	if modelName == "" {
		if execCtx, ok := context.Background().Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
			if metadata, ok := execCtx.GetVariable("metadata"); ok {
				if m, ok := metadata.(map[string]interface{}); ok {
					if model, ok := m["model"].(string); ok && model != "" {
						modelName = model
					}
				}
			}
		}
	}

	// 3. 回退：从 config 中获取
	if modelName == "" {
		modelName = n.config.Model
	}

	// 如果找到了模型名称，替换占位符
	if modelName != "" {
		key = strings.ReplaceAll(key, "{{model}}", modelName)
	} else {
		// 如果没有模型名称，使用默认值
		key = strings.ReplaceAll(key, "{{model}}", "default")
	}

	// 替换 {{hash}} - 基于内容生成哈希
	content := input.Content
	if content == "" && len(input.Messages) > 0 {
		// 从消息中提取用户内容
		for _, msg := range input.Messages {
			if msg.Role == "user" {
				content = msg.Content
				break
			}
		}
	}

	// 生成内容哈希(简化版)
	hash := n.simpleHash(content)
	key = strings.ReplaceAll(key, "{{hash}}", hash)

	return key
}

// simpleHash 生成简单哈希
func (n *CacheNode) simpleHash(content string) string {
	if content == "" {
		return "empty"
	}
	// 使用 []rune 按字符切片，避免切断多字节 UTF-8 字符
	runes := []rune(content)
	if len(runes) <= 16 {
		return content
	}
	return string(runes[:8]) + string(runes[len(runes)-8:])
}

// executeRead 执行读操作
func (n *CacheNode) executeRead(ctx context.Context, key string, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	var cachedContent string
	var cacheHit bool
	var cachedEntry *cache.CacheEntry

	// 1. 优先使用配置的读取存储
	if n.ReadStorage != nil {
		// 从 KVStore 读取
		data, err := n.ReadStorage.GetBytes(ctx, key)
		if err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] ReadStorage read error",
					"key", key,
					"storage", n.ReadStorageName,
					"error", err)
			}
		} else if len(data) > 0 {
			// 反序列化缓存条目
			entry, unmarshalErr := cache.UnmarshalCacheEntry(data)
			if unmarshalErr == nil {
				// 检查是否过期
				if time.Now().Before(entry.ExpiresAt) {
					cachedEntry = entry
					cachedContent = entry.Response
					cacheHit = true
					if logger != nil {
						logger.Info(fmt.Sprintf("[CacheNode] ReadStorage hit: %s (storage=%s, response_length=%d)",
							key, n.ReadStorageName, len(cachedContent)))
					}
				} else {
					if logger != nil {
						logger.Info(fmt.Sprintf("[CacheNode] ReadStorage expired: %s (storage=%s)",
							key, n.ReadStorageName))
					}
				}
			} else {
				if logger != nil {
					logger.Warn("[CacheNode] Failed to unmarshal cache entry",
						"key", key,
						"error", unmarshalErr,
						"data_length", len(data),
						"data_preview", string(data[:min(50, len(data))]))
				}
			}
		}
	}

	// 2. 统一门面 Lookup（尊重全局 cache.backend / stacking / external，触发 OnCacheHit）
	if !cacheHit && n.cacheFacade != nil {
		userInput := extractUserInput(input)
		if userInput == "" {
			userInput = input.Content
		}
		threshold, topK := facadeLookupParams(n)
		entry, ok, err := n.cacheFacade.Lookup(ctx, key, userInput, threshold, topK)
		if err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] Cache facade read error",
					"key", key,
					"error", err)
			}
		} else if ok && entry != nil {
			cachedEntry = entry
			cachedContent = entry.Response
			cacheHit = true
			if logger != nil {
				logger.Info(fmt.Sprintf("[CacheNode] Cache facade hit: %s label=%s (response_length=%d)",
					key, cache.HitLabel(entry), len(cachedContent)))
			}
		}
	}

	// 3. 回退：使用注入的缓存管理器（无门面时）
	if !cacheHit && n.cacheFacade == nil && n.CacheManager != nil {
		entry, err := n.CacheManager.Get(ctx, key)
		if err != nil {
			if logger != nil {
				logger.Warn("[CacheNode] Cache manager read error",
					"key", key,
					"error", err)
			}
		} else if entry != nil {
			cachedEntry = entry
			cachedContent = entry.Response
			cacheHit = true
			if logger != nil {
				logger.Info(fmt.Sprintf("[CacheNode] Cache manager hit: %s (response_length=%d)",
					key, len(cachedContent)))
			}
		}
	}

	// 4. 回退：从执行上下文读取（兼容旧逻辑）
	if !cacheHit && execCtx != nil {
		if val, ok := execCtx.GetVariable("cache_" + key); ok {
			cachedContent = val.(string)
			cacheHit = true
		}
	}

	// 5. 节点本地语义搜索：仅当无门面且全局允许 semantic/stacking 时启用（禁止绕过全局互斥）
	if !cacheHit && n.cacheFacade == nil && globalCacheAllowsSemantic() &&
		(n.Strategy == "semantic" || n.Strategy == "hybrid") && n.ReadVectorStore != nil && n.embeddingService != nil {
		if logger != nil {
			logger.Info("[CacheNode] KV miss, attempting semantic search",
				"key", key,
				"strategy", n.Strategy,
				"vector_storage", n.VectorStorageName)
		}

		// 提取用户输入用于向量化
		userInput := extractUserInput(input)
		if userInput == "" {
			if execCtx != nil {
				if v, ok := execCtx.GetVariable("input"); ok {
					if s, ok := v.(string); ok {
						userInput = s
					}
				}
			}
		}
		if userInput == "" {
			userInput = input.Content
		}

		if userInput != "" {
			// 向量化查询文本
			queryVector, err := n.embeddingService.GetEmbedding(ctx, userInput)
			if err != nil {
				if logger != nil {
					logger.Warn("[CacheNode] Failed to embed query for semantic search",
						"error", err)
				}
			} else if len(queryVector) > 0 {
				// 搜索相似向量
				topK := n.semanticTopK
				if topK <= 0 {
					topK = 5
				}
				threshold := n.semanticThreshold
				if threshold <= 0 {
					threshold = 0.85
				}

				results, err := n.ReadVectorStore.Search(ctx, queryVector, topK, nil)
				if err != nil {
					if logger != nil {
						logger.Warn("[CacheNode] VectorStore search error",
							"error", err)
					}
				} else if len(results) > 0 {
					// 取最高分且超过阈值的结果
					best := results[0]
					if best.Score >= threshold {
						if logger != nil {
							logger.Info("[CacheNode] Semantic cache hit",
								"score", best.Score,
								"threshold", threshold,
								"vector_id", best.ID)
						}

						// 从向量 metadata 中恢复缓存内容
						if resp, ok := best.Metadata["response"].(string); ok {
							cachedContent = resp
							cacheHit = true
							// 尝试恢复缓存条目
							cachedEntry = &cache.CacheEntry{
								Key:       best.ID,
								Response:  resp,
								Request:   best.Metadata["request"].(string),
								Timestamp: time.Now(),
								ExpiresAt: time.Now().Add(time.Duration(n.TTL) * time.Second),
							}
							if tsStr, ok := best.Metadata["timestamp"].(string); ok {
								if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
									cachedEntry.Timestamp = t
								}
							}
							if expStr, ok := best.Metadata["expires_at"].(string); ok {
								if t, err := time.Parse(time.RFC3339, expStr); err == nil {
									cachedEntry.ExpiresAt = t
								}
							}
							// 恢复计量元数据，供 TokenUsageNode 精确计量
							for _, mk := range []string{"model", "backend", "prompt_tokens", "completion_tokens", "total_tokens"} {
								if v, ok := best.Metadata[mk]; ok && v != nil {
									if cachedEntry.Metadata == nil {
										cachedEntry.Metadata = map[string]interface{}{}
									}
									cachedEntry.Metadata[mk] = v
								}
							}
						}
					} else {
						if logger != nil {
							logger.Info("[CacheNode] Semantic search result below threshold",
								"best_score", best.Score,
								"threshold", threshold)
						}
					}
				}
			}
		}
	}

	// 4b. 语义回退：如果 embedding 服务不可用但存储支持 SearchByText
	if !cacheHit && (n.Strategy == "semantic" || n.Strategy == "hybrid") && n.ReadVectorStore != nil && n.embeddingService == nil {
		if ftsStore, ok := n.ReadVectorStore.(storage.FullTextSearchStore); ok {
			if logger != nil {
				logger.Info("[CacheNode] KV miss + no embedding service, using SearchByText fallback")
			}

			userInput := extractUserInput(input)
			if userInput == "" {
				if execCtx != nil {
					if v, ok := execCtx.GetVariable("input"); ok {
						if s, ok := v.(string); ok {
							userInput = s
						}
					}
				}
			}
			if userInput == "" {
				userInput = input.Content
			}

			if userInput != "" {
				topK := n.semanticTopK
				if topK <= 0 {
					topK = 5
				}
				threshold := n.semanticThreshold
				if threshold <= 0 {
					threshold = 0.85
				}

				results, err := ftsStore.SearchByText(ctx, userInput, topK, threshold)
				if err != nil {
					if logger != nil {
						logger.Warn("[CacheNode] SearchByText error", "error", err)
					}
				} else if len(results) > 0 {
					best := results[0]
					if resp, ok := best.Metadata["response"].(string); ok {
						if logger != nil {
							logger.Info("[CacheNode] Semantic cache hit (via SearchByText)",
								"score", best.Score, "vector_id", best.ID)
						}
						cachedContent = resp
						cacheHit = true
						cachedEntry = &cache.CacheEntry{
							Key:       best.ID,
							Response:  resp,
							Request:   userInput,
							Timestamp: time.Now(),
							ExpiresAt: time.Now().Add(time.Duration(n.TTL) * time.Second),
						}
						// 恢复计量元数据，供 TokenUsageNode 精确计量
						for _, mk := range []string{"model", "backend", "prompt_tokens", "completion_tokens", "total_tokens"} {
							if v, ok := best.Metadata[mk]; ok && v != nil {
								if cachedEntry.Metadata == nil {
									cachedEntry.Metadata = map[string]interface{}{}
								}
								cachedEntry.Metadata[mk] = v
							}
						}
					}
				}
			}
		}
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] Read cache key: %s (strategy: %s, read_storage: %s, hit: %v)",
			key, n.Strategy, n.ReadStorageName, cacheHit))
	}

	// 构建输出
	metadata := map[string]interface{}{
		"operation":    "read",
		"cache_key":    key,
		"cache_hit":    cacheHit,
		"strategy":     n.Strategy,
		"read_storage": n.ReadStorageName,
		"storage_type": n.StorageType,
	}

	if cacheHit {
		// 缓存命中：将缓存响应传递给下游
		output := &NodeOutput{
			Content:  cachedContent,
			Metadata: metadata,
		}

		// 透传缓存条目中的模型与 token 用量，供 TokenUsageNode 精确计量
		if cachedEntry != nil && cachedEntry.Metadata != nil {
			for _, mk := range []string{"model", "backend", "prompt_tokens", "completion_tokens", "total_tokens"} {
				if v, ok := cachedEntry.Metadata[mk]; ok && v != nil {
					if s, isStr := v.(string); !isStr || s != "" {
						metadata[mk] = v
					}
				}
			}
		}

		// 如果有缓存的 messages，也传递
		if cachedEntry != nil && cachedEntry.Metadata != nil {
			if msgs, ok := cachedEntry.Metadata["messages"].([]Message); ok {
				output.Messages = msgs
			}
		}

		// 如果是流式缓存，传递流式数据
		if cachedEntry != nil && cachedEntry.IsStream {
			output.IsStream = true
			output.StreamData = cachedEntry.StreamData
			if logger != nil {
				logger.Info(fmt.Sprintf("[CacheNode] Returning stream data: %d chunks", len(cachedEntry.StreamData)))
			}
		}

		return output, nil
	}

	// 缓存未命中：返回空内容，标记未命中
	return &NodeOutput{
		Content:  "",
		Metadata: metadata,
	}, nil
}

// executeWrite 执行写操作
func (n *CacheNode) executeWrite(ctx context.Context, key string, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	// 获取要缓存的内容（优先使用 generator 节点的输出）
	content := input.Content
	var messages []Message

	// 从输入消息中提取 assistant 响应（如果有）
	if content == "" && len(input.Messages) > 0 {
		for i := len(input.Messages) - 1; i >= 0; i-- {
			if input.Messages[i].Role == "assistant" {
				content = input.Messages[i].Content
				messages = input.Messages
				break
			}
		}
	}

	// 如果仍然没有内容，尝试从上游节点结果获取
	if content == "" {
		if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
			if genResult, ok := execCtx.GetResult("generator"); ok && genResult != nil {
				content = genResult.Content
				messages = genResult.Messages
			}
		}
	}

	// 检查 generator 节点是否真正执行了（防止 cache_write 在 generator 被跳过时缓存 fallback 的原始输入）
	// 测试模式：如果 execCtx 中设置了 "test_mode" 变量，跳过 generator 执行检查
	if content != "" {
		if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
			if _, isTestMode := execCtx.GetVariable("test_mode"); !isTestMode {
				if _, ok := execCtx.GetResult("generator"); !ok {
					if logger != nil {
						logger.Warn("[CacheNode] Skipping cache write: generator was skipped (cache hit), content is fallback input",
							"key", key, "content", content)
					}
					return &NodeOutput{
						Content: content,
						Metadata: map[string]interface{}{
							"operation": "write",
							"cache_key": key,
							"skipped":   true,
							"reason":    "generator_not_executed",
						},
					}, nil
				}
			}
		}
	}

	if content == "" {
		if logger != nil {
			logger.Warn("[CacheNode] No content to cache, skipping write",
				"key", key)
		}
		return &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				"operation": "write",
				"cache_key": key,
				"skipped":   true,
				"reason":    "no_content",
			},
		}, nil
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] Write cache key: %s (ttl: %ds, strategy: %s, write_storage: %s, content_length: %d, eval_enabled: %v)",
			key, n.TTL, n.Strategy, n.WriteStorageName, len(content), n.EnableEvaluation))
	}

	// 评估：如果启用评估，检查是否值得缓存（任务B）
	if n.EnableEvaluation && n.CacheManager != nil {
		if n.CacheManager.ShouldEvaluateCache() {
			// 提取用户输入用于评估
			userInput := extractUserInput(input)
			if userInput == "" {
				userInput = input.Content
			}

			evalMessages := make([]evalplugin.Message, 0, len(messages))
			for _, msg := range messages {
				evalMessages = append(evalMessages, evalplugin.Message{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}

			result, err := n.CacheManager.EvaluateCacheEntry(ctx, userInput, content, evalMessages)
			if err == nil && result != nil {
				if logger != nil {
					logger.Info(fmt.Sprintf("[CacheNode] Evaluation result: score=%.2f, passed=%v, labels=%v",
						result.Score, result.ShouldCache, result.Labels))
				}

				// 如果评估未通过，跳过缓存
				if !result.ShouldCache {
					if logger != nil {
						logger.Info(fmt.Sprintf("[CacheNode] Cache entry rejected by evaluation: score=%.2f < threshold=%.2f",
							result.Score, n.EvalThreshold))
					}
					return &NodeOutput{
						Content:  content,
						Messages: messages,
						Metadata: map[string]interface{}{
							"operation":      "write",
							"cache_key":      key,
							"skipped":        true,
							"reason":         "evaluation_failed",
							"eval_score":     result.Score,
							"eval_labels":    result.Labels,
							"eval_threshold": n.EvalThreshold,
						},
					}, nil
				}

				// 评估通过，记录评估信息到元数据
				if result.Score > 0 {
					if input.Metadata == nil {
						input.Metadata = make(map[string]interface{})
					}
					input.Metadata["eval_score"] = result.Score
					input.Metadata["eval_labels"] = result.Labels
				}
			}
		}
	}

	// 提取用户输入（用于缓存的 request 字段）
	userInput := extractUserInput(input)
	if userInput == "" {
		if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
			if originalInput, ok := execCtx.GetVariable("input"); ok {
				if s, ok := originalInput.(string); ok {
					userInput = s
				}
			}
		}
	}

	// 检查是否为流式响应（从上游节点获取）
	var streamData []cache.StreamChunk
	var isStream bool
	if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
		if genResult, ok := execCtx.GetResult("generator"); ok && genResult != nil {
			if genResult.IsStream && len(genResult.StreamData) > 0 {
				isStream = true
				streamData = genResult.StreamData
				if logger != nil {
					logger.Info(fmt.Sprintf("[CacheNode] Detected stream response from generator: %d chunks", len(streamData)))
				}
			}
		}
	}

	// 构建缓存条目
	writeStorageName := n.WriteStorageName
	if writeStorageName == "" {
		writeStorageName = "default"
	}
	execCtxWrite, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	sessionID := sessionIDFromCacheInput(input, execCtxWrite)
	requestID := ""
	if input != nil && input.Metadata != nil {
		if rid, ok := input.Metadata["request_id"].(string); ok {
			requestID = rid
		}
	}
	backendName := ""
	modelName := ""
	if input != nil && input.Metadata != nil {
		if b, ok := input.Metadata["backend"].(string); ok {
			backendName = b
		}
		if m, ok := input.Metadata["model"].(string); ok {
			modelName = m
		}
	}
	meta := map[string]interface{}{
		"model":         modelName,
		"backend":       backendName,
		"messages":      messages,
		"strategy":      n.Strategy,
		"storage_type":  n.StorageType,
		"write_storage": writeStorageName,
		"read_storage":  n.ReadStorageName,
		"is_stream":     isStream,
		"stream_chunks": len(streamData),
	}
	// 记录 token 用量，供缓存命中时 TokenUsageNode 精确计量（避免用输入长度估算）
	if input != nil && input.Metadata != nil {
		for _, tk := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
			if v := tokenRecordInt(input.Metadata[tk]); v > 0 {
				meta[tk] = v
			}
		}
	}
	// 兜底：从 generator 节点结果补齐计量元数据。
	// 内置 answer_synthesizer 等业务节点不透传上游元数据，扁平 input.Metadata
	// 可能缺失 model/backend/token 字段；直接读取 generator 结果确保缓存条目
	// 携带真实用量，命中时 TokenUsageNode 可精确计量而非按输入长度估算。
	if execCtxWrite != nil {
		if genResult, ok := execCtxWrite.GetResult("generator"); ok && genResult != nil && genResult.Metadata != nil {
			gm := genResult.Metadata
			if modelName == "" {
				if m, ok := gm["model"].(string); ok {
					modelName = m
					meta["model"] = m
				}
			}
			if backendName == "" {
				if b, ok := gm["backend_id"].(string); ok {
					backendName = b
					meta["backend"] = b
				} else if b, ok := gm["backend"].(string); ok {
					backendName = b
					meta["backend"] = b
				}
			}
			if _, exists := meta["total_tokens"]; !exists {
				if v := tokenRecordInt(gm["total_tokens"]); v > 0 {
					meta["total_tokens"] = v
				} else if v := tokenRecordInt(gm["tokens"]); v > 0 {
					meta["total_tokens"] = v
				}
			}
			if _, exists := meta["prompt_tokens"]; !exists {
				if v := tokenRecordInt(gm["prompt_tokens"]); v > 0 {
					meta["prompt_tokens"] = v
				}
			}
			if _, exists := meta["completion_tokens"]; !exists {
				if pt, hasPT := meta["prompt_tokens"]; hasPT {
					if tt, hasTT := meta["total_tokens"]; hasTT {
						if ct := tokenRecordInt(tt) - tokenRecordInt(pt); ct > 0 {
							meta["completion_tokens"] = ct
						}
					}
				}
			}
		}
	}
	meta = cache.AttachRequestContextMetadata(meta, sessionID, requestID, backendName)
	if modelName != "" {
		if v, _ := meta["model"].(string); v == "" {
			meta["model"] = modelName
		}
	}

	cacheEntry := &cache.CacheEntry{
		Key:            key,
		Request:        userInput,
		Response:       content,
		Timestamp:      time.Now(),
		ExpiresAt:      time.Now().Add(time.Duration(n.TTL) * time.Second),
		StorageBackend: writeStorageName,
		IsStream:       isStream,
		StreamData:     streamData,
		Metadata:       meta,
	}

	// 写入缓存条目（直接传递 cacheEntry，由存储后端负责序列化）
	// 1. 优先写入配置的写入存储
	if n.WriteStorage != nil {
		ttl := time.Duration(n.TTL) * time.Second
		if err := n.WriteStorage.Set(ctx, key, cacheEntry, ttl); err != nil {
			if logger != nil {
				logger.Error("[CacheNode] WriteStorage write error",
					"key", key,
					"storage", n.WriteStorageName,
					"error", err)
			}
		} else {
			if logger != nil {
				logger.Info("[CacheNode] Written to WriteStorage successfully",
					"key", key,
					"storage", n.WriteStorageName,
					"ttl_seconds", n.TTL)
			}
		}
	}

	// 2. 节点本地向量写入：仅当无门面且全局允许 semantic/stacking
	if n.cacheFacade == nil && globalCacheAllowsSemantic() &&
		(n.Strategy == "semantic" || n.Strategy == "hybrid") && n.WriteVectorStore != nil && n.embeddingService != nil {
		if logger != nil {
			logger.Info("[CacheNode] Writing to VectorStore for semantic cache",
				"key", key,
				"strategy", n.Strategy,
				"vector_storage", n.VectorStorageName)
		}

		// 将用户输入向量化
		vector, err := n.embeddingService.GetEmbedding(ctx, userInput)
		if err != nil {
			if logger != nil {
				logger.Error("[CacheNode] Failed to embed user input for vector storage",
					"key", key,
					"error", err)
			}
			// 向量化失败不阻塞主流程，继续往下走
		} else if len(vector) > 0 {
			// 构建向量数据
			vecMeta := map[string]interface{}{
				"key":        key,
				"request":    userInput,
				"response":   content,
				"timestamp":  cacheEntry.Timestamp.Format(time.RFC3339),
				"expires_at": cacheEntry.ExpiresAt.Format(time.RFC3339),
			}
			// 携带计量元数据，语义命中时可恢复真实用量供 TokenUsageNode 计量
			for _, mk := range []string{"model", "backend", "prompt_tokens", "completion_tokens", "total_tokens"} {
				if v, ok := meta[mk]; ok && v != nil {
					vecMeta[mk] = v
				}
			}
			vec := storage.Vector{
				ID:       key,
				Vector:   vector,
				Metadata: vecMeta,
			}

			// 写入向量
			if err := n.WriteVectorStore.Insert(ctx, []storage.Vector{vec}); err != nil {
				if logger != nil {
					logger.Error("[CacheNode] Failed to write vector to VectorStore",
						"key", key,
						"vector_storage", n.VectorStorageName,
						"error", err)
				}
			} else {
				if logger != nil {
					logger.Info("[CacheNode] Successfully written vector to VectorStore",
						"key", key,
						"vector_storage", n.VectorStorageName,
						"vector_dim", len(vector))
				}
			}
		}
	}

	// 3. 统一门面 Store（按全局 backend：exact/semantic/external）
	ttl := time.Duration(n.TTL) * time.Second
	if n.cacheFacade != nil {
		if err := n.cacheFacade.Store(ctx, key, cacheEntry, ttl); err != nil {
			if logger != nil {
				logger.Error("[CacheNode] Cache facade write error",
					"key", key,
					"backend", n.cacheFacade.EffectiveBackend(),
					"error", err)
			}
		} else if logger != nil {
			logger.Info("[CacheNode] Cache facade written successfully",
				"key", key,
				"backend", n.cacheFacade.EffectiveBackend(),
				"session_id", sessionID,
				"ttl_seconds", n.TTL)
		}
	} else if n.CacheManager != nil {
		// 回退：写入缓存管理器
		if err := n.CacheManager.Set(ctx, key, cacheEntry, ttl); err != nil {
			if logger != nil {
				logger.Error("[CacheNode] Cache manager write error",
					"key", key,
					"strategy", n.Strategy,
					"error", err)
			}
		} else if logger != nil {
			logger.Info("[CacheNode] Cache manager written successfully",
				"key", key,
				"strategy", n.Strategy,
				"ttl_seconds", n.TTL)
		}
	}

	// 回退：写入执行上下文（兼容旧逻辑）
	if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
		execCtx.SetVariable("cache_"+key, content)
	}

	// QA拆分处理（任务C）：启用QA拆分且写入成功时执行
	if n.EnableQASplit && n.shouldSplitQA() {
		n.executeQASplit(ctx, userInput, content, messages, key, logger)
	}

	return &NodeOutput{
		Content: content,
		Metadata: map[string]interface{}{
			"operation":      "write",
			"cache_key":      key,
			"ttl":            n.TTL,
			"content_length": len(content),
			"strategy":       n.Strategy,
			"write_storage":  n.WriteStorageName,
			"storage_type":   n.StorageType,
			"written":        n.WriteStorage != nil || n.CacheManager != nil,
		},
	}, nil
}

// shouldSplitQA 检查是否应该执行QA拆分
func (n *CacheNode) shouldSplitQA() bool {
	// QA拆分需要语义缓存才能存储拆分结果
	// 因此只有在策略包含语义匹配时才启用
	return n.Strategy == "semantic" || n.Strategy == "hybrid"
}

// executeQASplit 执行QA拆分并存储到语义缓存
func (n *CacheNode) executeQASplit(ctx context.Context, question, answer string, messages []Message, originalKey string, logger Logger) {
	// 获取QA拆分器（优先使用注入的，否则从缓存管理器获取）
	qaSplitter := n.getQASplitter()
	if qaSplitter == nil || !qaSplitter.IsEnabled() {
		if logger != nil {
			logger.Warn("[CacheNode] QA splitter not available or not enabled")
		}
		return
	}

	if logger != nil {
		logger.Info("[CacheNode] Starting QA split",
			"question", utils.TruncateString(question, 50),
			"answer_length", len(answer))
	}

	// 执行QA拆分
	result, err := qaSplitter.SplitQA(ctx, question, answer)
	if err != nil {
		if logger != nil {
			logger.Warn("[CacheNode] QA split failed", "error", err)
		}
		return
	}

	if !result.Split || len(result.QAPairs) == 0 {
		if logger != nil {
			logger.Info("[CacheNode] QA split returned no pairs or no split needed")
		}
		return
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] QA split completed: %d pairs", len(result.QAPairs)))
	}

	// 获取语义缓存存储
	vectorStore := n.getSemanticCacheStorage()
	if vectorStore == nil {
		if logger != nil {
			logger.Warn("[CacheNode] Semantic cache storage not available for QA split results")
		}
		return
	}

	// 将每个QA对存储到语义缓存
	for i, pair := range result.QAPairs {
		// 生成语义缓存键
		qaKey := fmt.Sprintf("qa:%s:q%d", originalKey, i)

		// 构建语义缓存条目
		semanticEntry := &cache.CacheEntry{
			Key:            qaKey,
			Request:        pair.Question,
			Response:       pair.Answer,
			Timestamp:      time.Now(),
			ExpiresAt:      time.Now().Add(time.Duration(n.TTL) * time.Second),
			StorageBackend: n.WriteStorageName,
			Metadata: map[string]interface{}{
				"original_key":   originalKey,
				"qa_index":       i,
				"qa_split":       true,
				"source":         "qa_splitter",
				"parent_request": question,
			},
		}

		// 序列化并存储到语义缓存（使用语义缓存存储）
		cacheData, err := json.Marshal(semanticEntry)
		if err != nil {
			if logger != nil {
				logger.Error("[CacheNode] Failed to marshal QA semantic entry",
					"index", i, "error", err)
			}
			continue
		}

		ttl := time.Duration(n.TTL) * time.Second
		if err := vectorStore.Set(ctx, qaKey, cacheData, ttl); err != nil {
			if logger != nil {
				logger.Error("[CacheNode] Failed to store QA pair to semantic cache",
					"index", i, "error", err)
			}
		} else {
			if logger != nil {
				logger.Info(fmt.Sprintf("[CacheNode] Stored QA pair %d/%d to semantic cache: key=%s",
					i+1, len(result.QAPairs), qaKey))
			}
		}
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] QA split storage complete: %d pairs stored", len(result.QAPairs)))
	}
}

// getQASplitter 获取QA拆分器
func (n *CacheNode) getQASplitter() *processor.QASplitter {
	// 如果配置了自定义QA拆分服务，创建新的拆分器
	if n.QASplitPrompt != "" && n.QASplitService != "" {
		// TODO: 实现自定义QA拆分器创建
		return nil
	}

	// 使用缓存管理器中的QA拆分器
	if n.CacheManager != nil {
		if splitter := n.CacheManager.GetQASplitter(); splitter != nil {
			return splitter
		}
	}

	return nil
}

// getSemanticCacheStorage 获取语义缓存存储
func (n *CacheNode) getSemanticCacheStorage() storage.KVStore {
	// 优先使用配置的写入存储
	if n.WriteStorage != nil {
		return n.WriteStorage
	}

	// 回退到缓存管理器的语义缓存
	if n.CacheManager != nil {
		return n.CacheManager.GetSemanticCacheStore()
	}

	return nil
}

// extractUserInput 从输入中提取用户消息内容
func extractUserInput(input *NodeInput) string {
	if input == nil {
		return ""
	}
	// 优先从 Messages 中提取用户消息（适用于有对话历史的场景）
	for _, msg := range input.Messages {
		if msg.Role == "user" {
			return msg.Content
		}
	}
	// 如果没有 Messages，使用 input.Content（适用于单轮对话）
	if input.Content != "" {
		return input.Content
	}
	return ""
}

// executeDelete 执行删除操作
func (n *CacheNode) executeDelete(ctx context.Context, key string) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	if logger != nil {
		logger.Info(fmt.Sprintf("[CacheNode] Delete cache key: %s", key))
	}

	return &NodeOutput{
		Content: "",
		Metadata: map[string]interface{}{
			"operation":    "delete",
			"cache_key":    key,
			"strategy":     n.Strategy,
			"storage_type": n.StorageType,
		},
	}, nil
}

// ========== Phase 4: Token Usage Node ==========

// TokenUsageNode Token 计量节点 - 记录和统计 Token 使用
type TokenUsageNode struct {
	BaseNode
	Operation    string                 // 操作类型: record, query, aggregate
	StorageType  string                 // 存储类型: memory, sqlite, postgresql
	RecordFields []string               // 要记录的字段
	Config       map[string]interface{} // 存储特定配置
}

// NewTokenUsageNode 创建 Token 计量节点
func NewTokenUsageNode(config NodeConfig) (PipelineNode, error) {
	node := &TokenUsageNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     5,
			retryConfig: DefaultRetryConfig(),
		},
		Operation:    "record", // 默认记录操作
		StorageType:  "memory", // 默认内存存储
		RecordFields: []string{"prompt_tokens", "completion_tokens", "total_tokens", "model", "backend_id"},
		Config:       make(map[string]interface{}),
	}

	if config.CustomConfig != nil {
		if op, ok := config.CustomConfig["operation"].(string); ok {
			node.Operation = op
		}
		if storageType, ok := config.CustomConfig["storage_type"].(string); ok {
			node.StorageType = storageType
		}
		if fields, ok := config.CustomConfig["record_fields"].([]interface{}); ok {
			node.RecordFields = make([]string, len(fields))
			for i, f := range fields {
				node.RecordFields[i] = f.(string)
			}
		}
		if cfg, ok := config.CustomConfig["config"].(map[string]interface{}); ok {
			node.Config = cfg
		}
	}

	return node, nil
}

func (n *TokenUsageNode) Type() NodeType {
	return NodeTypeTokenUsage
}

func (n *TokenUsageNode) Validate() error {
	validOperations := map[string]bool{"record": true, "query": true, "aggregate": true}
	if !validOperations[n.Operation] {
		return fmt.Errorf("invalid operation: %s, must be record/query/aggregate", n.Operation)
	}

	validStorages := map[string]bool{"memory": true, "sqlite": true, "postgresql": true}
	if !validStorages[n.StorageType] {
		return fmt.Errorf("invalid storage_type: %s, must be memory/sqlite/postgresql", n.StorageType)
	}

	return nil
}

func (n *TokenUsageNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	// 获取执行上下文
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	// 解析模板变量
	if execCtx != nil {
		resolver := NewTemplateVarResolver(input, execCtx)

		// 解析 Operation
		if strings.Contains(n.Operation, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.Operation, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.Operation = s
				}
			}
		}

		// 解析 StorageType
		if strings.Contains(n.StorageType, "{{") {
			if resolved, err := resolver.Resolve(strings.Trim(n.StorageType, "{} ")); err == nil {
				if s, ok := resolved.(string); ok {
					n.StorageType = s
				}
			}
		}
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[TokenUsageNode] Executing %s operation on %s storage",
			n.Operation, n.StorageType))
	}

	switch n.Operation {
	case "record":
		return n.executeRecord(ctx, input)
	case "query":
		return n.executeQuery(ctx, input)
	case "aggregate":
		return n.executeAggregate(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", n.Operation)
	}
}

// executeRecord 执行记录操作
func (n *TokenUsageNode) executeRecord(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	record := make(map[string]interface{})
	mergeTokenUsageFromUpstream(record, input, execCtx)

	// 从当前节点 Metadata 补充/覆盖
	if input.Metadata != nil {
		if model, ok := input.Metadata["model"].(string); ok && model != "" && !strings.Contains(model, "{{") {
			record["model"] = model
		}
		if pt := tokenRecordInt(input.Metadata["prompt_tokens"]); pt > 0 {
			record["prompt_tokens"] = pt
		}
		if ct := tokenRecordInt(input.Metadata["completion_tokens"]); ct > 0 {
			record["completion_tokens"] = ct
		}
		if tt := tokenRecordInt(input.Metadata["total_tokens"]); tt > 0 {
			record["total_tokens"] = tt
		}
		if backendID, ok := input.Metadata["backend_id"].(string); ok && !strings.Contains(backendID, "{{") {
			record["backend_id"] = backendID
		}
		if userID, ok := input.Metadata["user_id"].(string); ok {
			record["user_id"] = userID
		}
		if reqID, ok := input.Metadata["request_id"].(string); ok {
			record["request_id"] = reqID
		}
	}

	// 3. 从 input.Metadata 获取 backend（如果没有 backend_id）
	if record["backend_id"] == nil {
		if backend, ok := input.Metadata["backend"].(string); ok && !strings.Contains(backend, "{{") {
			record["backend_id"] = backend
		}
	}

	// 如果没有 total_tokens，计算一个默认值
	if record["total_tokens"] == nil || record["total_tokens"] == 0 {
		promptTokens := 0
		completionTokens := 0
		if pt, ok := record["prompt_tokens"].(int); ok {
			promptTokens = pt
		}
		if ct, ok := record["completion_tokens"].(int); ok {
			completionTokens = ct
		}
		if promptTokens == 0 && completionTokens == 0 {
			// 估算: 按字符数 / 4
			contentLen := len(input.Content)
			promptTokens = contentLen / 4
		}
		record["total_tokens"] = promptTokens + completionTokens
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[TokenUsageNode] Recorded usage: model=%v, tokens=%v",
			record["model"], record["total_tokens"]))
	}

	persistTokenUsageFromRecord(ctx, input, record)

	return &NodeOutput{
		Content: input.Content,
		Metadata: map[string]interface{}{
			"operation":       "record",
			"recorded_fields": n.RecordFields,
			"usage_record":    record,
			"storage_type":    n.StorageType,
		},
	}, nil
}

func mergeTokenUsageFromUpstream(record map[string]interface{}, input *NodeInput, execCtx *ExecutionContext) {
	if record == nil {
		return
	}

	bestScore := -1
	var bestMeta map[string]interface{}

	applyMeta := func(meta map[string]interface{}) {
		if meta == nil {
			return
		}
		score := tokenUsageMetadataScore(meta)
		if score > bestScore {
			bestScore = score
			bestMeta = meta
		}
	}

	if input != nil && input.Metadata != nil {
		for _, val := range input.Metadata {
			depWrap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			if nested, ok := depWrap["metadata"].(map[string]interface{}); ok {
				applyMeta(nested)
			}
		}
	}

	if input != nil && input.UpstreamResults != nil {
		for _, result := range input.UpstreamResults {
			if result == nil {
				continue
			}
			applyMeta(result.Metadata)
		}
	}

	if execCtx != nil {
		for nodeID, result := range execCtx.results {
			if result == nil {
				continue
			}
			applyMeta(result.Metadata)
			_ = nodeID
		}
		if genResult, ok := execCtx.GetResult("generator"); ok && genResult != nil {
			applyMeta(genResult.Metadata)
		}
	}

	if bestMeta != nil {
		mergeTokenUsageFromNodeMetadata(record, bestMeta)
	}
}

func tokenUsageMetadataScore(meta map[string]interface{}) int {
	if meta == nil {
		return 0
	}
	score := 0
	if tokenRecordString(meta["model"]) != "" {
		score += 4
	}
	if tokenRecordInt(meta["tokens"]) > 0 || tokenRecordInt(meta["total_tokens"]) > 0 {
		score += 8
	}
	if tokenRecordInt(meta["prompt_tokens"]) > 0 {
		score += 2
	}
	if tokenRecordString(meta["backend_id"]) != "" || tokenRecordString(meta["backend"]) != "" {
		score += 1
	}
	return score
}

func mergeTokenUsageFromNodeMetadata(record, meta map[string]interface{}) {
	if record == nil || meta == nil {
		return
	}
	if model := tokenRecordString(meta["model"]); model != "" && !strings.Contains(model, "{{") {
		record["model"] = model
	}
	if backendID := tokenRecordString(meta["backend_id"]); backendID != "" && !strings.Contains(backendID, "{{") {
		record["backend_id"] = backendID
	} else if backend := tokenRecordString(meta["backend"]); backend != "" && !strings.Contains(backend, "{{") {
		record["backend_id"] = backend
	}
	total := tokenRecordInt(meta["tokens"])
	if total == 0 {
		total = tokenRecordInt(meta["total_tokens"])
	}
	if total > 0 {
		record["total_tokens"] = total
	}
	if prompt := tokenRecordInt(meta["prompt_tokens"]); prompt > 0 {
		record["prompt_tokens"] = prompt
	}
	if completion := tokenRecordInt(meta["completion_tokens"]); completion > 0 {
		record["completion_tokens"] = completion
	} else if total > 0 {
		if prompt, ok := record["prompt_tokens"].(int); ok && prompt > 0 && total > prompt {
			record["completion_tokens"] = total - prompt
		}
	}
}

func tokenRecordString(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func tokenRecordInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}

// executeQuery 执行查询操作
func (n *TokenUsageNode) executeQuery(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	queryParams := make(map[string]interface{})
	if input.Metadata != nil {
		if userID, ok := input.Metadata["query_user_id"].(string); ok {
			queryParams["user_id"] = userID
		}
		if model, ok := input.Metadata["query_model"].(string); ok {
			queryParams["model"] = model
		}
		if days, ok := input.Metadata["query_days"].(float64); ok {
			queryParams["days"] = int(days)
		}
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("[TokenUsageNode] Query usage with params: %v", queryParams))
	}

	// 返回模拟查询结果
	return &NodeOutput{
		Content: "",
		Metadata: map[string]interface{}{
			"operation":    "query",
			"query_params": queryParams,
			"result": map[string]interface{}{
				"total_tokens":  0,
				"request_count": 0,
				"avg_tokens":    0.0,
			},
			"storage_type": n.StorageType,
		},
	}, nil
}

// executeAggregate 执行聚合操作
func (n *TokenUsageNode) executeAggregate(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)

	if logger != nil {
		logger.Info("[TokenUsageNode] Aggregate usage statistics")
	}

	return &NodeOutput{
		Content: "",
		Metadata: map[string]interface{}{
			"operation":    "aggregate",
			"aggregates":   []string{"total_tokens", "request_count", "avg_tokens"},
			"storage_type": n.StorageType,
		},
	}, nil
}

// parseNodeConfigFromMap 从 map 解析节点配置
func parseNodeConfigFromMap(m map[string]interface{}) PipelineNodeConfig {
	config := PipelineNodeConfig{}

	if id, ok := m["id"].(string); ok {
		config.ID = id
	}
	if name, ok := m["name"].(string); ok {
		config.Name = name
	}
	if nodeType, ok := m["type"].(string); ok {
		config.Type = NodeType(nodeType)
	}
	if kind, ok := m["kind"].(string); ok {
		config.Kind = kind
	}
	if impl, ok := m["implementation"].(string); ok {
		config.Implementation = impl
	}
	if backend, ok := m["backend"].(string); ok {
		config.Backend = backend
	}
	if model, ok := m["model"].(string); ok {
		config.Model = model
	}
	if timeout, ok := m["timeout"].(int); ok {
		config.Timeout = timeout
	}
	if timeoutFloat, ok := m["timeout"].(float64); ok {
		config.Timeout = int(timeoutFloat)
	}

	return config
}

// ToolCallInjectorNode 工具调用注入节点 - 在Pipeline中注入工具调用指令
type ToolCallInjectorNode struct {
	BaseNode
	injectorConfig ToolCallInjectorConfig
}

// NewToolCallInjectorNode 创建工具调用注入节点
func NewToolCallInjectorNode(config NodeConfig) (PipelineNode, error) {
	node := &ToolCallInjectorNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}

	// 从CustomConfig解析ToolCallInjectorConfig
	if config.CustomConfig != nil {
		configBytes, err := json.Marshal(config.CustomConfig)
		if err != nil {
			return nil, fmt.Errorf("tool_call_injector node: marshal custom_config: %w", err)
		}
		if err := json.Unmarshal(configBytes, &node.injectorConfig); err != nil {
			return nil, fmt.Errorf("tool_call_injector node: unmarshal config: %w", err)
		}
	}

	// 验证配置
	if err := node.Validate(); err != nil {
		return nil, err
	}

	return node, nil
}

// Type 返回节点类型
func (n *ToolCallInjectorNode) Type() NodeType {
	return NodeTypeToolCallInjector
}

// Validate 验证节点配置
func (n *ToolCallInjectorNode) Validate() error {
	if len(n.injectorConfig.ToolCalls) == 0 {
		return fmt.Errorf("tool_call_injector node requires at least one tool_call definition")
	}
	for i, tc := range n.injectorConfig.ToolCalls {
		if tc.ID == "" {
			return fmt.Errorf("tool_call_injector node: tool_call[%d] requires id", i)
		}
		if tc.Function.Name == "" {
			return fmt.Errorf("tool_call_injector node: tool_call[%d] requires function.name", i)
		}
	}
	return nil
}

// Execute 执行工具调用注入
func (n *ToolCallInjectorNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	log := LoggerFromContext(ctx)

	// 1. 检查条件（如果配置了）
	if n.injectorConfig.Condition != "" {
		satisfied, err := n.evaluateCondition(ctx, input)
		if err != nil {
			if log != nil {
				log.Warn(fmt.Sprintf("[ToolCallInjectorNode] Condition evaluation error: %v, skipping", err))
			}
			// 条件评估错误时，跳过注入，返回原始输入
			return &NodeOutput{Content: input.Content}, nil
		}
		if !satisfied {
			if log != nil {
				log.Debug("[ToolCallInjectorNode] Condition not satisfied, skipping")
			}
			return &NodeOutput{Content: input.Content}, nil
		}
	}

	// 2. 解析模板变量并生成tool_calls
	toolCalls, err := n.resolveToolCalls(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("tool_call_injector node %q: resolve tool calls: %w", n.id, err)
	}

	if log != nil {
		log.Info(fmt.Sprintf("[ToolCallInjectorNode] Injecting %d tool calls", len(toolCalls)))
	}

	// 3. 返回结果，包含tool_calls
	return &NodeOutput{
		Content:   input.Content,
		ToolCalls: toolCalls,
		Metadata: map[string]interface{}{
			"node_type":      "tool_call_injector",
			"injected_count": len(toolCalls),
			"tool_call_ids":  n.extractToolCallIDs(toolCalls),
		},
	}, nil
}

// evaluateCondition 评估条件表达式
// 支持简单的布尔判断，如: "{{node.reviewer.score}} < 0.8"
func (n *ToolCallInjectorNode) evaluateCondition(ctx context.Context, input *NodeInput) (bool, error) {
	condition := n.injectorConfig.Condition
	if condition == "" {
		return true, nil
	}

	// 直接布尔值判断
	condition = strings.TrimSpace(condition)
	if condition == "true" || condition == "True" || condition == "TRUE" {
		return true, nil
	}
	if condition == "false" || condition == "False" || condition == "FALSE" {
		return false, nil
	}

	// 获取执行上下文
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)

	// 解析模板变量：提取所有 {{...}} 模式，逐一resolve后替换
	resolver := NewTemplateVarResolver(input, execCtx)
	resolvedStr := condition
	re := regexp.MustCompile(`\{\{(.+?)\}\}`)
	matches := re.FindAllStringSubmatchIndex(condition, -1)
	// 从后往前替换，避免索引偏移
	resolvedParts := make([]struct {
		start, end int
		value      string
	}, 0)
	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]
		path := strings.TrimSpace(condition[innerStart:innerEnd])
		val, err := resolver.Resolve(path)
		if err != nil {
			return false, fmt.Errorf("resolve condition: %w", err)
		}
		resolvedParts = append(resolvedParts, struct {
			start, end int
			value      string
		}{fullStart, fullEnd, fmt.Sprintf("%v", val)})
	}
	// 从后往前替换
	for i := len(resolvedParts) - 1; i >= 0; i-- {
		p := resolvedParts[i]
		resolvedStr = resolvedStr[:p.start] + p.value + resolvedStr[p.end:]
	}
	resolvedStr = strings.TrimSpace(resolvedStr)

	// 简单的布尔判断
	// 支持格式: "true", "false", "value == value", "value < value", "value > value"

	// 直接布尔值
	if resolvedStr == "true" || resolvedStr == "True" || resolvedStr == "TRUE" {
		return true, nil
	}
	if resolvedStr == "false" || resolvedStr == "False" || resolvedStr == "FALSE" {
		return false, nil
	}

	// 比较操作
	if strings.Contains(resolvedStr, "==") {
		parts := strings.SplitN(resolvedStr, "==", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			// 移除引号
			right = strings.Trim(right, "\"'")
			return left == right, nil
		}
	}

	if strings.Contains(resolvedStr, "!=") {
		parts := strings.SplitN(resolvedStr, "!=", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			right = strings.Trim(right, "\"'")
			return left != right, nil
		}
	}

	if strings.Contains(resolvedStr, "<") {
		parts := strings.SplitN(resolvedStr, "<", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			leftVal, err1 := strconv.ParseFloat(left, 64)
			rightVal, err2 := strconv.ParseFloat(right, 64)
			if err1 == nil && err2 == nil {
				return leftVal < rightVal, nil
			}
		}
	}

	if strings.Contains(resolvedStr, ">") {
		parts := strings.SplitN(resolvedStr, ">", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			leftVal, err1 := strconv.ParseFloat(left, 64)
			rightVal, err2 := strconv.ParseFloat(right, 64)
			if err1 == nil && err2 == nil {
				return leftVal > rightVal, nil
			}
		}
	}

	// 非空字符串视为true
	return resolvedStr != "", nil
}

// resolveToolCalls 解析工具调用定义中的模板变量
func (n *ToolCallInjectorNode) resolveToolCalls(ctx context.Context, input *NodeInput) ([]ToolCall, error) {
	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	resolver := NewTemplateVarResolver(input, execCtx)

	var toolCalls []ToolCall

	for _, tcDef := range n.injectorConfig.ToolCalls {
		// 解析函数参数中的模板变量
		// 如果解析失败，直接使用原始字符串（可能是纯JSON）
		resolvedArgs, err := resolver.Resolve(tcDef.Function.Arguments)
		if err != nil {
			// 解析失败，使用原始字符串
			resolvedArgs = tcDef.Function.Arguments
		}

		// 将resolvedArgs转换为字符串
		argsStr := fmt.Sprintf("%v", resolvedArgs)

		// 生成唯一的工具调用ID
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		toolCallID := fmt.Sprintf("pipeline_%s_%d_%s", tcDef.ID, time.Now().UnixNano(), hex.EncodeToString(b))

		toolCall := ToolCall{
			ID:   toolCallID,
			Type: "function",
			Function: FunctionCall{
				Name:      tcDef.Function.Name,
				Arguments: argsStr,
			},
		}

		toolCalls = append(toolCalls, toolCall)
	}

	return toolCalls, nil
}

// extractToolCallIDs 提取工具调用ID列表
func (n *ToolCallInjectorNode) extractToolCallIDs(toolCalls []ToolCall) []string {
	ids := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		ids[i] = tc.ID
	}
	return ids
}

// ============================================================================
// QuestionSplitterNode 问题拆分节点（内置 fallback）
// ============================================================================

// QuestionSplitterNode 问题拆分节点
// 内置 fallback 实现：直接透传原始问题，不做拆分
// 当 business.question_splitter 插件未注册时使用
type QuestionSplitterNode struct {
	BaseNode
}

// NewQuestionSplitterNode 创建问题拆分节点
func NewQuestionSplitterNode(config NodeConfig) (PipelineNode, error) {
	node := &QuestionSplitterNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}
	if err := node.Validate(); err != nil {
		return nil, err
	}
	return node, nil
}

func (n *QuestionSplitterNode) Validate() error { return nil }

func (n *QuestionSplitterNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	question := input.Content
	if question == "" {
		if msgs := input.Messages; len(msgs) > 0 {
			for _, m := range msgs {
				if m.Role == "user" {
					question = m.Content
					break
				}
			}
		}
	}

	// 空输入：跳过拆分并透传，不阻塞流水线（cache-pipeline 对该节点
	// 未配置 bypass_on_error，硬报错会导致空输入请求整体失败）
	if question == "" || strings.TrimSpace(question) == "" {
		return &NodeOutput{
			Content: input.Content,
			Metadata: map[string]interface{}{
				"should_split":      false,
				"complexity_score":  0.0,
				"question_type":     "simple",
				"strategy":          "passthrough",
				"sub_questions":     []interface{}{},
				"original_question": "",
				"skipped":           true,
				"reason":            "empty_input",
				"builtin_fallback":  true,
			},
		}, nil
	}

	// 内置 fallback：不拆分，直接透传原始问题
	// 返回 should_split=false，让后续节点知道不需要处理子问题
	output := &NodeOutput{
		Content: question,
		Metadata: map[string]interface{}{
			"should_split":       false,
			"complexity_score":   0.0,
			"question_type":      "simple",
			"strategy":           "passthrough",
			"sub_questions":      []interface{}{},
			"original_question":  question,
			"builtin_fallback":   true,
		},
	}

	return output, nil
}

// ============================================================================
// AnswerSynthesizerNode 答案合成节点（内置 fallback）
// ============================================================================

// AnswerSynthesizerNode 答案合成节点
// 内置 fallback 实现：直接返回第一个子答案或原始内容
// 当 business.answer_synthesizer 插件未注册时使用
type AnswerSynthesizerNode struct {
	BaseNode
}

// NewAnswerSynthesizerNode 创建答案合成节点
func NewAnswerSynthesizerNode(config NodeConfig) (PipelineNode, error) {
	node := &AnswerSynthesizerNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}
	if err := node.Validate(); err != nil {
		return nil, err
	}
	return node, nil
}

func (n *AnswerSynthesizerNode) Validate() error { return nil }

func (n *AnswerSynthesizerNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	content := input.Content

	// 尝试从 metadata 中获取子答案
	if input.Metadata != nil {
		if subAnswers, ok := input.Metadata["sub_answers"]; ok {
			if subAnswersList, ok := subAnswers.([]interface{}); ok && len(subAnswersList) > 0 {
				// 内置 fallback：直接返回第一个子答案
				if firstAnswer, ok := subAnswersList[0].(map[string]interface{}); ok {
					if answer, ok := firstAnswer["answer"].(string); ok && answer != "" {
						content = answer
					}
				}
			}
		}
	}

	output := &NodeOutput{
		Content: content,
		Metadata: map[string]interface{}{
			"synthesis_strategy": "builtin_passthrough",
			"sub_answer_count":   0,
			"builtin_fallback":   true,
		},
	}

	return output, nil
}
