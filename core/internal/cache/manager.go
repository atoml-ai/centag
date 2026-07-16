package cache

import (
	"context"
	"time"

	"centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/config"
	"centag/core/pkg/embedding"
	"centag/core/pkg/logger"
	"centag/core/pkg/processor"
	"centag/core/pkg/storage"

	"go.uber.org/zap"
)

// Manager 缓存管理器
type Manager struct {
	exactCache        *ExactMatchCache
	semanticCache     *SemanticCacheImpl
	strategy          CacheStrategy
	stats             *CacheMetrics
	config            *CacheConfig
	embeddingService  embedding.EmbeddingService
	semanticConfig    *SemanticCacheConfig
	kvStore           storage.KVStore           // KV存储
	vectorStore       storage.VectorStore       // 向量存储
	qaSplitter        *processor.QASplitter     // 问答拆分器
	evaluationManager EvaluationPipelineManager // 评估流水线管理器
	saveOnlyMode      bool                      // 仅保存模式：不写入语义缓存，不进行向量化
}

// EvaluationPipelineManager 评估流水线管理器接口
type EvaluationPipelineManager interface {
	// Execute 执行评估流水线（使用plugin.EvalInput）
	Execute(ctx context.Context, input *plugin.EvalInput) (*plugin.EvalOutput, error)
	// IsEnabled 检查评估是否启用
	IsEnabled() bool
}

// NewManager 创建缓存管理器
func NewManager(config *CacheConfig) (*Manager, error) {
	// 先创建共享的统计实例
	sharedMetrics := NewCacheMetrics()

	// 使用共享统计实例创建精确匹配缓存
	exactCache, err := NewExactMatchCacheWithMetrics(config, sharedMetrics)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		exactCache: exactCache,
		strategy:   CacheStrategyExact,
		stats:      sharedMetrics, // 使用同一个统计实例
		config:     config,
	}

	logger.Info("Cache manager initialized",
		zap.String("strategy", string(manager.strategy)),
		zap.Bool("enabled", config.Enabled))

	return manager, nil
}

// Get 获取缓存
func (m *Manager) Get(ctx context.Context, key string) (*CacheEntry, error) {
	if !m.config.Enabled {
		return nil, nil
	}

	// 根据策略选择缓存
	var entry *CacheEntry
	var err error

	switch m.strategy {
	case CacheStrategyExact:
		entry, err = m.exactCache.Get(ctx, key)
		if err != nil {
			logger.Error("Failed to get from exact cache", zap.Error(err))
			m.Miss()
			return nil, nil
		}
		if entry != nil {
			m.Hit()
		} else {
			m.Miss()
		}
		return entry, nil

	case CacheStrategySemantic:
		if m.semanticCache == nil {
			logger.Warn("Semantic cache not configured, fallback to exact cache")
			entry, err = m.exactCache.Get(ctx, key)
			if err != nil {
				logger.Error("Failed to get from exact cache", zap.Error(err))
				m.Miss()
				return nil, nil
			}
			if entry != nil {
				m.Hit()
			} else {
				m.Miss()
			}
			return entry, nil
		}

		// 注意: 语义搜索需要直接调用 SearchByQuery 方法,而不是通过 Get
		// 这里暂时返回未命中,使用者可以调用 SearchByQuery 进行语义搜索
		m.Miss()
		return nil, nil

	case CacheStrategyHybrid:
		// 先尝试精确匹配
		entry, err = m.exactCache.Get(ctx, key)
		if err == nil && entry != nil {
			m.Hit()
			return entry, nil
		}

		// 精确匹配失败,返回未命中
		// 语义搜索需要单独调用 SearchByQuery 方法
		m.Miss()
		return nil, nil

	default:
		entry, err = m.exactCache.Get(ctx, key)
		if err != nil {
			logger.Error("Failed to get from exact cache", zap.Error(err))
			m.Miss()
			return nil, nil
		}
		if entry != nil {
			m.Hit()
		} else {
			m.Miss()
		}
		return entry, nil
	}
}

