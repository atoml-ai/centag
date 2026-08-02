package pipeline

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// AgentPatternPipeline 代理模式流水线定义
type AgentPatternPipeline struct {
	SchemaVersion string                 `json:"schema_version,omitempty" yaml:"schema_version,omitempty"`
	ID            string                 `json:"id" yaml:"id"`
	Name          string                 `json:"name" yaml:"name"`
	Description   string                 `json:"description" yaml:"description"`
	Version       string                 `json:"version" yaml:"version"`
	ShortcutCode  string                 `json:"shortcut_code" yaml:"shortcut_code"`
	TenantID      string                 `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"` // 空=系统共享，非空=租户私有
	Nodes         []PipelineNodeConfig   `json:"nodes" yaml:"nodes"`
	GlobalConfig  GlobalPipelineConfig   `json:"global_config" yaml:"global_config"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// RoutingPolicy 路由策略（cost_optimal / balanced / quality_first / latency_first）
	RoutingPolicy string `json:"routing_policy,omitempty" yaml:"routing_policy,omitempty"`
	// PipelinePricing 流水线级别定价配置
	PipelinePricing *PipelinePricing `json:"pipeline_pricing,omitempty" yaml:"pipeline_pricing,omitempty"`
}

// PipelinePricing 流水线级别定价配置
type PipelinePricing struct {
	// CostPriceType 成本侧价格类型（默认 "cost"）
	CostPriceType string `json:"cost_price_type,omitempty" yaml:"cost_price_type,omitempty"`
	// RevenuePriceType 营收侧价格类型（默认 "revenue"）
	RevenuePriceType string `json:"revenue_price_type,omitempty" yaml:"revenue_price_type,omitempty"`
	// EnableDualPricing 是否启用双侧定价
	EnableDualPricing bool `json:"enable_dual_pricing,omitempty" yaml:"enable_dual_pricing,omitempty"`
}

// RouteConfig 分支路由配置
// 用于标识本节点是某个路由节点的下游分支
type RouteConfig struct {
	RouterNodeID string `json:"router_node_id" yaml:"router_node_id"`
	RouteValue   string `json:"route_value" yaml:"route_value"`
	IsDefault    bool   `json:"is_default" yaml:"is_default"`
}

// PipelineNodeConfig 流水线节点配置
type PipelineNodeConfig struct {
	ID              string                 `json:"id" yaml:"id"`
	Type            NodeType               `json:"type,omitempty" yaml:"type,omitempty"`
	Kind            string                 `json:"kind,omitempty" yaml:"kind,omitempty"`
	Implementation  string                 `json:"implementation,omitempty" yaml:"implementation,omitempty"`
	Name            string                 `json:"name" yaml:"name"`
	Backend         string                 `json:"backend,omitempty" yaml:"backend,omitempty"`
	Model           string                 `json:"model,omitempty" yaml:"model,omitempty"`
	Config          NodeConfig             `json:"config" yaml:"config"`
	Inputs          map[string]string      `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs         map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	ConfigSchemaRef string                 `json:"config_schema_ref,omitempty" yaml:"config_schema_ref,omitempty"`
	SecretsRef      map[string]string      `json:"secrets_ref,omitempty" yaml:"secrets_ref,omitempty"`
	Permissions     []string               `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Timeout         int                    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retry           *RetryConfig           `json:"retry,omitempty" yaml:"retry,omitempty"`
	Condition       string                 `json:"condition,omitempty" yaml:"condition,omitempty"`
	NextNodes       []string               `json:"next_nodes,omitempty" yaml:"next_nodes,omitempty"`
	DependsOn       []string               `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	RouteConfig     *RouteConfig           `json:"route_config,omitempty" yaml:"route_config,omitempty"`
	// FallbackPolicyID 引用全局降级策略 ID（空=继承流水线全局默认）。
	FallbackPolicyID string `json:"fallback_policy_id,omitempty" yaml:"fallback_policy_id,omitempty"`
}

