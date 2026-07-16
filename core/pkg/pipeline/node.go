package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"centag/core/internal/cache"
	"centag/core/pkg/logger"
	"centag/core/pkg/plugin"
	"centag/core/pkg/utils"
)

// NodeType 节点类型
type NodeType string

const (
	NodeTypeGenerator        NodeType = "generator"
	NodeTypeProcessor        NodeType = "processor"
	NodeTypeReviewer         NodeType = "reviewer"
	NodeTypeRouter           NodeType = "router"
	NodeTypeAggregator       NodeType = "aggregator"
	NodeTypeMemory           NodeType = "memory"
	NodeTypeAudit            NodeType = "audit"
	NodeTypeOptimize         NodeType = "optimize"
	NodeTypeLoopController   NodeType = "loop_controller"   // 循环控制器节点
	NodeTypeCache            NodeType = "cache"             // Phase 4: 缓存节点
	NodeTypeTokenUsage       NodeType = "token_usage"       // Phase 4: Token 计量节点
	NodeTypeScheduler        NodeType = "scheduler"         // 智能调度节点
	NodeTypeTransparentForward NodeType = "transparent_forward" // 透明 HTTP 转发节点
	NodeTypeToolCallInjector NodeType = "tool_call_injector" // 工具调用注入节点
)

func (nt NodeType) String() string {
	return string(nt)
}

func (nt NodeType) IsValid() bool {
	switch nt {
	case NodeTypeGenerator, NodeTypeProcessor, NodeTypeReviewer,
		NodeTypeRouter, NodeTypeAggregator,
		NodeTypeMemory, NodeTypeAudit, NodeTypeOptimize, NodeTypeLoopController,
		NodeTypeCache, NodeTypeTokenUsage, NodeTypeScheduler, NodeTypeTransparentForward,
		NodeTypeToolCallInjector:
		return true
	}
	return false
}

// PipelineNode 流水线节点接口
type PipelineNode interface {
	Type() NodeType
	ID() string
	Name() string
	Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error)
	GetConfig() NodeConfig
	Validate() error
	GetTimeout() int
	GetRetryConfig() *RetryConfig
	SetTimeout(timeout int)
	SetRetryConfig(retryConfig *RetryConfig)
}

// BaseNode 节点基础结构体
type BaseNode struct {
	id               string
	name             string
	nodeType         NodeType
	config           NodeConfig
	timeout          int
	retryConfig      *RetryConfig
	capabilityBroker CapabilityBroker
	permissions      []string
}

func (n *BaseNode) ID() string                   { return n.id }
func (n *BaseNode) Name() string                 { return n.name }
func (n *BaseNode) Type() NodeType               { return n.nodeType }
func (n *BaseNode) GetConfig() NodeConfig        { return n.config }
func (n *BaseNode) GetTimeout() int              { return n.timeout }
func (n *BaseNode) GetRetryConfig() *RetryConfig { return n.retryConfig }

// SetCapabilityBroker 设置能力代理
func (n *BaseNode) SetCapabilityBroker(broker CapabilityBroker) {
	n.capabilityBroker = broker
}

// GetCapabilityBroker 获取能力代理
func (n *BaseNode) GetCapabilityBroker() CapabilityBroker {
	return n.capabilityBroker
}

// SetPermissions 设置节点权限
func (n *BaseNode) SetPermissions(perms []string) {
	n.permissions = perms
}

// GetPermissions 获取节点权限
func (n *BaseNode) GetPermissions() []string {
	if len(n.permissions) > 0 {
		return n.permissions
	}
	return n.config.Permissions
}

// resolveLLMPermissions 计算调用 LLM 所需的权限列表：
//   - 若节点已显式配置 permissions，则使用之
//   - 否则按 "llm.call:<backend>:<model>" 构造（推荐：精确权限可让 CapabilityBroker
//     做出更好的后端选择 / 隔离决策）
//   - 后备：纯 "llm.call"（让 broker 自行选择默认后端）
func (n *BaseNode) resolveLLMPermissions() []string {
	if perms := n.GetPermissions(); len(perms) > 0 {
		return perms
	}
	if n.config.Backend != "" && n.config.Model != "" {
		return []string{fmt.Sprintf("llm.call:%s:%s", n.config.Backend, n.config.Model)}
	}
	return []string{"llm.call"}
}

