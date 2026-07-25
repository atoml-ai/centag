package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"centag/core/pkg/pipeline/promptstrategy"
	"centag/core/pkg/utils"
)

// UserPromptOpsConfig 定义 user_prompt_ops 节点的配置
type UserPromptOpsConfig struct {
	// Check 检查配置
	Check CheckConfig `json:"check"`
	// Optimize 优化配置
	Optimize OptimizeConfig `json:"optimize"`
	// LLM Phase B 占位（Phase A 忽略）
	LLM *LLMPlaceholderConfig `json:"llm,omitempty"`
}

// CheckConfig 检查配置
type CheckConfig struct {
	Enabled      bool     `json:"enabled"`
	DenyPatterns []string `json:"deny_patterns,omitempty"`
	// KeywordListRef 关键词列表引用（Phase A 可忽略）
	KeywordListRef string `json:"keyword_list_ref,omitempty"`
	// OnHit 命中处置: log | redact | block
	OnHit string `json:"on_hit,omitempty"`
}

// OptimizeConfig 优化配置
type OptimizeConfig struct {
	Enabled            bool `json:"enabled"`
	MaxUserChars       int  `json:"max_user_chars,omitempty"`
	CollapseWhitespace bool `json:"collapse_whitespace,omitempty"`
}

// LLMPlaceholderConfig LLM 占位配置（Phase A）
type LLMPlaceholderConfig struct {
	Enabled    bool   `json:"enabled"`
	Capability string `json:"capability,omitempty"`
	Backend    string `json:"backend,omitempty"`
	Model      string `json:"model,omitempty"`
	Mode       string `json:"mode,omitempty"`
}

// UserPromptOpsNode 用户 Prompt 操作节点 - 入站检查与优化
type UserPromptOpsNode struct {
	BaseNode
	opsConfig UserPromptOpsConfig
	denyRes   []*regexp.Regexp
}

// NewUserPromptOpsNode 创建用户 Prompt 操作节点
func NewUserPromptOpsNode(config NodeConfig) (PipelineNode, error) {
	node := &UserPromptOpsNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}

	// 解析配置
	if config.CustomConfig != nil {
		configBytes, err := json.Marshal(config.CustomConfig)
		if err != nil {
			return nil, fmt.Errorf("user_prompt_ops node: marshal custom_config: %w", err)
		}
		if err := json.Unmarshal(configBytes, &node.opsConfig); err != nil {
			return nil, fmt.Errorf("user_prompt_ops node: unmarshal config: %w", err)
		}
	}

	// 预编译正则
	for _, pattern := range node.opsConfig.Check.DenyPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("user_prompt_ops node: invalid deny_pattern %q: %w", pattern, err)
		}
		node.denyRes = append(node.denyRes, re)
	}

	return node, nil
}

// Type 返回节点类型
func (n *UserPromptOpsNode) Type() NodeType {
	return NodeTypeUserPromptOps
}

// Validate 验证节点配置
func (n *UserPromptOpsNode) Validate() error {
	// 配置可选，无强制校验
	return nil
}

// Execute 执行用户 Prompt 操作
func (n *UserPromptOpsNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	log := LoggerFromContext(ctx)

	// 获取原始请求体
	rawBody := n.getRawBody(ctx, input)
	if rawBody == nil {
		// 无 raw body，仅处理 Content
		return n.processContent(ctx, input, nil, log)
	}

	// 解析 chat body 中的 messages
	messages, err := promptstrategy.ParseChatBody(rawBody)
	if err != nil {
		if log != nil {
			log.Debug("[UserPromptOpsNode] failed to parse chat body, processing content only", "error", err)
		}
		return n.processContent(ctx, input, nil, log)
	}

	// 处理 messages
	result, blockErr := n.processMessages(messages, log)
	if blockErr != nil {
		return nil, blockErr
	}

	// 同步 raw body
	syncedBody, err := promptstrategy.SyncMessagesToRawBody(rawBody, result)
	if err != nil {
		if log != nil {
			log.Warn("[UserPromptOpsNode] failed to sync messages to raw body", "error", err)
		}
	}

	// 同步 metadata
	metadata := make(map[string]interface{})
	if input.Metadata != nil {
		for k, v := range input.Metadata {
			metadata[k] = v
		}
	}
	if syncedBody != nil {
		rawStr := string(syncedBody)
		metadata["raw_request_body"] = rawStr
		// 同步到执行上下文，确保下游 transparent_forward 读到改写后的 body
		n.promoteRawBodyToExecCtx(ctx, rawStr)
	}
	metadata["node_type"] = "user_prompt_ops"

	// 获取最后一条 user 消息作为 content
	content := n.extractLastUserContent(result)

	return &NodeOutput{
		Content:  content,
		Messages: n.toPipelineMessages(result),
		Metadata: metadata,
	}, nil
}

// processContent 无 raw body 时处理 Content 字段
func (n *UserPromptOpsNode) processContent(ctx context.Context, input *NodeInput, _ []promptstrategy.Message, log Logger) (*NodeOutput, error) {
	content := input.Content
	if content == "" {
		return &NodeOutput{Content: content}, nil
	}

	// 检查
	if n.opsConfig.Check.Enabled {
		hitAction := n.checkText(content)
		if hitAction != "" {
			return n.handleHit(ctx, hitAction, content, log)
		}
	}

	// 优化
	if n.opsConfig.Optimize.Enabled {
		content = n.optimizeText(content)
	}

	return &NodeOutput{Content: content}, nil
}

