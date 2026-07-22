package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/backend"
	"centag/core/internal/cache"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/plugin"
	"centag/core/pkg/storage"
	"centag/core/pkg/utils"
)

// ExecutionContext 流水线执行上下文
type ExecutionContext struct {
	mu          sync.RWMutex
	pipeline    *AgentPatternPipeline
	variables   map[string]interface{}
	results     map[string]*NodeOutput
	currentNode string
	startTime   time.Time
	nodeLogs    []NodeExecutionLog
	totalTokens int
}

// NewExecutionContext 创建执行上下文
func NewExecutionContext(pipeline *AgentPatternPipeline) *ExecutionContext {
	return &ExecutionContext{
		pipeline:    pipeline,
		variables:   make(map[string]interface{}),
		results:     make(map[string]*NodeOutput),
		nodeLogs:    make([]NodeExecutionLog, 0),
		startTime:   time.Now(),
		totalTokens: 0,
	}
}

func (ctx *ExecutionContext) SetVariable(key string, value interface{}) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.variables[key] = value
}

func (ctx *ExecutionContext) GetVariable(key string) (interface{}, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	v, ok := ctx.variables[key]
	return v, ok
}

func (ctx *ExecutionContext) SetResult(nodeID string, output *NodeOutput) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.results[nodeID] = output
}

func (ctx *ExecutionContext) GetResult(nodeID string) (*NodeOutput, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	result, ok := ctx.results[nodeID]
	return result, ok
}

// GetAllResults 返回所有节点的执行结果（深拷贝，线程安全）。
func (ctx *ExecutionContext) GetAllResults() map[string]*NodeOutput {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	out := make(map[string]*NodeOutput, len(ctx.results))
	for k, v := range ctx.results {
		out[k] = v
	}
	return out
}

// GetCurrentNode 返回最后执行的节点 ID。
func (ctx *ExecutionContext) GetCurrentNode() string {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.currentNode
}

func (ctx *ExecutionContext) GetLastOutput() *NodeOutput {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	if len(ctx.results) == 0 {
		return nil
	}
	if ctx.currentNode != "" {
		if result, ok := ctx.results[ctx.currentNode]; ok {
			return result
		}
	}
	for _, result := range ctx.results {
		return result
	}
	return nil
}

func (ctx *ExecutionContext) AddNodeLog(log NodeExecutionLog) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	// 脱敏：清理节点日志中的敏感信息
	log.ErrorMessage = MaskSensitiveData(log.ErrorMessage)
	ctx.nodeLogs = append(ctx.nodeLogs, log)
	ctx.totalTokens += log.InputTokens + log.OutputTokens
}

func (ctx *ExecutionContext) SetCurrentNode(nodeID string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.currentNode = nodeID
}

func (ctx *ExecutionContext) GetExecutionLog() *ExecutionLog {
	ctx.mu.RLock()
	nodeLogs := make([]NodeExecutionLog, len(ctx.nodeLogs))
	copy(nodeLogs, ctx.nodeLogs)
	totalTokens := ctx.totalTokens
	ctx.mu.RUnlock()

	endTime := time.Now()
	success := true
	var errMsg string
	for _, log := range nodeLogs {
		if !log.Success {
			success = false
			errMsg = log.ErrorMessage
			break
		}
	}

	return &ExecutionLog{
		PipelineID:   ctx.pipeline.ID,
		StartTime:    ctx.startTime,
		EndTime:      endTime,
		Duration:     endTime.Sub(ctx.startTime).Milliseconds(),
		NodeLogs:     nodeLogs,
		TotalTokens:  totalTokens,
		Success:      success,
		ErrorMessage: errMsg,
	}
}

// ExecutionGraph 执行图
type ExecutionGraph struct {
	nodes map[string]*ExecutionNode
	edges map[string][]string
}

type ExecutionNode struct {
	Config    PipelineNodeConfig
	Status    ExecutionStatus
	Input     *NodeInput
	Output    *NodeOutput
	Error     error
	StartTime *time.Time
	EndTime   *time.Time
}

type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusSuccess   ExecutionStatus = "success"
	StatusFailed    ExecutionStatus = "failed"
	StatusSkipped   ExecutionStatus = "skipped"
	StatusCancelled ExecutionStatus = "cancelled"
)

func NewExecutionGraph(pipeline *AgentPatternPipeline) *ExecutionGraph {
	graph := &ExecutionGraph{
		nodes: make(map[string]*ExecutionNode),
		edges: make(map[string][]string),
	}

	// 构建节点
	for _, nodeConfig := range pipeline.Nodes {
		graph.nodes[nodeConfig.ID] = &ExecutionNode{
			Config: nodeConfig,
			Status: StatusPending,
		}
	}

	// 构建边（依赖关系）
	for _, nodeConfig := range pipeline.Nodes {
		for _, dep := range nodeConfig.DependsOn {
			graph.edges[dep] = append(graph.edges[dep], nodeConfig.ID)
		}
		if len(nodeConfig.DependsOn) == 0 {
			graph.edges[nodeConfig.ID] = append(graph.edges[nodeConfig.ID], nodeConfig.NextNodes...)
		}
	}

	return graph
}

func (g *ExecutionGraph) GetNode(nodeID string) *ExecutionNode {
	return g.nodes[nodeID]
}

func (g *ExecutionGraph) GetDependencies(nodeID string) []string {
	var deps []string
	for from, toList := range g.edges {
		for _, to := range toList {
			if to == nodeID {
				deps = append(deps, from)
				break
			}
		}
	}
	return deps
}

// LayeredTopologicalSort 按依赖关系分层，返回分层后的节点列表
// 同层节点可以并行执行
func (g *ExecutionGraph) LayeredTopologicalSort() ([][]string, error) {
	inDegree := make(map[string]int)
	children := make(map[string][]string)

	// 初始化入度
	for id := range g.nodes {
		inDegree[id] = 0
	}

	// 构建入度和子节点映射
	for from, toList := range g.edges {
		for _, to := range toList {
			if _, ok := g.nodes[to]; ok {
				inDegree[to]++
				children[from] = append(children[from], to)
			}
		}
		if _, ok := inDegree[from]; !ok {
			inDegree[from] = 0
		}
		if _, ok := children[from]; !ok {
			children[from] = []string{}
		}
	}

	layers := [][]string{}
	processed := make(map[string]bool)

	for len(processed) < len(g.nodes) {
		currentLayer := []string{}
		for nodeID, degree := range inDegree {
			if degree == 0 && !processed[nodeID] {
				currentLayer = append(currentLayer, nodeID)
			}
		}

		if len(currentLayer) == 0 {
			return nil, fmt.Errorf("cycle detected in execution graph")
		}

		layers = append(layers, currentLayer)

		for _, nodeID := range currentLayer {
			processed[nodeID] = true
			for _, child := range children[nodeID] {
				inDegree[child]--
			}
		}
	}

	return layers, nil
}

func (g *ExecutionGraph) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}

	for from, toList := range g.edges {
		for _, to := range toList {
			if _, ok := g.nodes[to]; ok {
				inDegree[to]++
			}
		}
		// 确保from也在inDegree中
		if _, ok := inDegree[from]; !ok {
			inDegree[from] = 0
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range g.edges[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected in execution graph")
	}

	return result, nil
}

// ConditionEvaluator 条件评估器
type ConditionEvaluator struct {
	ctx *ExecutionContext
}

func NewConditionEvaluator(ctx *ExecutionContext) *ConditionEvaluator {
	return &ConditionEvaluator{ctx: ctx}
}

// Evaluate 评估条件表达式
// 支持格式：{{.node_id.metadata.field}} >/</==/!= value
func (e *ConditionEvaluator) Evaluate(condition string) bool {
	if condition == "" {
		return true
	}

	// 解析条件表达式
	// 格式: {{.node_id.metadata.field}} 或 {{node_id.metadata.field}} operator value
	re := regexp.MustCompile(`\{\{\.?([\w_]+)\.metadata\.([\w_]+)\}\}\s*(>|>=|<|<=|==|!=)\s*(.+)`)
	matches := re.FindStringSubmatch(condition)
	if len(matches) != 5 {
		// 如果无法解析，默认返回true
		return true
	}

	nodeID := matches[1]
	field := matches[2]
	operator := matches[3]
	rawValue := strings.TrimSpace(matches[4])

	// 获取节点输出结果
	result, ok := e.ctx.GetResult(nodeID)
	if !ok || result == nil || result.Metadata == nil {
		return false
	}

	fieldValue, ok := result.Metadata[field]
	if !ok {
		return false
	}

	return compareValues(fieldValue, operator, rawValue)
}

// compareValues 比较两个值
func compareValues(fieldValue interface{}, operator, rawValue string) bool {
	// 优先处理布尔值比较
	if fieldBool, ok := fieldValue.(bool); ok {
		// 将 rawValue 解析为布尔值
		var compareBool bool
		if rawValue == "true" {
			compareBool = true
		} else if rawValue == "false" {
			compareBool = false
		} else {
			// 无法解析，回退到字符串比较
			fieldStr := fmt.Sprintf("%v", fieldValue)
			switch operator {
			case "==":
				return fieldStr == rawValue
			case "!=":
				return fieldStr != rawValue
			}
			return false
		}

		switch operator {
		case "==":
			return fieldBool == compareBool
		case "!=":
			return fieldBool != compareBool
		}
		return false
	}

	// 尝试数值比较
	fieldFloat, fieldIsNum := toFloat64(fieldValue)
	valueFloat, valueIsNum := strconv.ParseFloat(rawValue, 64)

	if fieldIsNum && valueIsNum == nil {
		switch operator {
		case ">":
			return fieldFloat > valueFloat
		case ">=":
			return fieldFloat >= valueFloat
		case "<":
			return fieldFloat < valueFloat
		case "<=":
			return fieldFloat <= valueFloat
		case "==":
			return fieldFloat == valueFloat
		case "!=":
			return fieldFloat != valueFloat
		}
	}

	// 字符串比较
	fieldStr := fmt.Sprintf("%v", fieldValue)
	switch operator {
	case "==":
		return fieldStr == rawValue
	case "!=":
		return fieldStr != rawValue
	}

	return false
}

// toFloat64 将值转换为float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// PipelineEngine 流水线执行引擎
type PipelineEngine struct {
	nodeRegistry     *NodeRegistry
	pipelineRegistry *PipelineRegistry
	capabilityBroker CapabilityBroker
	logger           Logger
	storageManager   *storage.Manager // 存储管理器（用于 CacheNode 初始化存储）
	// storageHooks 按 pipelineID 缓存已创建的存储钩子
	storageHooks map[string]*StorageHook
	hookMu       sync.RWMutex
}

// getOrCreateStorageHook 获取或创建流水线对应的存储钩子
func (e *PipelineEngine) getOrCreateStorageHook(pipeline *AgentPatternPipeline) *StorageHook {
	if pipeline == nil || e.storageManager == nil {
		return nil
	}
	if !pipeline.GlobalConfig.HasStorageHook() {
		return nil
	}

	e.hookMu.RLock()
	hook, ok := e.storageHooks[pipeline.ID]
	e.hookMu.RUnlock()
	if ok {
		return hook
	}

	e.hookMu.Lock()
	defer e.hookMu.Unlock()
	// double check
	if hook, ok = e.storageHooks[pipeline.ID]; ok {
		return hook
	}
	if e.storageHooks == nil {
		e.storageHooks = make(map[string]*StorageHook)
	}
	hook = NewStorageHook(e.storageManager, pipeline, e.logger)
	e.storageHooks[pipeline.ID] = hook
	return hook
}

// InvalidateStorageHookCache 清除指定流水线的存储钩子缓存
// 当流水线配置（如 storage_type）发生变更时调用，确保下次执行使用最新配置
func (e *PipelineEngine) InvalidateStorageHookCache(pipelineID string) {
	e.hookMu.Lock()
	defer e.hookMu.Unlock()
	delete(e.storageHooks, pipelineID)
}

// SetCapabilityBroker 设置能力代理（支持延迟注入）
func (e *PipelineEngine) SetCapabilityBroker(broker CapabilityBroker) {
	e.capabilityBroker = broker
}

// LLMRequest LLM请求
type LLMRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	// Tools 工具定义（透传给后端，支持 function calling）
	Tools any `json:"tools,omitempty"`
	// ToolChoice 工具选择策略
	ToolChoice any `json:"tool_choice,omitempty"`
}

