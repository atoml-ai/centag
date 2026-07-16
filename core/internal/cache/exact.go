package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"go.uber.org/zap"
)

// cacheKeyNS 缓存条目的 KV 存储命名空间前缀，避免与 pipeline 存储钩子等其他数据混在一起。
const cacheKeyNS = "cache:"

// ExactMatchCache 精确匹配缓存实现
type ExactMatchCache struct {
	entries  map[string]*CacheEntry
	mu       sync.RWMutex
	config   *CacheConfig
	stats    *CacheMetrics
	stopChan chan struct{}
	kvStore  storage.KVStore // KV存储,用于持久化
}

// NewExactMatchCache 创建精确匹配缓存
func NewExactMatchCache(config *CacheConfig) (*ExactMatchCache, error) {
	return NewExactMatchCacheWithMetrics(config, nil)
}

// NewExactMatchCacheWithMetrics 创建精确匹配缓存（可指定统计实例）
func NewExactMatchCacheWithMetrics(config *CacheConfig, metrics *CacheMetrics) (*ExactMatchCache, error) {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 3600 * time.Second // 默认1小时
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute // 默认5分钟清理一次
	}

	// 如果没有提供统计实例，创建一个新的
	if metrics == nil {
		metrics = NewCacheMetrics()
	}

	cache := &ExactMatchCache{
		entries:  make(map[string]*CacheEntry),
		config:   config,
		stats:    metrics,
		stopChan: make(chan struct{}),
		kvStore:  nil, // 需要通过 SetKVStore 设置
	}

	// 启动清理协程
	if config.Enabled {
		go cache.cleanupLoop()
	}

	logger.Info("Exact match cache initialized",
		zap.Bool("enabled", config.Enabled),
		zap.Duration("default_ttl", config.DefaultTTL),
		zap.Duration("cleanup_interval", config.CleanupInterval))

	return cache, nil
}

// SetKVStore 设置KV存储用于持久化
func (c *ExactMatchCache) SetKVStore(kvStore storage.KVStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kvStore = kvStore
	logger.Info("KV store configured for exact cache")
}

// Get 获取缓存
func (c *ExactMatchCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	// 如果内存中不存在,尝试从存储加载
	if !exists && c.kvStore != nil {
		// 优先使用 GetBytes 获取原始字节数据
		nsKey := cacheKeyNS + key
		data, err := c.kvStore.GetBytes(ctx, nsKey)
		if err == nil && data != nil {
			loadedEntry, unmarshalErr := UnmarshalCacheEntry(data)
			if unmarshalErr == nil {
				// 检查是否过期
				if time.Now().Before(loadedEntry.ExpiresAt) {
					// 加载到内存
					c.mu.Lock()
					c.entries[key] = loadedEntry
					c.mu.Unlock()
					entry = loadedEntry
					exists = true
					logger.Debug("Cache loaded from storage", zap.String("key", key))
				} else {
					// 存储中已过期,删除
					c.kvStore.Delete(ctx, nsKey)
					logger.Debug("Cache entry expired in storage", zap.String("key", key))
				}
			} else {
				// 缓存条目数据损坏（不太可能，因为用命名空间前缀隔离了）
				logger.Debug("Failed to unmarshal cache entry from storage",
					zap.String("key", key),
					zap.Error(unmarshalErr))
			}
		}
	}

	if !exists {
		c.recordMiss()
		return nil, nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		// 同步删除存储中的数据
		if c.kvStore != nil {
			c.kvStore.Delete(ctx, cacheKeyNS+key)
		}
		c.recordMiss()
		return nil, nil
	}

	c.recordHit()
	c.updateLastAccess()

	logger.Debug("Cache hit", zap.String("key", key))

	return entry, nil
}

