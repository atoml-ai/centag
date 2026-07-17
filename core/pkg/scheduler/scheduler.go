package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// Scheduler 智能调度器
type Scheduler struct {
	config     *SchedulerConfig
	classifier *IntentClassifier
	backendMgr *backend.Manager
	selector   BackendSelector // 后端选择器，支持可配置策略
	scorer     *MultiDimensionScorer
	fastMatcher *FastMatcher // 快速匹配器（基于内置对照表）
	mu         sync.RWMutex
	stats      *SchedulerStats
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	IntentClassifier IntentClassifierConfig `json:"intent_classifier"`
	EnableLogging    bool                   `json:"enable_logging"`
	EnableStats      bool                   `json:"enable_stats"`
}

// SchedulerStats 调度统计
type SchedulerStats struct {
	TotalRequests      int64               `json:"total_requests"`
	ClassificationHits int64               `json:"classification_hits"` // 分类命中次数
	TaskTypeStats      map[TaskType]int64  `json:"task_type_stats"`
	ComplexityStats    map[ComplexityLevel]int64 `json:"complexity_stats"`
	AvgClassificationMs float64            `json:"avg_classification_ms"`
	lastUpdated        time.Time
}

// DefaultSchedulerConfig 返回默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		IntentClassifier: DefaultIntentClassifierConfig(),
		EnableLogging:    true,
		EnableStats:      true,
	}
}

// NewScheduler 创建智能调度器
func NewScheduler(config *SchedulerConfig, backendMgr *backend.Manager) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	scheduler := &Scheduler{
		config:     config,
		classifier: NewIntentClassifier(config.IntentClassifier),
		backendMgr: backendMgr,
		selector:   NewTaskTypeSelector(), // 后端选择器；#s 默认走多维评分
		scorer:     NewMultiDimensionScorer(),
		fastMatcher: NewFastMatcher(),
		stats: &SchedulerStats{
			TaskTypeStats:   make(map[TaskType]int64),
			ComplexityStats: make(map[ComplexityLevel]int64),
		},
	}

	// 初始化快速匹配器的后端缓存
	if backendMgr != nil {
		scheduler.fastMatcher.UpdateBackendCache(backendMgr.List())
	}

	// 安全日志输出（logger 可能未初始化）
	if config.EnableLogging {
		logger.Infof("[Scheduler] Initialized with model=%s, addr=%s",
			config.IntentClassifier.LocalModel,
			config.IntentClassifier.OllamaAddr)
	}

	return scheduler
}

// UpdateBackendCache 更新快速匹配器的后端缓存
func (s *Scheduler) UpdateBackendCache(backends []*backend.BackendConfig) {
	if s == nil || s.fastMatcher == nil {
		return
	}
	s.fastMatcher.UpdateBackendCache(backends)
}

// RecordRequestResult feeds production latency/success metrics into the scorer loop.
func (s *Scheduler) RecordRequestResult(backendID, model string, latencyMs int64, success bool, qualityScore float64) {
	if s == nil || s.scorer == nil || strings.TrimSpace(backendID) == "" {
		return
	}
	s.scorer.RecordRequestResult(backendID, model, latencyMs, success, qualityScore)
}

// SetSelector 设置后端选择器（用于动态替换选择策略）
func (s *Scheduler) SetSelector(selector BackendSelector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selector = selector
}

// GetSelector 获取当前后端选择器
func (s *Scheduler) GetSelector() BackendSelector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selector
}

// ScheduleDecision 调度决策结果
type ScheduleDecision struct {
	Intent           *ClassificationResult   `json:"intent"`            // 意图分类结果
	RecommendedBackendID string              `json:"recommended_backend_id"` // 推荐后端 ID
	RecommendedModel string                  `json:"recommended_model"`      // 推荐模型
	Reason           string                  `json:"reason"`                // 调度理由
	StrategyFactors  map[string]any          `json:"strategy_factors,omitempty"` // 策略可视化因子
	Alternatives     []BackendAlternative    `json:"alternatives"`          // 备选方案
	EstimatedCost    float64                 `json:"estimated_cost"`        // 预估成本
	EstimatedLatencyMs int64                 `json:"estimated_latency_ms"`  // 预估延迟
}