// LLMResponse LLM响应
type LLMResponse struct {
	Model            string     `json:"model"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	TokenUsage       int        `json:"token_usage"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	FinishReason     string     `json:"finish_reason,omitempty"`
}

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// NewPipelineEngine 创建流水线执行引擎
func NewPipelineEngine(
	nodeRegistry *NodeRegistry,
	pipelineRegistry *PipelineRegistry,
	capabilityBroker CapabilityBroker,
	logger Logger,
	storageManager *storage.Manager, // 新增：存储管理器
) *PipelineEngine {
	return &PipelineEngine{
		nodeRegistry:     nodeRegistry,
		pipelineRegistry: pipelineRegistry,
		capabilityBroker: capabilityBroker,
		logger:           logger,
		storageManager:   storageManager, // 新增
	}
}

// Execute 执行流水线（按 ID 从注册表获取定义）
func (e *PipelineEngine) Execute(ctx context.Context, pipelineID string, input *PipelineInput) (*PipelineOutput, error) {
	pipeline := e.pipelineRegistry.Get(pipelineID)
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline not found: %s", pipelineID)
	}
	return e.executePipeline(ctx, pipeline, input)
}

// ExecutePipelineDefinition 直接执行流水线定义（无需预先注册到注册表）。
// 用于前端画布的"测试"场景：流水线尚在编辑中、未保存到后端。
func (e *PipelineEngine) ExecutePipelineDefinition(ctx context.Context, pipeline *AgentPatternPipeline, input *PipelineInput) (*PipelineOutput, error) {
	if err := pipeline.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline: %w", err)
	}
	return e.executePipeline(ctx, pipeline, input)
}

// HasPipeline 检查流水线是否存在
func (e *PipelineEngine) HasPipeline(pipelineID string) bool {
	return e.pipelineRegistry.Exists(pipelineID)
}

// RegisterPipeline 注册流水线到引擎
func (e *PipelineEngine) RegisterPipeline(pipeline *AgentPatternPipeline) error {
	if e.pipelineRegistry == nil {
		return fmt.Errorf("pipeline registry not initialized")
	}
	return e.pipelineRegistry.Register(pipeline)
}

// GetPipelineConfig 返回流水线配置，不存在时返回 nil
func (e *PipelineEngine) GetPipelineConfig(pipelineID string) *AgentPatternPipeline {
	return e.pipelineRegistry.Get(pipelineID)
}

// PipelineStreamResult 流式执行结果
type PipelineStreamResult struct {
	// Chunk 非 nil 时表示一个流式数据块
	Chunk *plugin.StreamChunk
	// Output 非 nil 时表示流式执行完成，包含完整输出
	Output *PipelineOutput
}

// ExecuteStream 流式执行流水线。
// 返回一个 result channel：调用方持续读取直到 channel 关闭。
// 所有节点统一以阻塞方式执行（Execute()），节点间传递完整 NodeOutput；
// 流式分块由顶层 streamEmitter 根据 input.Stream 统一适配。
func (e *PipelineEngine) ExecuteStream(ctx context.Context, pipelineID string, input *PipelineInput) (<-chan PipelineStreamResult, error) {
	pipeline := e.pipelineRegistry.Get(pipelineID)
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline not found: %s", pipelineID)
	}
	return e.executePipelineStream(ctx, pipeline, input)
}

func (e *PipelineEngine) executePipelineStream(ctx context.Context, pipeline *AgentPatternPipeline, input *PipelineInput) (<-chan PipelineStreamResult, error) {
	resultCh := make(chan PipelineStreamResult, 64)

	go func() {
		defer close(resultCh)

		// ---------- 1. 初始化 ----------
		graph := NewExecutionGraph(pipeline)
		layers, err := graph.LayeredTopologicalSort()
		if err != nil {
			resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: fmt.Errorf("failed to build execution order: %w", err)}}
			return
		}

		streamStartFields := []interface{}{
			"pipeline_id", pipeline.ID,
			"layer_count", len(layers),
			"input_preview", MaskSensitiveData(utils.TruncateString(input.Content, 100)),
		}
		if requestID := RequestIDFromInput(input); requestID != "" {
			streamStartFields = append(streamStartFields, "request_id", requestID)
		}
		e.logger.Info("pipeline stream execution started", streamStartFields...)

		execCtx := NewExecutionContext(pipeline)
	execCtx.SetVariable("input", input.Content)
	execCtx.SetVariable("messages", input.Messages)
	execCtx.SetVariable("metadata", input.Metadata)
	execCtx.SetVariable("user_id", input.UserID)
	execCtx.SetVariable("session_id", input.SessionID)
	execCtx.SetVariable("tools", input.Tools)
	execCtx.SetVariable("tool_choice", input.ToolChoice)
		if requestID := RequestIDFromInput(input); requestID != "" {
			execCtx.SetVariable("request_id", requestID)
		}
		InjectCacheControlFromMetadata(execCtx, input.Metadata)

		ctx = context.WithValue(ctx, executionContextKey{}, execCtx)
		ctx = context.WithValue(ctx, loggerContextKey{}, e.logger)

		fallbackNodeSet := make(map[string]bool)
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			for _, fbID := range fg.FallbackNodes {
				fallbackNodeSet[fbID] = true
			}
		}

		parallelLimit := pipeline.GlobalConfig.ParallelLimit
		if parallelLimit < 1 {
			parallelLimit = 1
		}

		// ---------- 2. 逐层执行（全部走非流式 Execute()）----------
		// 架构原则：节点间数据传递统一为非流式（完整 NodeOutput），
		// 流式与否由调用方 req.Stream 在执行结束后通过 StreamAdapter 统一适配。
		// 这样可以让 optimize-mode / translate-mode 等最后节点是 processor 的
		// 流水线也能按 req.Stream=true 输出流式 chunk。
		for _, layer := range layers {
			if len(layer) == 0 {
				continue
			}

			activeNodes := make([]string, 0, len(layer))
			for _, nodeID := range layer {
				if !fallbackNodeSet[nodeID] {
					activeNodes = append(activeNodes, nodeID)
				}
			}
			activeNodes = e.filterRoutedNodes(activeNodes, graph, execCtx)
			if len(activeNodes) == 0 {
				continue
			}

			if len(activeNodes) == 1 {
				nodeID := activeNodes[0]
				if execErr := e.executeLayerNode(ctx, graph, execCtx, nodeID, pipeline); execErr != nil {
					resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: execErr}}
					return
				}
			} else {
				if execErr := e.executeLayerParallel(ctx, graph, execCtx, activeNodes, pipeline, parallelLimit, nil); execErr != nil {
					resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: execErr}}
					return
				}
			}
		}

		// ---------- 3. 降级组处理 ----------
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			primaryNode := graph.GetNode(fg.PrimaryNodeID)
			if primaryNode == nil {
				continue
			}
			primaryFailed := primaryNode.Status == StatusFailed || primaryNode.Error != nil
			if !primaryFailed {
				for _, fbID := range fg.FallbackNodes {
					if fbNode := graph.GetNode(fbID); fbNode != nil {
						fbNode.Status = StatusSkipped
					}
				}
				continue
			}
			for attemptIdx, fbID := range e.filterFallbackNodesByCircuit(graph, fg.FallbackNodes) {
				if attemptIdx >= fg.MaxAttempts-1 {
					break
				}
				if execErr := e.executeLayerNode(ctx, graph, execCtx, fbID, pipeline); execErr != nil {
					continue
				}
				if fbNode := graph.GetNode(fbID); fbNode != nil && fbNode.Status == StatusSuccess {
					break
				}
			}
		}

		// ---------- 4. 构建输出 ----------
		lastOutput := execCtx.GetLastOutput()
		if lastOutput == nil {
			resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: fmt.Errorf("pipeline produced no output")}}
			return
		}

		outputContent := resolvePipelineOutputContent(execCtx, lastOutput)

		execLog := execCtx.GetExecutionLog()
		finishFields := AppendRequestIDFields(ctx,
			"pipeline_id", pipeline.ID,
			"duration_ms", execLog.Duration,
			"success", execLog.Success,
			"total_tokens", execLog.TotalTokens,
			"node_count", len(execLog.NodeLogs),
			"last_node", execCtx.GetCurrentNode(),
			"result_count", len(execCtx.GetAllResults()),
			"response_preview", MaskSensitiveData(utils.TruncateString(outputContent, defaultMessagesPreviewMax)),
		)
		e.logger.Info("pipeline stream execution finished", finishFields...)

		// 存储钩子：流式流水线完成后归档执行记录
		if hook := e.getOrCreateStorageHook(pipeline); hook != nil {
			hook.OnPipelineComplete(ctx, execCtx)
		}

		// ---------- 5. 流式适配 ----------
		// 唯一流式决策来源是 req.Stream（即 input.Stream）。
		// 调度与流式适配分离：所有节点已执行完毕，此处只负责把 PipelineOutput 翻译为 stream 结果。
		streamEmitter(ctx, input.Stream, outputContent, lastOutput, execLog, execCtx, resultCh)
	}()

	return resultCh, nil
}