// Set 设置缓存。ttl=0 表示永久保存（不过期），ttl>0 表示相对过期时间。
func (c *ExactMatchCache) Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error {
	if !c.config.Enabled {
		return nil
	}

	// ttl=0 → 永久保存：设置一个 100 年后才过期的时间戳，KV store 侧 expires_at=NULL（永久）
	// ttl>0 → 正常相对过期时间
	if ttl == 0 {
		value.ExpiresAt = time.Now().Add(100 * 365 * 24 * time.Hour)
	} else {
		value.ExpiresAt = time.Now().Add(ttl)
	}

	// 检查是否超过最大缓存数
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config.MaxSize > 0 && int64(len(c.entries)) >= c.config.MaxSize {
		// 简单的LRU策略:删除最早的一个
		c.evictOldest()
	}

	value.Timestamp = time.Now()
	c.entries[key] = value

	// 异步持久化到存储（避免阻塞响应）
	if c.kvStore != nil {
		// 复制必要的数据，避免goroutine中引用问题
		persistKey := key
		persistValue := value
		persistTTL := ttl

		go func() {
			// 使用新的context，避免原context被取消
			persistCtx := context.Background()
			if err := c.kvStore.Set(persistCtx, cacheKeyNS+persistKey, persistValue, persistTTL); err != nil {
				logger.Warn("Failed to persist cache to storage (async)",
					zap.String("key", persistKey),
					zap.Error(err))
			} else {
				logger.Debug("Cache persisted (async)", zap.String("key", persistKey))
			}
		}()
	}

	logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))

	return nil
}

// Delete 删除缓存
func (c *ExactMatchCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)

	// 异步从存储中删除（避免阻塞）
	if c.kvStore != nil {
		deleteKey := key
		go func() {
			persistCtx := context.Background()
			if err := c.kvStore.Delete(persistCtx, cacheKeyNS+deleteKey); err != nil {
				logger.Warn("Failed to delete cache from storage (async)",
					zap.String("key", deleteKey),
					zap.Error(err))
			}
		}()
	}

	logger.Debug("Cache delete", zap.String("key", key))

	return nil
}

