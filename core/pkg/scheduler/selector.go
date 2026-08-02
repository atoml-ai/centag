package scheduler

import (
	"centag/core/pkg/backend"
)

// BackendSelector 后端选择策略接口
// 消除硬编码后端 ID，支持可配置的后端选择
type BackendSelector interface {
	// Select 为给定任务类型选择最合适的后端
	Select(taskType TaskType, backends []*backend.BackendConfig) (*backend.BackendConfig, string)
	// GetPriority 返回该选择器的优先级
	GetPriority() int
}

// TaskTypeSelector 基于任务类型的默认后端选择器
// 替代原来 scheduler.go 中的硬编码 selectForXXX 方法
type TaskTypeSelector struct {
	// TaskPriorities 配置各任务类型的后端优先级
	// 格式: taskType -> []BackendID (按优先级排序)
	TaskPriorities map[TaskType][]string
	// DefaultSelector 默认选择器（当 TaskPriorities 未配置时使用）
	DefaultSelector BackendSelector
}

// NewTaskTypeSelector 创建基于任务类型的选择器
func NewTaskTypeSelector() *TaskTypeSelector {
	return &TaskTypeSelector{
		TaskPriorities: make(map[TaskType][]string),
	}
}

// Select 根据任务类型选择后端
func (s *TaskTypeSelector) Select(taskType TaskType, backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 1. 如果有配置的后端优先级，使用配置
	if priorityList, ok := s.TaskPriorities[taskType]; ok {
		return s.selectByPriorityList(priorityList, backends, taskType)
	}

	// 2. 使用默认选择逻辑
	return s.defaultSelect(taskType, backends)
}

// selectByPriorityList 根据配置的后端优先级列表选择
func (s *TaskTypeSelector) selectByPriorityList(priorityList []string, backends []*backend.BackendConfig, taskType TaskType) (*backend.BackendConfig, string) {
	for _, backendID := range priorityList {
		for _, b := range backends {
			if b.ID == backendID && b.Enabled {
				return b, s.getReason(taskType, b)
			}
		}
	}
	return nil, "无可用的后端"
}

// defaultSelect 默认选择逻辑（基于任务类型启发式规则）
func (s *TaskTypeSelector) defaultSelect(taskType TaskType, backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	switch taskType {
	case TaskCodeGeneration:
		return s.selectForCode(backends)
	case TaskSimpleChat:
		return s.selectForSimpleChat(backends)
	case TaskComplexReasoning:
		return s.selectForComplexReasoning(backends)
	case TaskLongText:
		return s.selectForLongText(backends)
	case TaskEmbedding:
		return s.selectForEmbedding(backends)
	case TaskTranslation:
		return s.selectForTranslation(backends)
	case TaskCreative:
		return s.selectForCreative(backends)
	case TaskAnalysis:
		return s.selectForAnalysis(backends)
	default:
		return s.selectDefault(backends)
	}
}

// selectForCode 代码生成任务选择
func (s *TaskTypeSelector) selectForCode(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 优先级: bigmodel > ppinfra > ollama-local
	priorityList := []string{"bigmodel", "ppinfra", "ollama-local"}
	return s.selectByPriorityList(priorityList, backends, TaskCodeGeneration)
}

// selectForSimpleChat 简单对话任务选择
func (s *TaskTypeSelector) selectForSimpleChat(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 优先本地，其次低成本
	priorityList := []string{"ollama-local", "bigmodel", "ppinfra"}
	return s.selectByPriorityList(priorityList, backends, TaskSimpleChat)
}

// selectForComplexReasoning 复杂推理任务选择
func (s *TaskTypeSelector) selectForComplexReasoning(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 优先级: bigmodel > ppinfra
	priorityList := []string{"bigmodel", "ppinfra"}
	return s.selectByPriorityList(priorityList, backends, TaskComplexReasoning)
}

// selectForLongText 长文本任务选择
func (s *TaskTypeSelector) selectForLongText(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// Kimi 擅长长文档处理
	priorityList := []string{"ppinfra", "bigmodel"}
	return s.selectByPriorityList(priorityList, backends, TaskLongText)
}

// selectForEmbedding 向量嵌入任务选择
func (s *TaskTypeSelector) selectForEmbedding(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 优先本地，零成本
	priorityList := []string{"ollama-local"}
	return s.selectByPriorityList(priorityList, backends, TaskEmbedding)
}

// selectForTranslation 翻译任务选择
func (s *TaskTypeSelector) selectForTranslation(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 性价比优先
	priorityList := []string{"ppinfra", "bigmodel"}
	return s.selectByPriorityList(priorityList, backends, TaskTranslation)
}

// selectForCreative 创意写作任务选择
func (s *TaskTypeSelector) selectForCreative(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 高质量优先
	priorityList := []string{"bigmodel", "ppinfra"}
	return s.selectByPriorityList(priorityList, backends, TaskCreative)
}

// selectForAnalysis 数据分析任务选择
func (s *TaskTypeSelector) selectForAnalysis(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	// 强推理能力优先
	priorityList := []string{"bigmodel", "ppinfra"}
	return s.selectByPriorityList(priorityList, backends, TaskAnalysis)
}

