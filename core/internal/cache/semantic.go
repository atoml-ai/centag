package cache

import (
	"context"
	"fmt"
	"centag/core/pkg/embedding"
	"centag/core/pkg/logger"
	"centag/core/pkg/storage"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SemanticCacheEntry 语义缓存条目
type SemanticCacheEntry struct {
	CacheEntry
	Embedding []float32 `json:"embedding"` // 嵌入向量
}

// SemanticMatchResult 语义匹配结果
type SemanticMatchResult struct {
	Entry    *SemanticCacheEntry
	Score    float32 // 相似度分数 (0-1)
	Distance float32 // 距离 (用于调试)
}

// SemanticCacheConfig 语义缓存配置
type SemanticCacheConfig struct {
	CacheConfig
	// Threshold 相似度阈值 (0-1), 超过此值才认为匹配
	Threshold float32 `json:"threshold" yaml:"threshold"`
	// TopK 返回前K个最相似的缓存
	TopK int `json:"top_k" yaml:"top_k"`
	// DistanceType 距离计算类型 (cosine, euclidean)
	DistanceType string `json:"distance_type" yaml:"distance_type"`
	// EnableAutoEmbedding 是否自动向量化新缓存
	EnableAutoEmbedding bool `json:"enable_auto_embedding" yaml:"enable_auto_embedding"`

	// TextDirectHitThreshold 文本直接命中阈值（0-1）。
	// 当存储后端实现了 FullTextSearchStore 接口时生效：
	//   - PostgreSQL：pg_trgm 三元组相似度，天然 0-1，阈值 0.85 较严格
	//   - Elasticsearch：BM25 经 tanh(score/10) 归一化，阈值 0.85 对应原始分约 15
	// 文本相似度达到此阈值则直接返回缓存，跳过 Embedding API 调用。
	// 设为 0 则禁用直接命中逻辑（退化为纯向量搜索）。
	TRGMDirectHitThreshold float32 `json:"trgm_direct_hit_threshold" yaml:"trgm_direct_hit_threshold"`

	// TextPreFilterThreshold 文本预筛候选集阈值（0-1）。
	// 低于此归一化分数的结果在文本预筛阶段被过滤，不进入后续 Embedding + 向量精排。
	// 建议值 0.30，设为 0 则禁用预筛。
	TRGMPreFilterThreshold float32 `json:"trgm_pre_filter_threshold" yaml:"trgm_pre_filter_threshold"`
}

// DefaultSemanticCacheConfig 返回默认配置
func DefaultSemanticCacheConfig() *SemanticCacheConfig {
	return &SemanticCacheConfig{
		CacheConfig: CacheConfig{
			Enabled:         true,
			DefaultTTL:      3600 * time.Second,
			CleanupInterval: 5 * time.Minute,
			MaxSize:         1000,
		},
		Threshold:              0.85,
		TopK:                   5,
		DistanceType:           "cosine",
		EnableAutoEmbedding:    true,
		TRGMDirectHitThreshold: 0.85,
		TRGMPreFilterThreshold: 0.30,
	}
}

// SemanticCacheImpl 语义缓存实现
type SemanticCacheImpl struct {
	entries          map[string]*SemanticCacheEntry
	mu               sync.RWMutex
	config           *SemanticCacheConfig
	stats            *CacheMetrics
	stopChan         chan struct{}
	embeddingService embedding.EmbeddingService
	vectorStore      storage.VectorStore // 向量存储,用于持久化
}

// NewSemanticCache 创建语义缓存
func NewSemanticCache(config *SemanticCacheConfig, embeddingSvc embedding.EmbeddingService) (*SemanticCacheImpl, error) {
	return NewSemanticCacheWithMetrics(config, embeddingSvc, nil)
}

// NewSemanticCacheWithMetrics 创建语义缓存（可指定统计实例）
func NewSemanticCacheWithMetrics(config *SemanticCacheConfig, embeddingSvc embedding.EmbeddingService, metrics *CacheMetrics) (*SemanticCacheImpl, error) {
	if config == nil {
		config = DefaultSemanticCacheConfig()
	}

	if config.DefaultTTL == 0 {
		config.DefaultTTL = 3600 * time.Second
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	if embeddingSvc == nil {
		return nil, fmt.Errorf("embedding service is required")
	}

	// 如果没有提供统计实例，创建一个新的
	if metrics == nil {
		metrics = NewCacheMetrics()
	}

	cache := &SemanticCacheImpl{
		entries:          make(map[string]*SemanticCacheEntry),
		config:           config,
		stats:            metrics,
		stopChan:         make(chan struct{}),
		embeddingService: embeddingSvc,
		vectorStore:      nil, // 需要通过 SetVectorStore 设置
	}

	// 启动清理协程
	if config.Enabled {
		go cache.cleanupLoop()
	}

	// 获取向量模型和存储后端信息
	providerInfo := "未配置"
	if embeddingSvc != nil {
		info := embeddingSvc.GetProviderInfo()
		providerInfo = fmt.Sprintf("%s/%s (维度:%d)", info.Provider, info.Model, info.Dimension)
	}

	vectorStoreInfo := "未配置"
	if cache.vectorStore != nil {
		info := cache.vectorStore.GetStoreInfo()
		vectorStoreInfo = info.Type
	}

	logger.Infof("✓ 语义缓存初始化完成 - 阈值: %.2f, 距离算法: %s, 自动向量化: %t, 向量模型: %s, 存储后端: %s",
		config.Threshold, config.DistanceType, config.EnableAutoEmbedding, providerInfo, vectorStoreInfo)

	return cache, nil
}

// SetVectorStore 设置向量存储用于持久化
func (c *SemanticCacheImpl) SetVectorStore(vectorStore storage.VectorStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vectorStore = vectorStore
	logger.Info("Vector store configured for semantic cache")
}

// SetEmbeddingService 设置嵌入服务
func (c *SemanticCacheImpl) SetEmbeddingService(svc embedding.EmbeddingService) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.embeddingService = svc
	logger.Info("Embedding service configured for semantic cache")
}

// Get 获取缓存 (精确匹配)
func (c *SemanticCacheImpl) Get(ctx context.Context, key string) (*CacheEntry, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		c.recordMiss()
		return nil, nil
	}

	c.recordHit()

	logger.Debug("Semantic cache hit (exact)", zap.String("key", key))

	return &entry.CacheEntry, nil
}

