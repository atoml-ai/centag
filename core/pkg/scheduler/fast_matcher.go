package scheduler

import (
	"sort"
	"strconv"
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
)

// BackendRecommendation 后端推荐配置
type BackendRecommendation struct {
	BackendID string   `json:"backend_id"` // 推荐后端 ID
	Model     string   `json:"model"`      // 推荐模型
	Priority  int      `json:"priority"`   // 优先级（越小越优先）
	Reason    string   `json:"reason"`     // 推荐理由
	Fallbacks []string `json:"fallbacks"`  // 备选后端 ID 列表
}

// TaskBackendMapping 任务类型到后端的映射
type TaskBackendMapping struct {
	TaskType TaskType                `json:"task_type"`
	Backends []BackendRecommendation `json:"backends"`
}

// FastMatcher 快速匹配器（基于内置对照表）
type FastMatcher struct {
	// categoryToTask: category 名称到任务类型的映射
	categoryToTask map[string]TaskType

	// taskToBackends: 任务类型到后端推荐列表
	taskToBackends map[TaskType][]BackendRecommendation

	// backendCache: 后端配置缓存
	backendCache map[string]*backend.BackendConfig

	// configManager: 配置管理器（可选）
	configManager *FastMatcherConfigManager

	// modelMatcher: 模型匹配器，用于在后端模型列表中智能匹配最佳模型
	modelMatcher *backend.ModelMatcher
}

// NewFastMatcher 创建快速匹配器
func NewFastMatcher() *FastMatcher {
	m := &FastMatcher{
		categoryToTask: make(map[string]TaskType),
		taskToBackends: make(map[TaskType][]BackendRecommendation),
		backendCache:   make(map[string]*backend.BackendConfig),
		modelMatcher:   backend.NewModelMatcher(backend.DefaultModelMatchingConfig()),
	}
	m.initMappings()
	return m
}

// NewFastMatcherWithConfig 创建带配置的快速匹配器
func NewFastMatcherWithConfig(configManager *FastMatcherConfigManager) *FastMatcher {
	m := &FastMatcher{
		categoryToTask: make(map[string]TaskType),
		taskToBackends: make(map[TaskType][]BackendRecommendation),
		backendCache:   make(map[string]*backend.BackendConfig),
		configManager:  configManager,
		modelMatcher:   backend.NewModelMatcher(backend.DefaultModelMatchingConfig()),
	}

	// 从配置初始化
	if configManager != nil {
		m.loadFromConfig(configManager.GetConfig())
	} else {
		m.initMappings()
	}

	return m
}

// loadFromConfig 从配置加载映射
func (m *FastMatcher) loadFromConfig(config *FastMatcherConfig) {
	if config == nil {
		m.initMappings()
		return
	}

	// 加载 category → 任务类型映射
	m.categoryToTask = make(map[string]TaskType)
	for category, taskType := range config.CategoryToTask {
		m.categoryToTask[category] = taskType
	}

	// 加载任务类型 → 后端推荐映射
	m.taskToBackends = make(map[TaskType][]BackendRecommendation)
	for taskType, recs := range config.TaskToBackends {
		backendRecs := make([]BackendRecommendation, len(recs))
		for i, rec := range recs {
			backendRecs[i] = BackendRecommendation{
				BackendID: rec.BackendID,
				Model:     rec.Model,
				Priority:  rec.Priority,
				Reason:    rec.Reason,
				Fallbacks: rec.Fallbacks,
			}
		}
		m.taskToBackends[taskType] = backendRecs
	}
}