// CallLLM 通过 CapabilityBroker 发起一次 LLM 调用（非流式）。
// 封装所有节点共用的样板逻辑：
//   - 检查 broker 是否可用
//   - 解析/构造 permissions（含 backend/model 推断）
//   - 获取 LLMClient 并执行 Chat
//
// 错误信息会自动附带 node_id 与 kind 便于定位。
// 返回的 error 涵盖 broker 不可用、client 获取失败、Chat 失败等所有场景，
// 由 caller 决定是 fallback 还是直接终止 pipeline。
func (n *BaseNode) CallLLM(ctx context.Context, kind string, req *LLMRequest) (*LLMResponse, error) {
	broker := n.capabilityBroker
	if broker == nil {
		return nil, fmt.Errorf("%s node %q: capability broker not available", kind, n.id)
	}
	perms := n.resolveLLMPermissions()
	client, err := broker.GetLLMClient(ctx, perms)
	if err != nil {
		return nil, fmt.Errorf("%s node %q: get llm client: %w", kind, n.id, err)
	}
	if client == nil {
		return nil, fmt.Errorf("%s node %q: llm client is nil", kind, n.id)
	}
	if req == nil {
		return nil, fmt.Errorf("%s node %q: nil llm request", kind, n.id)
	}

	log := LoggerFromContext(ctx)
	backendID := n.config.Backend
	model := req.Model
	if model == "" {
		model = n.config.Model
	}
	if log != nil {
		reqFields := AppendRequestIDFields(ctx,
			"node_id", n.id,
			"kind", kind,
			"backend_id", backendID,
			"model", model,
			"message_count", len(req.Messages),
			"messages_preview", MaskSensitiveData(FormatMessagesPreview(req.Messages, defaultMessagesPreviewMax)),
			"temperature", req.Temperature,
			"max_tokens", req.MaxTokens,
			"stream", req.Stream,
		)
		log.Info("[backend] request", reqFields...)
	}

	started := time.Now()
	resp, err := client.Chat(ctx, req)
	if log != nil {
		respFields := AppendRequestIDFields(ctx,
			"node_id", n.id,
			"kind", kind,
			"backend_id", backendID,
			"model", model,
			"duration_ms", time.Since(started).Milliseconds(),
		)
		if err != nil {
			log.Error("[backend] response", append(respFields, "error", err.Error())...)
		} else if resp != nil {
			log.Info("[backend] response", append(respFields,
				"response_model", resp.Model,
				"tokens", resp.TokenUsage,
				"response_preview", MaskSensitiveData(utils.TruncateString(resp.Content, defaultMessagesPreviewMax)),
			)...)
		}
	}
	return resp, err
}

// CallLLMStream 通过 CapabilityBroker 发起一次流式 LLM 调用。
// 返回 chunk channel，caller 需完整消费 channel 直到关闭。
func (n *BaseNode) CallLLMStream(ctx context.Context, kind string, req *LLMRequest) (<-chan plugin.StreamChunk, error) {
	broker := n.capabilityBroker
	if broker == nil {
		return nil, fmt.Errorf("%s node %q: capability broker not available", kind, n.id)
	}
	perms := n.resolveLLMPermissions()
	client, err := broker.GetLLMStreamClient(ctx, perms)
	if err != nil {
		return nil, fmt.Errorf("%s node %q: get llm stream client: %w", kind, n.id, err)
	}
	if client == nil {
		return nil, fmt.Errorf("%s node %q: llm stream client is nil", kind, n.id)
	}
	if req == nil {
		return nil, fmt.Errorf("%s node %q: nil llm request", kind, n.id)
	}
	return client.ChatStream(ctx, req)
}

// LoggerFromContext 从 context 中安全提取 Logger。
// 提取失败时返回 nil（调用方可自行 nil-check 而无需重复处理 error）。
// 适用于所有节点 Execute 入口的 "logger, _ := ctx.Value(...)" 模式。
func LoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return nil
	}
	if l, ok := ctx.Value(loggerContextKey{}).(Logger); ok {
		return l
	}
	return nil
}