// SearchByQuery 根据查询搜索相似缓存。
//
// 两阶段策略（当存储后端支持 FullTextSearchStore 时自动启用）：
//  1. pg_trgm 文本预筛：无需 Embedding，毫秒级返回候选集。
//     若最高文本相似度 >= TRGMDirectHitThreshold，直接返回缓存，跳过 Embedding 调用。
//  2. Embedding + 向量精排：对文本预筛未能直接命中的情况，走原有向量搜索逻辑。
func (c *SemanticCacheImpl) SearchByQuery(ctx context.Context, query string, threshold float32, topK int) ([]*CacheEntry, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	if threshold <= 0 {
		threshold = c.config.Threshold
	}
	if topK <= 0 {
		topK = c.config.TopK
	}

	normalizedQuery := embedding.NormalizeText(query)

	// =========================================================================
	// 阶段 1：文本预筛（当存储后端实现了 FullTextSearchStore 时自动启用）
	//   PostgreSQL → pg_trgm 三元组相似度（天然 0-1）
	//   Elasticsearch → BM25 经 tanh(score/10) 归一化（0-1）
	// =========================================================================
	if c.vectorStore != nil &&
		c.config.TRGMDirectHitThreshold > 0 &&
		c.config.TRGMPreFilterThreshold > 0 {

		if fts, ok := c.vectorStore.(storage.FullTextSearchStore); ok {
			candidateSize := topK * 4
			if candidateSize < 20 {
				candidateSize = 20
			}

			textResults, err := fts.SearchByText(ctx, normalizedQuery, candidateSize, c.config.TRGMPreFilterThreshold)
			if err != nil {
				// pg_trgm 不可用或索引缺失时，记录警告并降级到纯向量搜索
				logger.Warnf("文本预筛失败，降级到向量搜索: %v", err)
			} else if len(textResults) > 0 {
				bestTextScore := textResults[0].Score
				logger.Infof("→ 文本预筛完成 - 候选数: %d, 最高文本相似度: %.4f (直接命中阈值: %.2f)",
					len(textResults), bestTextScore, c.config.TRGMDirectHitThreshold)

				if bestTextScore >= c.config.TRGMDirectHitThreshold {
					// 文本相似度极高：直接命中，跳过 Embedding API 调用
					logger.Infof("✓ trgm 直接命中 (%.4f >= %.2f)，跳过 Embedding 调用",
						bestTextScore, c.config.TRGMDirectHitThreshold)

					now := time.Now()
					var directEntries []*CacheEntry
					for _, r := range textResults {
						if r.Score < c.config.TRGMDirectHitThreshold {
							break // 结果已按分数降序排列
						}
						// Go 层过期检查（与 Search() 行为一致）
						if expiresAt, ok := r.Metadata["expires_at"].(float64); ok {
							if now.After(time.Unix(int64(expiresAt), 0)) {
								continue
							}
						}
						entry := c.buildCacheEntryFromMetadata(r)
						if entry == nil {
							continue
						}
						if entry.Metadata == nil {
							entry.Metadata = make(map[string]interface{})
						}
						entry.Metadata["similarity_score"] = r.Score
						entry.Metadata["similarity_distance"] = 1.0 - r.Score
						entry.Metadata["match_method"] = "trgm_direct"
						directEntries = append(directEntries, entry)
					}

					if len(directEntries) > 0 {
						c.recordSemanticHit(len(directEntries))
						logger.Infof("✓ trgm 直接命中返回 %d 条缓存（已跳过 Embedding）", len(directEntries))
						return directEntries, nil
					}
					// 候选全部过期，继续走向量搜索兜底
					logger.Infof("  trgm 直接命中候选均已过期，降级到向量搜索")
				}
			}
		}
	}

	// =========================================================================
	// 阶段 2：Embedding + 向量精排（原有逻辑，无改动）
	// =========================================================================

	providerInfo := c.embeddingService.GetProviderInfo()
	logger.Infof("→ 获取查询向量 - 查询: %s, 模型: %s/%s, 维度: %d",
		normalizedQuery, providerInfo.Provider, providerInfo.Model, providerInfo.Dimension)

	queryEmbedding, err := c.embeddingService.GetEmbedding(ctx, normalizedQuery)
	if err != nil {
		logger.Error("Failed to get query embedding",
			zap.String("query", query),
			zap.String("normalized_query", normalizedQuery),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}

	logger.Infof("✓ 查询向量已生成 - 维度: %d, 来源: %s/%s",
		len(queryEmbedding), providerInfo.Provider, providerInfo.Model)

	results := make([]*SemanticMatchResult, 0)

	// 从向量存储搜索
	if c.vectorStore != nil {
		storeInfo := c.vectorStore.GetStoreInfo()
		logger.Infof("→ 向量存储搜索 - 后端: %s, TopK: %d, 阈值: %.2f",
			storeInfo.Type, topK, threshold)

		searchResults, err := c.vectorStore.Search(ctx, queryEmbedding, topK, nil)
		if err != nil {
			logger.Errorf("✗ 向量存储搜索失败: %v", err)
		} else {
			logger.Infof("✓ 向量存储搜索完成 - 找到 %d 个结果", len(searchResults))
			now := time.Now()
			for _, result := range searchResults {
				// 检查过期时间(从metadata中获取)
				if expiresAt, ok := result.Metadata["expires_at"].(float64); ok {
					if now.After(time.Unix(int64(expiresAt), 0)) {
						continue
					}
				}

				// ElasticSearch KNN 返回的 Score 已经是相似度（0-1），不是距离
				// 不需要 1.0 - result.Score 转换
				score := result.Score

				// 记录所有搜索结果的相似度（用于调试）
				logger.Debug("Semantic search result",
					zap.Float32("similarity", score),
					zap.Float32("threshold", threshold),
					zap.Bool("above_threshold", score >= threshold))

				// 保留所有结果，包括低于阈值的（用于前端显示"最接近的问题"）
				// 从结果中重建 CacheEntry
				entry := c.buildCacheEntryFromMetadata(result)
				if entry != nil {
					results = append(results, &SemanticMatchResult{
						Entry: &SemanticCacheEntry{
							CacheEntry: *entry,
						},
						Score:    score,
						Distance: 1.0 - score, // 距离 = 1 - 相似度
					})

					// 只有超过阈值的才加载到内存
					if score >= threshold {
						c.mu.Lock()
						if _, exists := c.entries[entry.Key]; !exists {
							c.entries[entry.Key] = &SemanticCacheEntry{
								CacheEntry: *entry,
								Embedding:  result.Vector,
							}
						}
						c.mu.Unlock()
					}
				}
			}
		}
	}

	// 计算内存中缓存条目的相似度
	c.mu.RLock()
	now := time.Now()
	memoryMatchCount := 0
	for _, entry := range c.entries {
		// 跳过过期条目
		if now.After(entry.ExpiresAt) {
			continue
		}

		// 跳过没有嵌入向量的条目
		if entry.Embedding == nil {
			continue
		}

		// 计算相似度
		score, err := c.calculateSimilarity(queryEmbedding, entry.Embedding)
		if err != nil {
			logger.Warn("Failed to calculate similarity",
				zap.String("key", entry.Key),
				zap.Error(err))
			continue
		}

		if score >= threshold {
			memoryMatchCount++
		}

		// 保留所有结果（包括低于阈值的），用于前端显示
		// 检查是否已在结果中(避免重复)
		duplicate := false
		for _, r := range results {
			if r.Entry.Key == entry.Key {
				duplicate = true
				break
			}
		}
		if !duplicate {
			results = append(results, &SemanticMatchResult{
				Entry:    entry,
				Score:    score,
				Distance: 1.0 - score, // 转换为距离
			})
		}
	}
	c.mu.RUnlock()

	// 按相似度排序(降序)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 返回前K个结果
	if len(results) > topK {
		results = results[:topK]
	}

	// 统计：只有超过阈值的才算命中
	hitsCount := 0
	for _, r := range results {
		if r.Score >= threshold {
			hitsCount++
		}
	}

	if hitsCount > 0 {
		c.recordSemanticHit(hitsCount)
	}

	logger.Infof("语义比对完成 - 找到 %d 个结果, 超过阈值 %.2f 的: %d 个, 最佳相似度: %.4f",
		len(results), threshold, hitsCount, func() float32 {
			if len(results) > 0 {
				return results[0].Score
			}
			return 0
		}())

	// 转换为 CacheEntry 数组
	entries := make([]*CacheEntry, len(results))
	for i, result := range results {
		entries[i] = &result.Entry.CacheEntry
		// 在metadata中记录相似度分数
		if entries[i].Metadata == nil {
			entries[i].Metadata = make(map[string]interface{})
		}
		entries[i].Metadata["similarity_score"] = result.Score
		entries[i].Metadata["similarity_distance"] = result.Distance
	}

	// 添加详细日志:显示所有搜索结果的相似度
	logger.Infof("语义搜索完成 - 查询: %s, 找到 %d 个结果, 阈值: %.2f",
		query, len(results), threshold)
	if len(results) > 0 {
		for i, result := range results {
			status := "✗ 未命中"
			if result.Score >= threshold {
				status = "✓ 命中"
			}
			logger.Infof("  [%d] %s - key: %s, 相似度: %.4f/%.2f, 查询: %s",
				i+1, status, result.Entry.Key, result.Score, threshold, result.Entry.Request)
		}
	} else {
		logger.Infof("  没有找到任何缓存条目")
	}

	return entries, nil
}

// buildCacheEntryFromMetadata 从向量存储的元数据构建 CacheEntry
func (c *SemanticCacheImpl) buildCacheEntryFromMetadata(result storage.SearchResult) *CacheEntry {
	entry := &CacheEntry{
		Key:      result.ID,
		Metadata: make(map[string]interface{}),
	}

	// 从metadata中提取字段
	if request, ok := result.Metadata["request"].(string); ok {
		entry.Request = request
	}
	if response, ok := result.Metadata["response"].(string); ok {
		entry.Response = response
	}
	if timestamp, ok := result.Metadata["timestamp"].(float64); ok {
		entry.Timestamp = time.Unix(int64(timestamp), 0)
	}
	if expiresAt, ok := result.Metadata["expires_at"].(float64); ok {
		entry.ExpiresAt = time.Unix(int64(expiresAt), 0)
	}

	// 复制其他元数据
	for k, v := range result.Metadata {
		if k != "request" && k != "response" && k != "timestamp" && k != "expires_at" {
			entry.Metadata[k] = v
		}
	}

	return entry
}

// InsertWithEmbedding 使用嵌入向量插入缓存
func (c *SemanticCacheImpl) InsertWithEmbedding(ctx context.Context, entry *CacheEntry, vector []float32) error {
	if !c.config.Enabled {
		return nil
	}

	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// 如果没有提供嵌入向量,自动生成
	if vector == nil && c.config.EnableAutoEmbedding {
		var err error
		normalizedRequest := embedding.NormalizeText(entry.Request)

		providerInfo := c.embeddingService.GetProviderInfo()
		logger.Infof("→ 获取向量中 - 模型: %s/%s, API: %s, 维度: %d",
			providerInfo.Provider, providerInfo.Model, providerInfo.BaseURL, providerInfo.Dimension)

		vector, err = c.embeddingService.GetEmbedding(ctx, normalizedRequest)
		if err != nil {
			logger.Error("Failed to auto-generate embedding",
				zap.String("request", entry.Request),
				zap.String("normalized_request", normalizedRequest),
				zap.Error(err))
			return fmt.Errorf("failed to auto-generate embedding: %w", err)
		}

		logger.Infof("✓ 向量已生成 - key: %s, 查询: %s, 维度: %d, 来源: %s/%s",
			entry.Key, entry.Request, len(vector), providerInfo.Provider, providerInfo.Model)
	} else if vector != nil {
		providerInfo := c.embeddingService.GetProviderInfo()
		logger.Infof("✓ 向量已设置 - key: %s, 查询: %s, 维度: %d, 来源: %s/%s",
			entry.Key, entry.Request, len(vector), providerInfo.Provider, providerInfo.Model)
	}

	// 设置过期时间
	if entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = time.Now().Add(c.config.DefaultTTL)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否超过最大缓存数
	if c.config.MaxSize > 0 && int64(len(c.entries)) >= c.config.MaxSize {
		c.evictOldest()
	}

	// 创建语义缓存条目
	semEntry := &SemanticCacheEntry{
		CacheEntry: *entry,
		Embedding:  vector,
	}

	semEntry.Timestamp = time.Now()
	c.entries[entry.Key] = semEntry

	// 异步持久化到向量存储（避免阻塞）
	if c.vectorStore != nil && vector != nil {
		// 复制数据避免goroutine引用问题
		persistKey := entry.Key
		persistVector := make([]float32, len(vector))
		copy(persistVector, vector)
		persistMetadata := c.buildMetadata(entry)

		go func() {
			persistCtx := context.Background()
			vectors := []storage.Vector{
				{
					ID:       persistKey,
					Vector:   persistVector,
					Metadata: persistMetadata,
				},
			}
			if err := c.vectorStore.Insert(persistCtx, vectors); err != nil {
				logger.Warn("Failed to persist vector to storage (async)",
					zap.String("key", persistKey),
					zap.Error(err))
			} else {
				logger.Infof("✓ 向量已持久化到存储 - key: %s, 维度: %d", persistKey, len(persistVector))
			}
		}()
	}

	logger.Debug("Semantic cache inserted", zap.String("key", entry.Key))

	return nil
}

// buildMetadata 构建向量存储的元数据
func (c *SemanticCacheImpl) buildMetadata(entry *CacheEntry) map[string]interface{} {
	metadata := make(map[string]interface{})

	// 复制原始元数据
	for k, v := range entry.Metadata {
		metadata[k] = v
	}

	// 添加缓存相关的元数据
	metadata["request"] = entry.Request
	metadata["response"] = entry.Response
	metadata["timestamp"] = entry.Timestamp.Unix()
	metadata["expires_at"] = entry.ExpiresAt.Unix()

	return metadata
}

// Set 设置缓存 (精确匹配,不自动向量化)
func (c *SemanticCacheImpl) Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error {
	if !c.config.Enabled {
		return nil
	}

	value.Key = key
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}
	value.ExpiresAt = time.Now().Add(ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config.MaxSize > 0 && int64(len(c.entries)) >= c.config.MaxSize {
		c.evictOldest()
	}

	value.Timestamp = time.Now()
	c.entries[key] = &SemanticCacheEntry{
		CacheEntry: *value,
	}

	logger.Debug("Semantic cache set (no embedding)", zap.String("key", key))

	return nil
}

// Delete 删除缓存
func (c *SemanticCacheImpl) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)

	// 从向量存储删除
	if c.vectorStore != nil {
		if err := c.vectorStore.Delete(ctx, []string{key}); err != nil {
			logger.Warn("Failed to delete vector from storage",
				zap.String("key", key),
				zap.Error(err))
		}
	}

	logger.Debug("Semantic cache deleted", zap.String("key", key))

	return nil
}