// BackendAlternative 备选后端
type BackendAlternative struct {
	BackendID      string  `json:"backend_id"`
	BackendName    string  `json:"backend_name"`
	Model          string  `json:"model"`
	Score          float64 `json:"score"`
	Reason         string  `json:"reason"`
}

// Schedule 智能调度入口函数（默认 balance 策略 = 六维评分）
func (s *Scheduler) Schedule(question string, requestedModel string) (*ScheduleDecision, error) {
	return s.ScheduleWithStrategy(question, requestedModel, "balance")
}

// ScheduleWithCategory 直接根据 category 推荐后端（跳过意图分类器）。
// 专为 auto-build 设计，确保 category → 任务类型映射准确。
func (s *Scheduler) ScheduleWithCategory(category string, strategy string) (*ScheduleDecision, error) {
	if s.fastMatcher == nil {
		return nil, fmt.Errorf("fast matcher not initialized")
	}

	// 使用 fast matcher 直接匹配 category → 任务类型
	taskType := s.fastMatcher.MatchCategory(category)
	logger.Infof("[Scheduler] ScheduleWithCategory: category=%s, taskType=%s, strategy=%s", category, taskType, strategy)

	// 根据任务类型推荐后端
	decision := s.fastMatcher.RecommendBackend(taskType, strategy)
	logger.Infof("[Scheduler] ScheduleWithCategory result: backend=%s, model=%s, reason=%s",
		decision.RecommendedBackendID, decision.RecommendedModel, decision.Reason)

	return decision, nil
}

// ScheduleWithStrategy 按策略调度：balance/cost/quality/latency 走六维评分；legacy/keyword 走任务类型启发式；fast 走内置对照表快速匹配。
func (s *Scheduler) ScheduleWithStrategy(question string, requestedModel string, strategy string) (*ScheduleDecision, error) {
	startTime := time.Now()

	// 1. 意图分类
	intent, err := s.classifier.Classify(question)
	if err != nil {
		logger.Warnf("[Scheduler] Classification failed, using fallback: %v", err)
		intent = s.classifier.getDefaultClassification(question)
	}

	// 2. 更新统计
	if s.config.EnableStats {
		s.updateStats(intent, time.Since(startTime))
	}

	// 3. 根据意图与策略推荐后端
	var decision *ScheduleDecision
	if strategy == "fast" && s.fastMatcher != nil {
		// 快速匹配模式：基于内置对照表，无需探测
		logger.Infof("[Scheduler] Fast match: taskType=%s, strategy=%s", intent.TaskType, strategy)
		decision = s.fastMatcher.RecommendBackend(intent.TaskType, strategy)
		logger.Infof("[Scheduler] Fast match result: backend=%s, model=%s, reason=%s",
			decision.RecommendedBackendID, decision.RecommendedModel, decision.Reason)
	} else if shouldUseScoring(strategy) && s.scorer != nil {
		decision = s.recommendByScoring(intent, requestedModel, strategy)
	} else {
		decision = s.recommendBackend(intent, requestedModel)
	}
	decision.Intent = intent

	// 4. 日志记录
	if s.config.EnableLogging {
		logger.Infof("[Scheduler] Decision: task=%s, backend=%s, model=%s, reason=%s, strategy=%s",
			intent.TaskType, decision.RecommendedBackendID, decision.RecommendedModel, decision.Reason, strategy)
	}

	return decision, nil
}