// streamEmitter 将完整的 PipelineOutput 翻译为一系列 PipelineStreamResult。
// 这是引擎顶层唯一的流式适配点：
//   - input.Stream=true → 通过 StreamAdapter 分块输出
//   - input.Stream=false → 单 chunk 完整输出
//
// 设计动机：把流式分块职责从执行调度中分离，使执行路径（executePipeline）保持纯调度语义，
// 未来若引入流式语义分块（句子/段落）或更细粒度的 SSE 字段构造，
// 只需修改此处而无需触动调度核心。
func streamEmitter(
	ctx context.Context,
	stream bool,
	outputContent string,
	lastOutput *NodeOutput,
	execLog *ExecutionLog,
	execCtx *ExecutionContext,
	resultCh chan<- PipelineStreamResult,
) {
	// 提取 tool_calls 和 finish_reason（function calling 支持）
	var toolCalls []ToolCall
	var finishReason string
	var reasoningContent string
	if lastOutput != nil {
		toolCalls = lastOutput.ToolCalls
		finishReason = lastOutput.FinishReason
		reasoningContent = lastOutput.ReasoningContent
	}

	if stream {
		// transparent_forward 等节点的 Content 可能是上游完整 SSE（含 data: 前缀）。
		// 不可再经 StreamAdapter 当普通文本分块，否则客户端会把 "data:..." 当成回答正文。
		if isRawPassthroughNodeOutput(lastOutput) {
			// 仅下发 Output，由 ModeDispatcher 原样写出。
		} else if len(toolCalls) > 0 {
			// 如果有 tool_calls，直接作为单个 chunk 发送（不分块）
			resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{
				Content:          outputContent,
				ReasoningContent: reasoningContent,
				ToolCalls:        convertToPluginToolCalls(toolCalls),
				FinishReason:     finishReason,
				Done:             true,
			}}
		} else {
			adapter := NewStreamAdapter()
			streamCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			for chunk := range adapter.Adapt(streamCtx, outputContent) {
				if chunk.Error != nil {
					resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: chunk.Error}}
					return
				}
				// 最后一个 chunk 携带 finish_reason
				if chunk.Done && finishReason != "" {
					chunk.FinishReason = finishReason
				}
				resultCh <- PipelineStreamResult{Chunk: &chunk}
			}
		}
	} else {
		// 非分块模式：单条正文 chunk（Done 留给 writeStreamResponse 发 [DONE]）
		chunk := &plugin.StreamChunk{
			Content:          outputContent,
			ReasoningContent: reasoningContent,
			FinishReason:     finishReason,
		}
		if len(toolCalls) > 0 {
			chunk.ToolCalls = convertToPluginToolCalls(toolCalls)
		}
		resultCh <- PipelineStreamResult{Chunk: chunk}
	}

	// 构建最终 PipelineOutput，填充节点级执行结果
	var nodeOutputs map[string]*NodeOutput
	var lastNode string
	if execCtx != nil {
		nodeOutputs = execCtx.GetAllResults()
		lastNode = execCtx.GetCurrentNode()
	}

	finalOutput := &PipelineOutput{
		Content:          outputContent,
		Passed:           lastOutput.Passed,
		Score:            lastOutput.Score,
		Feedback:         lastOutput.Feedback,
		Suggestions:      lastOutput.Suggestions,
		Messages:         lastOutput.Messages,
		Metadata:         lastOutput.Metadata,
		ExecutionLog:     execLog,
		NodeOutputs:      nodeOutputs,
		LastNode:         lastNode,
	}
	if len(lastOutput.ToolCalls) > 0 {
		finalOutput.ToolCalls = lastOutput.ToolCalls
	}
	if lastOutput.FinishReason != "" {
		finalOutput.FinishReason = lastOutput.FinishReason
	}
	if lastOutput.ReasoningContent != "" {
		finalOutput.ReasoningContent = lastOutput.ReasoningContent
	}
	resultCh <- PipelineStreamResult{Output: finalOutput}
}

func isRawPassthroughNodeOutput(out *NodeOutput) bool {
	if out == nil || out.Metadata == nil {
		return false
	}
	v, ok := out.Metadata["raw_passthrough"].(bool)
	return ok && v
}

// resolvePipelineOutputContent 从最后节点输出解析应对外返回的正文。
// reviewer 等节点可能 Content 为空，需回退到上游 generator/generate 或缓存命中结果。
func resolvePipelineOutputContent(execCtx *ExecutionContext, lastOutput *NodeOutput) string {
	if execCtx != nil {
		if cacheResult, cacheOk := execCtx.GetResult("cache_read"); cacheOk && cacheResult != nil {
			if cacheHit, metaOk := cacheResult.Metadata["cache_hit"].(bool); metaOk && cacheHit && strings.TrimSpace(cacheResult.Content) != "" {
				return cacheResult.Content
			}
		}
	}
	if lastOutput == nil {
		return ""
	}
	outputContent := lastOutput.Content
	if lastOutput.Passed != nil && outputContent == "" {
		for _, nodeID := range []string{"transparent_forward", "generator", "generate"} {
			if genResult, ok := execCtx.GetResult(nodeID); ok && genResult != nil && genResult.Content != "" {
				outputContent = genResult.Content
				break
			}
		}
	}
	if outputContent == "" {
		for _, nodeID := range []string{"transparent_forward", "generator", "generate"} {
			if genResult, ok := execCtx.GetResult(nodeID); ok && genResult != nil && genResult.Content != "" {
				outputContent = genResult.Content
				break
			}
		}
	}
	if outputContent == "" {
		if cacheResult, cacheOk := execCtx.GetResult("cache_read"); cacheOk && cacheResult != nil {
			if cacheHit, metaOk := cacheResult.Metadata["cache_hit"].(bool); metaOk && cacheHit && cacheResult.Content != "" {
				outputContent = cacheResult.Content
			}
		}
	}
	return outputContent
}

// executeLayerParallel 并行执行一层中的多个非流式节点
func (e *PipelineEngine) executeLayerParallel(ctx context.Context, graph *ExecutionGraph, execCtx *ExecutionContext, nodeIDs []string, pipeline *AgentPatternPipeline, parallelLimit int, resultCh chan<- PipelineStreamResult) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallelLimit)
	var mu sync.Mutex
	var firstErr error

	for _, nodeID := range nodeIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			execErr := e.executeLayerNode(ctx, graph, execCtx, id, pipeline)
			mu.Lock()
			if execErr != nil && firstErr == nil {
				firstErr = execErr
			}
			mu.Unlock()
		}(nodeID)
	}
	wg.Wait()
	return firstErr
}

func (e *PipelineEngine) executePipeline(ctx context.Context, pipeline *AgentPatternPipeline, input *PipelineInput) (*PipelineOutput, error) {
	// 1. 构建执行图
	graph := NewExecutionGraph(pipeline)

	// 2. 分层拓扑排序
	layers, err := graph.LayeredTopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to build execution order: %w", err)
	}

	// 统计总节点数
	totalNodes := 0
	for _, layer := range layers {
		totalNodes += len(layer)
	}

	startFields := []interface{}{
		"pipeline_id", pipeline.ID,
		"layer_count", len(layers),
		"total_nodes", totalNodes,
		"parallel_limit", pipeline.GlobalConfig.ParallelLimit,
		"input_preview", MaskSensitiveData(utils.TruncateString(input.Content, 100)),
	}
	if requestID := RequestIDFromInput(input); requestID != "" {
		startFields = append(startFields, "request_id", requestID)
	}
	e.logger.Info("pipeline execution started", startFields...)

	// 3. 初始化执行上下文
	execCtx := NewExecutionContext(pipeline)
	execCtx.SetVariable("input", input.Content)
	execCtx.SetVariable("messages", input.Messages)
	execCtx.SetVariable("metadata", input.Metadata)
	// 注入 scene 作为一等上下文变量（供 template_resolver 的 context.scene 使用）
	if input.Metadata != nil {
		if scene, ok := input.Metadata["scene"].(string); ok && scene != "" {
			execCtx.SetVariable("scene", scene)
		}
	}
	execCtx.SetVariable("user_id", input.UserID)
	execCtx.SetVariable("session_id", input.SessionID)
	execCtx.SetVariable("tools", input.Tools)
	execCtx.SetVariable("tool_choice", input.ToolChoice)
	if requestID := RequestIDFromInput(input); requestID != "" {
		execCtx.SetVariable("request_id", requestID)
	}
	InjectCacheControlFromMetadata(execCtx, input.Metadata)

	// 4. 将 execCtx 和 logger 注入 Go context，供节点内部使用
	ctx = context.WithValue(ctx, executionContextKey{}, execCtx)
	ctx = context.WithValue(ctx, loggerContextKey{}, e.logger)

	// 5. 构建 fallback 节点集合（备用节点不参与常规分层执行；主节点失败时不立即终止）
	fallbackNodeSet := make(map[string]bool)
	fallbackPrimarySet := make(map[string]bool)
	for _, fg := range pipeline.GlobalConfig.FallbackGroups {
		fallbackPrimarySet[fg.PrimaryNodeID] = true
		for _, fbID := range fg.FallbackNodes {
			fallbackNodeSet[fbID] = true
		}
	}

	// 6. 执行节点（分层并行）
	parallelLimit := pipeline.GlobalConfig.ParallelLimit
	if parallelLimit < 1 {
		parallelLimit = 1
	}

	for layerIdx, layer := range layers {
		if len(layer) == 0 {
			continue
		}

		// 过滤掉 fallback 节点（它们由降级组处理）
		activeNodes := make([]string, 0, len(layer))
		for _, nodeID := range layer {
			if !fallbackNodeSet[nodeID] {
				activeNodes = append(activeNodes, nodeID)
			}
		}

		// 路由预过滤：如果同层中有非 default 分支匹配了路由结果， suppress 所有 default 分支
		activeNodes = e.filterRoutedNodes(activeNodes, graph, execCtx)

		if len(activeNodes) == 0 {
			continue
		}

		// 单节点层：串行执行（避免 goroutine 开销）
		if len(activeNodes) == 1 {
			nodeID := activeNodes[0]
			execErr := e.executeLayerNode(ctx, graph, execCtx, nodeID, pipeline)
			if execErr != nil {
				// 如果该节点是降级组主节点，不立即终止——让降级组处理
				if fallbackPrimarySet[nodeID] {
					e.logger.Warn("primary node failed, deferring to fallback group",
						"primary_node_id", nodeID,
					)
				} else {
					return nil, execErr
				}
			}
			continue
		}

		// 多节点层：并行执行
		var wg sync.WaitGroup
		sem := make(chan struct{}, parallelLimit)
		var mu sync.Mutex
		var firstErr error

		for _, nodeID := range activeNodes {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				execErr := e.executeLayerNode(ctx, graph, execCtx, id, pipeline)

				mu.Lock()
				if execErr != nil && firstErr == nil {
					// 降级组主节点失败不由层循环处理——让降级组处理
					if !fallbackPrimarySet[id] {
						firstErr = execErr
					}
				}
				mu.Unlock()
			}(nodeID)
		}
		wg.Wait()

		if firstErr != nil {
			return nil, firstErr
		}

		e.logger.Debug("layer execution completed",
			"layer_index", layerIdx,
			"node_count", len(activeNodes),
		)
	}

	// 7. 降级组处理：主节点失败时按序执行备用节点
	for _, fg := range pipeline.GlobalConfig.FallbackGroups {
		primaryNode := graph.GetNode(fg.PrimaryNodeID)
		if primaryNode == nil {
			continue
		}

		primaryFailed := primaryNode.Status == StatusFailed || primaryNode.Error != nil
		if !primaryFailed {
			// 主节点成功，跳过降级
			for _, fbID := range fg.FallbackNodes {
				if fbNode := graph.GetNode(fbID); fbNode != nil {
					fbNode.Status = StatusSkipped
					e.logger.Info("fallback node skipped (primary succeeded)",
						"primary_node_id", fg.PrimaryNodeID,
						"fallback_node_id", fbID,
					)
				}
			}
			continue
		}

		// 主节点失败，依次尝试备用节点（跳过熔断打开的后端）
		maxAttempts := fg.MaxAttempts
		if maxAttempts < 1 {
			maxAttempts = len(fg.FallbackNodes) + 1
		}
		fallbackSuccess := false
		for attemptIdx, fbID := range e.filterFallbackNodesByCircuit(graph, fg.FallbackNodes) {
			if attemptIdx >= maxAttempts-1 {
				break
			}
			e.logger.Warn("executing fallback node (primary failed)",
				"primary_node_id", fg.PrimaryNodeID,
				"fallback_node_id", fbID,
				"attempt", attemptIdx+1,
			)
			execErr := e.executeLayerNode(ctx, graph, execCtx, fbID, pipeline)
			if execErr != nil {
				e.logger.Warn("fallback node also failed",
					"fallback_node_id", fbID,
					"error", execErr,
				)
				continue
			}
			if fbNode := graph.GetNode(fbID); fbNode != nil && fbNode.Status == StatusSuccess {
				fallbackSuccess = true
				e.logger.Info("fallback node succeeded",
					"fallback_node_id", fbID,
				)
				break
			}
		}

		if !fallbackSuccess {
			return nil, fmt.Errorf("all fallback attempts failed for primary node %s", fg.PrimaryNodeID)
		}
	}

	// 8. 构建输出
	lastOutput := execCtx.GetLastOutput()
	if lastOutput == nil {
		return nil, fmt.Errorf("pipeline produced no output")
	}

	execLog := execCtx.GetExecutionLog()
	e.logger.Info("pipeline execution finished",
		"pipeline_id", pipeline.ID,
		"duration_ms", execLog.Duration,
		"success", execLog.Success,
		"total_tokens", execLog.TotalTokens,
	)

	outputContent := resolvePipelineOutputContent(execCtx, lastOutput)

	pipelineOut := &PipelineOutput{
		Content:      outputContent,
		Messages:     lastOutput.Messages,
		Metadata:     lastOutput.Metadata,
		ExecutionLog: execLog,
		Passed:       lastOutput.Passed,
		Score:        lastOutput.Score,
		NodeOutputs:  execCtx.GetAllResults(),
		LastNode:     execCtx.GetCurrentNode(),
	}
	if cacheResult, ok := execCtx.GetResult("cache_read"); ok && cacheResult != nil && cacheResult.Metadata != nil {
		if hit, ok := cacheResult.Metadata["cache_hit"].(bool); ok && hit {
			if pipelineOut.Metadata == nil {
				pipelineOut.Metadata = make(map[string]interface{})
			}
			for k, v := range cacheResult.Metadata {
				pipelineOut.Metadata[k] = v
			}
		}
	}
	if lastOutput.Feedback != "" {
		pipelineOut.Feedback = lastOutput.Feedback
	}
	if len(lastOutput.Suggestions) > 0 {
		pipelineOut.Suggestions = lastOutput.Suggestions
	}

	// 合并所有节点的 tool_calls 到顶层
	// 包括 LLM 返回的 tool_calls 和 Pipeline 注入的 tool_calls
	allToolCalls := e.mergeToolCalls(execCtx)
	if len(allToolCalls) > 0 {
		pipelineOut.ToolCalls = allToolCalls
		// 如果有 tool_calls，设置 finish_reason
		if pipelineOut.FinishReason == "" {
			pipelineOut.FinishReason = "tool_calls"
		}
	}

	if lastOutput.FinishReason != "" && len(allToolCalls) == 0 {
		pipelineOut.FinishReason = lastOutput.FinishReason
	}
	if lastOutput.ReasoningContent != "" {
		pipelineOut.ReasoningContent = lastOutput.ReasoningContent
	}

	// 存储钩子：流水线完成后归档执行记录
	if hook := e.getOrCreateStorageHook(pipeline); hook != nil {
		hook.OnPipelineComplete(ctx, execCtx)
	}

	return pipelineOut, nil
}