// Exists 检查缓存是否存在
func (c *SemanticCacheImpl) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear 清空所有缓存
func (c *SemanticCacheImpl) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 收集所有key
	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}

	c.entries = make(map[string]*SemanticCacheEntry)
	c.resetStats()

	// 从向量存储删除
	if c.vectorStore != nil && len(keys) > 0 {
		if err := c.vectorStore.Delete(ctx, keys); err != nil {
			logger.Warn("Failed to clear vector storage", zap.Error(err))
		}
	}

	logger.Info("Semantic cache cleared")

	return nil
}

// Stats 获取缓存统计
func (c *SemanticCacheImpl) Stats(ctx context.Context) (*CacheStats, error) {
	stats := c.stats.GetStats()

	c.mu.RLock()
	stats.TotalEntries = int64(len(c.entries))
	c.mu.RUnlock()

	return stats, nil
}

// calculateSimilarity 计算相似度
func (c *SemanticCacheImpl) calculateSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimensions do not match: %d vs %d", len(a), len(b))
	}

	switch c.config.DistanceType {
	case "cosine":
		return c.cosineSimilarity(a, b), nil
	case "euclidean":
		return c.euclideanSimilarity(a, b), nil
	default:
		return c.cosineSimilarity(a, b), nil
	}
}