// initMappings 初始化内置对照表
func (m *FastMatcher) initMappings() {
	// 1. category → 任务类型映射（基于关键词）
	m.categoryToTask = map[string]TaskType{
		// 代码生成
		"code":       TaskCodeGeneration,
		"代码":         TaskCodeGeneration,
		"python":     TaskCodeGeneration,
		"py":         TaskCodeGeneration,
		"java":       TaskCodeGeneration,
		"javascript": TaskCodeGeneration,
		"js":         TaskCodeGeneration,
		"typescript": TaskCodeGeneration,
		"ts":         TaskCodeGeneration,
		"go":         TaskCodeGeneration,
		"golang":     TaskCodeGeneration,
		"rust":       TaskCodeGeneration,
		"cpp":        TaskCodeGeneration,
		"c++":        TaskCodeGeneration,
		"php":        TaskCodeGeneration,
		"ruby":       TaskCodeGeneration,
		"swift":      TaskCodeGeneration,
		"kotlin":     TaskCodeGeneration,
		"sql":        TaskCodeGeneration,
		"shell":      TaskCodeGeneration,
		"bash":       TaskCodeGeneration,
		"程序":         TaskCodeGeneration,
		"函数":         TaskCodeGeneration,
		"方法":         TaskCodeGeneration,
		"脚本":         TaskCodeGeneration,
		"算法":         TaskCodeGeneration,
		"类":          TaskCodeGeneration,
		"接口":         TaskCodeGeneration,
		"模块":         TaskCodeGeneration,
		"库":          TaskCodeGeneration,
		"leetcode":   TaskCodeGeneration,
		"实现":         TaskCodeGeneration,
		"编写":         TaskCodeGeneration,

		// 翻译
		"translate":   TaskTranslation,
		"translation": TaskTranslation,
		"翻译":          TaskTranslation,

		// 摘要
		"summary": TaskLongText,
		"摘要":      TaskLongText,
		"总结":      TaskLongText,

		// 创意写作
		"creative": TaskCreative,
		"story":    TaskCreative,
		"poem":     TaskCreative,
		"创意":       TaskCreative,
		"写作":       TaskCreative,
		"小说":       TaskCreative,
		"诗歌":       TaskCreative,
		"故事":       TaskCreative,

		// 分析推理
		"analysis": TaskAnalysis,
		"reason":   TaskComplexReasoning,
		"math":     TaskComplexReasoning,
		"推理":       TaskComplexReasoning,
		"分析":       TaskAnalysis,
		"数学":       TaskComplexReasoning,
		"逻辑":       TaskComplexReasoning,
		"数据":       TaskAnalysis,
		"图表":       TaskAnalysis,

		// 对话（默认）
		"chat": TaskSimpleChat,
	}

	// 2. 任务类型 → 后端推荐列表（按优先级排序）
	// 注意：这里只定义推荐候选集和默认优先级，实际选择逻辑在 RecommendBackend 中根据策略动态调整
	m.taskToBackends = map[TaskType][]BackendRecommendation{
		TaskCodeGeneration: {
			{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 1, Reason: "代码生成能力强，GLM-4-flash 免费"},
			{BackendID: "deepseek", Model: "deepseek-coder", Priority: 2, Reason: "DeepSeek Coder 专业代码模型"},
			{BackendID: "ppio", Model: "deepseek-coder", Priority: 3, Reason: "PPIO 提供 DeepSeek Coder"},
			{BackendID: "nvidia-free-key", Model: "deepseek-ai/DeepSeek-Coder-V2-Instruct", Priority: 4, Reason: "NVIDIA 免费 API，支持代码模型"},
			{BackendID: "ollama-local", Model: "codellama", Priority: 5, Reason: "本地代码模型（降级方案）"},
		},
		TaskSimpleChat: {
			{BackendID: "ollama-local", Model: "llama3", Priority: 1, Reason: "本地模型，零成本，隐私保护"},
			{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 2, Reason: "智谱 GLM-4-flash 免费"},
			{BackendID: "deepseek", Model: "deepseek-chat", Priority: 3, Reason: "DeepSeek Chat 性价比高"},
			{BackendID: "ppio", Model: "qwen3.5", Priority: 4, Reason: "PPIO 提供通义千问"},
			{BackendID: "nvidia-free-key", Model: "meta/llama-3.3-70b-instruct", Priority: 5, Reason: "NVIDIA 免费 Llama 3.3 70B"},
		},
		TaskComplexReasoning: {
			{BackendID: "deepseek", Model: "deepseek-reasoner", Priority: 1, Reason: "DeepSeek Reasoner 推理能力强"},
			{BackendID: "bigmodel", Model: "GLM-4", Priority: 2, Reason: "智谱 GLM-4 综合能力强"},
			{BackendID: "ppio", Model: "deepseek-reasoner", Priority: 3, Reason: "PPIO 提供 DeepSeek Reasoner"},
			{BackendID: "ollama-local", Model: "llama3", Priority: 5, Reason: "本地推理模型（降级方案）"},
		},
		TaskLongText: {
			{BackendID: "bigmodel", Model: "GLM-4-32K", Priority: 1, Reason: "智谱 GLM-4-32K 支持长文本"},
			{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 支持长上下文"},
			{BackendID: "ppio", Model: "kimi", Priority: 3, Reason: "Kimi 擅长长文档处理"},
			{BackendID: "ollama-local", Model: "llama3", Priority: 5, Reason: "本地模型（降级方案）"},
		},
		TaskTranslation: {
			{BackendID: "bigmodel", Model: "GLM-4-flash", Priority: 1, Reason: "智谱 GLM-4-flash 免费，翻译质量好"},
			{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 翻译能力强"},
			{BackendID: "ppio", Model: "qwen3.5", Priority: 3, Reason: "PPIO 提供通义千问翻译"},
			{BackendID: "ollama-local", Model: "llama3", Priority: 5, Reason: "本地翻译模型（降级方案）"},
		},
		TaskCreative: {
			{BackendID: "bigmodel", Model: "GLM-4", Priority: 1, Reason: "智谱 GLM-4 创意写作能力强"},
			{BackendID: "deepseek", Model: "deepseek-chat", Priority: 2, Reason: "DeepSeek Chat 创意能力好"},
			{BackendID: "ppio", Model: "qwen3.5", Priority: 3, Reason: "PPIO 通义千问创意写作"},
			{BackendID: "ollama-local", Model: "llama3", Priority: 5, Reason: "本地创意模型（降级方案）"},
		},
		TaskAnalysis: {
			{BackendID: "bigmodel", Model: "GLM-4", Priority: 1, Reason: "智谱 GLM-4 数据分析能力强"},
			{BackendID: "deepseek", Model: "deepseek-reasoner", Priority: 2, Reason: "DeepSeek Reasoner 推理分析能力强"},
			{BackendID: "ppio", Model: "deepseek-reasoner", Priority: 3, Reason: "PPIO 提供 DeepSeek Reasoner"},
			{BackendID: "ollama-local", Model: "llama3", Priority: 5, Reason: "本地分析模型（降级方案）"},
		},
		TaskEmbedding: {
			{BackendID: "ollama-local", Model: "nomic-embed-text", Priority: 1, Reason: "本地嵌入模型，零成本"},
		},
	}
}

// UpdateBackendCache 更新后端配置缓存
func (m *FastMatcher) UpdateBackendCache(backends []*backend.BackendConfig) {
	m.backendCache = make(map[string]*backend.BackendConfig)
	for _, b := range backends {
		m.backendCache[b.ID] = b
	}
}

// ReloadConfig 重新加载配置
func (m *FastMatcher) ReloadConfig() error {
	if m.configManager == nil {
		return nil
	}

	if err := m.configManager.Load(); err != nil {
		return err
	}

	m.loadFromConfig(m.configManager.GetConfig())
	return nil
}

// GetConfigManager 获取配置管理器
func (m *FastMatcher) GetConfigManager() *FastMatcherConfigManager {
	return m.configManager
}

// MatchCategory 根据 category 名称快速匹配任务类型
func (m *FastMatcher) MatchCategory(category string) TaskType {
	// 精确匹配
	if taskType, ok := m.categoryToTask[strings.ToLower(strings.TrimSpace(category))]; ok {
		return taskType
	}

	// 部分匹配
	for keyword, taskType := range m.categoryToTask {
		if strings.Contains(strings.ToLower(category), keyword) {
			return taskType
		}
	}

	// 默认返回简单对话
	return TaskSimpleChat
}

// RecommendBackend 根据任务类型推荐后端。
func (m *FastMatcher) RecommendBackend(taskType TaskType, strategy string) *ScheduleDecision {
	decision := &ScheduleDecision{
		Alternatives: make([]BackendAlternative, 0),
	}

	recommendations, ok := m.taskToBackends[taskType]
	if !ok || len(recommendations) == 0 {
		recommendations = m.taskToBackends[TaskSimpleChat]
	}

	// 过滤出已启用且有可用模型的后端
	// 对于非 embedding 任务，要求后端至少有一个 LLM 生成模型（非 embedding 模型）
	var enabledRecs []BackendRecommendation
	var availableRecs []BackendRecommendation
	for _, rec := range recommendations {
		b, ok := m.backendCache[rec.BackendID]
		if !ok || !b.Enabled {
			continue
		}
		// 检查是否有可用的 LLM 模型（非 embedding 模型）
		if taskType != TaskEmbedding {
			llmModels := filterLLMGenerationModels(b.SupportedModels)
			if len(llmModels) == 0 {
				logger.Infof("[FastMatcher] Skipping backend %s: no LLM generation models (has %d total models)",
					rec.BackendID, len(b.SupportedModels))
				continue
			}
		}
		enabledRecs = append(enabledRecs, rec)
		if backendSchedulingAvailable(b) {
			availableRecs = append(availableRecs, rec)
		} else {
			logger.Infof("[FastMatcher] Skipping backend %s: unavailable for scheduling (status=%s, last_error=%s)",
				rec.BackendID,
				func() string {
					if b.HealthStatus == nil {
						return ""
					}
					return b.HealthStatus.Status
				}(),
				func() string {
					if b.HealthStatus == nil {
						return ""
					}
					return b.HealthStatus.LastError
				}())
		}
	}

	if len(enabledRecs) == 0 {
		decision.Reason = "无可用后端"
		return decision
	}
	if len(availableRecs) == 0 {
		decision.Reason = "无可用后端（全部候选后端当前不可用）"
		return decision
	}

	// 根据策略选择
	// 按策略排序
	var sortedRecs []BackendRecommendation
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "balance":
		// selectBalanced 已经返回了单个最优，但为了统一处理，直接赋值
		bestRec := m.selectBalanced(availableRecs)
		b := m.backendCache[bestRec.BackendID]
		model := m.findBestModel(b, bestRec.Model, taskType)
		if model != "" || taskType == TaskEmbedding {
			decision.RecommendedBackendID = bestRec.BackendID
			decision.RecommendedModel = model
			decision.Reason = bestRec.Reason
			decision.StrategyFactors = m.buildStrategyFactors(strategy, taskType, bestRec, b)
			return decision
		}
		// 最优不可用，继续遍历其他候选人
		sortedRecs = availableRecs
	case "cost", "price":
		sortedRecs = m.sortByCostPriority(availableRecs)
	case "quality":
		sortedRecs = m.sortByQualityPriority(availableRecs)
	case "latency", "speed":
		sortedRecs = m.sortByLatencyPriority(availableRecs)
	default:
		// fast / 未知 → 按内置优先级
		sortedRecs = m.adjustByPriority(availableRecs)
	}

	// 遍历排好序的候选人，选择第一个有可用模型的后端
	for _, rec := range sortedRecs {
		b := m.backendCache[rec.BackendID]
		if b == nil {
			continue
		}
		model := m.findBestModel(b, rec.Model, taskType)
		if model != "" || taskType == TaskEmbedding {
			decision.RecommendedBackendID = rec.BackendID
			decision.RecommendedModel = model
			decision.Reason = rec.Reason
			decision.StrategyFactors = m.buildStrategyFactors(strategy, taskType, rec, b)
			return decision
		}
		if b.Type == "ollama" && !isEmbeddingModel(rec.Model) {
			// Ollama 可直接返回首选模型名（即使不在 SupportedModels 中，也可动态拉取）
			decision.RecommendedBackendID = rec.BackendID
			decision.RecommendedModel = rec.Model
			decision.Reason = rec.Reason
			decision.StrategyFactors = m.buildStrategyFactors(strategy, taskType, rec, b)
			return decision
		}
	}

	decision.Reason = "无可用后端（或所有候选后端无可用的 LLM 模型）"
	return decision
}

// selectBalanced 平衡策略：综合评分选择最优后端。
//
// 平衡策略目标是“稳定可用 + 性能可接受 + 业务偏好可控”，而不是单纯追求模型数量。
// 评分维度（高分优先）：
//   - 管理员权重（Weight，主导项，0~100）
//   - 内置优先级（Priority，越小越优）
//   - 健康状态（healthy 加分，unhealthy 扣分）
//   - 观测时延（越低越优，作为稳定性关键因子）
//   - 模型覆盖度（仅作弱区分，避免“模型多但慢/不稳”反超）
//   - 额度/计费错误惩罚（余额不足、quota 限流等强惩罚）
func (m *FastMatcher) selectBalanced(recommendations []BackendRecommendation) BackendRecommendation {
	type scored struct {
		rec   BackendRecommendation
		score float64
	}
	var candidates []scored
	maxModels := 1
	for _, rec := range recommendations {
		if b, ok := m.backendCache[rec.BackendID]; ok {
			if len(b.SupportedModels) > maxModels {
				maxModels = len(b.SupportedModels)
			}
		}
	}

	for _, rec := range recommendations {
		b, ok := m.backendCache[rec.BackendID]
		if !ok {
			continue
		}

		// 管理员权重：主导业务偏好（0~100）。
		weightScore := float64(b.Weight)

		// 内置优先级：越小越优。范围大致 [1.2, 6.0]，再做适度放大。
		priorityScore := (6.0 / float64(rec.Priority)) * 1.2

		// 健康状态：healthy 加分，unknown 轻微扣分，unhealthy 强扣分。
		healthScore := 0.0
		switch backendHealthRank(b) {
		case 0:
			healthScore = 4.0
		case 1:
			healthScore = -1.0
		default:
			healthScore = -15.0
		}

		// 延迟加分：越低越高，未知延迟不加分也不扣分。
		latencyScore := 0.0
		if b != nil && b.HealthStatus != nil && b.HealthStatus.ResponseTime > 0 {
			latencyScore = 8.0 / (1.0 + float64(b.HealthStatus.ResponseTime)/800.0)
		}

		// 模型覆盖度：弱区分，避免“模型越多就一定更优”。
		modelRatio := float64(0)
		if maxModels > 0 {
			modelRatio = float64(len(b.SupportedModels)) / float64(maxModels)
		}
		modelScore := modelRatio * 1.2

		// 额度/计费错误惩罚：若后端最近报过余额/额度问题，强烈降权。
		billingPenalty := 0.0
		if b != nil && b.HealthStatus != nil {
			lastErr := strings.ToLower(strings.TrimSpace(b.HealthStatus.LastError))
			if strings.Contains(lastErr, "not_enough_balance") ||
				strings.Contains(lastErr, "not enough balance") ||
				strings.Contains(lastErr, "insufficient") ||
				strings.Contains(lastErr, "quota") ||
				strings.Contains(lastErr, "余额不足") ||
				strings.Contains(lastErr, "额度不足") {
				billingPenalty = -50.0
			}
		}

		score := weightScore + priorityScore + healthScore + latencyScore + modelScore + billingPenalty

		candidates = append(candidates, scored{rec: rec, score: score})
		logger.Debugf("[FastMatcher] Balanced candidate: %s score=%.2f (weight=%.1f, priority=%.2f, health=%.2f, latency=%.2f, model=%.2f, billing=%.2f)",
			rec.BackendID, score, weightScore, priorityScore, healthScore, latencyScore, modelScore, billingPenalty)
	}

	// 选最高分
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}

	logger.Infof("[FastMatcher] Balanced pick: backend=%s (score=%.2f, weight=%d, priority=%d, models=%d)",
		best.rec.BackendID, best.score,
		func() int {
			if b, ok := m.backendCache[best.rec.BackendID]; ok {
				return b.Weight
			}
			return 0
		}(), best.rec.Priority,
		func() int {
			if b, ok := m.backendCache[best.rec.BackendID]; ok {
				return len(b.SupportedModels)
			}
			return 0
		}())

	return best.rec
}

