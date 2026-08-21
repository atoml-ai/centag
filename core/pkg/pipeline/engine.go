package pipeline

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"centag/core/internal/cache"
	"centag/core/pkg/backend"
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
	// 同一 node_id 可能先失败再被策略降级成功：以该节点最后一条日志为准。
	lastByNode := make(map[string]NodeExecutionLog, len(nodeLogs))
	order := make([]string, 0, len(nodeLogs))
	for _, log := range nodeLogs {
		if _, seen := lastByNode[log.NodeID]; !seen {
			order = append(order, log.NodeID)
		}
		lastByNode[log.NodeID] = log
	}
	success := true
	var errMsg string
	for _, id := range order {
		log := lastByNode[id]
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
	cacheFacade      *cache.Facade    // v0.3.3: 与 ProxyCache 共用的 Lookup/Store 门面
	// storageHooks 按 pipelineID 缓存已创建的存储钩子
	storageHooks map[string]*StorageHook
	hookMu       sync.RWMutex
}

// SetCacheFacade injects the shared cache Lookup/Store facade for CacheNode.
func (e *PipelineEngine) SetCacheFacade(f *cache.Facade) {
	if e != nil {
		e.cacheFacade = f
	}
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
	pipeline := e.LookupPipeline(ctx, pipelineID)
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline not found: %s", pipelineID)
	}
	return e.executePipeline(ctx, pipeline, input)
}

// LookupPipeline resolves a pipeline for execution: owner-scoped private copy
// first (when ctx carries OwnerScope), otherwise system registry.
func (e *PipelineEngine) LookupPipeline(ctx context.Context, pipelineID string) *AgentPatternPipeline {
	if e == nil || e.pipelineRegistry == nil {
		return nil
	}
	if scope := OwnerScopeFromContext(ctx); scope != "" {
		return e.pipelineRegistry.GetByTenant(scope, pipelineID)
	}
	return e.pipelineRegistry.Get(pipelineID)
}

// ExecutePipelineDefinition 直接执行流水线定义（无需预先注册到注册表）。
// 用于前端画布的"测试"场景：流水线尚在编辑中、未保存到后端。
func (e *PipelineEngine) ExecutePipelineDefinition(ctx context.Context, pipeline *AgentPatternPipeline, input *PipelineInput) (*PipelineOutput, error) {
	if pipeline != nil {
		for i := range pipeline.Nodes {
			pipeline.Nodes[i].Normalize()
		}
	}
	if err := pipeline.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline: %w", err)
	}
	return e.executePipeline(ctx, pipeline, input)
}

// HasPipeline 检查流水线是否存在于系统或任意租户空间。
// ModeDispatcher 用它把 mode/默认流水线 ID 解析为可执行 ID；必须能看到用户私有副本。
// 实际执行仍走 LookupPipeline / HasPipelineContext（带 OwnerScope）。
func (e *PipelineEngine) HasPipeline(pipelineID string) bool {
	if e == nil || e.pipelineRegistry == nil {
		return false
	}
	return e.pipelineRegistry.ExistsAnywhere(pipelineID)
}