// cosineSimilarity 余弦相似度
func (c *SemanticCacheImpl) cosineSimilarity(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// euclideanSimilarity 欧氏距离相似度
func (c *SemanticCacheImpl) euclideanSimilarity(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	distance := float32(math.Sqrt(float64(sum)))
	// 转换为相似度 (距离越小,相似度越高)
	maxDistance := float32(math.Sqrt(float64(len(a)))) // 最大可能距离
	return 1.0 - (distance / maxDistance)
}

// evictOldest 淘汰最早的缓存条目
func (c *SemanticCacheImpl) evictOldest() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.stats.RecordEviction()
	}
}

// cleanupLoop 定期清理过期缓存
func (c *SemanticCacheImpl) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

// cleanup 清理过期缓存
func (c *SemanticCacheImpl) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evictedCount := 0

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			evictedCount++
		}
	}

	if evictedCount > 0 {
		for i := 0; i < evictedCount; i++ {
			c.stats.RecordEviction()
		}

		logger.Debug("Semantic cache cleanup",
			zap.Int("evicted", evictedCount),
			zap.Int("remaining", len(c.entries)))
	}
}

// recordHit 记录命中
func (c *SemanticCacheImpl) recordHit() {
	c.stats.RecordHit(string(CacheStrategyExact), 0)
}