// recommendBackend 根据意图推荐后端
// 如果配置了 BackendSelector，则使用 Selector；否则回退到原有的硬编码逻辑
func (s *Scheduler) recommendBackend(intent *ClassificationResult, requestedModel string) *ScheduleDecision {
	decision := &ScheduleDecision{
		Alternatives: make([]BackendAlternative, 0),
	}

	// 获取所有启用的后端
	backends := s.getEnabledBackends()
	if len(backends) == 0 {
		decision.Reason = "无可用后端"
		return decision
	}

	// 如果配置了 selector，使用 selector
	if s.selector != nil {
		recommended, reason := s.selector.Select(intent.TaskType, backends)
		if recommended != nil {
			decision.RecommendedBackendID = recommended.ID
			decision.RecommendedModel = s.findBestModel(recommended, requestedModel, intent.TaskType)
			decision.Reason = reason
		} else {
			decision.Reason = "Selector 无法选择后端"
		}
		// 生成备选方案
		decision.Alternatives = s.generateAlternatives(backends, recommended, intent)
		return decision
	}

	// 回退到原有的硬编码逻辑（兼容未配置 selector 的场景）
	var recommended *backend.BackendConfig
	var reason string

	switch intent.TaskType {
	case TaskCodeGeneration:
		recommended, reason = s.selectForCodeGeneration(backends, intent)
	case TaskSimpleChat:
		recommended, reason = s.selectForSimpleChat(backends, intent)
	case TaskComplexReasoning:
		recommended, reason = s.selectForComplexReasoning(backends, intent)
	case TaskLongText:
		recommended, reason = s.selectForLongText(backends, intent)
	case TaskEmbedding:
		recommended, reason = s.selectForEmbedding(backends, intent)
	case TaskTranslation:
		recommended, reason = s.selectForTranslation(backends, intent)
	case TaskCreative:
		recommended, reason = s.selectForCreative(backends, intent)
	case TaskAnalysis:
		recommended, reason = s.selectForAnalysis(backends, intent)
	default:
		recommended, reason = s.selectByModelMatching(backends, requestedModel)
	}

	if recommended != nil {
		decision.RecommendedBackendID = recommended.ID
		decision.RecommendedModel = s.findBestModel(recommended, requestedModel, intent.TaskType)
		decision.Reason = reason
	}

	// 生成备选方案
	decision.Alternatives = s.generateAlternatives(backends, recommended, intent)

	return decision
}

// selectForCodeGeneration 代码生成任务选择
func (s *Scheduler) selectForCodeGeneration(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：bigmodel > ppinfra(deepseek) > ollama-local
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "代码生成任务，优先使用智谱 GLM 模型"
		}
	}
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "代码任务，使用 PPIO DeepSeek 模型"
		}
	}
	// 降级到本地
	for _, b := range backends {
		if b.ID == "ollama-local" && b.Enabled {
			return b, "代码任务，使用本地模型（降级方案）"
		}
	}
	return nil, "无适合代码生成的后端"
}

// selectForSimpleChat 简单对话任务选择
func (s *Scheduler) selectForSimpleChat(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：ollama-local > bigmodel(flash) > ppinfra(qwen3.5)
	// 优先本地，其次低成本
	if intent.Complexity == ComplexityLow {
		for _, b := range backends {
			if b.ID == "ollama-local" && b.Enabled {
				return b, "简单对话，使用本地模型节省成本"
			}
		}
	}
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "简单对话，使用智谱 Flash 模型（低成本）"
		}
	}
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "简单对话，使用 PPIO 通义千问"
		}
	}
	return nil, "无适合简单对话的后端"
}

// selectForComplexReasoning 复杂推理任务选择
func (s *Scheduler) selectForComplexReasoning(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：bigmodel(glm-5) > ppinfra(glm-5)
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "复杂推理，使用智谱 GLM-5 高质量模型"
		}
	}
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "复杂推理，使用 PPIO GLM-5"
		}
	}
	return nil, "无适合复杂推理的后端"
}

// selectForLongText 长文本任务选择
func (s *Scheduler) selectForLongText(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：ppinfra(kimi) > bigmodel
	// Kimi 擅长长文档处理
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "长文本处理，使用 Kimi 模型（擅长长文档）"
		}
	}
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "长文本处理，使用智谱模型（128K 上下文）"
		}
	}
	return nil, "无适合长文本处理的后端"
}

// selectForEmbedding 向量嵌入任务选择
func (s *Scheduler) selectForEmbedding(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：ollama-local(bge-m3) > 其他
	for _, b := range backends {
		if b.ID == "ollama-local" && b.Enabled {
			return b, "向量嵌入，使用本地 BGE-M3 模型（零成本）"
		}
	}
	return nil, "无适合向量嵌入的后端"
}