// Exists 检查缓存是否存在
func (c *ExactMatchCache) Exists(ctx context.Context, key string) (bool, error) {
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
func (c *ExactMatchCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.resetStats()

	// 清空存储
	if c.kvStore != nil {
		if err := c.kvStore.FlushDB(ctx); err != nil {
			logger.Warn("Failed to clear storage", zap.Error(err))
		}
	}

	logger.Info("Cache cleared")

	return nil
}

// Stats 获取缓存统计
func (c *ExactMatchCache) Stats(ctx context.Context) (*CacheStats, error) {
	stats := c.stats.GetStats()

	c.mu.RLock()
	// TotalEntries 字段需要额外添加
	stats.TotalEntries = int64(len(c.entries))
	c.mu.RUnlock()

	return stats, nil
}

// cleanupLoop 定期清理过期缓存
func (c *ExactMatchCache) cleanupLoop() {
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
func (c *ExactMatchCache) cleanup() {
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

		logger.Debug("Cache cleanup",
			zap.Int("evicted", evictedCount),
			zap.Int("remaining", len(c.entries)))
	}
}

// evictOldest 淘汰最早的缓存条目
func (c *ExactMatchCache) evictOldest() {
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

// recordHit 记录命中
func (c *ExactMatchCache) recordHit() {
	c.stats.RecordHit(string(CacheStrategyExact), 0)
}

// recordMiss 记录未命中
func (c *ExactMatchCache) recordMiss() {
	c.stats.RecordMiss(0)
}

// updateLastAccess 更新最后访问时间
func (c *ExactMatchCache) updateLastAccess() {
	// 新的统计系统会自动记录时间
}

// resetStats 重置统计
func (c *ExactMatchCache) resetStats() {
	c.stats.Reset()
}

// Stop 停止缓存
func (c *ExactMatchCache) Stop() {
	close(c.stopChan)
	logger.Info("Exact match cache stopped")
}

// List 列出所有缓存条目
func (c *ExactMatchCache) List() []map[string]interface{} {
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

		// 提取额外字段供前端展示
		model, question := extractCacheEntryFields(key, entry)

		// 确定存储位置
		storageLocation := "memory"
		if c.kvStore != nil {
			storeInfo := c.kvStore.GetStoreInfo()
			storageLocation = storeInfo.Type
		}

		result = append(result, map[string]interface{}{
			"key":        key,
			"timestamp":  entry.Timestamp,
			"created_at": entry.Timestamp, // 添加前端期望的字段名
			"expires_at": entry.ExpiresAt,
			"request":    entry.Request,
			"response":   responsePreview, // 只返回前200字符
			"metadata":   entry.Metadata,
			"model":      model,           // 提取的模型字段
			"question":   question,        // 提取的问题字段
			"similarity": nil,             // 精确匹配无相似度
			"storage":    storageLocation, // 存储位置: memory, redis, elasticsearch 等
		})
	}

	// 如果有KV存储,也从持久化存储读取所有缓存条目
	// 注意: 这可能返回很多条目,在生产环境中应该添加分页支持
	if c.kvStore != nil {
		ctx := context.Background()

		// 尝试使用GetAll方法获取所有缓存条目(包括已过期的)
		// 类型断言检查是否支持GetAll
		type getAller interface {
			GetAll(context.Context, string) (map[string][]byte, error)
		}

		if getAll, ok := c.kvStore.(getAller); ok {
			// 使用GetAll方法获取所有值(包括已过期的)
			values, err := getAll.GetAll(ctx, "cache:*")
			if err != nil {
				logger.Warn("List cache using GetAll failed", zap.Error(err))
			}
			if err == nil && len(values) > 0 {
				for nsKey, data := range values {
					// 去掉缓存命名空间前缀，得到原始 key
					key := strings.TrimPrefix(nsKey, cacheKeyNS)
					// 检查是否已经在内存结果中
					alreadyInResult := false
					for _, item := range result {
						if itemKey, ok := item["key"].(string); ok && itemKey == key {
							alreadyInResult = true
							break
						}
					}

					if !alreadyInResult {
						// 反序列化缓存条目
						entry, unmarshalErr := UnmarshalCacheEntry(data)
						if unmarshalErr == nil {
							responsePreview := string(data)
							if len(responsePreview) > 200 {
								responsePreview = responsePreview[:200]
							}

							// 提取额外字段供前端展示
							model, question := extractCacheEntryFields(key, entry)

							// 确定存储位置(从KV存储加载的)
							storageLocation := c.kvStore.GetStoreInfo().Type

							result = append(result, map[string]interface{}{
								"key":        key,
								"timestamp":  entry.Timestamp,
								"created_at": entry.Timestamp, // 添加前端期望的字段名
								"expires_at": entry.ExpiresAt,
								"request":    entry.Request,
								"response":   responsePreview, // 只返回前200字符
								"metadata":   entry.Metadata,
								"model":      model,                             // 提取的模型字段
								"question":   question,                          // 提取的问题字段
								"similarity": nil,                               // 精确匹配无相似度
								"expired":    time.Now().After(entry.ExpiresAt), // 标记是否过期
								"storage":    storageLocation,                   // 存储位置: redis, elasticsearch 等
							})
				} else {
					// 共享 KV 存储中可能包含非缓存数据（如 pipeline 存储钩子），静默跳过
					logger.Debug("Skipping non-cache entry in KV store",
						zap.String("key", key),
						zap.Error(unmarshalErr))
				}
				}
			}
		} else {
			// 回退到旧的Keys+Get方法
			keys, err := c.kvStore.Keys(ctx, "cache:*")
			if err != nil {
				logger.Warn("List cache from KV storage (fallback) failed", zap.Error(err))
			}
			if err == nil && len(keys) > 0 {
				for _, nsKey := range keys {
					// 去掉缓存命名空间前缀，得到原始 key
					key := strings.TrimPrefix(nsKey, cacheKeyNS)
					// 检查是否已经在内存结果中
					alreadyInResult := false
					for _, item := range result {
						if itemKey, ok := item["key"].(string); ok && itemKey == key {
							alreadyInResult = true
							break
						}
					}

					if !alreadyInResult {
						// 从KV存储获取缓存条目（使用带命名空间前缀的键）
						val, err := c.kvStore.Get(ctx, nsKey)
						if err == nil && val != nil {
							// Get方法返回的是[]byte,包含CacheEntry的JSON字符串
							data, ok := val.([]byte)
							if !ok {
								logger.Warn("Unexpected value type from KV store",
									zap.String("key", key),
									zap.String("type", fmt.Sprintf("%T", val)))
								continue
							}

							// 反序列化缓存条目
							entry, unmarshalErr := UnmarshalCacheEntry(data)
							if unmarshalErr == nil {
								responsePreview := string(data)
								if len(responsePreview) > 200 {
									responsePreview = responsePreview[:200]
								}

								// 提取额外字段供前端展示
								model, question := extractCacheEntryFields(key, entry)

								// 确定存储位置(从KV存储加载的)
								storageLocation := c.kvStore.GetStoreInfo().Type

								result = append(result, map[string]interface{}{
									"key":        key,
									"timestamp":  entry.Timestamp,
									"created_at": entry.Timestamp, // 添加前端期望的字段名
									"expires_at": entry.ExpiresAt,
									"request":    entry.Request,
									"response":   responsePreview, // 只返回前200字符
									"metadata":   entry.Metadata,
									"model":      model,                             // 提取的模型字段
									"question":   question,                          // 提取的问题字段
									"similarity": nil,                               // 精确匹配无相似度
									"expired":    time.Now().After(entry.ExpiresAt), // 标记是否过期
									"storage":    storageLocation,                   // 存储位置: redis, elasticsearch 等
								})
							}
						} else {
							// 忽略已过期的缓存，只有真正错误时记录。
							if err != nil {
								logger.Debug("Failed to get entry from KV store",
									zap.String("key", key),
									zap.Error(err))
							}
						}
					}
				}
			}
		}
	}
	}

	return result
}

// extractCacheEntryFields 从缓存条目中提取model和question字段
// 用于前端展示，增强UI可读性
func extractCacheEntryFields(key string, entry *CacheEntry) (model string, question string) {
	// 从metadata中提取model
	if entry.Metadata != nil {
		if m, ok := entry.Metadata["model"].(string); ok {
			model = m
		}
	}

	// 提取问题文本
	isSplitQA := false
	if entry.Metadata != nil {
		if split, ok := entry.Metadata["is_split_qa"].(bool); ok {
			isSplitQA = split
		}
	}

	// 1. 如果是拆分的QA，key本身就是问题
	if isSplitQA {
		question = key
	} else if entry.Request != "" {
		// 2. 否则尝试从request JSON中解析出问题（取最后一条用户消息）
		var req map[string]interface{}
		if err := json.Unmarshal([]byte(entry.Request), &req); err == nil {
			if messages, ok := req["messages"].([]interface{}); ok && len(messages) > 0 {
				// 遍历messages，找到最后一条用户消息
				for i := len(messages) - 1; i >= 0; i-- {
					if msg, ok := messages[i].(map[string]interface{}); ok {
						if role, ok := msg["role"].(string); ok && role == "user" {
							if content, ok := msg["content"].(string); ok {
								question = content
								break
							}
						}
					}
				}
			}
		} else {
			// 3. 如果不是JSON，尝试从metadata中获取request_text
			if entry.Metadata != nil {
				if reqText, ok := entry.Metadata["request_text"].(string); ok {
					question = reqText
				}
			}
			// 4. 如果还是没有，直接使用request作为问题（可能是纯文本格式）
			if question == "" {
				question = entry.Request
			}
		}
	}

	return model, question
}

// GenerateKey 生成缓存键
func GenerateKey(request interface{}) (string, error) {
	// 序列化请求
	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 记录调试日志
	logger.Info("Generating cache key",
		zap.String("request_json", string(data)),
		zap.String("data_length", fmt.Sprintf("%d", len(data))))

	// 计算SHA256哈希
	hash := sha256.Sum256(data)
	key := fmt.Sprintf("%x", hash)[:16]

	logger.Info("Generated cache key", zap.String("key", key))

	// 返回十六进制字符串
	return key, nil
}

// GenerateKeyFromPrompt 从prompt生成缓存键
func GenerateKeyFromPrompt(prompt string, model string) string {
	key := fmt.Sprintf("%s:%s", model, prompt)
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)[:16]
}