// recordSemanticHit 记录语义命中
func (c *SemanticCacheImpl) recordSemanticHit(count int) {
	// 只记录一次命中，不管返回多少个结果
	c.stats.RecordHit(string(CacheStrategySemantic), 0)
}

// recordMiss 记录未命中
func (c *SemanticCacheImpl) recordMiss() {
	c.stats.RecordMiss(0)
}

// resetStats 重置统计
func (c *SemanticCacheImpl) resetStats() {
	c.stats.Reset()
}

// Stop 停止缓存
func (c *SemanticCacheImpl) Stop() {
	close(c.stopChan)
	logger.Info("Semantic cache stopped")
}

// List 列出所有缓存条目（包括内存和向量存储）
func (c *SemanticCacheImpl) List() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(c.entries))

	// 添加内存中的缓存条目
	for key, entry := range c.entries {
		responsePreview := ""
		if len(entry.Response) > 200 {
			responsePreview = entry.Response[:200]
		} else {
			responsePreview = entry.Response
		}

		// 提取模型和问题字段
		model, question := extractCacheEntryFields(key, &entry.CacheEntry)

		// 确定存储位置
		storageLocation := "memory"
		if c.vectorStore != nil {
			storeInfo := c.vectorStore.GetStoreInfo()
			storageLocation = storeInfo.Type
		}

		result = append(result, map[string]interface{}{
			"key":        key,
			"timestamp":  entry.Timestamp,
			"created_at": entry.Timestamp,
			"expires_at": entry.ExpiresAt,
			"request":    entry.Request,
			"response":   responsePreview,
			"metadata":   entry.Metadata,
			"model":      model,
			"question":   question,
			"similarity": nil,
			"storage":    storageLocation, // 存储位置: memory, elasticsearch, redis, chromadb
		})
	}

	// 如果有向量存储,尝试从中加载额外的条目
	// 注意: 为了避免重复,这里可以只添加不在内存中的条目
	// 但为了简单,我们先返回内存中的条目
	return result
}