// adjustByPriority 按内置优先级排序（Priority 值越小越靠前）
func (m *FastMatcher) adjustByPriority(recommendations []BackendRecommendation) []BackendRecommendation {
	result := make([]BackendRecommendation, len(recommendations))
	copy(result, recommendations)
	// 已按 Priority 升序排列，保持不变
	return result
}

// adjustByStrategy 根据策略调整推荐顺序（保留用于向后兼容，新代码走 RecommendBackend 中的 switch）
func (m *FastMatcher) adjustByStrategy(recommendations []BackendRecommendation, strategy string) []BackendRecommendation {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "cost", "price":
		return m.sortByCostPriority(recommendations)
	case "quality":
		return m.sortByQualityPriority(recommendations)
	case "latency", "speed":
		return m.sortByLatencyPriority(recommendations)
	default:
		return recommendations
	}
}

// sortByCostPriority 按成本优先排序
func (m *FastMatcher) sortByCostPriority(recommendations []BackendRecommendation) []BackendRecommendation {
	// 成本优先：本地优先 + 可配置成本元数据 + 管理员权重 + 响应延迟（次要）。
	result := make([]BackendRecommendation, len(recommendations))
	copy(result, recommendations)
	sort.SliceStable(result, func(i, j int) bool {
		bi := m.backendCache[result[i].BackendID]
		bj := m.backendCache[result[j].BackendID]
		scoreI := costPreferenceScore(result[i], bi)
		scoreJ := costPreferenceScore(result[j], bj)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return result[i].Priority < result[j].Priority
	})
	return result
}