// BaseNode 不实现 Execute 和 Validate，由具体节点类型实现
func (n *BaseNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	return nil, fmt.Errorf("BaseNode does not implement Execute")
}

func (n *BaseNode) Validate() error {
	return fmt.Errorf("BaseNode does not implement Validate")
}

func (n *BaseNode) SetID(id string)                         { n.id = id }
func (n *BaseNode) SetName(name string)                     { n.name = name }
func (n *BaseNode) SetType(nodeType NodeType)               { n.nodeType = nodeType }
func (n *BaseNode) SetConfig(config NodeConfig)             { n.config = config }
func (n *BaseNode) SetTimeout(timeout int)                  { n.timeout = timeout }
func (n *BaseNode) SetRetryConfig(retryConfig *RetryConfig) { n.retryConfig = retryConfig }

// NodeInput 节点输入数据
type NodeInput struct {
	Content  string                 `json:"content"`
	Messages []Message              `json:"messages,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
	// UpstreamResults 保存所有已执行上游节点的完整输出，供 template_vars 路径解析使用
	UpstreamResults map[string]*NodeOutput `json:"upstream_results,omitempty"`
	// Tools 客户端请求中的工具定义（透传给后端，支持 function calling）
	Tools any `json:"tools,omitempty"`
	// ToolChoice 工具选择策略（透传给后端）
	ToolChoice any `json:"tool_choice,omitempty"`
}

// Message 对话消息
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

// ToolCall 工具调用（与 plugin.ToolCall 保持兼容）
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// NodeOutput 节点输出数据
type NodeOutput struct {
	Content     string                 `json:"content"`
	Messages    []Message              `json:"messages,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Passed      *bool                  `json:"passed,omitempty"`
	Score       *float64               `json:"score,omitempty"`
	Feedback    string                 `json:"feedback,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	// ToolCalls LLM 返回的工具调用（function calling）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// FinishReason LLM 完成原因（如 tool_calls/stop）
	FinishReason string `json:"finish_reason,omitempty"`
	// ReasoningContent 推理内容（DeepSeek thinking 模式等，需回传给后端）
	ReasoningContent string `json:"reasoning_content,omitempty"`
	// StreamData 流式数据块（用于流式响应缓存）
	IsStream   bool                `json:"is_stream,omitempty"`
	StreamData []cache.StreamChunk `json:"stream_data,omitempty"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	Backend        string                 `json:"backend" yaml:"backend"`
	Model          string                 `json:"model" yaml:"model"`
	PromptTemplate string                 `json:"prompt_template,omitempty" yaml:"prompt_template,omitempty"`
	SystemPrompt   string                 `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	Temperature    *float64               `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens      *int                   `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty" yaml:"custom_config,omitempty"`
	// TemplateVars 自定义模板变量绑定：key=变量名，value=数据路径表达式
	// 支持: input.content / node.<id>.content / node.<id>.metadata.<key>
	//       context.timestamp / context.user_id / context.session_id / literal:<value>
	TemplateVars  map[string]string `json:"template_vars,omitempty" yaml:"template_vars,omitempty"`
	MaxInputBytes int64             `json:"max_input_bytes,omitempty" yaml:"max_input_bytes,omitempty"`
	// SecretsRef 密钥引用，格式: "secret_key" 或 "vault:secret_name"
	// 在模板中通过 {{ .secrets.secret_key }} 访问
	SecretsRef []string `json:"secrets_ref,omitempty" yaml:"secrets_ref,omitempty"`
	// Permissions 节点所需的权限列表
	// 示例: ["llm.call", "secrets.read", "network.outbound"]
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts     int    `json:"max_attempts" yaml:"max_attempts"`
	BackoffStrategy string `json:"backoff_strategy" yaml:"backoff_strategy"`
	InitialDelay    int    `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay        int    `json:"max_delay" yaml:"max_delay"`
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		BackoffStrategy: "exponential",
		InitialDelay:    1000,
		MaxDelay:        30000,
	}
}

func (r *RetryConfig) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	switch r.BackoffStrategy {
	case "fixed":
		return time.Duration(r.InitialDelay) * time.Millisecond
	case "linear":
		return time.Duration(r.InitialDelay*attempt) * time.Millisecond
	case "exponential":
		delay := r.InitialDelay * (1 << (attempt - 1))
		if delay > r.MaxDelay {
			delay = r.MaxDelay
		}
		return time.Duration(delay) * time.Millisecond
	default:
		return time.Duration(r.InitialDelay) * time.Millisecond
	}
}

// ToolCallDefinition 工具调用定义
type ToolCallDefinition struct {
	// ID 工具调用ID（唯一标识）
	ID string `json:"id" yaml:"id"`
	// Type 工具类型（固定 "function"）
	Type string `json:"type" yaml:"type"`
	// Function 函数定义
	Function FunctionDefinition `json:"function" yaml:"function"`
}

// FunctionDefinition 函数定义
type FunctionDefinition struct {
	// Name 函数名称
	Name string `json:"name" yaml:"name"`
	// Arguments 参数（JSON字符串，支持模板变量）
	Arguments string `json:"arguments" yaml:"arguments"`
}

// ToolCallInjectorConfig 工具调用注入节点配置
type ToolCallInjectorConfig struct {
	// ToolCalls 工具调用定义列表
	ToolCalls []ToolCallDefinition `json:"tool_calls" yaml:"tool_calls"`
	// Condition 触发条件（简单布尔判断，支持模板变量）
	// 示例: "{{node.reviewer.score}} < 0.8"
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// NodeFactory 节点工厂函数类型
type NodeFactory func(config NodeConfig) (PipelineNode, error)

// NodeRegistry 节点注册表
type NodeRegistry struct {
	factories         map[NodeType]NodeFactory
	plugins           map[string]NodePlugin
	securityValidator *PluginSecurityValidator
	admissionChecker  *AdmissionChecker
	businessRegistry  *BusinessPluginRegistry
}

// SetBusinessRegistry 设置业务插件注册表
func (r *NodeRegistry) SetBusinessRegistry(br *BusinessPluginRegistry) {
	r.businessRegistry = br
}

// GetBusinessRegistry 获取业务插件注册表
func (r *NodeRegistry) GetBusinessRegistry() *BusinessPluginRegistry {
	return r.businessRegistry
}

type remoteHealthCheckController interface {
	StartHealthCheck()
	StopHealthCheck()
}

func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		factories: make(map[NodeType]NodeFactory),
		plugins:   make(map[string]NodePlugin),
	}
}

func (r *NodeRegistry) SetSecurityValidator(validator *PluginSecurityValidator) {
	r.securityValidator = validator
}

func (r *NodeRegistry) SetAdmissionChecker(checker *AdmissionChecker) {
	r.admissionChecker = checker
}

func (r *NodeRegistry) GetSecurityValidator() *PluginSecurityValidator {
	return r.securityValidator
}

func (r *NodeRegistry) GetAdmissionChecker() *AdmissionChecker {
	return r.admissionChecker
}

func (r *NodeRegistry) Register(nodeType NodeType, factory NodeFactory) error {
	if !nodeType.IsValid() {
		return fmt.Errorf("invalid node type: %s", nodeType)
	}
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}
	r.factories[nodeType] = factory
	return nil
}

func (r *NodeRegistry) RegisterPlugin(plugin NodePlugin) error {
	if plugin == nil {
		return fmt.Errorf("node plugin cannot be nil")
	}
	descriptor := plugin.Descriptor()
	implementation := NormalizeImplementation(descriptor.Implementation)
	if implementation == "" {
		return fmt.Errorf("node plugin implementation cannot be empty")
	}

	if r.securityValidator != nil && r.securityValidator.IsEnabled() {
		if err := r.securityValidator.ValidateSource(implementation); err != nil {
			return fmt.Errorf("security validation failed: %w", err)
		}
	}

	if r.admissionChecker != nil && r.admissionChecker.cfg.Enabled {
		result := r.admissionChecker.CheckAll(descriptor, 30, r.securityValidator)
		if !result.Passed {
			return fmt.Errorf("admission check failed: %s", result.Summary())
		}
		if len(result.Warnings) > 0 {
			for _, w := range result.Warnings {
				logger.Warnf("[ADMISSION] %s: %s", descriptor.Implementation, w)
			}
		}
	}

	// 同 implementation 重新注册时，先停止旧远程插件的健康检查，避免协程泄漏。
	if old, exists := r.plugins[implementation]; exists && old != plugin {
		if ctrl, ok := old.(remoteHealthCheckController); ok {
			ctrl.StopHealthCheck()
		}
	}
	r.plugins[implementation] = plugin
	if ctrl, ok := plugin.(remoteHealthCheckController); ok {
		ctrl.StartHealthCheck()
	}
	return nil
}

func (r *NodeRegistry) Create(nodeType NodeType, config NodeConfig) (PipelineNode, error) {
	factory, ok := r.factories[nodeType]
	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}
	return factory(config)
}

func (r *NodeRegistry) CreateFromConfig(config PipelineNodeConfig, nodeConfig NodeConfig) (PipelineNode, error) {
	implementation := NormalizeImplementation(config.Implementation)
	if implementation == "" {
		implementation = BuiltinImplementationForType(config.Type)
	}
	if implementation != "" {
		if plugin, ok := r.plugins[implementation]; ok {
			// 若插件声明支持流式，则直接通过工厂创建节点（保留 ExecuteStream 方法）
			if plugin.Descriptor().SupportsStream {
				node, err := r.Create(config.Type, nodeConfig)
				if err != nil {
					return nil, err
				}
				// 保留 config.ID/Name/Type（工厂函数通常不设置这些字段）
				if setter, ok := node.(interface{ SetID(string) }); ok && config.ID != "" {
					setter.SetID(config.ID)
				}
				if setter, ok := node.(interface{ SetName(string) }); ok && config.Name != "" {
					setter.SetName(config.Name)
				}
				if setter, ok := node.(interface{ SetType(NodeType) }); ok && config.Type != "" {
					setter.SetType(config.Type)
				}
				return node, nil
			}
			return NewPluginBackedNode(config, nodeConfig, plugin), nil
		}
		if IsRemoteImplementation(implementation) {
			if r.securityValidator != nil && r.securityValidator.IsEnabled() {
				if err := r.securityValidator.ValidateSource(implementation); err != nil {
					return nil, fmt.Errorf("security validation failed for remote plugin: %w", err)
				}
			}

			plugin := NewRemoteNodePlugin(implementation)
			r.plugins[implementation] = plugin
			if ctrl, ok := plugin.(remoteHealthCheckController); ok {
				ctrl.StartHealthCheck()
			}
			return NewPluginBackedNode(config, nodeConfig, plugin), nil
		}
	}
	return r.Create(config.Type, nodeConfig)
}

func (r *NodeRegistry) IsRegistered(nodeType NodeType) bool {
	_, ok := r.factories[nodeType]
	return ok
}

func (r *NodeRegistry) IsPluginRegistered(implementation string) bool {
	_, ok := r.plugins[NormalizeImplementation(implementation)]
	return ok
}

func (r *NodeRegistry) GetRegisteredTypes() []NodeType {
	types := make([]NodeType, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	return types
}

func (r *NodeRegistry) GetPluginDescriptors() []NodePluginDescriptor {
	descriptors := make([]NodePluginDescriptor, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		descriptors = append(descriptors, plugin.Descriptor())
	}
	return descriptors
}

func (r *NodeRegistry) GetPlugin(implementation string) (NodePlugin, bool) {
	plugin, ok := r.plugins[NormalizeImplementation(implementation)]
	return plugin, ok
}

func (r *NodeRegistry) StopRemoteHealthChecks() {
	for _, plugin := range r.plugins {
		if ctrl, ok := plugin.(remoteHealthCheckController); ok {
			ctrl.StopHealthCheck()
		}
	}
}

type PluginBackedNode struct {
	BaseNode
	kind           string
	implementation string
	plugin         NodePlugin
}

func NewPluginBackedNode(config PipelineNodeConfig, nodeConfig NodeConfig, plugin NodePlugin) *PluginBackedNode {
	nodeType := config.Type
	if nodeType == "" {
		nodeType = NodeType(config.Kind)
	}
	implementation := NormalizeImplementation(config.Implementation)
	if implementation == "" {
		implementation = plugin.Descriptor().Implementation
	}
	return &PluginBackedNode{
		BaseNode: BaseNode{
			id:          config.ID,
			name:        config.Name,
			nodeType:    nodeType,
			config:      nodeConfig,
			timeout:     60,
			retryConfig: DefaultRetryConfig(),
		},
		kind:           config.Kind,
		implementation: implementation,
		plugin:         plugin,
	}
}

func (n *PluginBackedNode) Validate() error {
	return n.plugin.ValidateConfig(n.config)
}

func (n *PluginBackedNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	maxInputBytes := DefaultMaxInputBytes
	if n.config.MaxInputBytes > 0 {
		maxInputBytes = n.config.MaxInputBytes
	}
	if input != nil {
		size := estimateInputSize(input)
		if size > maxInputBytes {
			return nil, fmt.Errorf("node %s input size %d exceeds limit %d", n.id, size, maxInputBytes)
		}
	}

	req := &NodeExecutionRequest{
		SchemaVersion:    PipelinePluginSchemaVersion,
		NodeID:           n.id,
		NodeName:         n.name,
		NodeType:         n.nodeType,
		Kind:             n.kind,
		Implementation:   n.implementation,
		Config:           n.config,
		Input:            input,
		CapabilityBroker: n.capabilityBroker,
		MaxInputBytes:    maxInputBytes,
		MaxOutputBytes:   DefaultMaxOutputBytes,
	}

	// 解析 secrets_ref：权限与解析失败均视为执行错误，避免静默降级。
	if len(n.config.SecretsRef) > 0 {
		if n.capabilityBroker == nil {
			return nil, fmt.Errorf("node %s requires secrets_ref but capability broker is not configured", n.id)
		}
		resolver, err := n.capabilityBroker.GetSecretsResolver(ctx, n.GetPermissions())
		if err != nil {
			return nil, fmt.Errorf("node %s cannot acquire secrets resolver: %w", n.id, err)
		}
		secrets := make(map[string]string, len(n.config.SecretsRef))
		for _, ref := range n.config.SecretsRef {
			value, err := resolver.Resolve(ref)
			if err != nil {
				return nil, fmt.Errorf("node %s failed to resolve secret %q: %w", n.id, ref, err)
			}
			secrets[ref] = value
		}
		req.Secrets = secrets
	}
	if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
		req.PipelineID = execCtx.pipeline.ID
		req.Context = map[string]interface{}{
			"pipeline_id": execCtx.pipeline.ID,
		}
		if userID, ok := execCtx.GetVariable("user_id"); ok {
			req.Context["user_id"] = userID
		}
		if sessionID, ok := execCtx.GetVariable("session_id"); ok {
			req.Context["session_id"] = sessionID
		}
		if traceID, ok := execCtx.GetVariable("trace_id"); ok {
			req.TraceID = traceID.(string)
		}
		if requestID, ok := execCtx.GetVariable("request_id"); ok {
			req.RequestID = requestID.(string)
		}
		if timeout := n.GetTimeout(); timeout > 0 {
			req.Deadline = time.Now().Add(time.Duration(timeout) * time.Second).UnixNano()
		}
	}
	resp, err := n.plugin.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Output == nil {
		return nil, fmt.Errorf("node plugin %s returned empty output", n.implementation)
	}
	return resp.Output, nil
}

func estimateInputSize(input *NodeInput) int64 {
	if input == nil {
		return 0
	}
	size := int64(len(input.Content))
	for _, msg := range input.Messages {
		size += int64(len(msg.Role) + len(msg.Content))
	}
	if input.Metadata != nil {
		if data, err := json.Marshal(input.Metadata); err == nil {
			size += int64(len(data))
		}
	}
	for nodeID, result := range input.UpstreamResults {
		size += int64(len(nodeID))
		if result != nil {
			size += int64(len(result.Content))
			if result.Metadata != nil {
				if data, err := json.Marshal(result.Metadata); err == nil {
					size += int64(len(data))
				}
			}
		}
	}
	return size
}