// ListFromVectorStore 从向量存储中列出所有语义缓存条目
func (c *SemanticCacheImpl) ListFromVectorStore(ctx context.Context, limit int, offset int) ([]map[string]interface{}, error) {
	if c.vectorStore == nil {
		return []map[string]interface{}{}, nil
	}

	// 使用 VectorStore.ListAll 获取所有向量存储中的条目
	collection := c.vectorStore.GetDefaultCollection()
	docs, _, err := c.vectorStore.ListAll(ctx, collection, limit, offset)
	if err != nil {
		logger.LogError("failed to list vectors from storage", err)
		return nil, err
	}

	// 插件已将后端特有结构归一化为 VectorEntry，此处直接使用标准字段。
	storageLocation := c.vectorStore.GetStoreInfo().Type
	result := make([]map[string]interface{}, 0, len(docs))
	for _, entry := range docs {
		meta := entry.Metadata
		if meta == nil {
			meta = map[string]interface{}{}
		}

		// 从统一的 Metadata 中读取业务字段
		requestStr, _ := meta["request"].(string)
		respStr, _ := meta["response"].(string)

		// 解析 expires_at（存储为 Unix 时间戳 float64）
		var expiresAt time.Time
		switch v := meta["expires_at"].(type) {
		case float64:
			if v > 0 {
				expiresAt = time.Unix(int64(v), 0)
			}
		case int64:
			if v > 0 {
				expiresAt = time.Unix(v, 0)
			}
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				expiresAt = parsed
			}
		}

		// 提取模型和问题
		modelStr, detectedQuestion := extractCacheEntryFields("", &CacheEntry{
			Request:  requestStr,
			Metadata: meta,
		})

		// 响应预览（最多 200 字节）
		responsePreview := respStr
		if len(responsePreview) > 200 {
			responsePreview = responsePreview[:200]
		}

		result = append(result, map[string]interface{}{
			"key":        entry.ID,
			"timestamp":  entry.CreatedAt,
			"created_at": entry.CreatedAt,
			"expires_at": expiresAt,
			"request":    requestStr,
			"response":   responsePreview,
			"metadata":   meta,
			"model":      modelStr,
			"question":   detectedQuestion,
			"similarity": nil,
			"storage":    storageLocation,
		})
	}

	return result, nil
}