// selectDefault 默认选择
func (s *TaskTypeSelector) selectDefault(backends []*backend.BackendConfig) (*backend.BackendConfig, string) {
	for _, b := range backends {
		if b.Enabled {
			return b, "默认选择"
		}
	}
	return nil, "无可用后端"
}

// getReason 获取选择原因
func (s *TaskTypeSelector) getReason(taskType TaskType, b *backend.BackendConfig) string {
	reasons := map[TaskType]string{
		TaskCodeGeneration:   "代码生成任务",
		TaskSimpleChat:      "简单对话任务",
		TaskComplexReasoning: "复杂推理任务",
		TaskLongText:        "长文本处理任务",
		TaskEmbedding:       "向量嵌入任务",
		TaskTranslation:     "翻译任务",
		TaskCreative:        "创意写作任务",
		TaskAnalysis:        "数据分析任务",
	}
	return reasons[taskType] + "，使用后端: " + b.Name
}

// GetPriority 返回选择器优先级
func (s *TaskTypeSelector) GetPriority() int {
	return 100
}

// ConfigDrivenSelector 配置驱动的后端选择器
// 支持从配置文件加载后端优先级配置
type ConfigDrivenSelector struct {
	*TaskTypeSelector
	backendMgr *backend.Manager
}

// NewConfigDrivenSelector 创建配置驱动的选择器
func NewConfigDrivenSelector(backendMgr *backend.Manager) *ConfigDrivenSelector {
	return &ConfigDrivenSelector{
		TaskTypeSelector: NewTaskTypeSelector(),
		backendMgr:        backendMgr,
	}
}

// LoadPrioritiesFromConfig 从配置加载后端优先级
// config 格式示例:
// {
//   "code-generation": ["bigmodel", "ppinfra"],
//   "simple-chat": ["ollama-local", "bigmodel"]
// }
func (s *ConfigDrivenSelector) LoadPrioritiesFromConfig(config map[string][]string) {
	for taskTypeStr, priorities := range config {
		taskType := TaskType(taskTypeStr)
		s.TaskPriorities[taskType] = priorities
	}
}

// RoutingSelector 成本感知路由选择器
// 基于 BackendCandidate 和 ScoreWeights 实现三策略选择
type RoutingSelector struct {
	scorer *MultiDimensionScorer
}

// NewRoutingSelector 创建路由选择器
func NewRoutingSelector(scorer *MultiDimensionScorer) *RoutingSelector {
	return &RoutingSelector{
		scorer: scorer,
	}
}

// SelectByRoutingPolicy 根据路由策略选择最优候选后端
func (s *RoutingSelector) SelectByRoutingPolicy(candidates []*BackendCandidate, policy RoutingPolicyType) *BackendCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 过滤已启用的候选
	enabled := make([]*BackendCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Enabled {
			enabled = append(enabled, c)
		}
	}

	if len(enabled) == 0 {
		return nil
	}

	// 根据策略选择
	switch policy {
	case RoutingPolicyCostOptimal:
		return s.selectCostOptimal(enabled)
	case RoutingPolicyQualityFirst:
		return s.selectQualityFirst(enabled)
	case RoutingPolicyLatencyFirst:
		return s.selectLatencyFirst(enabled)
	default:
		return s.selectBalanced(enabled)
	}
}

// selectCostOptimal 成本优先选择：选择成本最低的候选
func (s *RoutingSelector) selectCostOptimal(candidates []*BackendCandidate) *BackendCandidate {
	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.DynamicCostPer1k < best.DynamicCostPer1k {
			best = c
		}
	}
	return best
}

// selectQualityFirst 质量优先选择：选择质量最高的候选
func (s *RoutingSelector) selectQualityFirst(candidates []*BackendCandidate) *BackendCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 简化：选择第一个候选（质量由模型决定）
	return candidates[0]
}

// selectLatencyFirst 延迟优先选择：选择延迟最低的候选
func (s *RoutingSelector) selectLatencyFirst(candidates []*BackendCandidate) *BackendCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 简化：选择第一个候选（延迟由监控数据决定）
	return candidates[0]
}

// selectBalanced 平衡模式选择：使用多维度评分
func (s *RoutingSelector) selectBalanced(candidates []*BackendCandidate) *BackendCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 使用评分器进行多维度评分（使用已有的平衡权重）
	weights := DefaultWeights()
	intent := &ClassificationResult{TaskType: TaskSimpleChat}
	scores := s.scorer.ScoreCandidates(candidates, intent, weights)

	if len(scores) == 0 {
		return nil
	}

	// 返回评分最高的候选
	bestScore := scores[0]
	for _, score := range scores[1:] {
		if score.TotalScore > bestScore.TotalScore {
			bestScore = score
		}
	}

	// 找到对应的候选
	for _, c := range candidates {
		if c.BackendID == bestScore.BackendID {
			return c
		}
	}

	return candidates[0]
}