// filterRoutedNodes 路由预过滤：从同层节点中选出唯一应执行的分支。
//
// 规则：
//  1. 如果有非 default 节点匹配了 selected_route，只返回该匹配节点。
//  2. 如果没有非 default 匹配，但有 default 节点，返回 default 节点（兜底）。
//  3. 没有路由配置的节点不受影响。
func (e *PipelineEngine) filterRoutedNodes(nodeIDs []string, graph *ExecutionGraph, execCtx *ExecutionContext) []string {
	var routeNodes []*ExecutionNode
	var defaultNodeIDs []string
	var plainNodeIDs []string
	for _, id := range nodeIDs {
		node := graph.GetNode(id)
		if node == nil || node.Config.RouteConfig == nil {
			plainNodeIDs = append(plainNodeIDs, id)
			continue
		}
		if node.Config.RouteConfig.IsDefault {
			defaultNodeIDs = append(defaultNodeIDs, id)
		} else {
			routeNodes = append(routeNodes, node)
		}
	}

	// 没有路由节点，无需过滤
	if len(routeNodes) == 0 && len(defaultNodeIDs) == 0 {
		return nodeIDs
	}

	// 只有 default 节点（没有非 default 路由节点），直接返回全部
	if len(routeNodes) == 0 {
		return nodeIDs
	}

	// 查找匹配的非 default 节点
	matchedNodeID := ""
	for _, node := range routeNodes {
		rc := node.Config.RouteConfig
		routerResult, ok := execCtx.GetResult(rc.RouterNodeID)
		if !ok || routerResult == nil || routerResult.Metadata == nil {
			continue
		}
		selectedRoute, _ := routerResult.Metadata["selected_route"].(string)
		if selectedRoute == rc.RouteValue || selectedRoute == node.Config.ID {
			matchedNodeID = node.Config.ID
			break
		}
	}

	result := make([]string, 0, len(nodeIDs))

	if matchedNodeID != "" {
		// 匹配到非 default 节点 → 仅保留该匹配节点，跳过其他所有路由节点
		for _, id := range nodeIDs {
			node := graph.GetNode(id)
			if node == nil || node.Config.RouteConfig == nil {
				result = append(result, id)
				continue
			}
			if id == matchedNodeID {
				result = append(result, id)
			} else {
				node.Status = StatusSkipped
				e.logger.Info("node skipped: suppressed by matched non-default branch",
					"node_id", id,
					"matched_node_id", matchedNodeID,
				)
			}
		}
	} else if len(defaultNodeIDs) > 0 {
		// 没有非 default 匹配 → 仅保留 default 节点（兜底），跳过其他路由节点
		for _, id := range nodeIDs {
			node := graph.GetNode(id)
			if node == nil || node.Config.RouteConfig == nil {
				result = append(result, id)
				continue
			}
			if node.Config.RouteConfig.IsDefault {
				result = append(result, id)
			} else {
				node.Status = StatusSkipped
				e.logger.Info("node skipped: no route matched, falling back to default",
					"node_id", id,
				)
			}
		}
	} else {
		// 没有匹配也没有 default → 保留所有非路由节点（不应发生，防御性处理）
		for _, id := range nodeIDs {
			node := graph.GetNode(id)
			if node != nil && node.Config.RouteConfig != nil {
				node.Status = StatusSkipped
				continue
			}
			result = append(result, id)
		}
	}

	return result
}