// sortByQualityPriority 按质量优先排序
func (m *FastMatcher) sortByQualityPriority(recommendations []BackendRecommendation) []BackendRecommendation {
	// 质量优先：优先显式质量元数据、健康状态、模型覆盖与内置优先级，降低本地模型偏置。
	result := make([]BackendRecommendation, len(recommendations))
	copy(result, recommendations)
	sort.SliceStable(result, func(i, j int) bool {
		bi := m.backendCache[result[i].BackendID]
		bj := m.backendCache[result[j].BackendID]
		scoreI := qualityPreferenceScore(result[i], bi)
		scoreJ := qualityPreferenceScore(result[j], bj)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return result[i].Priority < result[j].Priority
	})
	return result
}

// sortByLatencyPriority 按延迟优先排序
func (m *FastMatcher) sortByLatencyPriority(recommendations []BackendRecommendation) []BackendRecommendation {
	// 延迟优先：优先 healthy，其次按实测 response_time 升序，再用本地优先作为兜底。
	result := make([]BackendRecommendation, len(recommendations))
	copy(result, recommendations)
	sort.SliceStable(result, func(i, j int) bool {
		bi := m.backendCache[result[i].BackendID]
		bj := m.backendCache[result[j].BackendID]
		healthI := backendHealthRank(bi)
		healthJ := backendHealthRank(bj)
		if healthI != healthJ {
			return healthI < healthJ
		}
		latI := backendObservedLatency(bi)
		latJ := backendObservedLatency(bj)
		if latI != latJ {
			return latI < latJ
		}
		localI := isLocalBackendID(result[i].BackendID)
		localJ := isLocalBackendID(result[j].BackendID)
		if localI != localJ {
			return localI
		}
		return result[i].Priority < result[j].Priority
	})
	return result
}