// processMessages 处理消息列表
func (n *UserPromptOpsNode) processMessages(messages []promptstrategy.Message, log Logger) ([]promptstrategy.Message, error) {
	// 定位最后一条 user 消息
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "user") {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx < 0 {
		return messages, nil
	}

	text := messages[lastUserIdx].Content

	// 检查：与 processContent / handleHit 对齐（log / redact / block）
	if n.opsConfig.Check.Enabled {
		hitAction := n.checkText(text)
		if hitAction != "" {
			switch hitAction {
			case "block":
				return nil, n.handleHitError(hitAction, text)
			case "redact":
				n.logHit(log, "redact", text)
				messages[lastUserIdx].Content = "[REDACTED]"
				text = messages[lastUserIdx].Content
			default: // log：仅记录，继续后续优化
				n.logHit(log, "log", text)
			}
		}
	}

	// 优化
	if n.opsConfig.Optimize.Enabled {
		messages[lastUserIdx].Content = n.optimizeText(text)
	}

	return messages, nil
}

// checkText 检查文本，返回命中的 action（空表示未命中）
func (n *UserPromptOpsNode) checkText(text string) string {
	action := n.opsConfig.Check.OnHit
	if action == "" {
		action = "log" // 默认仅记录
	}

	// 检查 deny patterns
	for _, re := range n.denyRes {
		if re.MatchString(text) {
			return action
		}
	}

	// 检查密钥形态启发式
	if looksLikeSecretKey(text) {
		return action
	}

	return ""
}

// looksLikeSecretKey 启发式检测是否包含密钥
func looksLikeSecretKey(text string) bool {
	lower := strings.ToLower(text)
	// 常见密钥前缀
	prefixes := []string{
		"sk-", "ak_", "ak-", "secret_", "bearer ",
		"-----begin", "api_key=",
	}
	for _, prefix := range prefixes {
		if strings.Contains(lower, prefix) {
			return true
		}
	}
	return false
}

// optimizeText 优化文本
func (n *UserPromptOpsNode) optimizeText(text string) string {
	// 空白折叠
	if n.opsConfig.Optimize.CollapseWhitespace {
		text = collapseWhitespace(text)
	}

	// 最大长度截断
	if n.opsConfig.Optimize.MaxUserChars > 0 {
		text = utils.TruncateString(text, n.opsConfig.Optimize.MaxUserChars)
	}

	return text
}

// collapseWhitespace 折叠连续空白字符
func collapseWhitespace(text string) string {
	// 替换连续空白为单个空格，保留换行
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}

// handleHit 处理命中（返回 NodeOutput）
func (n *UserPromptOpsNode) handleHit(ctx context.Context, action, text string, log Logger) (*NodeOutput, error) {
	switch action {
	case "block":
		return nil, n.handleHitError(action, text)
	case "redact":
		n.logHit(log, "redact", text)
		// redact 时替换文本为占位符
		return &NodeOutput{
			Content:  "[REDACTED]",
			Metadata: map[string]interface{}{"node_type": "user_prompt_ops", "action": "redact"},
		}, nil
	default: // log
		n.logHit(log, "log", text)
		return &NodeOutput{Content: text}, nil
	}
}

// handleHitError 处理命中错误（block 模式）
func (n *UserPromptOpsNode) handleHitError(action, text string) error {
	_ = text // 错误信息不含原文，避免密钥泄漏
	return fmt.Errorf("prompt_strategy_blocked: %s (detected pattern in user prompt)", action)
}

func (n *UserPromptOpsNode) logHit(log Logger, action, text string) {
	if log == nil {
		return
	}
	log.Info("[UserPromptOpsNode] deny pattern hit",
		"action", action,
		"preview", redactPreview(text),
	)
}

// redactPreview 脱敏预览（先 Mask 再截断）
func redactPreview(text string) string {
	masked := MaskSensitiveData(text)
	return utils.TruncateString(masked, 80)
}

// promoteRawBodyToExecCtx 将改写后的 raw_request_body 写回执行上下文 metadata
func (n *UserPromptOpsNode) promoteRawBodyToExecCtx(ctx context.Context, rawStr string) {
	execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if !ok || execCtx == nil {
		return
	}
	execCtx.SetVariable("raw_request_body", rawStr)
	if meta, ok := execCtx.GetVariable("metadata"); ok {
		if m, ok := meta.(map[string]interface{}); ok && m != nil {
			m["raw_request_body"] = rawStr
		}
	}
}

// getRawBody 从 input 或 execution context 获取 raw_request_body
func (n *UserPromptOpsNode) getRawBody(ctx context.Context, input *NodeInput) []byte {
	// 从 input metadata 获取
	if input.Metadata != nil {
		if raw, ok := input.Metadata["raw_request_body"].(string); ok && raw != "" {
			return []byte(raw)
		}
	}

	// 从 execution context 获取
	if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
		if raw, ok := execCtx.GetVariable("raw_request_body"); ok {
			if rawStr, ok := raw.(string); ok && rawStr != "" {
				return []byte(rawStr)
			}
		}
	}

	return nil
}

// extractLastUserContent 提取最后一条 user 消息的 content
func (n *UserPromptOpsNode) extractLastUserContent(messages []promptstrategy.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "user") {
			return messages[i].Content
		}
	}
	return ""
}

// toPipelineMessages 转换为 pipeline.Message
func (n *UserPromptOpsNode) toPipelineMessages(messages []promptstrategy.Message) []Message {
	result := make([]Message, len(messages))
	for i, m := range messages {
		result[i] = Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}