// executeLayerNode 执行单个节点（供分层并行调用）
// 返回 error 表示致命错误需要终止流水线；返回 nil 表示节点正常完成或跳过/降级
func (e *PipelineEngine) executeLayerNode(ctx context.Context, graph *ExecutionGraph, execCtx *ExecutionContext, nodeID string, pipeline *AgentPatternPipeline) error {
	execNode := graph.GetNode(nodeID)
	if execNode == nil {
		return nil
	}

	// 检查执行条件
	// 无论是否有依赖，都在执行前评估条件
	// 对于有依赖的节点，依赖节点的结果已经保存在 execCtx 中，可以用于条件评估
	if execNode.Config.Condition != "" {
		evaluator := NewConditionEvaluator(execCtx)
		conditionResult := evaluator.Evaluate(execNode.Config.Condition)
		e.logger.Info("condition evaluation before execution",
			"node_id", nodeID,
			"condition", execNode.Config.Condition,
			"result", conditionResult,
		)
		if !conditionResult {
			execNode.Status = StatusSkipped
			e.logger.Info("node skipped due to condition",
				"node_id", nodeID,
				"condition", execNode.Config.Condition,
			)
			return nil
		}
	}

	// 分支路由判断：如果该节点配置了 RouteConfig，检查是否匹配上游路由节点的选择
	if rc := execNode.Config.RouteConfig; rc != nil {
		routerResult, ok := execCtx.GetResult(rc.RouterNodeID)
		if !ok || routerResult == nil || routerResult.Metadata == nil {
			execNode.Status = StatusSkipped
			e.logger.Warn("node skipped: route result not available",
				"node_id", nodeID,
				"router_node_id", rc.RouterNodeID,
				"expected_route", rc.RouteValue,
			)
			return nil
		}

		selectedRoute, _ := routerResult.Metadata["selected_route"].(string)
		// 兼容两种写法：selected_route 可以是 route_value（如 "code"）也可以是节点 ID（如 "code-generator"）
		matched := selectedRoute == rc.RouteValue || selectedRoute == nodeID
		if !matched && !rc.IsDefault {
			execNode.Status = StatusSkipped
			e.logger.Info("node skipped due to route selection",
				"node_id", nodeID,
				"router_node_id", rc.RouterNodeID,
				"selected_route", selectedRoute,
				"expected_route", rc.RouteValue,
				"is_default", rc.IsDefault,
			)
			return nil
		}
	}

	retryAttempts := 0
	if execNode.Config.Retry != nil {
		retryAttempts = execNode.Config.Retry.MaxAttempts
	}
	execFields := AppendRequestIDFields(ctx,
		"node_id", nodeID,
		"node_type", execNode.Config.Type,
		"backend", execNode.Config.Config.Backend,
		"model", execNode.Config.Config.Model,
		"timeout_s", execNode.Config.Timeout,
		"max_retry", retryAttempts,
	)
	e.logger.Info("executing node", execFields...)

	// 存储钩子：节点执行前加载上下文
	if hook := e.getOrCreateStorageHook(pipeline); hook != nil {
		hook.OnNodeStart(ctx, nodeID, execCtx)
	}

	// 准备输入（调试：记录 execCtx 状态）
	if e.logger != nil {
		if inputVar, ok := execCtx.GetVariable("input"); ok {
			prepFields := AppendRequestIDFields(ctx,
				"node_id", nodeID,
				"input_preview", utils.TruncateString(fmt.Sprintf("%v", inputVar), 100),
			)
			e.logger.Info("executeLayerNode: preparing input for node", prepFields...)
		} else {
			e.logger.Warn("executeLayerNode: original input variable NOT FOUND in execCtx",
				"node_id", nodeID,
			)
		}
	}
	nodeInput := e.prepareNodeInput(execNode.Config, execCtx)
	resolvedBackendID := nodeBackendID(execNode.Config, execNode.Config.Config)

	// 执行节点
	output, err := e.executeNode(ctx, execNode.Config, nodeInput)
	skippedCircuit := isCircuitBreakerSkipError(err)
	if skippedCircuit {
		e.logger.Warn("node skipped: circuit breaker open",
			"node_id", nodeID,
			"backend_id", resolvedBackendID,
		)
	}
	recordNodeCircuitOutcome(resolvedBackendID, err == nil, skippedCircuit)
	if err != nil {
		// 尝试策略降级：节点级 → 流水线级
		if policy := e.resolveFallbackPolicy(execNode.Config, pipeline); policy != nil {
			fallbackOutput, fallbackErr := e.executePolicyFallback(ctx, execNode.Config, nodeInput, policy)
			if fallbackErr == nil && fallbackOutput != nil {
				output = fallbackOutput
				execNode.Status = StatusSuccess
				execNode.Output = output
				e.logger.Info("node execution succeeded via fallback policy",
					"node_id", nodeID,
					"policy_id", policy.ID,
				)
				// 记录降级元数据
				if output.Metadata == nil {
					output.Metadata = make(map[string]interface{})
				}
				output.Metadata["fallback_policy_id"] = policy.ID
				output.Metadata["fallback_used"] = true
				output.Metadata["fallback_from_node"] = nodeID
				output.Metadata["fallback_from_backend"] = resolvedBackendID
			} else {
				e.logger.Warn("fallback policy exhausted",
					"node_id", nodeID,
					"policy_id", policy.ID,
					"error", fallbackErr,
				)
			}
		}

		// 如果降级未成功，继续原有错误处理
		if output == nil {
			execNode.Status = StatusFailed
			execNode.Error = err

			if pipeline.GlobalConfig.BypassOnError {
			bypassReason := MaskSensitiveData(err.Error())
			e.logger.Warn("node execution failed, bypass with fallback",
				"node_id", nodeID,
				"error", bypassReason,
			)
			lastOutput := execCtx.GetLastOutput()
			if hasUsableBypassOutput(lastOutput) {
				output = cloneNodeOutput(lastOutput)
				if output.Metadata == nil {
					output.Metadata = make(map[string]interface{})
				}
				output.Metadata["bypass"] = nodeID            // 字符串值，与 SSE 事件 schema 兼容
				output.Metadata["bypass_node"] = nodeID
				output.Metadata["bypass_reason"] = bypassReason
				e.logger.Warn("node execution bypassed with prior output",
					"node_id", nodeID,
					"fallback_output_len", len(strings.TrimSpace(output.Content)),
				)
			} else {
				e.logger.Error("node execution failed and bypass fallback is unavailable",
					"node_id", nodeID,
					"error", bypassReason,
				)
				return fmt.Errorf("node %s execution failed and no usable fallback output is available: %w", nodeID, err)
			}
			e.logger.Warn("bypassing error with fallback output", "node_id", nodeID)
		} else {
				e.logger.Error("node execution failed",
					"node_id", nodeID,
					"error", err,
				)
				return fmt.Errorf("node %s execution failed: %w", nodeID, err)
			}
		}
	} else {
		execNode.Status = StatusSuccess
		execNode.Output = output
		doneFields := AppendRequestIDFields(ctx,
			"node_id", nodeID,
			"output_length", len(output.Content),
			"response_preview", MaskSensitiveData(utils.TruncateString(output.Content, defaultMessagesPreviewMax)),
		)
		e.logger.Info("node execution completed", doneFields...)

		// 存储钩子：节点执行后保存结果
		if hook := e.getOrCreateStorageHook(pipeline); hook != nil {
			hook.OnNodeComplete(ctx, nodeID, output, execCtx)
		}
	}

	execCtx.SetResult(nodeID, output)
	execCtx.SetCurrentNode(nodeID)

	return nil
}