func backendHealthRank(b *backend.BackendConfig) int {
	// 0=healthy, 1=unknown, 2=unhealthy
	if b == nil || b.HealthStatus == nil || strings.TrimSpace(b.HealthStatus.Status) == "" {
		return 1
	}
	if strings.EqualFold(strings.TrimSpace(b.HealthStatus.Status), "healthy") {
		return 0
	}
	return 2
}

func backendSchedulingAvailable(b *backend.BackendConfig) bool {
	if b == nil {
		return false
	}
	// 可用性优先：调度阶段直接排除已知不可用状态。
	if b.HealthStatus == nil {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(b.HealthStatus.Status))
	switch status {
	case "", "healthy", "unknown":
		// 继续检查错误信号
	case "checking", "unhealthy":
		return false
	default:
		// 未知状态保守放行，交给下游策略排序。
	}

	lastErr := strings.ToLower(strings.TrimSpace(b.HealthStatus.LastError))
	if lastErr == "" {
		return true
	}
	// 明确的不可用错误：余额/额度、鉴权、权限拒绝等。
	hardUnavailableSignals := []string{
		"not_enough_balance",
		"not enough balance",
		"insufficient",
		"quota",
		"余额不足",
		"额度不足",
		"authentication failed",
		"unauthorized",
		"invalid api key",
		"http 401",
		"http 403",
		"forbidden",
	}
	for _, signal := range hardUnavailableSignals {
		if strings.Contains(lastErr, signal) {
			return false
		}
	}
	return true
}