// selectForTranslation 翻译任务选择
func (s *Scheduler) selectForTranslation(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：ppinfra > bigmodel
	// 性价比优先
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "翻译任务，使用 PPIO（性价比高）"
		}
	}
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "翻译任务，使用智谱模型"
		}
	}
	return nil, "无适合翻译的后端"
}

// selectForCreative 创意写作任务选择
func (s *Scheduler) selectForCreative(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：bigmodel(glm-5) > ppinfra
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "创意写作，使用智谱 GLM-5（高质量）"
		}
	}
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "创意写作，使用 PPIO"
		}
	}
	return nil, "无适合创意写作的后端"
}

// selectForAnalysis 数据分析任务选择
func (s *Scheduler) selectForAnalysis(backends []*backend.BackendConfig, intent *ClassificationResult) (*backend.BackendConfig, string) {
	// 优先级：bigmodel(glm-5) > ppinfra(deepseek)
	for _, b := range backends {
		if b.ID == "bigmodel" && b.Enabled {
			return b, "数据分析，使用智谱 GLM-5（强推理能力）"
		}
	}
	for _, b := range backends {
		if b.ID == "ppinfra" && b.Enabled {
			return b, "数据分析，使用 DeepSeek 模型"
		}
	}
	return nil, "无适合数据分析的后端"
}

// selectByModelMatching 基于模型匹配的默认选择
func (s *Scheduler) selectByModelMatching(backends []*backend.BackendConfig, requestedModel string) (*backend.BackendConfig, string) {
	if requestedModel == "" {
		// 返回第一个启用的后端
		for _, b := range backends {
			if b.Enabled {
				return b, fmt.Sprintf("默认选择后端：%s", b.Name)
			}
		}
		return nil, "无可用后端"
	}

	// 使用现有的后端选择器
	selector := backend.NewBackendSelector(backend.DefaultModelMatchingConfig())
	selected, actualModel, err := selector.SelectBackendByModel(requestedModel, backends)
	if err != nil {
		logger.Warnf("[Scheduler] Model matching failed: %v", err)
		return nil, fmt.Sprintf("模型匹配失败：%v", err)
	}

	return selected, fmt.Sprintf("基于模型匹配选择：%s -> %s", requestedModel, actualModel)
}

// findBestModel 在给定后端中查找最佳模型。
// 会过滤掉 embedding 模型，确保为 LLM 生成任务选择正确的模型。
//
// 选择策略（与 FastMatcher 对齐）：
//  1. 有 requestedModel → 精确/兼容匹配
//  2. 无 requestedModel → 优先 ProbeModel（用户在 UI 设的默认模型）
//  3. 再回退到第一个非 embedding 的 SupportedModels
func (s *Scheduler) findBestModel(cfg *backend.BackendConfig, requestedModel string, taskType TaskType) string {
	if requestedModel == "" {
		// 可用性优先：显式 probe_model 即后端默认对话模型
		if cfg != nil && taskType != TaskEmbedding {
			if probe := strings.TrimSpace(cfg.ProbeModel); probe != "" && !isEmbeddingModel(probe) {
				for _, mapping := range cfg.SupportedModels {
					if strings.EqualFold(mapping.ActualModel, probe) || strings.EqualFold(mapping.RequestedModel, probe) {
						return mapping.ActualModel
					}
				}
				if cfg.Type == "ollama" {
					return probe
				}
				// 列表里没有同名项时仍信任 probe_model（用户显式配置）
				return probe
			}
		}
		// 返回第一个非 embedding 的 LLM 模型
		for _, mapping := range cfg.SupportedModels {
			if !isEmbeddingModel(mapping.ActualModel) {
				return mapping.ActualModel
			}
		}
		// 如果是 embedding 任务，允许返回 embedding 模型
		if taskType == TaskEmbedding && len(cfg.SupportedModels) > 0 {
			return cfg.SupportedModels[0].ActualModel
		}
		return ""
	}

	// 查找精确匹配
	for _, mapping := range cfg.SupportedModels {
		if strings.EqualFold(mapping.RequestedModel, requestedModel) || strings.EqualFold(mapping.ActualModel, requestedModel) {
			return mapping.ActualModel
		}
	}

	// 使用 ModelMatcher 做智能兼容匹配（仅从 LLM 模型中匹配）
	llmModels := filterLLMGenerationModels(cfg.SupportedModels)
	if len(llmModels) > 0 {
		matcher := backend.NewModelMatcher(backend.DefaultModelMatchingConfig())
		result := matcher.Match(requestedModel, []*backend.BackendConfig{cfg})
		if result != nil && !isEmbeddingModel(result.ActualModel) {
			return result.ActualModel
		}
		// 回退到第一个 LLM 模型
		return llmModels[0].ActualModel
	}

	// Ollama 后端可以直接返回 requestedModel（可动态拉取）
	if cfg.Type == "ollama" && !isEmbeddingModel(requestedModel) {
		return requestedModel
	}

	if len(cfg.SupportedModels) > 0 {
		return cfg.SupportedModels[0].ActualModel
	}

	return requestedModel
}