func cloneNodeOutput(src *NodeOutput) *NodeOutput {
	if src == nil {
		return nil
	}
	dst := &NodeOutput{
		Content:  src.Content,
		Passed:   src.Passed,
		Score:    src.Score,
		Feedback: src.Feedback,
		IsStream: src.IsStream,
	}
	if len(src.Messages) > 0 {
		dst.Messages = append([]Message(nil), src.Messages...)
	}
	if len(src.Suggestions) > 0 {
		dst.Suggestions = append([]string(nil), src.Suggestions...)
	}
	if len(src.StreamData) > 0 {
		dst.StreamData = append([]cache.StreamChunk(nil), src.StreamData...)
	}
	if src.Metadata != nil {
		dst.Metadata = make(map[string]interface{}, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	return dst
}

func hasUsableBypassOutput(output *NodeOutput) bool {
	if output == nil {
		return false
	}
	if strings.TrimSpace(output.Content) != "" {
		return true
	}
	if output.Passed != nil || output.Score != nil {
		return true
	}
	if strings.TrimSpace(output.Feedback) != "" || len(output.Suggestions) > 0 {
		return true
	}
	return false
}

// resolveFallbackPolicy 解析节点应使用的降级策略。
// 优先级：节点级 FallbackPolicyID → 流水线级 FallbackPolicyID → 自动策略。
func (e *PipelineEngine) resolveFallbackPolicy(nodeConfig PipelineNodeConfig, pipeline *AgentPatternPipeline) *config.GlobalFallbackPolicy {
	store := config.GetFallbackPolicyStore()

	// 1. 节点级策略
	if nodeConfig.FallbackPolicyID != "" {
		if p := store.GetEnabled(nodeConfig.FallbackPolicyID); p != nil {
			return p
		}
	}

	// 2. 流水线级策略
	if pipeline.GlobalConfig.FallbackPolicyID != "" {
		if p := store.GetEnabled(pipeline.GlobalConfig.FallbackPolicyID); p != nil {
			return p
		}
	}

	// 3. 流水线仍使用旧版 FallbackGroups 时，不走新策略
	if len(pipeline.GlobalConfig.FallbackGroups) > 0 {
		return nil
	}

	// 4. 自动策略（同模型跨后端）
	model := nodeConfig.Config.Model
	if model == "" {
		model = "{{requested_model}}"
	}
	return store.BuildAutoPolicy(model)
}

// executePolicyFallback 执行策略降级：按优先级依次尝试备选 backend+model。
func (e *PipelineEngine) executePolicyFallback(
	ctx context.Context,
	originalConfig PipelineNodeConfig,
	input *NodeInput,
	policy *config.GlobalFallbackPolicy,
) (*NodeOutput, error) {
	originalBackend := originalConfig.Config.Backend
	originalModel := originalConfig.Config.Model

	for _, rule := range policy.SortedRules() {
		// 解析占位符 {{system.default_backend}} / {{requested_model}} 等
		resolvedBackend, resolvedModel := ResolveVirtualVars(rule.BackendID, rule.Model)

		// 对于 {{requested_model}}，使用客户端请求的模型
		if rule.Model == "{{requested_model}}" {
			if input != nil && input.Metadata != nil {
				if m, ok := input.Metadata["model"].(string); ok && m != "" {
					resolvedModel = m
				}
			}
			if resolvedModel == "" {
				resolvedModel = originalModel
			}
		}

		// 跳过与原始配置相同的规则（解析后比较）
		if resolvedBackend == originalBackend && resolvedModel == originalModel {
			continue
		}

		// 跳过熔断中的后端
		if isCircuitOpenForBackend(resolvedBackend) {
			e.logger.Warn("fallback rule skipped: circuit breaker open",
				"policy_id", policy.ID,
				"backend_id", resolvedBackend,
			)
			continue
		}

		// 构建降级节点配置
		fallbackConfig := originalConfig
		fallbackConfig.Config.Backend = resolvedBackend
		fallbackConfig.Config.Model = resolvedModel
		fallbackConfig.FallbackPolicyID = "" // 防止递归降级

		// 设置超时
		if rule.TimeoutSec > 0 {
			fallbackConfig.Timeout = rule.TimeoutSec
		}

		e.logger.Info("trying fallback rule",
			"policy_id", policy.ID,
			"backend_id", resolvedBackend,
			"model", resolvedModel,
			"priority", rule.Priority,
		)

		// 执行降级节点
		output, err := e.executeNode(ctx, fallbackConfig, input)
		if err != nil {
			e.logger.Warn("fallback rule failed",
				"policy_id", policy.ID,
				"backend_id", resolvedBackend,
				"model", resolvedModel,
				"error", err,
			)
			continue
		}

		return output, nil
	}

	return nil, fmt.Errorf("all fallback rules exhausted for policy %s", policy.ID)
}

func (e *PipelineEngine) prepareNodeInput(config PipelineNodeConfig, execCtx *ExecutionContext) *NodeInput {
	input := &NodeInput{
		Metadata:        make(map[string]interface{}),
		Context:         make(map[string]interface{}),
		UpstreamResults: make(map[string]*NodeOutput),
	}

	// 将流水线原始输入注入 metadata["question"]，作为通用的"原始输入"快捷访问键。
	// 任何节点都可以通过 {{.question}} 拿到原始用户输入，而无需 template_vars 绑定。
	// （等价于 template_vars 配置 question = context.input_content）
	if content, ok := execCtx.GetVariable("input"); ok {
		input.Metadata["question"] = content
	}
	if metadata, ok := execCtx.GetVariable("metadata"); ok {
		if m, ok := metadata.(map[string]interface{}); ok {
			for k, v := range m {
				input.Metadata[k] = v
			}
		}
	}

	// 将所有已执行节点的完整输出注入 UpstreamResults，供 template_vars 路径解析使用
	for nodeID, result := range execCtx.results {
		input.UpstreamResults[nodeID] = result
	}

	// 获取依赖节点的输出作为主输入内容
	if len(config.DependsOn) > 0 {
		for _, depID := range config.DependsOn {
			if result, ok := execCtx.GetResult(depID); ok {
				input.Content = result.Content
				input.Messages = result.Messages
				input.Metadata[depID] = map[string]interface{}{
					"content":     result.Content,
					"messages":    result.Messages,
					"metadata":    result.Metadata,
					"passed":      result.Passed,
					"score":       result.Score,
					"feedback":    result.Feedback,
					"suggestions": result.Suggestions,
				}
			}
		}
	}

	// 根节点（如 generator）使用请求中的完整对话历史
	if len(input.Messages) == 0 {
		if msgs, ok := execCtx.GetVariable("messages"); ok {
			if pipelineMessages, ok := msgs.([]Message); ok && len(pipelineMessages) > 0 {
				input.Messages = pipelineMessages
			}
		}
		// 透传 tools 和 tool_choice（支持 function calling）
		if tools, ok := execCtx.GetVariable("tools"); ok && tools != nil {
			input.Tools = tools
		}
		if tc, ok := execCtx.GetVariable("tool_choice"); ok && tc != nil {
			input.ToolChoice = tc
		}
	}

	// 如果没有从依赖节点获取到内容，使用初始输入（特别重要：fallback 节点通常没有 DependsOn）
	if input.Content == "" {
		if content, ok := execCtx.GetVariable("input"); ok {
			if s, ok := content.(string); ok {
				input.Content = s
			} else {
				// 类型断言失败，记录警告
				if e.logger != nil {
					e.logger.Warn("prepareNodeInput: original input is not string, trying fmt.Sprint",
						"node_id", config.ID,
						"input_type", fmt.Sprintf("%T", content),
					)
					input.Content = fmt.Sprint(content)
				}
			}
		} else {
			if e.logger != nil {
				e.logger.Warn("prepareNodeInput: original input variable not found",
					"node_id", config.ID,
				)
			}
		}
	}

	if len(config.Inputs) > 0 {
		resolver := NewTemplateVarResolver(input, execCtx)
		for key, path := range config.Inputs {
			if key == "" || path == "" {
				continue
			}
			value, err := resolver.Resolve(path)
			if err != nil {
				if e.logger != nil {
					e.logger.Warn("failed to resolve node input binding",
						"node_id", config.ID,
						"input_key", key,
						"path", path,
						"error", err,
					)
				}
				continue
			}
			switch key {
			case "content":
				if value == nil {
					continue
				}
				if s, ok := value.(string); ok {
					input.Content = s
				} else {
					input.Content = fmt.Sprintf("%v", value)
				}
			case "metadata":
				if m, ok := value.(map[string]interface{}); ok {
					for mk, mv := range m {
						input.Metadata[mk] = mv
					}
				}
			default:
				input.Metadata[key] = value
			}
		}
	}

	return input
}

func (e *PipelineEngine) executeNode(ctx context.Context, config PipelineNodeConfig, input *NodeInput) (*NodeOutput, error) {
	// Config.Backend/Model 已在 Normalize() 阶段归一化，直接使用即可
	nodeConfig := config.Config

	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if config.Type == NodeTypeGenerator || config.Type == NodeTypeProcessor ||
		config.Type == NodeTypeReviewer || config.Type == NodeTypeAudit || config.Type == NodeTypeOptimize {
		applySchedulingOverrides(&nodeConfig, input, execCtx)
	}

	if backendID := nodeBackendID(config, nodeConfig); isCircuitOpenForBackend(backendID) {
		return nil, fmt.Errorf("circuit breaker open for backend %s", backendID)
	}

	// 创建节点实例。新格式优先按 implementation 解析；旧格式自动映射到 builtin.<type>。
	node, err := e.nodeRegistry.CreateFromConfig(config, nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	if setter, ok := node.(interface{ SetID(string) }); ok {
		setter.SetID(config.ID)
	}
	if setter, ok := node.(interface{ SetName(string) }); ok {
		setter.SetName(config.Name)
	}
	if setter, ok := node.(interface{ SetType(NodeType) }); ok {
		setter.SetType(config.Type)
	}

	// 将 PipelineNodeConfig 级别的 Timeout / Retry 注入节点（覆盖节点默认值）
	if config.Timeout > 0 {
		node.SetTimeout(config.Timeout)
	}
	if config.Retry != nil {
		node.SetRetryConfig(config.Retry)
	}

	// 注入能力代理
	if e.capabilityBroker != nil {
		if base, ok := node.(interface{ SetCapabilityBroker(CapabilityBroker) }); ok {
			base.SetCapabilityBroker(e.capabilityBroker)
		}
	}

	// Phase 4A: 注入缓存策略插件到 CacheNode
	// 当节点类型为 cache 且配置了 use_strategy_plugin 时，通过 CapabilityBroker 获取策略并注入
	if config.Type == NodeTypeCache {
		if cacheNode, ok := node.(interface {
			SetStrategyPlugin(CacheStrategyCapability)
			GetStrategyPlugin() CacheStrategyCapability
			IsUsingStrategyPlugin() bool
		}); ok && !cacheNode.IsUsingStrategyPlugin() {
			// 从节点配置中读取策略名称，默认使用 "exact"
			strategyName := "exact"
			if config.Config.CustomConfig != nil {
				if sn, ok := config.Config.CustomConfig["cache_strategy"].(string); ok && sn != "" {
					strategyName = sn
				}
			}
			if e.capabilityBroker != nil {
				strat, err := e.capabilityBroker.GetCacheStrategy(ctx, strategyName, []string{"cache.read", "cache.write"})
				if err != nil {
					e.logger.Debug("cache strategy not available (provider may not be configured)",
						"node_id", config.ID,
						"strategy", strategyName,
						"error", err,
					)
				} else if strat != nil {
					cacheNode.SetStrategyPlugin(strat)
					e.logger.Info("cache strategy plugin injected",
						"node_id", config.ID,
						"strategy", strat.StrategyName(),
					)
				}
			}
		}
	}

	// 初始化缓存节点的存储后端（如果节点支持）
	if initializer, ok := node.(interface {
		InitializeStorages(ctx context.Context) error
	}); ok {
		e.logger.Info("initializing storages for node",
			"node_id", config.ID,
			"node_type", config.Type,
		)
		if err := initializer.InitializeStorages(ctx); err != nil {
			// 尝试从 Config.CustomConfig 获取存储名称（用于诊断）
			readStorage := ""
			writeStorage := ""
			if config.Config.CustomConfig != nil {
				if rs, ok := config.Config.CustomConfig["read_storage_name"].(string); ok {
					readStorage = rs
				}
				if ws, ok := config.Config.CustomConfig["write_storage_name"].(string); ok {
					writeStorage = ws
				}
			}
			e.logger.Warn("failed to initialize storages for node",
				"node_id", config.ID,
				"node_type", config.Type,
				"read_storage", readStorage,
				"write_storage", writeStorage,
				"error", err,
			)
			// 存储初始化失败不阻断执行，节点会回退到默认行为
		} else {
			e.logger.Info("storages initialized for node",
				"node_id", config.ID,
			)
		}
	} else {
		e.logger.Debug("node does not support InitializeStorages",
			"node_id", config.ID,
			"node_type", config.Type,
		)
	}

	// 将 backendID 注入 context，供 CapabilityBroker 的 llm.call 使用。
	nodeExecCtx := ctx
	if config.Config.Backend != "" {
		nodeExecCtx = context.WithValue(ctx, backendIDContextKey{}, config.Config.Backend)
	}

	// 将 storage.Manager 注入 context，供 CacheNode.InitializeStorages 使用
	if e.storageManager != nil {
		nodeExecCtx = context.WithValue(nodeExecCtx, storageManagerKey{}, e.storageManager)
	}

	// 特殊处理 LoopController 节点
	if config.Type == NodeTypeLoopController {
		return e.executeLoopController(nodeExecCtx, config, input)
	}

	// 执行（带重试）
	return e.executeWithRetry(nodeExecCtx, node, input)
}

func (e *PipelineEngine) executeWithRetry(ctx context.Context, node PipelineNode, input *NodeInput) (*NodeOutput, error) {
	retryConfig := node.GetRetryConfig()
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}

	var lastErr error
	var lastOutput *NodeOutput
	nodeLog := NodeExecutionLog{
		NodeID:   node.ID(),
		NodeType: node.Type(),
	}

	// 填充插件相关信息
	if pluginNode, ok := node.(*PluginBackedNode); ok {
		nodeLog.Implementation = pluginNode.implementation
		nodeLog.Kind = pluginNode.kind
		if desc := pluginNode.plugin.Descriptor(); desc.Version != "" {
			nodeLog.PluginVersion = desc.Version
		}
		// 检查熔断状态
		if rnp, ok := pluginNode.plugin.(*RemoteNodePlugin); ok {
			if rnp.IsCircuitOpen() {
				nodeLog.CircuitState = "open"
			} else {
				nodeLog.CircuitState = "closed"
			}
		}
	} else {
		// 内置节点
		nodeLog.Implementation = "builtin." + node.Type().String()
		nodeLog.Kind = KindForBuiltinType(node.Type())
	}

	for attempt := 0; attempt <= retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := retryConfig.CalculateDelay(attempt)
			time.Sleep(delay)
			e.logger.Warn("retrying node execution",
				"node_id", node.ID(),
				"attempt", attempt,
				"delay", delay,
			)
			nodeLog.RetryCount = attempt
		}

		// 设置超时
		nodeCtx := ctx
		var cancel context.CancelFunc
		if node.GetTimeout() > 0 {
			nodeCtx, cancel = context.WithTimeout(ctx, time.Duration(node.GetTimeout())*time.Second)
		}

		startTime := time.Now()
		output, err := node.Execute(nodeCtx, input)
		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		if cancel != nil {
			cancel()
		}

		nodeLog.StartTime = startTime
		nodeLog.EndTime = endTime
		nodeLog.Duration = duration

		if err == nil {
			lastOutput = output
			nodeLog.Success = true
			nodeLog.ErrorMessage = ""
			// 从metadata中提取token使用量和模型信息
			if output != nil && output.Metadata != nil {
				if tokens, ok := output.Metadata["tokens"]; ok {
					nodeLog.OutputTokens = intFromAny(tokens)
				}
				if promptTokens, ok := output.Metadata["prompt_tokens"]; ok {
					nodeLog.InputTokens = intFromAny(promptTokens)
				}
				if model, ok := output.Metadata["model"]; ok {
					if modelStr, ok := model.(string); ok {
						nodeLog.Model = modelStr
					}
				}
			}
			// 填充输入和输出大小
			if input != nil {
				nodeLog.InputSize = estimateInputSize(input)
			}
			if output != nil {
				nodeLog.OutputSize = int64(len(output.Content))
			}
			// 记录成功指标
			GlobalPluginMetrics.RecordCall(nodeLog.Implementation, true, time.Duration(nodeLog.Duration)*time.Millisecond, nil)
			break
		}

		lastErr = err
		nodeLog.Success = false
		nodeLog.ErrorMessage = MaskSensitiveData(err.Error())
		// 记录失败指标
		GlobalPluginMetrics.RecordCall(nodeLog.Implementation, false, time.Duration(nodeLog.Duration)*time.Millisecond, err)

		// 错误分类：判断是否应重试
		errType, statusCode, providerErrCode := classifyNodeError(err)
		retryable := config.IsRetryableError(errType, statusCode, providerErrCode)

		if !retryable {
			e.logger.Warn("node execution failed with non-retryable error, stopping",
				"node_id", node.ID(),
				"attempt", attempt,
				"error_type", errType,
				"status_code", statusCode,
				"error", err,
			)
			break
		}

		if attempt < retryConfig.MaxAttempts {
			e.logger.Warn("node execution failed, will retry",
				"node_id", node.ID(),
				"attempt", attempt,
				"max_attempts", retryConfig.MaxAttempts,
				"error_type", errType,
				"status_code", statusCode,
				"error", err,
			)
		} else {
			e.logger.Error("node execution failed",
				"node_id", node.ID(),
				"attempt", attempt,
				"max_attempts", retryConfig.MaxAttempts,
				"error_type", errType,
				"status_code", statusCode,
				"error", err,
			)
		}
	}

	// 记录节点执行日志到执行上下文
	if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
		execCtx.AddNodeLog(nodeLog)
	}

	if lastErr != nil && lastOutput == nil {
		return nil, fmt.Errorf("node execution failed after %d attempts: %w", retryConfig.MaxAttempts, lastErr)
	}

	return lastOutput, nil
}