func backendObservedLatency(b *backend.BackendConfig) int64 {
	// 未探测到延迟时给较大值，确保有观测值的后端优先。
	const unknownLatency = int64(1<<62 - 1)
	if b == nil || b.HealthStatus == nil || b.HealthStatus.ResponseTime <= 0 {
		return unknownLatency
	}
	return b.HealthStatus.ResponseTime
}

func isLocalBackendID(backendID string) bool {
	id := strings.ToLower(backendID)
	return strings.Contains(id, "ollama") || strings.Contains(id, "local")
}

func (m *FastMatcher) buildStrategyFactors(strategy string, taskType TaskType, rec BackendRecommendation, b *backend.BackendConfig) map[string]any {
	factors := map[string]any{
		"strategy":      strings.ToLower(strings.TrimSpace(strategy)),
		"task_type":     string(taskType),
		"backend_id":    rec.BackendID,
		"priority":      rec.Priority,
		"local_backend": isLocalBackendID(rec.BackendID),
	}
	if b == nil {
		return factors
	}
	factors["weight"] = b.Weight
	if b.HealthStatus != nil {
		factors["health_status"] = b.HealthStatus.Status
		if b.HealthStatus.ResponseTime > 0 {
			factors["observed_latency_ms"] = b.HealthStatus.ResponseTime
		}
	}
	if price, ok := metadataFloat(b.Metadata, "unit_price", "cost_per_1k", "price_per_1k"); ok {
		factors["unit_price"] = price
	}
	if v, ok := metadataFloat(b.Metadata, "cost_score"); ok {
		factors["cost_score"] = v
	}
	if v, ok := metadataFloat(b.Metadata, "quality_score"); ok {
		factors["quality_score"] = v
	}

	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "cost", "price":
		factors["score_hint"] = costPreferenceScore(rec, b)
	case "quality":
		factors["score_hint"] = qualityPreferenceScore(rec, b)
	case "latency", "speed":
		factors["health_rank"] = backendHealthRank(b)
	case "balance":
		lat := backendObservedLatency(b)
		if lat < (1<<62 - 1) {
			factors["observed_latency_ms"] = lat
		}
	}
	return factors
}