// Normalize 将顶层 Backend/Model 归一化到 Config 中，确保 Config 是后端/模型信息的权威来源。
// 调用时机：从 YAML 模板、JSON API 或数据库加载 PipelineNodeConfig 后立即调用。
//
// 语义：
//   - 顶层字段显式设置时始终覆盖 Config 内部值，保证 API 创建的完整流水线对象写入生效。
//   - Normalize 只做正向复制（顶层 → Config），不清除顶层字段，
//     以便 API 响应中同时暴露顶层和 Config 两层字段，供前端直接读取。
func (pnc *PipelineNodeConfig) Normalize() {
	if pnc.Backend != "" {
		pnc.Config.Backend = pnc.Backend
	}
	if pnc.Model != "" {
		pnc.Config.Model = pnc.Model
	}
}

// FallbackGroup 降级组
// 声明主节点与备用节点的降级关系
type FallbackGroup struct {
	PrimaryNodeID string   `json:"primary_node_id" yaml:"primary_node_id"`
	FallbackNodes []string `json:"fallback_nodes" yaml:"fallback_nodes"`
	MaxAttempts   int      `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
}

// StorageHookConfig 流水线存储钩子配置
type StorageHookConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	AutoSave      bool   `json:"auto_save" yaml:"auto_save"`
	SaveInterval  int    `json:"save_interval" yaml:"save_interval"`
	RetentionDays int    `json:"retention_days" yaml:"retention_days"`
}

// HookConfig 单个钩子配置
type HookConfig struct {
	Type        string                 `json:"type" yaml:"type"`
	On          []string               `json:"on" yaml:"on"`
	StorageName string                 `json:"storage_name,omitempty" yaml:"storage_name,omitempty"` // 为空则使用默认存储
	StorageType string                 `json:"storage_type,omitempty" yaml:"storage_type,omitempty"` // "kv"(默认)、"knowledge"、"vector"
	Config      map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// GlobalPipelineConfig 流水线全局配置
// 注: 流式与否由客户端 req.Stream 决定，与流水线配置无关。
// 旧版 stream_mode 字段已废弃并移除，详见 docs/exec-plans/active/2026-06-05-pipeline-stream-decoupling.md。
type GlobalPipelineConfig struct {
	Timeout         int              `json:"timeout" yaml:"timeout"`
	MaxRetries      int              `json:"max_retries" yaml:"max_retries"`
	BypassOnError   bool             `json:"bypass_on_error" yaml:"bypass_on_error"`
	ParallelLimit   int              `json:"parallel_limit" yaml:"parallel_limit"`
	LogLevel        string           `json:"log_level,omitempty" yaml:"log_level,omitempty"`
	FallbackGroups  []FallbackGroup  `json:"fallback_groups,omitempty" yaml:"fallback_groups,omitempty"`
	// FallbackPolicyID 引用全局降级策略 ID（优先于 FallbackGroups）。
	FallbackPolicyID string `json:"fallback_policy_id,omitempty" yaml:"fallback_policy_id,omitempty"`
	// SystemPrompt 流水线级别的默认 system_prompt，用于所有 generator 节点。
	// 当节点自身的 config.system_prompt 为空时，回退到此值。
	// 模板变量（如 {{.question}}）同样生效。
	SystemPrompt string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	// Storage 流水线存储配置
	StorageConfig *StorageHookConfig `json:"storage,omitempty" yaml:"storage,omitempty"`
	// Hooks 流水线钩子列表
	Hooks []HookConfig `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

// HasStorageHook 检查是否配置了存储钩子
func (g GlobalPipelineConfig) HasStorageHook() bool {
	if g.StorageConfig == nil || !g.StorageConfig.Enabled {
		return false
	}
	for _, h := range g.Hooks {
		if h.Type == "storage" {
			return true
		}
	}
	return false
}

// StorageNamespace 返回存储命名空间（优先使用配置值，否则用 pipeline ID 兜底）
func (g GlobalPipelineConfig) StorageNamespace(pipelineID string) string {
	if g.StorageConfig != nil && g.StorageConfig.Namespace != "" {
		return g.StorageConfig.Namespace
	}
	return pipelineID
}

func DefaultGlobalConfig() GlobalPipelineConfig {
	return GlobalPipelineConfig{
		Timeout:       120,
		MaxRetries:    3,
		BypassOnError: true,
		ParallelLimit: 4,
		LogLevel:      "info",
	}
}

// Validate 验证流水线配置有效性
func (p *AgentPatternPipeline) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("pipeline id is required")
	}
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	// 允许空节点草稿流水线（仅含基础信息），执行时再校验节点完整性
	if len(p.Nodes) == 0 {
		return nil
	}

	// 验证节点ID唯一性
	nodeIDs := make(map[string]bool)
	for _, node := range p.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node id: %s", node.ID)
		}
		nodeIDs[node.ID] = true

		pluginNode := node.Implementation != "" || node.Kind != ""
		if node.Type == "" && !pluginNode {
			return fmt.Errorf("node %s: type or implementation is required", node.ID)
		}
		if node.Type != "" && !node.Type.IsValid() && !pluginNode {
			return fmt.Errorf("invalid node type: %s for node %s", node.Type, node.ID)
		}
		// 草稿节点允许暂不配置 backend/model，执行前由引擎校验。
	}

	// 验证依赖关系
	for _, node := range p.Nodes {
		for _, dep := range node.DependsOn {
			if !nodeIDs[dep] {
				return fmt.Errorf("node %s: dependency %s not found", node.ID, dep)
			}
		}
		for _, next := range node.NextNodes {
			if !nodeIDs[next] {
				return fmt.Errorf("node %s: next node %s not found", node.ID, next)
			}
		}
	}

	// 检测循环依赖
	if err := p.detectCycle(); err != nil {
		return err
	}

	return nil
}