// classifyNodeError 从节点执行错误中提取错误类型、HTTP 状态码、提供方错误码。
// 返回值：(errorType, statusCode, providerErrorCode)
//
//	errorType: "http_status" | "timeout" | "network" | "provider_error" | "unknown"
func classifyNodeError(err error) (string, int, string) {
	if err == nil {
		return "unknown", 0, ""
	}
	msg := err.Error()

	// 超时错误
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "i/o timeout") {
		return "timeout", 0, ""
	}

	// 网络错误
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network") ||
		strings.Contains(msg, "dial tcp") {
		return "network", 0, ""
	}

	// HTTP 状态码错误（transparent_forward_node 格式："upstream returned %d: ..."）
	if strings.Contains(msg, "upstream returned ") {
		var code int
		if _, scanErr := fmt.Sscanf(msg, "transparent_forward node %*q: upstream returned %d", &code); scanErr == nil {
			return "http_status", code, ""
		}
	}

	// OpenAI 插件格式："API error (status %d): ..."
	if strings.Contains(msg, "API error (status ") {
		var code int
		if _, scanErr := fmt.Sscanf(msg, "API error (status %d)", &code); scanErr == nil {
			return "http_status", code, ""
		}
	}

	// 提供方 JSON 错误码（如 "rate_limit_error"）
	for _, code := range []string{
		"rate_limit_error", "server_error", "timeout", "insufficient_quota",
		"overloaded", "capacity_exceeded", "error",
	} {
		if strings.Contains(msg, code) {
			return "provider_error", 0, code
		}
	}

	return "unknown", 0, ""
}

// executionContextKey 用于在context中存储ExecutionContext的key
type executionContextKey struct{}

// loggerContextKey 用于在context中存储Logger的key
type loggerContextKey struct{}

// executeLoopController 执行循环控制器节点
func (e *PipelineEngine) executeLoopController(ctx context.Context, config PipelineNodeConfig, input *NodeInput) (*NodeOutput, error) {
	// 获取 LoopController 配置
	loopConfig := extractLoopConfig(config.Config.CustomConfig)
	if loopConfig == nil {
		return nil, fmt.Errorf("loop_controller node requires custom_config with loop configuration")
	}

	execCtx, _ := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if execCtx == nil {
		return nil, fmt.Errorf("execution context not found")
	}

	e.logger.Info("starting loop controller execution",
		"node_id", config.ID,
		"max_iterations", loopConfig.MaxIterations,
		"condition", loopConfig.Condition,
	)

	var lastOutput *NodeOutput
	var lastErr error

	for i := 0; i < loopConfig.MaxIterations; i++ {
		// 设置循环变量
		execCtx.SetVariable(loopConfig.LoopVariable, i)
		execCtx.SetVariable(fmt.Sprintf("%s_iteration", loopConfig.LoopVariable), i+1)

		e.logger.Info("loop iteration",
			"node_id", config.ID,
			"iteration", i+1,
			"max_iterations", loopConfig.MaxIterations,
		)

		// 执行子图
		subgraphOutput, err := e.executeSubgraph(ctx, config.ID, loopConfig.Subgraph, input, execCtx)
		if err != nil {
			lastErr = err
			e.logger.Error("subgraph execution failed",
				"node_id", config.ID,
				"iteration", i+1,
				"error", err,
			)
			break
		}

		lastOutput = subgraphOutput

		// 评估循环条件
		evaluator := NewConditionEvaluator(execCtx)
		shouldContinue := evaluator.Evaluate(loopConfig.Condition)

		e.logger.Info("loop condition evaluated",
			"node_id", config.ID,
			"iteration", i+1,
			"condition", loopConfig.Condition,
			"should_continue", shouldContinue,
		)

		if !shouldContinue {
			e.logger.Info("loop condition met, exiting loop",
				"node_id", config.ID,
				"iteration", i+1,
			)
			break
		}
	}

	if lastOutput == nil {
		return &NodeOutput{
			Content: "",
			Metadata: map[string]interface{}{
				loopConfig.LoopVariable: loopConfig.MaxIterations,
				"iteration":             loopConfig.MaxIterations,
				"max_iterations":        loopConfig.MaxIterations,
				"condition":             loopConfig.Condition,
				"loop_completed":        true,
				"final_iteration":       loopConfig.MaxIterations,
			},
		}, lastErr
	}

	// 添加循环完成标记
	if lastOutput.Metadata == nil {
		lastOutput.Metadata = make(map[string]interface{})
	}
	lastOutput.Metadata["loop_completed"] = true
	lastOutput.Metadata["final_iteration"] = execCtx.variables[loopConfig.LoopVariable]
	lastOutput.Metadata[loopConfig.LoopVariable] = execCtx.variables[loopConfig.LoopVariable]

	return lastOutput, lastErr
}

// LoopConfig 循环控制器配置
type LoopConfig struct {
	MaxIterations int
	Condition     string
	LoopVariable  string
	Subgraph      []PipelineNodeConfig
}

// extractLoopConfig 从 custom_config 中提取循环配置
func extractLoopConfig(customConfig map[string]interface{}) *LoopConfig {
	if customConfig == nil {
		return nil
	}

	config := &LoopConfig{
		MaxIterations: 3,
		LoopVariable:  "loop_count",
	}

	if mi, ok := customConfig["max_iterations"]; ok {
		switch v := mi.(type) {
		case int:
			config.MaxIterations = v
		case float64:
			config.MaxIterations = int(v)
		case int64:
			config.MaxIterations = int(v)
		}
	}

	if cond, ok := customConfig["condition"].(string); ok {
		config.Condition = cond
	}

	if lv, ok := customConfig["loop_variable"].(string); ok {
		config.LoopVariable = lv
	}

	if sg, ok := customConfig["subgraph"].([]interface{}); ok {
		for _, item := range sg {
			if nodeConfigMap, ok := item.(map[string]interface{}); ok {
				nodeConfig := parsePipelineNodeConfigFromMap(nodeConfigMap)
				config.Subgraph = append(config.Subgraph, nodeConfig)
			}
		}
	}

	return config
}

// executeSubgraph 执行子图
func (e *PipelineEngine) executeSubgraph(ctx context.Context, parentNodeID string, subgraph []PipelineNodeConfig, input *NodeInput, execCtx *ExecutionContext) (*NodeOutput, error) {
	if len(subgraph) == 0 {
		return nil, fmt.Errorf("subgraph is empty")
	}

	e.logger.Info("executing subgraph",
		"parent_node_id", parentNodeID,
		"node_count", len(subgraph),
	)

	var lastOutput *NodeOutput

	for _, nodeConfig := range subgraph {
		// 准备节点输入
		nodeInput := e.prepareNodeInput(nodeConfig, execCtx)

		// 执行节点
		output, err := e.executeNode(ctx, nodeConfig, nodeInput)
		if err != nil {
			return nil, fmt.Errorf("subgraph node %s execution failed: %w", nodeConfig.ID, err)
		}

		// 保存结果
		execCtx.SetResult(nodeConfig.ID, output)
		execCtx.SetCurrentNode(nodeConfig.ID)
		lastOutput = output
	}

	return lastOutput, nil
}

// parsePipelineNodeConfigFromMap 从 map 解析 PipelineNodeConfig
func parsePipelineNodeConfigFromMap(m map[string]interface{}) PipelineNodeConfig {
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
		config.Config.Backend = backend
	}
	if model, ok := m["model"].(string); ok {
		config.Config.Model = model
	}
	if timeout, ok := m["timeout"].(int); ok {
		config.Timeout = timeout
	}
	if timeoutFloat, ok := m["timeout"].(float64); ok {
		config.Timeout = int(timeoutFloat)
	}

	return config
}

