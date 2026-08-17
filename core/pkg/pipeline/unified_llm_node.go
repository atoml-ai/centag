package pipeline

import (
	"centag/core/pkg/pipeline/promptstrategy"
	"context"
	"fmt"
	"strings"
)

// UnifiedLLMNode 统一的LLM调用节点
// 合并 GeneratorNode 和 ProcessorNode，支持多种操作类型
type UnifiedLLMNode struct {
	BaseNode
	// Operation 操作类型：generate / optimize / translate / summarize
	Operation string

	// SystemPrompt system prompt配置
	SystemPrompt string
	// SystemPromptStrategy system prompt 策略（passthrough/append/replace）
	SystemPromptStrategy promptstrategy.SystemMode
	// AppendPosition append 模式下的插入位置
	AppendPosition promptstrategy.AppendPosition

	// Temperature 温度参数
	Temperature float64
	// MaxTokens 最大token数
	MaxTokens int

	// TargetLang 翻译目标语言（仅translate操作）
	TargetLang string
}

// NewUnifiedLLMNode 创建统一LLM调用节点
func NewUnifiedLLMNode(config NodeConfig) (PipelineNode, error) {
	node := &UnifiedLLMNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     60,
			retryConfig: DefaultRetryConfig(),
		},
		Operation:  "generate",
		Temperature: 0.7,
		MaxTokens:   2048,
	}

	// 从配置中读取操作类型
	if config.CustomConfig != nil {
		if op, ok := config.CustomConfig["operation"].(string); ok {
			node.Operation = op
		}
		if lang, ok := config.CustomConfig["target_lang"].(string); ok {
			node.TargetLang = lang
		}
	}

	// 从配置中读取参数
	if config.Temperature != nil {
		node.Temperature = *config.Temperature
	}
	if config.MaxTokens != nil {
		node.MaxTokens = *config.MaxTokens
	}
	if config.SystemPrompt != "" {
		node.SystemPrompt = config.SystemPrompt
	}

	// 解析 system_prompt_strategy
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

// Type 返回节点类型
func (n *UnifiedLLMNode) Type() NodeType {
	return NodeTypeGenerator // 保持向后兼容
}

// Validate 验证节点配置
func (n *UnifiedLLMNode) Validate() error {
	if n.config.Backend == "" {
		return fmt.Errorf("unified LLM node requires backend")
	}
	if n.config.Model == "" {
		return fmt.Errorf("unified LLM node requires model")
	}

	// 验证操作类型
	validOperations := map[string]bool{
		"generate":  true,
		"optimize":  true,
		"translate": true,
		"summarize": true,
	}
	if !validOperations[n.Operation] {
		return fmt.Errorf("invalid operation type: %s", n.Operation)
	}

	// translate操作需要目标语言
	if n.Operation == "translate" && n.TargetLang == "" {
		return fmt.Errorf("translate operation requires target_lang")
	}

	return nil
}

// Execute 执行节点操作
func (n *UnifiedLLMNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	switch n.Operation {
	case "generate":
		return n.executeGenerate(ctx, input)
	case "optimize":
		return n.executeOptimize(ctx, input)
	case "translate":
		return n.executeTranslate(ctx, input)
	case "summarize":
		return n.executeSummarize(ctx, input)
	default:
		return n.executeGenerate(ctx, input)
	}
}

// executeGenerate 执行生成操作
func (n *UnifiedLLMNode) executeGenerate(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// 复用GeneratorNode的逻辑
	generatorNode := &GeneratorNode{
		BaseNode:             n.BaseNode,
		Temperature:          n.Temperature,
		MaxTokens:            n.MaxTokens,
		SystemPrompt:         n.SystemPrompt,
		SystemPromptStrategy: n.SystemPromptStrategy,
		AppendPosition:       n.AppendPosition,
	}
	return generatorNode.Execute(ctx, input)
}

// executeOptimize 执行优化操作
func (n *UnifiedLLMNode) executeOptimize(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// 复用ProcessorNode的逻辑
	processorNode := &ProcessorNode{
		BaseNode:  n.BaseNode,
		Operation: "optimize",
	}
	return processorNode.Execute(ctx, input)
}

// executeTranslate 执行翻译操作
func (n *UnifiedLLMNode) executeTranslate(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// 复用ProcessorNode的逻辑
	processorNode := &ProcessorNode{
		BaseNode:  n.BaseNode,
		Operation: "translate",
		TargetLang: n.TargetLang,
	}
	return processorNode.Execute(ctx, input)
}

// executeSummarize 执行摘要操作
func (n *UnifiedLLMNode) executeSummarize(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	// 复用ProcessorNode的逻辑
	processorNode := &ProcessorNode{
		BaseNode:  n.BaseNode,
		Operation: "summarize",
	}
	return processorNode.Execute(ctx, input)
}

// GetOperationName 获取操作名称
func (n *UnifiedLLMNode) GetOperationName() string {
	return n.Operation
}

// SetOperation 设置操作类型
func (n *UnifiedLLMNode) SetOperation(operation string) {
	n.Operation = operation
}

// SetTargetLang 设置翻译目标语言
func (n *UnifiedLLMNode) SetTargetLang(lang string) {
	n.TargetLang = lang
}

// IsDeprecated 是否已废弃（用于兼容性检查）
func (n *UnifiedLLMNode) IsDeprecated() bool {
	return false
}

// GetDeprecatedReason 获取废弃原因
func (n *UnifiedLLMNode) GetDeprecatedReason() string {
	return ""
}

// GetReplacementNode 获取替代节点类型
func (n *UnifiedLLMNode) GetReplacementNode() string {
	return ""
}