// generateAlternatives 生成备选方案
func (s *Scheduler) generateAlternatives(
	backends []*backend.BackendConfig,
	excluded *backend.BackendConfig,
	intent *ClassificationResult,
) []BackendAlternative {
	alternatives := make([]BackendAlternative, 0)

	for _, b := range backends {
		if b.ID == excluded.ID || !b.Enabled {
			continue
		}

		score := s.calculateAlternativeScore(b, intent)
		if score > 0.3 { // 阈值
			alternatives = append(alternatives, BackendAlternative{
				BackendID:   b.ID,
				BackendName: b.Name,
				Model:       s.findBestModel(b, "", intent.TaskType),
				Score:       score,
				Reason:      s.getAlternativeReason(b, intent),
			})
		}
	}

	return alternatives
}

// calculateAlternativeScore 计算备选后端评分
func (s *Scheduler) calculateAlternativeScore(backend *backend.BackendConfig, intent *ClassificationResult) float64 {
	score := 0.5 // 基础分

	// 根据任务类型调整
	switch intent.TaskType {
	case TaskCodeGeneration:
		if backend.ID == "bigmodel" {
			score = 0.9
		} else if backend.ID == "ppinfra" {
			score = 0.7
		}
	case TaskSimpleChat:
		if backend.ID == "ollama-local" {
			score = 0.9
		} else if backend.Weight < 30 {
			score += 0.2 // 低成本加分
		}
	case TaskComplexReasoning:
		if backend.ID == "bigmodel" {
			score = 0.9
		}
	}

	// 优先级加分
	if backend.Priority > 0 {
		score += float64(backend.Priority) * 0.05
	}

	// 权重加分
	if backend.Weight > 50 {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// getAlternativeReason 获取备选理由
func (s *Scheduler) getAlternativeReason(backend *backend.BackendConfig, intent *ClassificationResult) string {
	switch backend.ID {
	case "ollama-local":
		return "本地运行，零成本，低延迟"
	case "ppinfra":
		return "性价比高，模型丰富"
	case "bigmodel":
		return "推理能力强，质量高"
	default:
		return "备选方案"
	}
}

// getEnabledBackends 获取所有启用的后端
func (s *Scheduler) getEnabledBackends() []*backend.BackendConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.backendMgr == nil {
		return nil
	}

	all := s.backendMgr.List()
	enabled := make([]*backend.BackendConfig, 0)
	for _, b := range all {
		if b.Enabled {
			enabled = append(enabled, b)
		}
	}

	return enabled
}

// updateStats 更新统计
func (s *Scheduler) updateStats(intent *ClassificationResult, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.TotalRequests++
	s.stats.TaskTypeStats[intent.TaskType]++
	s.stats.ComplexityStats[intent.Complexity]++

	if intent.Confidence >= 0.7 {
		s.stats.ClassificationHits++
	}

	// 更新平均耗时
	totalMs := float64(s.stats.TotalRequests) * s.stats.AvgClassificationMs
	newMs := float64(duration.Milliseconds())
	s.stats.AvgClassificationMs = (totalMs + newMs) / (float64(s.stats.TotalRequests) + 1)
	s.stats.lastUpdated = time.Now()
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := *s.stats
	stats.TaskTypeStats = make(map[TaskType]int64)
	stats.ComplexityStats = make(map[ComplexityLevel]int64)
	for k, v := range s.stats.TaskTypeStats {
		stats.TaskTypeStats[k] = v
	}
	for k, v := range s.stats.ComplexityStats {
		stats.ComplexityStats[k] = v
	}

	return &stats
}

// ReloadConfig 重新加载配置
func (s *Scheduler) ReloadConfig(config *SchedulerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	if config != nil {
		s.classifier = NewIntentClassifier(config.IntentClassifier)
		logger.Infof("[Scheduler] Configuration reloaded: model=%s",
			config.IntentClassifier.LocalModel)
	}
}

// Close 关闭调度器
func (s *Scheduler) Close() {
	if s.classifier != nil {
		s.classifier.Close()
	}
	logger.Info("[Scheduler] Closed")
}

// AuditSchedule 审核模式调度入口
// 由执行模型完成请求，审核模型对结果进行审核
func (s *Scheduler) AuditSchedule(
	ctx context.Context,
	question string,
	requestedModel string,
	auditConfig *AuditConfig,
	executeFunc func(context.Context, string, string) (string, error),
) (*AuditDecision, error) {
	// 1. 使用审核配置
	if auditConfig == nil {
		auditConfig = DefaultAuditConfig()
	}

	logger.Info("[Scheduler] Audit mode started",
		zap.String("executor_backend", auditConfig.ExecutorBackendID),
		zap.String("auditor_backend", auditConfig.AuditorBackendID),
		zap.String("auditor_model", auditConfig.AuditorModel))

	// 2. 执行阶段：调用执行模型
	executorAnswer, err := executeFunc(ctx, auditConfig.ExecutorBackendID, requestedModel)
	if err != nil {
		logger.Error("[Scheduler] Executor failed", zap.Error(err))
		return nil, fmt.Errorf("executor failed: %w", err)
	}

	// 3. 审核阶段
	auditor := NewAuditor(auditConfig, s.backendMgr)
	auditResult, auditErr := auditor.Audit(ctx, question, executorAnswer, requestedModel)

	// 4. 处理审核结果
	if auditErr != nil {
		logger.Warn("[Scheduler] Audit failed", zap.Error(auditErr))
		if auditConfig.BypassOnTimeout {
			// 超时绕过
			return s.createBypassDecision(executorAnswer, auditErr, auditConfig), nil
		}
		return nil, fmt.Errorf("audit failed: %w", auditErr)
	}

	// 5. 决策阶段
	if auditResult.Passed {
		logger.Info("[Scheduler] Audit passed",
			zap.Float64("score", auditResult.Score),
			zap.String("feedback", auditResult.Feedback))
		return s.createPassDecision(executorAnswer, auditResult, auditConfig), nil
	}

	// 6. 重试逻辑
	if auditConfig.AutoRetry && auditConfig.MaxRetries > 0 {
		return s.handleRetry(ctx, question, requestedModel, executorAnswer, auditResult, auditConfig, executeFunc)
	}

	logger.Info("[Scheduler] Audit rejected",
		zap.Float64("score", auditResult.Score),
		zap.String("feedback", auditResult.Feedback))
	return s.createRejectDecision(executorAnswer, auditResult, auditConfig), nil
}

// createPassDecision 创建通过决策
func (s *Scheduler) createPassDecision(answer string, auditResult *AuditResult, config *AuditConfig) *AuditDecision {
	return &AuditDecision{
		ExecutorBackendID: config.ExecutorBackendID,
		AuditorBackendID:  config.AuditorBackendID,
		ExecutorModel:     "", // 由执行函数填充
		AuditorModel:      config.AuditorModel,
		OriginalAnswer:    answer,
		FinalAnswer:       answer,
		AuditResult:       auditResult,
		RetryCount:        0,
		Action:            "pass",
		Reason:            fmt.Sprintf("审核通过，评分：%.2f", auditResult.Score),
	}
}

// createBypassDecision 创建绕过决策
func (s *Scheduler) createBypassDecision(answer string, err error, config *AuditConfig) *AuditDecision {
	return &AuditDecision{
		ExecutorBackendID: config.ExecutorBackendID,
		AuditorBackendID:  config.AuditorBackendID,
		ExecutorModel:     "",
		AuditorModel:      config.AuditorModel,
		OriginalAnswer:    answer,
		FinalAnswer:       answer,
		AuditResult: &AuditResult{
			Passed:   false,
			Score:    0,
			Feedback: fmt.Sprintf("审核失败，已绕过：%v", err),
		},
		RetryCount: 0,
		Action:     "bypass",
		Reason:     "审核超时或失败，已绕过审核直接返回",
	}
}

// createRejectDecision 创建拒绝决策
func (s *Scheduler) createRejectDecision(answer string, auditResult *AuditResult, config *AuditConfig) *AuditDecision {
	return &AuditDecision{
		ExecutorBackendID: config.ExecutorBackendID,
		AuditorBackendID:  config.AuditorBackendID,
		ExecutorModel:     "",
		AuditorModel:      config.AuditorModel,
		OriginalAnswer:    answer,
		FinalAnswer:       answer,
		AuditResult:       auditResult,
		RetryCount:        0,
		Action:            "reject",
		Reason:            fmt.Sprintf("审核未通过，评分：%.2f，反馈：%s", auditResult.Score, auditResult.Feedback),
	}
}

// handleRetry 处理重试逻辑
func (s *Scheduler) handleRetry(
	ctx context.Context,
	question string,
	requestedModel string,
	originalAnswer string,
	firstAuditResult *AuditResult,
	config *AuditConfig,
	executeFunc func(context.Context, string, string) (string, error),
) (*AuditDecision, error) {
	logger.Info("[Scheduler] Starting retry",
		zap.Int("max_retries", config.MaxRetries),
		zap.Float64("first_score", firstAuditResult.Score))

	auditor := NewAuditor(config, s.backendMgr)
	bestDecision := s.createRejectDecision(originalAnswer, firstAuditResult, config)
	bestScore := firstAuditResult.Score

	for retry := 1; retry <= config.MaxRetries; retry++ {
		logger.Info("[Scheduler] Retry attempt", zap.Int("retry", retry))

		// 重新执行
		retryAnswer, err := executeFunc(ctx, config.ExecutorBackendID, requestedModel)
		if err != nil {
			logger.Warn("[Scheduler] Retry execution failed", zap.Int("retry", retry), zap.Error(err))
			continue
		}

		// 重新审核
		retryResult, err := auditor.Audit(ctx, question, retryAnswer, requestedModel)
		if err != nil {
			logger.Warn("[Scheduler] Retry audit failed", zap.Int("retry", retry), zap.Error(err))
			continue
		}

		// 检查是否通过
		if retryResult.Passed {
			logger.Info("[Scheduler] Retry passed",
				zap.Int("retry", retry),
				zap.Float64("score", retryResult.Score))
			return &AuditDecision{
				ExecutorBackendID: config.ExecutorBackendID,
				AuditorBackendID:  config.AuditorBackendID,
				ExecutorModel:     "",
				AuditorModel:      config.AuditorModel,
				OriginalAnswer:    retryAnswer,
				FinalAnswer:       retryAnswer,
				AuditResult:       retryResult,
				RetryCount:        retry,
				Action:            "pass",
				Reason:            fmt.Sprintf("第 %d 次重试后审核通过，评分：%.2f", retry, retryResult.Score),
			}, nil
		}

		// 保留最佳结果
		if retryResult.Score > bestScore {
			bestScore = retryResult.Score
			bestDecision = &AuditDecision{
				ExecutorBackendID: config.ExecutorBackendID,
				AuditorBackendID:  config.AuditorBackendID,
				ExecutorModel:     "",
				AuditorModel:      config.AuditorModel,
				OriginalAnswer:    retryAnswer,
				FinalAnswer:       retryAnswer,
				AuditResult:       retryResult,
				RetryCount:        retry,
				Action:            "reject",
				Reason:            fmt.Sprintf("所有重试均未通过，最佳评分：%.2f（第 %d 次重试）", bestScore, retry),
			}
		}
	}

	logger.Info("[Scheduler] All retries completed",
		zap.Float64("best_score", bestScore),
		zap.String("action", bestDecision.Action))

	return bestDecision, nil
}

// OptimizeSchedule 优化模式调度入口
// 由执行模型完成请求，优化模型对结果进行优化后返回
func (s *Scheduler) OptimizeSchedule(
	ctx context.Context,
	question string,
	requestedModel string,
	optimizeConfig *OptimizeConfig,
	executeFunc func(context.Context, string, string) (string, error),
) (*OptimizeDecision, error) {
	// 1. 使用优化配置
	if optimizeConfig == nil {
		optimizeConfig = DefaultOptimizeConfig()
	}

	logger.Info("[Scheduler] Optimize mode started",
		zap.String("executor_backend", optimizeConfig.ExecutorBackendID),
		zap.String("optimizer_backend", optimizeConfig.OptimizerBackend),
		zap.String("optimizer_model", optimizeConfig.OptimizerModel))

	// 2. 执行阶段：调用执行模型
	executorAnswer, err := executeFunc(ctx, optimizeConfig.ExecutorBackendID, requestedModel)
	if err != nil {
		logger.Error("[Scheduler] Executor failed", zap.Error(err))
		return nil, fmt.Errorf("executor failed: %w", err)
	}

	// 3. 优化阶段
	optimizer := NewOptimizer(optimizeConfig, s.backendMgr)
	optimizeResult, optimizeErr := optimizer.Optimize(ctx, question, executorAnswer, requestedModel)

	// 4. 处理优化结果
	if optimizeErr != nil {
		logger.Warn("[Scheduler] Optimization failed", zap.Error(optimizeErr))
		if optimizeConfig.BypassOnTimeout {
			// 优化失败，降级返回原始答案
			return s.createOptimizeBypassDecision(executorAnswer, optimizeErr, optimizeConfig), nil
		}
		return nil, fmt.Errorf("optimize failed: %w", optimizeErr)
	}

	// 5. 决策阶段 - 优化成功
	if optimizeResult.Optimized && optimizeResult.OptimizedText != "" {
		logger.Info("[Scheduler] Optimization succeeded",
			zap.Bool("improved", optimizeResult.Optimized),
			zap.Int64("duration_ms", optimizeResult.DurationMs))
		return s.createOptimizedDecision(executorAnswer, optimizeResult, optimizeConfig), nil
	}

	// 6. 优化结果为空，降级返回原始答案
	logger.Warn("[Scheduler] Optimization returned empty result, bypassing to original")
	return s.createOptimizeBypassDecision(executorAnswer, fmt.Errorf("optimization returned empty"), optimizeConfig), nil
}

// createOptimizedDecision 创建优化成功决策
func (s *Scheduler) createOptimizedDecision(originalAnswer string, optimizeResult *OptimizeResult, config *OptimizeConfig) *OptimizeDecision {
	return &OptimizeDecision{
		ExecutorBackendID: config.ExecutorBackendID,
		OptimizerBackend:  config.OptimizerBackend,
		ExecutorModel:     "",
		OptimizerModel:    config.OptimizerModel,
		OriginalAnswer:    originalAnswer,
		OptimizeResult:   optimizeResult,
		FinalAnswer:      optimizeResult.OptimizedText,
		RetryCount:       0,
		Action:           "optimized",
		Reason:           fmt.Sprintf("优化成功，返回优化后答案（原始长度: %d，优化后长度: %d）",
			len(originalAnswer), len(optimizeResult.OptimizedText)),
	}
}

// createOptimizeBypassDecision 创建优化降级决策
func (s *Scheduler) createOptimizeBypassDecision(originalAnswer string, err error, config *OptimizeConfig) *OptimizeDecision {
	return &OptimizeDecision{
		ExecutorBackendID: config.ExecutorBackendID,
		OptimizerBackend:  config.OptimizerBackend,
		ExecutorModel:    "",
		OptimizerModel:   config.OptimizerModel,
		OriginalAnswer:   originalAnswer,
		OptimizeResult: &OptimizeResult{
			Optimized:  false,
			Original:   originalAnswer,
			OptimizedText: originalAnswer,
		},
		FinalAnswer: originalAnswer,
		RetryCount: 0,
		Action:     "bypass",
		Reason:     fmt.Sprintf("优化失败或超时，返回原始答案: %v", err),
	}
}