// Set 设置缓存
func (m *Manager) Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error {
	if !m.config.Enabled {
		return nil
	}

	// 从全局配置读取当前策略，与读取路径保持一致
	currentStrategy := CacheStrategySemantic // 默认仅语义
	if cfg := config.Get(); cfg != nil && cfg.Cache.Strategy != "" {
		currentStrategy = CacheStrategy(cfg.Cache.Strategy)
	}

	// 仅语义策略时跳过精确缓存写入
	if currentStrategy != CacheStrategySemantic {
		if err := m.exactCache.Set(ctx, key, value, ttl); err != nil {
			logger.Error("Failed to set exact cache", zap.Error(err))
			return err
		}
		logger.Debug("Exact cache written", zap.String("key", key), zap.String("strategy", string(currentStrategy)))
	} else {
		logger.Debug("Exact cache write skipped (semantic-only strategy)", zap.String("key", key))
	}

	// 记录设置操作
	m.SetCache()

	// 如果启用了语义缓存,也写入语义缓存(异步执行,避免阻塞响应)
	// 仅保存模式下不写入语义缓存，不进行向量化
	if m.saveOnlyMode {
		logger.Debug("SaveOnly mode: skip semantic cache write and embedding")
		return nil
	}
	// 精确匹配策略：只写精确缓存，不写语义缓存、不走向量化（与读路径 strategy=exact 一致）
	if currentStrategy == CacheStrategyExact {
		logger.Debug("Exact-only cache strategy: skip semantic cache write and embedding", zap.String("key", key))
		return nil
	}
	if m.semanticCache != nil && m.semanticConfig != nil && m.semanticConfig.EnableAutoEmbedding {
		// 检查是否因 QA split 而需要跳过主请求的语义缓存写入。
		// 当 QA split 启用时，主请求语义缓存的写入决策延迟到 QA split 结果确定后再执行：
		//   - 拆分成功（多对）→ 子问题各自写入语义缓存，主请求不再重复写入
		//   - 未拆分（原子问题）→ QA split 回调中补写主请求的语义缓存
		if skip, ok := value.Metadata["skip_semantic_cache"].(bool); ok && skip {
			logger.Info("Semantic cache write deferred to QA split callback", zap.String("key", key))
			return nil
		}

		// 复制必要的数据,避免goroutine中引用问题
		semanticCache := m.semanticCache
		cacheValue := value

		go func() {
			// 使用新的context,避免原context被取消
			semanticCtx := context.Background()
			if err := semanticCache.InsertWithEmbedding(semanticCtx, cacheValue, nil); err != nil {
				// 语义缓存写入失败不影响精确缓存
				logger.Warn("Failed to set semantic cache (async)", zap.Error(err))
			} else {
				logger.Debug("Semantic cache set (async)")
			}
		}()
	}

	return nil
}

// Delete 删除缓存
func (m *Manager) Delete(ctx context.Context, key string) error {
	return m.exactCache.Delete(ctx, key)
}