func metadataFloat(meta map[string]string, keys ...string) (float64, bool) {
	if len(meta) == 0 {
		return 0, false
	}
	for _, key := range keys {
		v := strings.TrimSpace(meta[key])
		if v == "" {
			continue
		}
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func costPreferenceScore(rec BackendRecommendation, b *backend.BackendConfig) float64 {
	if b == nil {
		return 0
	}
	score := 0.0

	// 1) 本地模型通常零边际成本，给高优先级。
	if isLocalBackendID(rec.BackendID) {
		score += 200
	}

	// 2) 显式成本元数据（可选）：unit_price/cost_per_1k 越低分越高；cost_score 越高分越高。
	if unitPrice, ok := metadataFloat(b.Metadata, "unit_price", "cost_per_1k", "price_per_1k"); ok && unitPrice >= 0 {
		score += 120.0 / (1.0 + unitPrice)
	}
	if costScore, ok := metadataFloat(b.Metadata, "cost_score"); ok {
		score += costScore * 100.0
	}

	// 3) 管理员权重可表达运营成本偏好：权重越低，越偏成本优先。
	w := b.Weight
	if w < 0 {
		w = 0
	}
	if w > 100 {
		w = 100
	}
	score += float64(100-w) * 0.8

	// 4) 延迟作为次要 tie-breaker（低延迟略加分）。
	lat := backendObservedLatency(b)
	if lat > 0 && lat < (1<<62-1) {
		score += 40.0 / (1.0 + float64(lat)/100.0)
	}

	// 5) 健康状态与内置优先级微调。
	switch backendHealthRank(b) {
	case 0:
		score += 10
	case 2:
		score -= 30
	}
	score -= float64(rec.Priority) * 2.0
	return score
}

func qualityPreferenceScore(rec BackendRecommendation, b *backend.BackendConfig) float64 {
	if b == nil {
		return 0
	}
	score := 0.0

	// 1) 显式质量元数据（可选）。
	if qualityScore, ok := metadataFloat(b.Metadata, "quality_score"); ok {
		score += qualityScore * 100.0
	}

	// 2) 优先级（越小越优）与模型覆盖度（可用 LLM 模型越多越稳）。
	priority := rec.Priority
	if priority <= 0 {
		priority = 10
	}
	if priority > 10 {
		priority = 10
	}
	score += float64(10-priority) * 5.0
	llmCount := len(filterLLMGenerationModels(b.SupportedModels))
	if llmCount > 6 {
		llmCount = 6
	}
	score += float64(llmCount) * 3.0

	// 3) 质量优先更看重健康稳定性，弱化本地后端默认优先。
	switch backendHealthRank(b) {
	case 0:
		score += 20
	case 2:
		score -= 40
	}
	if isLocalBackendID(rec.BackendID) {
		score -= 20
	}

	// 4) 管理员权重作为业务偏好补充。
	w := b.Weight
	if w < 0 {
		w = 0
	}
	if w > 100 {
		w = 100
	}
	score += float64(w) * 0.2
	return score
}

// embeddingModelKeywords 向量化/嵌入模型的名称关键词
var embeddingModelKeywords = []string{
	"embed", "embedding", "bge-", "bge_", "nomic-embed",
	"text-embedding", "e5-", "gte-", "jina-embed",
	"stella_", "instructor", "multilingual-e5",
}

// isEmbeddingModel 判断模型名是否为向量化/嵌入模型
func isEmbeddingModel(modelName string) bool {
	lower := strings.ToLower(modelName)
	for _, kw := range embeddingModelKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// filterLLMGenerationModels 从模型列表中过滤出可用于 LLM 生成/聊天的模型（排除 embedding/rerank 模型）
func filterLLMGenerationModels(models []backend.ModelMapping) []backend.ModelMapping {
	var filtered []backend.ModelMapping
	for _, m := range models {
		if !isEmbeddingModel(m.ActualModel) && !isEmbeddingModel(m.RequestedModel) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// findBestModel 为后端选择最佳模型。
//
// 选择策略（按优先级）：
//  1. 精确匹配 preferredModel（不区分大小写）
//  2. 如果 taskType 是 TaskEmbedding，返回第一个匹配的 embedding 模型
//  3. 使用 ModelMatcher 做智能兼容匹配（名称+参数量+家族三维度）
//  4. Ollama 后端：如果无可匹配模型，返回 preferredModel（可动态拉取）
//  5. 回退到第一个非 embedding 的 LLM 模型
func (m *FastMatcher) findBestModel(b *backend.BackendConfig, preferredModel string, taskType TaskType) string {
	// 可用性优先：若后端显式配置了 probe_model，优先将其作为稳定可用模型使用。
	// 这样可避免同一后端下“部分模型可用、部分模型余额不足”时选到不可用模型。
	if b != nil && taskType != TaskEmbedding {
		if probe := strings.TrimSpace(b.ProbeModel); probe != "" && !isEmbeddingModel(probe) {
			for _, sm := range b.SupportedModels {
				if strings.EqualFold(sm.ActualModel, probe) || strings.EqualFold(sm.RequestedModel, probe) {
					logger.Infof("[FastMatcher] Using probe_model: %s in backend %s", sm.ActualModel, b.ID)
					return sm.ActualModel
				}
			}
			if b.Type == "ollama" {
				// Ollama 支持按名称动态拉取，允许直接回退到 probe_model。
				logger.Infof("[FastMatcher] Using probe_model on ollama fallback: %s in backend %s", probe, b.ID)
				return probe
			}
		}
	}

	if preferredModel != "" {
		// 1. 精确匹配（不区分大小写）
		for _, sm := range b.SupportedModels {
			if strings.EqualFold(sm.ActualModel, preferredModel) || strings.EqualFold(sm.RequestedModel, preferredModel) {
				logger.Infof("[FastMatcher] Exact match found: %s in backend %s", sm.ActualModel, b.ID)
				return sm.ActualModel
			}
		}

		// 2. 对于 embedding 任务，允许选择 embedding 模型
		if taskType == TaskEmbedding {
			for _, sm := range b.SupportedModels {
				if isEmbeddingModel(sm.ActualModel) || isEmbeddingModel(sm.RequestedModel) {
					logger.Infof("[FastMatcher] Embedding model match: %s in backend %s", sm.ActualModel, b.ID)
					return sm.ActualModel
				}
			}
			// Ollama 可动态拉取 embedding 模型
			if b.Type == "ollama" {
				return preferredModel
			}
			logger.Warnf("[FastMatcher] No embedding model found in backend %s for task %s", b.ID, taskType)
			return ""
		}

		// 3. 非 embedding 任务：使用 ModelMatcher 智能匹配
		//    过滤掉 embedding 模型，只从 LLM 生成模型中匹配
		llmModels := filterLLMGenerationModels(b.SupportedModels)
		if len(llmModels) > 0 {
			if m.modelMatcher != nil {
				result := m.modelMatcher.Match(preferredModel, []*backend.BackendConfig{b})
				if result != nil && !isEmbeddingModel(result.ActualModel) {
					logger.Infof("[FastMatcher] ModelMatcher match: %s (score=%.3f) in backend %s for preferred %s",
						result.ActualModel, result.CompatibilityScore, b.ID, preferredModel)
					return result.ActualModel
				}
			}

			// 回退：从 LLM 模型列表中选第一个
			best := llmModels[0].ActualModel
			logger.Infof("[FastMatcher] Fallback to first LLM model: %s in backend %s (preferred %s not matched)",
				best, b.ID, preferredModel)
			return best
		}

		// 4. Ollama 后端：模型列表可能为空，直接返回 preferredModel
		if b.Type == "ollama" && !isEmbeddingModel(preferredModel) {
			logger.Infof("[FastMatcher] Ollama fallback: using preferred model %s (can pull dynamically)", preferredModel)
			return preferredModel
		}

		// 5. 没有可用的 LLM 模型
		logger.Warnf("[FastMatcher] No LLM generation model found in backend %s for preferred model %s",
			b.ID, preferredModel)
		return ""
	}

	// preferredModel 为空时：返回第一个非 embedding 的 LLM 模型
	llmModels := filterLLMGenerationModels(b.SupportedModels)
	if len(llmModels) > 0 {
		logger.Infof("[FastMatcher] First LLM model: %s in backend %s", llmModels[0].ActualModel, b.ID)
		return llmModels[0].ActualModel
	}

	// 如果所有模型都是 embedding 模型且任务类型是 embedding，返回第一个
	if taskType == TaskEmbedding && len(b.SupportedModels) > 0 {
		return b.SupportedModels[0].ActualModel
	}

	logger.Warnf("[FastMatcher] No suitable model found in backend %s (task=%s, total models=%d)",
		b.ID, taskType, len(b.SupportedModels))
	return ""
}