// detectCycle 检测配置中是否存在循环依赖
func (p *AgentPatternPipeline) detectCycle() error {
	graph := make(map[string][]string)
	for _, node := range p.Nodes {
		graph[node.ID] = node.NextNodes
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(nodeID string) error
	dfs = func(nodeID string) error {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, neighbor := range graph[nodeID] {
			if !visited[neighbor] {
				if err := dfs(neighbor); err != nil {
					return err
				}
			} else if recStack[neighbor] {
				return fmt.Errorf("cycle detected involving node: %s", neighbor)
			}
		}

		recStack[nodeID] = false
		return nil
	}

	for _, node := range p.Nodes {
		if !visited[node.ID] {
			if err := dfs(node.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetNode 根据ID获取节点配置
func (p *AgentPatternPipeline) GetNode(nodeID string) *PipelineNodeConfig {
	for i := range p.Nodes {
		if p.Nodes[i].ID == nodeID {
			return &p.Nodes[i]
		}
	}
	return nil
}

// GetStartNodes 获取没有依赖的起始节点
func (p *AgentPatternPipeline) GetStartNodes() []PipelineNodeConfig {
	var starts []PipelineNodeConfig
	for _, node := range p.Nodes {
		if len(node.DependsOn) == 0 {
			starts = append(starts, node)
		}
	}
	return starts
}

// GetEndNodes 获取没有后续节点的结束节点
func (p *AgentPatternPipeline) GetEndNodes() []PipelineNodeConfig {
	var ends []PipelineNodeConfig
	for _, node := range p.Nodes {
		if len(node.NextNodes) == 0 {
			ends = append(ends, node)
		}
	}
	return ends
}

// PipelineInput 流水线输入
type PipelineInput struct {
	Content   string                 `json:"content"`
	Messages  []Message              `json:"messages,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	// Stream 表示客户端请求是否要求流式输出。
	// 流式与否由调用方在请求中决定，与流水线全局配置无关；
	// 引擎内部所有节点统一非流式执行，由顶层 StreamAdapter 负责按需分块。
	Stream bool `json:"stream,omitempty"`
	// Tools 客户端请求中的工具定义（透传给后端，支持 function calling）
	Tools any `json:"tools,omitempty"`
	// ToolChoice 工具选择策略（透传给后端）
	ToolChoice any `json:"tool_choice,omitempty"`
}

// RequestIDFromInput 从流水线输入 metadata 提取 request_id（用于日志关联）。
func RequestIDFromInput(input *PipelineInput) string {
	if input == nil || input.Metadata == nil {
		return ""
	}
	if id, ok := input.Metadata["request_id"].(string); ok {
		return id
	}
	return ""
}

// PipelineOutput 流水线输出
type PipelineOutput struct {
	Content      string                 `json:"content"`
	Messages     []Message              `json:"messages,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ExecutionLog *ExecutionLog          `json:"execution_log,omitempty"`
	NodeOutputs  map[string]*NodeOutput `json:"node_outputs,omitempty"` // 各节点输出
	LastNode     string                 `json:"last_node,omitempty"`    // 最后执行的节点
	// 审核结果顶层字段（当流水线包含 reviewer 节点时填充，调用方无需解析 metadata）
	Passed      *bool    `json:"passed,omitempty"`
	Score       *float64 `json:"score,omitempty"`
	Feedback    string   `json:"feedback,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	// ToolCalls LLM 返回的工具调用（function calling），从最后节点输出提升到顶层，
	// 供非流式响应出口（writeResponse）直接读取，避免 tool_calls 丢失。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// FinishReason LLM 完成原因（如 tool_calls/stop/length），从最后节点输出提升到顶层。
	FinishReason string `json:"finish_reason,omitempty"`
	// ReasoningContent 推理内容（DeepSeek thinking 模式等），从最后节点输出提升到顶层。
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// ExecutionLog 执行日志
type ExecutionLog struct {
	PipelineID   string             `json:"pipeline_id"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
	Duration     int64              `json:"duration_ms"`
	NodeLogs     []NodeExecutionLog `json:"node_logs"`
	TotalTokens  int                `json:"total_tokens"`
	Success      bool               `json:"success"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

// NodeExecutionLog 节点执行日志
type NodeExecutionLog struct {
	NodeID         string    `json:"node_id"`
	NodeType       NodeType  `json:"node_type"`
	Implementation string    `json:"implementation,omitempty"` // 插件实现
	Kind           string    `json:"kind,omitempty"`           // 插件类型
	PluginVersion  string    `json:"plugin_version,omitempty"` // 插件版本
	Model          string    `json:"model,omitempty"`          // 使用的模型
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Duration       int64     `json:"duration_ms"`
	InputTokens    int       `json:"input_tokens"`
	OutputTokens   int       `json:"output_tokens"`
	InputSize      int64     `json:"input_size,omitempty"`  // 输入大小（字节）
	OutputSize     int64     `json:"output_size,omitempty"` // 输出大小（字节）
	ErrorCode      string    `json:"error_code,omitempty"`  // 错误码
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	RetryCount     int       `json:"retry_count"`
	// 远程插件相关
	HTTPStatus    int    `json:"http_status,omitempty"`
	LatencyMs     int64  `json:"latency_ms,omitempty"`
	CircuitState  string `json:"circuit_state,omitempty"` // "open", "closed"
}

// PipelineRegistry 流水线注册表
type PipelineRegistry struct {
	mu              sync.RWMutex
	pipelines       map[string]*AgentPatternPipeline                    // 全局流水线（系统预设）
	tenantPipelines map[string]map[string]*AgentPatternPipeline         // tenant_id -> pipeline_id -> pipeline
	store           PipelineStore
}

// NewPipelineRegistry creates a registry backed by memory only.
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{
		pipelines:       make(map[string]*AgentPatternPipeline),
		tenantPipelines: make(map[string]map[string]*AgentPatternPipeline),
	}
}

// NewPipelineRegistryWithStore creates a registry backed by both memory and persistent store.
func NewPipelineRegistryWithStore(store PipelineStore) *PipelineRegistry {
	return &PipelineRegistry{
		pipelines:       make(map[string]*AgentPatternPipeline),
		tenantPipelines: make(map[string]map[string]*AgentPatternPipeline),
		store:           store,
	}
}

// Register validates and registers a pipeline in memory and persists it if store is present.
func (r *PipelineRegistry) Register(pipeline *AgentPatternPipeline) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}
	// 归一化所有节点，确保 Config.Backend/Model 为唯一出口
	for i := range pipeline.Nodes {
		pipeline.Nodes[i].Normalize()
	}
	if err := pipeline.Validate(); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}
	pipeline.TenantID = ""

	if r.store != nil {
		if _, err := r.store.Get(pipeline.ID); err == nil {
			if err := r.store.Update(pipeline); err != nil {
				return fmt.Errorf("failed to update pipeline in store: %w", err)
			}
		} else {
			if err := r.store.Create(pipeline); err != nil {
				return fmt.Errorf("failed to create pipeline in store: %w", err)
			}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 清除同 ID 的租户副本，防止 ListAll/ListByTenant 中旧租户副本遮盖新全局副本
	for tid := range r.tenantPipelines {
		delete(r.tenantPipelines[tid], pipeline.ID)
	}

	r.pipelines[pipeline.ID] = pipeline
	return nil
}

// RegisterForTenant 为指定租户注册流水线（租户隔离）
func (r *PipelineRegistry) RegisterForTenant(tenantID string, pipeline *AgentPatternPipeline) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}
	// 归一化所有节点，确保 Config.Backend/Model 为唯一出口
	for i := range pipeline.Nodes {
		pipeline.Nodes[i].Normalize()
	}
	if err := pipeline.Validate(); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}
	pipeline.TenantID = tenantID

	if r.store != nil {
		if err := r.store.CreateForTenant(tenantID, pipeline); err != nil {
			return fmt.Errorf("failed to create pipeline for tenant in store: %w", err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tenantPipelines[tenantID] == nil {
		r.tenantPipelines[tenantID] = make(map[string]*AgentPatternPipeline)
	}
	r.tenantPipelines[tenantID][pipeline.ID] = pipeline
	return nil
}

// Get retrieves a pipeline from memory.
func (r *PipelineRegistry) Get(pipelineID string) *AgentPatternPipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pipelines[pipelineID]
}

// GetByTenant 按租户获取流水线（租户隔离）
// 优先返回租户专属流水线，如果不存在则返回系统预设
func (r *PipelineRegistry) GetByTenant(tenantID, pipelineID string) *AgentPatternPipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 优先查找租户专属流水线
	if tenantPipes, exists := r.tenantPipelines[tenantID]; exists {
		if p, ok := tenantPipes[pipelineID]; ok {
			return p
		}
	}

	// 回退到系统预设
	return r.pipelines[pipelineID]
}

// OwnsInTenant 判断流水线是否属于指定租户（不含系统预设回退）
func (r *PipelineRegistry) OwnsInTenant(tenantID, pipelineID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tenantPipes, ok := r.tenantPipelines[tenantID]; ok {
		_, exists := tenantPipes[pipelineID]
		return exists
	}
	return false
}

// DeleteScoped 按作用域删除流水线：管理员删除全局；租户仅可删除本租户私有流水线
func (r *PipelineRegistry) DeleteScoped(tenantID, pipelineID string) error {
	if tenantID == "" {
		return r.RemoveComplete(pipelineID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tenantPipes, ok := r.tenantPipelines[tenantID]
	if !ok {
		return fmt.Errorf("cannot delete system pipeline")
	}
	if _, owned := tenantPipes[pipelineID]; !owned {
		return fmt.Errorf("cannot delete system pipeline")
	}

	if r.store != nil {
		if err := r.store.DeleteForTenant(tenantID, pipelineID); err != nil {
			return fmt.Errorf("failed to delete pipeline from store: %w", err)
		}
	}
	delete(tenantPipes, pipelineID)
	// 若 DB 中仍有系统共享副本则保留/刷新全局缓存；否则清除陈旧全局内存项
	if r.store != nil {
		if p, err := r.store.Get(pipelineID); err == nil && p.TenantID == "" {
			r.pipelines[pipelineID] = p
		} else {
			delete(r.pipelines, pipelineID)
		}
	} else {
		delete(r.pipelines, pipelineID)
	}
	return nil
}

// RemoveComplete deletes a pipeline from persistent store, global memory, and all tenant copies.
func (r *PipelineRegistry) RemoveComplete(pipelineID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.store != nil {
		if err := r.store.Delete(pipelineID); err != nil {
			return fmt.Errorf("failed to delete pipeline from store: %w", err)
		}
	}
	delete(r.pipelines, pipelineID)
	for tid := range r.tenantPipelines {
		delete(r.tenantPipelines[tid], pipelineID)
	}
	return nil
}

// Remove deletes a pipeline from memory and store (ignores store errors; prefer RemoveComplete).
func (r *PipelineRegistry) Remove(pipelineID string) {
	_ = r.RemoveComplete(pipelineID)
}

// RemoveFromTenant 从指定租户中删除流水线（仅内存租户副本；持久化层尚未按租户隔离）
func (r *PipelineRegistry) RemoveFromTenant(tenantID, pipelineID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tenantPipes, exists := r.tenantPipelines[tenantID]; exists {
		delete(tenantPipes, pipelineID)
	}
}

// ListAll returns every pipeline in memory (global + all tenants), sorted by ID.
// Shortcut codes are globally unique in the store; tenant copies use distinct IDs.
func (r *PipelineRegistry) ListAll() []*AgentPatternPipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()

	merged := make(map[string]*AgentPatternPipeline, len(r.pipelines))
	for id, p := range r.pipelines {
		merged[id] = p
	}
	for _, tenantPipes := range r.tenantPipelines {
		for id, p := range tenantPipes {
			merged[id] = p
		}
	}

	list := make([]*AgentPatternPipeline, 0, len(merged))
	for _, p := range merged {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

// List returns all pipelines from memory, sorted by ID.
func (r *PipelineRegistry) List() []*AgentPatternPipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*AgentPatternPipeline, 0, len(r.pipelines))
	for _, p := range r.pipelines {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

// ListByTenant 按租户获取流水线列表（租户隔离）
// 合并系统预设和租户专属流水线，租户专属覆盖系统预设
func (r *PipelineRegistry) ListByTenant(tenantID string) []*AgentPatternPipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()

	merged := make(map[string]*AgentPatternPipeline)

	// 先加入系统预设
	for id, p := range r.pipelines {
		merged[id] = p
	}

	// 租户专属覆盖系统预设
	if tenantPipes, exists := r.tenantPipelines[tenantID]; exists {
		for id, p := range tenantPipes {
			merged[id] = p
		}
	}

	list := make([]*AgentPatternPipeline, 0, len(merged))
	for _, p := range merged {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

// Exists checks if a pipeline exists in memory.
func (r *PipelineRegistry) Exists(pipelineID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.pipelines[pipelineID]
	return ok
}

// ExistsInTenant 检查流水线是否存在于指定租户
func (r *PipelineRegistry) ExistsInTenant(tenantID, pipelineID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 检查租户专属
	if tenantPipes, exists := r.tenantPipelines[tenantID]; exists {
		if _, ok := tenantPipes[pipelineID]; ok {
			return true
		}
	}

	// 检查系统预设
	_, ok := r.pipelines[pipelineID]
	return ok
}

// LoadFromStore loads all enabled pipelines from persistent store into memory.
func (r *PipelineRegistry) LoadFromStore() error {
	if r.store == nil {
		return nil
	}

	pipelines, err := r.store.List()
	if err != nil {
		return fmt.Errorf("failed to load pipelines from store: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range pipelines {
		if p.TenantID != "" {
			if r.tenantPipelines[p.TenantID] == nil {
				r.tenantPipelines[p.TenantID] = make(map[string]*AgentPatternPipeline)
			}
			r.tenantPipelines[p.TenantID][p.ID] = p
			delete(r.pipelines, p.ID)
		} else {
			r.pipelines[p.ID] = p
		}
	}

	return nil
}

// LoadFromStoreByTenant 按租户从存储加载流水线
func (r *PipelineRegistry) LoadFromStoreByTenant(tenantID string) error {
	if r.store == nil {
		return nil
	}

	pipelines, err := r.store.ListByTenant(tenantID)
	if err != nil {
		return fmt.Errorf("failed to load pipelines from store for tenant %s: %w", tenantID, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tenantPipelines[tenantID] == nil {
		r.tenantPipelines[tenantID] = make(map[string]*AgentPatternPipeline)
	}
	for _, p := range pipelines {
		r.tenantPipelines[tenantID][p.ID] = p
	}

	return nil
}

// GetStore returns the underlying store for advanced usage.
func (r *PipelineRegistry) GetStore() PipelineStore {
	return r.store
}