// Clear 清空缓存
func (m *Manager) Clear(ctx context.Context) error {
	// 清除精确匹配缓存
	if err := m.exactCache.Clear(ctx); err != nil {
		return err
	}

	// 清除语义缓存
	if m.semanticCache != nil {
		if err := m.semanticCache.Clear(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stats 获取统计信息
func (m *Manager) Stats() *CacheStats {
	stats := m.stats.GetStats()

	// 获取缓存的总条目数(包括精确匹配和语义缓存)
	totalEntries := int64(0)

	// 从精确匹配缓存获取条目数
	if m.exactCache != nil {
		entries := m.exactCache.List()
		totalEntries += int64(len(entries))
	}

	// 从语义缓存获取条目数
	if m.semanticCache != nil {
		// 尝试从向量存储获取条目数
		if m.vectorStore != nil {
			collection := m.vectorStore.GetDefaultCollection()
			// 先检查集合是否存在
			exists, err := m.vectorStore.CollectionExists(context.Background(), collection)
			if err == nil && exists {
				_, totalCount, err := m.vectorStore.ListAll(context.Background(), collection, 0, 0)
				if err == nil && totalCount > 0 {
					totalEntries += totalCount
				} else {
					// 如果向量存储统计失败，使用内存中的条目数
					entries := m.semanticCache.List()
					totalEntries += int64(len(entries))
				}
			} else {
				// 集合不存在或检查失败，使用内存中的条目数
				entries := m.semanticCache.List()
				totalEntries += int64(len(entries))
			}
		} else {
			// 没有向量存储，使用内存中的条目数
			entries := m.semanticCache.List()
			totalEntries += int64(len(entries))
		}
	}

	stats.TotalEntries = totalEntries
	return stats
}

// Hit 记录缓存命中
func (m *Manager) Hit() {
	m.stats.RecordHit(string(m.strategy), 0)
}

// RecordHit 记录缓存命中(指定策略)
func (m *Manager) RecordHit(strategy string, latency time.Duration) {
	m.stats.RecordHit(strategy, latency)
}

// Miss 记录缓存未命中
func (m *Manager) Miss() {
	m.stats.RecordMiss(0)
}

// Set 记录缓存设置
func (m *Manager) SetCache() {
	m.stats.RecordSet()
}

// Eviction 记录缓存淘汰
func (m *Manager) Eviction() {
	m.stats.RecordEviction()
}

// SetStrategy 设置缓存策略
func (m *Manager) SetStrategy(strategy CacheStrategy) {
	m.strategy = strategy
	logger.Info("Cache strategy changed", zap.String("strategy", string(strategy)))
}

// GetStrategy 获取当前缓存策略
func (m *Manager) GetStrategy() CacheStrategy {
	return m.strategy
}

// SetSemanticCache 设置语义缓存
func (m *Manager) SetSemanticCache(cache *SemanticCacheImpl) {
	m.semanticCache = cache
	logger.Info("Semantic cache configured")
}

// SetEmbeddingService 设置嵌入服务
func (m *Manager) SetEmbeddingService(svc embedding.EmbeddingService) {
	m.embeddingService = svc
	logger.Info("Embedding service configured")
}

// SetSemanticConfig 设置语义缓存配置
func (m *Manager) SetSemanticConfig(config *SemanticCacheConfig) {
	m.semanticConfig = config
	logger.Info("Semantic cache config updated",
		zap.Float32("threshold", config.Threshold),
		zap.Int("top_k", config.TopK))
}

// SetKVStore 设置KV存储用于持久化
func (m *Manager) SetKVStore(kvStore storage.KVStore) {
	m.kvStore = kvStore
	if m.exactCache != nil {
		m.exactCache.SetKVStore(kvStore)
	}
	// 如果设置了KVStore,记录日志;否则精确匹配只使用内存
	if kvStore != nil {
		logger.Info("KV store configured for cache manager (exact cache persistence enabled)")
	} else {
		logger.Info("No KV store configured, exact cache will use memory only (no persistence)")
	}
}

// GetKVStore 获取KV存储
func (m *Manager) GetKVStore() storage.KVStore {
	return m.kvStore
}

// GetVectorStore 获取向量存储
func (m *Manager) GetVectorStore() storage.VectorStore {
	return m.vectorStore
}

// GetMetrics 获取统计实例（供内部组件共享使用）
func (m *Manager) GetMetrics() *CacheMetrics {
	return m.stats
}

// SetVectorStore 设置向量存储用于持久化
func (m *Manager) SetVectorStore(vectorStore storage.VectorStore) {
	m.vectorStore = vectorStore
	if m.semanticCache != nil {
		m.semanticCache.SetVectorStore(vectorStore)
	}
	logger.Info("Vector store configured for cache manager")
}

// ResetStats 重置统计
func (m *Manager) ResetStats() {
	m.stats.Reset()
	logger.Info("Cache stats reset")
}

// GetExactCache 获取精确匹配缓存
func (m *Manager) GetExactCache() *ExactMatchCache {
	return m.exactCache
}

// GetSemanticCache 获取语义缓存
func (m *Manager) GetSemanticCache() *SemanticCacheImpl {
	return m.semanticCache
}

// SearchByQuery 语义搜索 (公开方法)
func (m *Manager) SearchByQuery(ctx context.Context, query string, threshold float32, topK int) ([]*CacheEntry, error) {
	if m.semanticCache == nil {
		return nil, nil
	}

	return m.semanticCache.SearchByQuery(ctx, query, threshold, topK)
}

// GetSemanticThreshold 返回当前内存中的语义阈值（供 GET 接口与代理使用）
func (m *Manager) GetSemanticThreshold() float32 {
	if m.semanticConfig != nil {
		return m.semanticConfig.Threshold
	}
	return 0.85
}

// UpdateSemanticThreshold 将语义阈值写入内存（不从这里读 kvStore，由 SET 先写存储再调此方法）
func (m *Manager) UpdateSemanticThreshold(threshold float32) error {
	if m.semanticCache == nil {
		return nil // 语义缓存未配置，忽略
	}
	if m.semanticConfig == nil {
		m.semanticConfig = &SemanticCacheConfig{}
	}
	m.semanticConfig.Threshold = threshold
	logger.Info("Semantic threshold updated", zap.Float32("threshold", threshold))
	return nil
}

// Stop 停止缓存管理器
func (m *Manager) Stop() {
	if m.exactCache != nil {
		m.exactCache.Stop()
	}
	if m.semanticCache != nil {
		m.semanticCache.Stop()
	}

	logger.Info("Cache manager stopped")
}

// SetQASplitter 设置问答拆分器
func (m *Manager) SetQASplitter(splitter *processor.QASplitter) {
	m.qaSplitter = splitter
	logger.Info("QA splitter configured",
		zap.Bool("enabled", splitter.IsEnabled()))
}

// GetQASplitter 获取问答拆分器
func (m *Manager) GetQASplitter() *processor.QASplitter {
	return m.qaSplitter
}

// GetSemanticCacheStore 获取语义缓存存储（返回 KVStore 以满足 CacheManager 接口）
func (m *Manager) GetSemanticCacheStore() storage.KVStore {
	return m.kvStore
}

// SetSaveOnlyMode 设置仅保存模式
// 仅保存模式下：不写入主请求的语义缓存，不进行向量化，但允许问答拆分（拆分结果写入语义缓存）
func (m *Manager) SetSaveOnlyMode(enabled bool) {
	m.saveOnlyMode = enabled
	logger.Info("SaveOnly mode changed", zap.Bool("enabled", enabled))
}

// GetSaveOnlyMode 获取仅保存模式状态
func (m *Manager) GetSaveOnlyMode() bool {
	return m.saveOnlyMode
}

// ShouldSplitQA 检查是否需要拆分问答对
// QA Split 的目的是将问答对存入语义缓存，因此只有在语义缓存启用时才有意义
func (m *Manager) ShouldSplitQA() bool {
	// 仅保存模式下不进行问答拆分
	if m.saveOnlyMode {
		return false
	}
	// QA Split 需要语义缓存才能存储拆分结果
	if m.semanticCache == nil {
		return false
	}
	return m.qaSplitter != nil && m.qaSplitter.IsEnabled()
}

// SetEvaluationManager 设置评估流水线管理器
func (m *Manager) SetEvaluationManager(evaluationManager EvaluationPipelineManager) {
	m.evaluationManager = evaluationManager
	logger.Info("Evaluation manager configured in cache manager")
}

// ShouldEvaluateCache 检查是否需要评估缓存
func (m *Manager) ShouldEvaluateCache() bool {
	return m.evaluationManager != nil && m.evaluationManager.IsEnabled()
}

// EvaluateCacheEntry 评估缓存条目是否值得缓存
func (m *Manager) EvaluateCacheEntry(ctx context.Context, question, answer string, historyMessages []plugin.Message) (*EvaluationResult, error) {
	if m.evaluationManager == nil || !m.evaluationManager.IsEnabled() {
		return &EvaluationResult{ShouldCache: true, Score: 100}, nil
	}

	input := &plugin.EvalInput{
		Question:         question,
		OriginalQuestion: question,
		Answer:           answer,
		HistoryMessages:  historyMessages,
		IsExpanded:       false,
	}

	output, err := m.evaluationManager.Execute(ctx, input)
	if err != nil {
		logger.Error("Failed to evaluate cache entry", zap.Error(err))
		return &EvaluationResult{ShouldCache: true, Score: 100}, nil
	}

	return &EvaluationResult{
		ShouldCache: output.Passed,
		Score:       output.Score,
		Labels:      output.Labels,
		Details:     output.Details,
	}, nil
}