// intFromAny 从interface{}中提取int值
func intFromAny(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

// DefaultLLMProvider 默认的 LLM 提供者实现
// 使用全局后端管理器和插件管理器创建 LLM 客户端
type DefaultLLMProvider struct {
	backendManager *backend.Manager
	pluginManager  *plugin.Manager
}

// NewDefaultLLMProvider 创建默认的 LLM 提供者
func NewDefaultLLMProvider(backendManager *backend.Manager, pluginManager *plugin.Manager) *DefaultLLMProvider {
	return &DefaultLLMProvider{
		backendManager: backendManager,
		pluginManager:  pluginManager,
	}
}

// ConfigIncompleteError 配置未完成错误
type ConfigIncompleteError struct {
	Code    string
	Message string
	Hint    string
}

func (e *ConfigIncompleteError) Error() string {
	return fmt.Sprintf("[%s] %s (Hint: %s)", e.Code, e.Message, e.Hint)
}

// CreateClient 创建 LLM 客户端（支持虚拟变量解析和降级策略）
func (p *DefaultLLMProvider) CreateClient(backendID, model string) (LLMClient, error) {
	if p.backendManager == nil {
		return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend manager not available"))
	}

	// 1. 解析虚拟变量
	resolvedBackend, resolvedModel := p.resolveVirtualVars(backendID, model)
	// 1b. 兜底：已有 default_backend，但 default_model 为空时，取该后端的首选模型
	if strings.TrimSpace(resolvedModel) == "" && strings.TrimSpace(resolvedBackend) != "" && p.backendManager != nil {
		if b, err := p.backendManager.Get(resolvedBackend); err == nil {
			if preferred := backend.PreferredDefaultModel(b); preferred != "" {
				resolvedModel = preferred
				logger.Info("Using backend preferred model as default_model fallback",
					logger.GetField("backend_id", resolvedBackend),
					logger.GetField("model", resolvedModel))
			}
		}
	}

	// 2. 检查是否配置了默认后端
	if resolvedBackend == "" {
		return nil, &ConfigIncompleteError{
			Code:    "DEFAULT_BACKEND_NOT_CONFIGURED",
			Message: "系统未配置默认后端，无法执行流水线",
			Hint:    "请在系统设置中配置 Default Backend ID 和 Default Model，或设置环境变量 LLM_PROXY_DEFAULT_BACKEND_ID 和 LLM_PROXY_DEFAULT_MODEL",
		}
	}

	// 3. 尝试使用解析后的后端
	backendConfig, err := p.backendManager.Get(resolvedBackend)
	if err != nil {
		// 4. 尝试使用降级后端
		fallbackBackend, fallbackModel := p.resolveFallback(resolvedBackend, resolvedModel)
		if fallbackBackend != nil {
			logger.Info("Using fallback backend",
				logger.GetField("requested_backend", resolvedBackend),
				logger.GetField("fallback_backend", fallbackBackend.ID),
				logger.GetField("requested_model", resolvedModel),
				logger.GetField("fallback_model", fallbackModel))
			return p.createClientFromConfig(fallbackBackend, fallbackModel)
		}
		return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend %q not found and no fallback available: %w", resolvedBackend, err))
	}

	if !backendConfig.Enabled {
		// 尝试使用降级后端
		fallbackBackend, fallbackModel := p.resolveFallback(resolvedBackend, resolvedModel)
		if fallbackBackend != nil {
			logger.Info("Using fallback backend (requested is disabled)",
				logger.GetField("requested_backend", resolvedBackend),
				logger.GetField("fallback_backend", fallbackBackend.ID))
			return p.createClientFromConfig(fallbackBackend, fallbackModel)
		}
		return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend %q is disabled", resolvedBackend))
	}

	if !backend.IsUsableLLMBackend(backendConfig) {
		// 尝试使用降级后端
		fallbackBackend, fallbackModel := p.resolveFallback(resolvedBackend, resolvedModel)
		if fallbackBackend != nil {
			logger.Info("Using fallback backend (requested is not usable)",
				logger.GetField("requested_backend", resolvedBackend),
				logger.GetField("fallback_backend", fallbackBackend.ID))
			return p.createClientFromConfig(fallbackBackend, fallbackModel)
		}
		if strings.EqualFold(strings.TrimSpace(backendConfig.Type), "ollama") {
			return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend %q missing base_url", resolvedBackend))
		}
		return nil, backend.NewNoBackendAPIKeyError(resolvedBackend)
	}

	return p.createClientFromConfig(backendConfig, resolvedModel)
}

// resolveVirtualVars 解析虚拟变量
func (p *DefaultLLMProvider) resolveVirtualVars(backendID, model string) (string, string) {
	return ResolveVirtualVars(backendID, model)
}

// ResolveVirtualVars 解析 {{system.default_backend}} 和 {{system.default_model}} 虚拟变量。
// 空字符串也触发解析（等价于虚拟变量占位符）。
// 若已解析出 default_backend 但 default_model 仍为空，则兜底使用该后端的首选模型
//（ProbeModel → SupportedModels[0]）。
func ResolveVirtualVars(backendID, model string) (string, string) {
	cfg := config.Get()
	if cfg == nil {
		return backendID, model
	}

	resolvedBackend := backendID
	resolvedModel := model

	if backendID == "{{system.default_backend}}" || backendID == "" {
		resolvedBackend = cfg.Proxy.DefaultBackendID
	}

	if model == "{{system.default_model}}" || model == "" {
		resolvedModel = cfg.Proxy.DefaultModel
	}

	if strings.TrimSpace(resolvedModel) == "" && strings.TrimSpace(resolvedBackend) != "" {
		if mgr := backend.GetManager(); mgr != nil {
			if b, err := mgr.Get(resolvedBackend); err == nil {
				resolvedModel = backend.PreferredDefaultModel(b)
			}
		}
	}

	return resolvedBackend, resolvedModel
}

// resolveFallback 解析降级配置
func (p *DefaultLLMProvider) resolveFallback(requestedBackend, requestedModel string) (*backend.BackendConfig, string) {
	cfg := config.Get()
	if cfg == nil {
		return nil, ""
	}

	// 优先使用配置的降级后端
	if cfg.Proxy.FallbackBackendID != "" {
		fallbackConfig, err := p.backendManager.Get(cfg.Proxy.FallbackBackendID)
		if err == nil && fallbackConfig.Enabled && backend.IsUsableLLMBackend(fallbackConfig) {
			fallbackModel := cfg.Proxy.FallbackModel
			if fallbackModel == "" {
				fallbackModel = requestedModel
			}
			return fallbackConfig, fallbackModel
		}
	}

	// 如果降级后端也不可用，尝试使用默认后端（如果与请求的后端不同）
	if cfg.Proxy.DefaultBackendID != "" && cfg.Proxy.DefaultBackendID != requestedBackend {
		defaultConfig, err := p.backendManager.Get(cfg.Proxy.DefaultBackendID)
		if err == nil && defaultConfig.Enabled && backend.IsUsableLLMBackend(defaultConfig) {
			return defaultConfig, requestedModel
		}
	}

	// 最后尝试使用任意可用后端
	enabled := p.backendManager.GetEnabled()
	if len(enabled) > 0 {
		for _, b := range enabled {
			if backend.IsUsableLLMBackend(b) {
				return b, requestedModel
			}
		}
	}

	return nil, ""
}

// createClientFromConfig 从配置创建客户端
func (p *DefaultLLMProvider) createClientFromConfig(cfg *backend.BackendConfig, model string) (LLMClient, error) {
	if !backend.IsUsableLLMBackend(cfg) {
		if strings.EqualFold(strings.TrimSpace(cfg.Type), "ollama") {
			return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend %q missing base_url", cfg.ID))
		}
		return nil, backend.NewNoBackendAPIKeyError(cfg.ID)
	}

	pluginName := getPluginNameForBackend(cfg)
	backendPlugin, err := p.pluginManager.GetBackend(pluginName)
	if err != nil {
		return nil, fmt.Errorf("backend plugin %q not found: %w", pluginName, err)
	}

	return &llmClient{
		backendPlugin: backendPlugin,
		backendConfig: cfg,
		model:         model,
	}, nil
}

// llmClient LLM 客户端实现
type llmClient struct {
	backendPlugin plugin.BackendPlugin
	backendConfig *backend.BackendConfig
	model         string
}

// Chat 调用 LLM（非流式）
func (c *llmClient) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// 构建 ProxyRequest
	proxyReq := &plugin.ProxyRequest{
		Model:       c.model,
		Messages:    convertMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
		BackendID:   c.backendConfig.ID,
	}
	// 透传 tools/tool_choice（通过 RawBody，让后端插件能完整构造 function calling 请求）
	// Prefer resolved client model over req.Model (may still contain {{system.default_model}}).
	rawBody := buildRawBodyFromLLMRequest(req, false)
	applyResolvedModelToRawBody(rawBody, c.model)
	proxyReq.RawBody = rawBody

	// 调用后端插件
	resp, err := c.backendPlugin.CallModel(ctx, proxyReq)
	if err != nil {
		return nil, fmt.Errorf("backend call failed: %w", err)
	}

	return &LLMResponse{
		Model:            resp.Model,
		Content:          resp.Content,
		ReasoningContent: resp.ReasoningContent,
		TokenUsage:       resp.TokensUsed,
		ToolCalls:        convertPipelineToolCalls(resp.ToolCalls),
		FinishReason:     resp.FinishReason,
	}, nil
}

// convertPipelineToolCalls 将 plugin.ToolCall 转为 pipeline.ToolCall
func convertPipelineToolCalls(tcs []plugin.ToolCall) []ToolCall {
	if len(tcs) == 0 {
		return nil
	}
	result := make([]ToolCall, len(tcs))
	for i, tc := range tcs {
		result[i] = ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// convertToPluginToolCalls 将 pipeline.ToolCall 转为 plugin.ToolCall
func convertToPluginToolCalls(tcs []ToolCall) []plugin.ToolCall {
	if len(tcs) == 0 {
		return nil
	}
	result := make([]plugin.ToolCall, len(tcs))
	for i, tc := range tcs {
		result[i] = plugin.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: plugin.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// ChatStream 流式调用 LLM
func (c *llmClient) ChatStream(ctx context.Context, req *LLMRequest) (<-chan plugin.StreamChunk, error) {
	proxyReq := &plugin.ProxyRequest{
		Model:       c.model,
		Messages:    convertMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		BackendID:   c.backendConfig.ID,
	}
	// 透传 tools/tool_choice
	rawBody := buildRawBodyFromLLMRequest(req, true)
	applyResolvedModelToRawBody(rawBody, c.model)
	proxyReq.RawBody = rawBody

	return c.backendPlugin.CallModelStream(ctx, proxyReq)
}

func applyResolvedModelToRawBody(rawBody map[string]interface{}, model string) {
	if rawBody == nil {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" || model == "{{system.default_model}}" {
		delete(rawBody, "model")
		return
	}
	rawBody["model"] = model
}

// buildRawBodyFromLLMRequest 构造 RawBody，包含 messages/tools/tool_choice 等完整字段，
// 让后端插件（如 openai-backend）能通过 RawBody 透传 function calling 相关字段。
func buildRawBodyFromLLMRequest(req *LLMRequest, stream bool) map[string]interface{} {
	rawBody := map[string]interface{}{
		"messages": convertMessagesToRaw(req.Messages),
		"stream":   stream,
	}
	if req.Model != "" {
		rawBody["model"] = req.Model
	}
	if req.Temperature != 0 {
		rawBody["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		rawBody["max_tokens"] = req.MaxTokens
	}
	if req.Tools != nil {
		rawBody["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		rawBody["tool_choice"] = req.ToolChoice
	}
	return rawBody
}

// convertMessagesToRaw 将 pipeline.Message 转为 map（保留 tool_calls/tool_call_id）
func convertMessagesToRaw(messages []Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		if msg.ReasoningContent != "" {
			m["reasoning_content"] = msg.ReasoningContent
		}
		if len(msg.ToolCalls) > 0 {
			tcs := make([]map[string]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				tcs[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}
			}
			m["tool_calls"] = tcs
		}
		result[i] = m
	}
	return result
}

// convertMessages 将 pipeline.Message 转换为 plugin.Message（透传 tool_calls 和 tool_call_id）
func convertMessages(messages []Message) []plugin.Message {
	result := make([]plugin.Message, len(messages))
	for i, msg := range messages {
		m := plugin.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]plugin.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				m.ToolCalls[j] = plugin.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: plugin.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		result[i] = m
	}
	return result
}

// getPluginNameForBackend 根据后端配置获取插件名称
func getPluginNameForBackend(b *backend.BackendConfig) string {
	backendType := strings.ToLower(b.Type)
	switch backendType {
	case "ollama":
		return "ollama-backend"
	case "anthropic":
		return "anthropic-backend"
	case "openai":
		return "openai-backend"
	default:
		// 检查BaseURL做兼容性判断
		baseURL := strings.ToLower(b.BaseURL)
		if strings.Contains(baseURL, "ollama") ||
			strings.Contains(baseURL, ":21434") {
			return "ollama-backend"
		}
		return "openai-backend"
	}
}

// mergeToolCalls 合并所有节点的 tool_calls
// 包括 LLM 返回的 tool_calls 和 Pipeline 注入的 tool_calls
func (e *PipelineEngine) mergeToolCalls(execCtx *ExecutionContext) []ToolCall {
	var allToolCalls []ToolCall

	// 遍历所有节点的输出，收集 tool_calls
	for _, result := range execCtx.GetAllResults() {
		if result != nil && len(result.ToolCalls) > 0 {
			allToolCalls = append(allToolCalls, result.ToolCalls...)
		}
	}

	return allToolCalls
}