// HasPipelineContext 检查流水线是否对当前 OwnerScope 可见（自有或系统预设）。
func (e *PipelineEngine) HasPipelineContext(ctx context.Context, pipelineID string) bool {
	return e.LookupPipeline(ctx, pipelineID) != nil
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
	pipeline := e.LookupPipeline(ctx, pipelineID)
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
		fallbackPrimarySet := make(map[string]bool)
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			fallbackPrimarySet[fg.PrimaryNodeID] = true
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
					// 与非流式 Execute 对齐：主节点失败交给降级组，勿提前终止
					if fallbackPrimarySet[nodeID] {
						e.logger.Warn("primary node failed (stream), deferring to fallback group",
							"primary_node_id", nodeID,
						)
					} else {
						resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: execErr}}
						return
					}
				}
			} else {
				if execErr := e.executeLayerParallel(ctx, graph, execCtx, activeNodes, pipeline, parallelLimit, fallbackPrimarySet); execErr != nil {
					resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{Error: execErr}}
					return
				}
			}
		}

		// ---------- 3. 降级组处理（与非流式共用 executeFallbackGroup）----------
		for _, fg := range pipeline.GlobalConfig.FallbackGroups {
			primaryNode := graph.GetNode(fg.PrimaryNodeID)
			if primaryNode == nil {
				continue
			}

			fallbackSuccess, lastFallbackErr := e.executeFallbackGroup(ctx, graph, execCtx, pipeline, fg, primaryNode)
			if !fallbackSuccess {
				resultCh <- PipelineStreamResult{Chunk: &plugin.StreamChunk{
					Error: buildFallbackGroupError(fg.PrimaryNodeID, primaryNode.Error, lastFallbackErr),
				}}
				return
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

	pipelineID := ""
	if execLog != nil {
		pipelineID = execLog.PipelineID
	}
	traceOut := &PipelineOutput{
		Content:      outputContent,
		Metadata:     nil,
		ExecutionLog: execLog,
	}
	if lastOutput != nil {
		traceOut.Metadata = lastOutput.Metadata
	}
	ApplyResponseTraceBanner(traceOut, pipelineID)
	outputContent = traceOut.Content
	if lastOutput != nil && traceOut.Metadata != nil {
		lastOutput.Metadata = traceOut.Metadata
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
		Content:      outputContent,
		Passed:       lastOutput.Passed,
		Score:        lastOutput.Score,
		Feedback:     lastOutput.Feedback,
		Suggestions:  lastOutput.Suggestions,
		Messages:     lastOutput.Messages,
		Metadata:     lastOutput.Metadata,
		ExecutionLog: execLog,
		NodeOutputs:  nodeOutputs,
		LastNode:     lastNode,
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

// executeLayerParallel 并行执行一层中的多个非流式节点。
// fallbackPrimarySet 中的主节点失败不向上返回，交由后续 FallbackGroups 处理。
func (e *PipelineEngine) executeLayerParallel(ctx context.Context, graph *ExecutionGraph, execCtx *ExecutionContext, nodeIDs []string, pipeline *AgentPatternPipeline, parallelLimit int, fallbackPrimarySet map[string]bool) error {
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
				if fallbackPrimarySet == nil || !fallbackPrimarySet[id] {
					firstErr = execErr
				}
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

		fallbackSuccess, lastFallbackErr := e.executeFallbackGroup(ctx, graph, execCtx, pipeline, fg, primaryNode)
		if !fallbackSuccess {
			return nil, buildFallbackGroupError(fg.PrimaryNodeID, primaryNode.Error, lastFallbackErr)
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

	ApplyResponseTraceBanner(pipelineOut, pipeline.ID)
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
	resolvedBackendID := nodeBackendIDContext(ctx, execNode.Config, execNode.Config.Config)

	// 执行节点
	output, err := e.executeNode(ctx, execNode.Config, nodeInput)
	skippedCircuit := isCircuitBreakerSkipError(err)
	if skippedCircuit {
		e.logger.Warn("node skipped: circuit breaker open",
			"node_id", nodeID,
			"backend_id", resolvedBackendID,
		)
	}
	// 余额/额度失败不记熔断，避免挡住同后端 fallback_model
	// 模型不存在 / 余额不足：应继续降级，但不计入熔断（否则同后端其它模型也被挡）。
	// 免费档 429 限流同样豁免，避免免费档流量高峰把整个后端打成 open。
	skipCircuitRecord := skippedCircuit || isBillingQuotaError(err) || isModelNotFoundNodeError(err)
	recordNodeCircuitOutcome(resolvedBackendID, nodeModelContext(ctx, execNode.Config, execNode.Config.Config), err == nil, skipCircuitRecord, err)
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
				output.Metadata["fallback_reason"] = MaskSensitiveData(err.Error())
				if from := resolveClientRequestedModel(nodeInput, ""); from != "" {
					output.Metadata["fallback_from_model"] = from
				}
				if to := firstMetaString(output.Metadata, "executor_model", "model", "billing_fallback_to_model"); to != "" {
					output.Metadata["fallback_to_model"] = to
				}
				AnnotateFallbackNotice(output)
				// 补一条成功日志，避免总览 success 仍被首轮失败 attempt 拉成 false
				execCtx.AddNodeLog(NodeExecutionLog{
					NodeID:       nodeID,
					NodeType:     execNode.Config.Type,
					Success:      true,
					ErrorMessage: "",
				})
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
					output.Metadata["bypass"] = nodeID // 字符串值，与 SSE 事件 schema 兼容
					output.Metadata["bypass_node"] = nodeID
					output.Metadata["bypass_reason"] = bypassReason
					e.logger.Warn("node execution bypassed with prior output",
						"node_id", nodeID,
						"fallback_output_len", len(strings.TrimSpace(output.Content)),
					)
					execCtx.AddNodeLog(NodeExecutionLog{
						NodeID:       nodeID,
						NodeType:     execNode.Config.Type,
						Success:      true,
						ErrorMessage: "",
					})
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
// 优先级：节点级 FallbackPolicyID → 流水线级 FallbackPolicyID → 内置 same-backend-cross-model → 自动策略。
func (e *PipelineEngine) resolveFallbackPolicy(nodeConfig PipelineNodeConfig, pipeline *AgentPatternPipeline) *config.GlobalFallbackPolicy {
	store := config.GetFallbackPolicyStore()
	if store == nil {
		return nil
	}

	// 1. 节点级策略
	if nodeConfig.FallbackPolicyID != "" {
		if p := store.GetEnabled(nodeConfig.FallbackPolicyID); p != nil {
			return p
		}
	}

	// 2. 流水线级策略
	if pipeline != nil && pipeline.GlobalConfig.FallbackPolicyID != "" {
		if p := store.GetEnabled(pipeline.GlobalConfig.FallbackPolicyID); p != nil {
			return p
		}
	}

	// 3. 流水线仍使用旧版 FallbackGroups 时，不走新策略（由 FallbackGroups 负责）
	if pipeline != nil && len(pipeline.GlobalConfig.FallbackGroups) > 0 {
		return nil
	}

	// 4. 默认：同后端跨模型（含 system.fallback_model），覆盖 CreditsError 等场景
	if p := store.GetEnabled("same-backend-cross-model"); p != nil {
		return p
	}

	// 5. 自动策略（同模型跨后端）
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
	origBackendResolved, origModelResolved := ResolveVirtualVarsContext(ctx, originalBackend, originalModel)
	// 与配置默认值比较不够：客户端可能仍打付费 model，而 default 已是免费档
	failedModel := resolveClientRequestedModel(input, "")
	if failedModel == "" {
		failedModel = origModelResolved
	}

	tryRule := func(resolvedBackend, resolvedModel string, priority int) (*NodeOutput, bool) {
		resolvedBackend = strings.TrimSpace(resolvedBackend)
		resolvedModel = strings.TrimSpace(resolvedModel)
		if resolvedBackend == "" || resolvedModel == "" {
			return nil, false
		}
		if strings.Contains(resolvedBackend, "{{") || strings.Contains(resolvedModel, "{{") {
			return nil, false
		}
		// 跳过与「实际失败模型」相同的规则
		if strings.EqualFold(resolvedBackend, origBackendResolved) && strings.EqualFold(resolvedModel, failedModel) {
			e.logger.Warn("fallback rule skipped: same as failed model",
				"policy_id", policy.ID,
				"backend_id", resolvedBackend,
				"model", resolvedModel,
			)
			return nil, false
		}
		fallbackConfig := originalConfig
		fallbackConfig.Config.Backend = resolvedBackend
		fallbackConfig.Config.Model = resolvedModel
		fallbackConfig.FallbackPolicyID = ""

		e.logger.Info("trying fallback rule",
			"policy_id", policy.ID,
			"backend_id", resolvedBackend,
			"model", resolvedModel,
			"failed_model", failedModel,
			"priority", priority,
		)

		ruleInput := cloneNodeInputForFallback(input, resolvedModel)
		output, err := e.executeNode(ctx, fallbackConfig, ruleInput)
		if err != nil {
			if isCircuitBreakerSkipError(err) {
				e.logger.Warn("fallback rule skipped: circuit breaker open",
					"policy_id", policy.ID,
					"backend_id", resolvedBackend,
				)
			} else {
				e.logger.Warn("fallback rule failed",
					"policy_id", policy.ID,
					"backend_id", resolvedBackend,
					"model", resolvedModel,
					"error", err,
				)
			}
			return nil, false
		}
		if !isUsableFallbackNodeOutput(output) {
			e.logger.Warn("fallback rule returned unusable output",
				"policy_id", policy.ID,
				"backend_id", resolvedBackend,
				"model", resolvedModel,
				"status_code", outputStatusCode(output),
				"preview", utils.TruncateString(output.Content, 160),
			)
			return nil, false
		}
		return output, true
	}

	for _, rule := range policy.SortedRules() {
		resolvedBackend, resolvedModel := resolveFallbackRuleTarget(ctx, rule, input, origBackendResolved, origModelResolved)
		// 同后端跨模型：规则里即使写了 system.default_backend，也钉死在失败节点的实际后端。
		// 否则直连钉死 opencode-zen 时，会误跳到系统默认 bigmodel-ai。
		if policy.Strategy == config.StrategySameBackendDifferentModel && origBackendResolved != "" {
			resolvedBackend = origBackendResolved
		}
		if strings.TrimSpace(resolvedBackend) == "" || strings.TrimSpace(resolvedModel) == "" {
			e.logger.Warn("fallback rule skipped: unresolved placeholder",
				"policy_id", policy.ID,
				"backend_id", rule.BackendID,
				"model", rule.Model,
			)
			continue
		}
		if out, ok := tryRule(resolvedBackend, resolvedModel, rule.Priority); ok {
			return out, nil
		}
	}

	// 策略规则耗尽后：强制再试同后端免费档（覆盖 default 已是 free、但请求体仍是付费模型的情况）
	if free := pickFreeTierModel(origBackendResolved); free != "" {
		if out, ok := tryRule(origBackendResolved, free, 99); ok {
			return out, nil
		}
	}

	return nil, fmt.Errorf("all fallback rules exhausted for policy %s (failed_model=%s)", policy.ID, failedModel)
}

// resolveFallbackRuleTarget 解析规则中的 backend/model 占位符；绝不把 "{{...}}" 原样发给上游。
func resolveFallbackRuleTarget(ctx context.Context, rule config.FallbackRule, input *NodeInput, origBackend, origModel string) (string, string) {
	resolvedBackend, resolvedModel := ResolveVirtualVarsContext(ctx, rule.BackendID, rule.Model) // fallback rules use global defaults

	switch strings.TrimSpace(rule.Model) {
	case "{{requested_model}}":
		resolvedModel = resolveClientRequestedModel(input, origModel)
	case "{{system.fallback_model}}":
		if strings.TrimSpace(resolvedModel) == "" || strings.Contains(resolvedModel, "{{") {
			resolvedModel = pickFreeTierModel(resolvedBackend)
			if resolvedModel == "" {
				resolvedModel = pickFreeTierModel(origBackend)
			}
		}
	}

	if strings.Contains(resolvedBackend, "{{") {
		resolvedBackend = ""
	}
	if strings.Contains(resolvedModel, "{{") {
		resolvedModel = ""
	}
	return strings.TrimSpace(resolvedBackend), strings.TrimSpace(resolvedModel)
}

func resolveClientRequestedModel(input *NodeInput, fallback string) string {
	if input != nil && input.Metadata != nil {
		if m, ok := input.Metadata["model"].(string); ok {
			m = strings.TrimSpace(m)
			if m != "" && !strings.Contains(m, "{{") {
				return m
			}
		}
		if raw, ok := input.Metadata["raw_request_body"].(string); ok {
			if m := extractJSONModel([]byte(raw)); m != "" && !strings.Contains(m, "{{") {
				return m
			}
		}
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" && !strings.Contains(fallback, "{{") {
		return fallback
	}
	return ""
}

func outputStatusCode(out *NodeOutput) int {
	if out == nil || out.Metadata == nil {
		return 0
	}
	switch v := out.Metadata["status_code"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// isUsableFallbackNodeOutput 拒绝「节点没报错但 body 仍是上游错误」的假成功。
func isUsableFallbackNodeOutput(out *NodeOutput) bool {
	if out == nil {
		return false
	}
	if sc := outputStatusCode(out); sc >= 400 {
		return false
	}
	content := strings.TrimSpace(out.Content)
	if content == "" {
		return false
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, `"type":"error"`) ||
		strings.Contains(lower, `"type": "error"`) ||
		strings.Contains(lower, "modelerror") ||
		strings.Contains(lower, "creditserror") ||
		strings.Contains(lower, "is not supported") {
		return false
	}
	return true
}

func cloneNodeInputForFallback(input *NodeInput, model string) *NodeInput {
	if input == nil {
		return &NodeInput{Metadata: map[string]interface{}{"model": model}}
	}
	out := &NodeInput{
		Content:         input.Content,
		Messages:        input.Messages,
		Tools:           input.Tools,
		ToolChoice:      input.ToolChoice,
		Metadata:        make(map[string]interface{}, len(input.Metadata)+2),
		Context:         input.Context,
		UpstreamResults: input.UpstreamResults,
	}
	for k, v := range input.Metadata {
		out.Metadata[k] = v
	}
	model = strings.TrimSpace(model)
	if model != "" {
		out.Metadata["model"] = model
		if raw, ok := out.Metadata["raw_request_body"].(string); ok && strings.TrimSpace(raw) != "" {
			out.Metadata["raw_request_body"] = string(forceBodyModel([]byte(raw), model))
		}
	}
	return out
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
				// 提升上游改写的 raw_request_body（user_prompt_ops 等），覆盖流水线原始 metadata
				if result.Metadata != nil {
					if raw, ok := result.Metadata["raw_request_body"]; ok && raw != nil {
						if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
							input.Metadata["raw_request_body"] = s
						}
					}
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

	// 固定出站（直连）不拦熔断：唯一后端被 open 后整条链路不可用，且半开探测也无从谈起。
	if backendID := nodeBackendIDContext(ctx, config, nodeConfig); isCircuitOpenForBackend(backendID) && !isFixedEgressNodeConfig(nodeConfig) {
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

	// Phase 4A / v0.3.3: 仅当显式 use_strategy_plugin=true 时注入策略插件。
	// 默认走传统 CacheNode / Lookup 门面，避免盖住完整 exact/semantic 路径。
	if config.Type == NodeTypeCache {
		if cn, ok := node.(*CacheNode); ok && e.cacheFacade != nil {
			cn.SetCacheFacade(e.cacheFacade)
			if m := e.cacheFacade.Manager(); m != nil {
				cn.SetCacheManager(m)
			}
			e.logger.Info("cache facade injected into node",
				"node_id", config.ID,
				"backend", e.cacheFacade.EffectiveBackend(),
			)
		} else if config.Type == NodeTypeCache && e.cacheFacade == nil {
			e.logger.Warn("cache facade not available on engine, node will rely on storages only",
				"node_id", config.ID,
			)
		}
		if cacheNode, ok := node.(interface {
			SetStrategyPlugin(CacheStrategyCapability)
			IsUsingStrategyPlugin() bool
		}); ok && !cacheNode.IsUsingStrategyPlugin() {
			usePlugin, strategyName := resolveCacheStrategyPluginInjection(config.Config.CustomConfig)
			if usePlugin && e.capabilityBroker != nil {
				cacheStrat, err := e.capabilityBroker.GetCacheStrategy(ctx, strategyName, []string{"cache.read", "cache.write"})
				if err != nil {
					e.logger.Debug("cache strategy not available (provider may not be configured)",
						"node_id", config.ID,
						"strategy", strategyName,
						"error", err,
					)
				} else if cacheStrat != nil {
					cacheNode.SetStrategyPlugin(cacheStrat)
					e.logger.Info("cache strategy plugin injected",
						"node_id", config.ID,
						"strategy", cacheStrat.StrategyName(),
					)
				}
			}
		}
	}

	// 将 backendID 注入 context，供 CapabilityBroker 的 llm.call 使用。
	nodeExecCtx := ctx
	if config.Config.Backend != "" {
		nodeExecCtx = context.WithValue(ctx, backendIDContextKey{}, config.Config.Backend)
	}

	// 将 storage.Manager 注入 context，供 CacheNode.InitializeStorages 使用。
	// 必须在下方调用 InitializeStorages 之前注入，否则节点拿不到存储管理器。
	if e.storageManager != nil {
		nodeExecCtx = context.WithValue(nodeExecCtx, storageManagerKey{}, e.storageManager)
	}

	// 初始化缓存节点的存储后端（如果节点支持）
	if initializer, ok := node.(interface {
		InitializeStorages(ctx context.Context) error
	}); ok {
		e.logger.Info("initializing storages for node",
			"node_id", config.ID,
			"node_type", config.Type,
		)
		if err := initializer.InitializeStorages(nodeExecCtx); err != nil {
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

	// max_attempts 语义：表示「首次执行之外的额外重试次数」。
	// MaxAttempts=0 → 仅执行 1 次；MaxAttempts=3 → 最多执行 4 次（含首次）。
	attemptsExecuted := 0
	for attempt := 0; attempt <= retryConfig.MaxAttempts; attempt++ {
		attemptsExecuted = attempt + 1
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
		return nil, fmt.Errorf("node execution failed after %d attempts: %w", attemptsExecuted, lastErr)
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

	// 账户池已耗尽：所有 Key 均已尝试且失败，executeWithRetry 不应再重试，
	// 否则 N 个 Key × M 次重试 = N×M 倍请求放大。
	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) && upstreamErr.PoolExhausted {
		return "pool_exhausted", upstreamErr.StatusCode, ""
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

	// 临时限流（429 + "try again later"）：可重试，但不应触发换模型/换后端降级。
	// 必须优先于 IsBillingOrQuotaFailure 判断——后者会把 429 + FreeUsageLimitError
	// 归为 billing，但 "try again later" 明确指示应等待后重试同 Key。
	if code := upstreamStatusCodeOf(err); code == 429 && config.IsTemporaryRateLimit(code, msg) {
		return "rate_limit", 429, ""
	}

	// 余额/额度类（含 CreditsError）优先归为 billing，供降级路径识别
	if config.IsBillingOrQuotaFailure(0, msg) {
		return "billing", 0, "insufficient_quota"
	}

	// HTTP 状态码错误：优先结构化 UpstreamError，其次文本兜底解析。
	if code := upstreamStatusCodeOf(err); code != 0 {
		return "http_status", code, ""
	}
	if code, ok := extractUpstreamStatusCode(msg); ok {
		return "http_status", code, ""
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
		"insufficient_credits", "CreditsError", "not_enough_balance",
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
func (p *DefaultLLMProvider) CreateClient(ctx context.Context, backendID, model string) (LLMClient, error) {
	if p.backendManager == nil {
		return nil, backend.NewNoUsableBackendError(fmt.Errorf("backend manager not available"))
	}

	// 1. 解析虚拟变量（携带用户 ProxyDefaults）
	resolvedBackend, resolvedModel := p.resolveVirtualVarsContext(ctx, backendID, model)
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

// resolveVirtualVars 解析虚拟变量（无请求上下文，用于非请求路径）
func (p *DefaultLLMProvider) resolveVirtualVars(backendID, model string) (string, string) {
	return ResolveVirtualVars(backendID, model)
}

// resolveVirtualVarsContext 解析虚拟变量，携带请求上下文以支持 Team 用户默认值覆盖。
func (p *DefaultLLMProvider) resolveVirtualVarsContext(ctx context.Context, backendID, model string) (string, string) {
	return ResolveVirtualVarsContext(ctx, backendID, model)
}

// ResolveVirtualVars 解析 {{system.default_*}} / {{system.fallback_*}} 虚拟变量。
// 空字符串也触发解析（等价于虚拟变量占位符）。
// 若已解析出 backend 但 model 仍为空，则兜底使用该后端的首选模型
// （ProbeModel → SupportedModels[0]）。
func ResolveVirtualVars(backendID, model string) (string, string) {
	return ResolveVirtualVarsContext(context.Background(), backendID, model)
}

// ResolveVirtualVarsContext 与 ResolveVirtualVars 相同，但优先使用 ctx 中的
// 请求级代理默认值（Team 普通用户的「我的默认后端」）。
func ResolveVirtualVarsContext(ctx context.Context, backendID, model string) (string, string) {
	cfg := config.Get()
	if cfg == nil {
		return backendID, model
	}

	defaultBackend := cfg.Proxy.DefaultBackendID
	defaultModel := cfg.Proxy.DefaultModel
	if ov, ok := config.ProxyDefaultsFromContext(ctx); ok {
		hasBackend := strings.TrimSpace(ov.DefaultBackendID) != ""
		if hasBackend {
			defaultBackend = ov.DefaultBackendID
			defaultModel = ""
		} else if strings.TrimSpace(ov.DefaultModel) != "" {
			defaultModel = ov.DefaultModel
		}
	}

	resolvedBackend := backendID
	resolvedModel := model

	switch backendID {
	case "{{system.default_backend}}", "":
		resolvedBackend = defaultBackend
	case "{{system.fallback_backend}}":
		// 优先从 ModelVariables 读取（Model Config 页面配置的值）
		if v, ok := cfg.ModelVariables.SystemVariables["system.fallback_backend"]; ok && v != "" {
			resolvedBackend = v
		} else {
			resolvedBackend = cfg.Proxy.FallbackBackendID
			if strings.TrimSpace(resolvedBackend) == "" {
				resolvedBackend = defaultBackend
			}
		}
	case "{{system.classify_backend}}":
		resolvedBackend = cfg.ModelVariables.SystemVariables["system.classify_backend"]
		if strings.TrimSpace(resolvedBackend) == "" {
			resolvedBackend = defaultBackend
		}
	}

	switch model {
	case "{{system.default_model}}", "":
		resolvedModel = defaultModel
	case "{{system.fallback_model}}":
		// 优先从 ModelVariables 读取（Model Config 页面配置的值）
		if v, ok := cfg.ModelVariables.SystemVariables["system.fallback_model"]; ok && v != "" {
			resolvedModel = v
		} else {
			// 未配置时保持空，由下方挑选「不同于 default_model」的免费档
			resolvedModel = cfg.Proxy.FallbackModel
		}
	case "{{system.classify_model}}":
		resolvedModel = cfg.ModelVariables.SystemVariables["system.classify_model"]
		if strings.TrimSpace(resolvedModel) == "" {
			resolvedModel = defaultModel
		}
	}

	if strings.TrimSpace(resolvedModel) == "" && strings.TrimSpace(resolvedBackend) != "" {
		if mgr := backend.GetManager(); mgr != nil {
			if b, err := mgr.Get(resolvedBackend); err == nil {
				if model == "{{system.fallback_model}}" {
					resolvedModel = pickDistinctFreeTierModel(b, defaultModel)
				}
				if strings.TrimSpace(resolvedModel) == "" {
					resolvedModel = backend.PreferredDefaultModel(b)
				}
			}
		}
	}

	return resolvedBackend, resolvedModel
}

// pickDistinctFreeTierModel 在后端模型列表中选一个免费档，且尽量不等于 avoidModel。
func pickDistinctFreeTierModel(b *backend.BackendConfig, avoidModel string) string {
	if b == nil {
		return ""
	}
	avoid := strings.ToLower(strings.TrimSpace(avoidModel))
	var firstFree string
	for _, m := range b.SupportedModels {
		for _, name := range []string{m.ActualModel, m.RequestedModel} {
			name = strings.TrimSpace(name)
			if name == "" || !backend.ModelHasFreeTier(name) {
				continue
			}
			if firstFree == "" {
				firstFree = name
			}
			if avoid == "" || !strings.EqualFold(name, avoid) {
				return name
			}
		}
	}
	return firstFree
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

// resolveCacheStrategyPluginInjection decides whether to inject a cache strategy
// plugin and which strategy name to request. Injection is opt-in only.
func resolveCacheStrategyPluginInjection(custom map[string]interface{}) (usePlugin bool, strategyName string) {
	if custom != nil {
		if v, ok := custom["use_strategy_plugin"].(bool); ok {
			usePlugin = v
		}
		if sn, ok := custom["cache_strategy"].(string); ok && sn != "" {
			strategyName = sn
		} else if sn, ok := custom["strategy"].(string); ok && sn != "" {
			strategyName = sn
		}
	}
	if !usePlugin {
		return false, ""
	}
	if strategyName == "" {
		if cfg := config.Get(); cfg != nil {
			strategyName = cfg.Cache.Backend
			if strategyName == "" {
				strategyName = cfg.Cache.Strategy
			}
		}
	}
	if strategyName == "" {
		strategyName = "exact"
	}
	return true, strategyName
}
