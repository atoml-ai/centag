package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"go.uber.org/zap"
)

// KVStore KV 存储实现（基于 Elasticsearch）
type KVStore struct {
	client *Client
	index  string
}

// NewKVStore 创建 KV 存储
func NewKVStore(client *Client, index string) *KVStore {
	return &KVStore{
		client: client,
		index:  index,
	}
}

// Set 设置键值对
func (k *KVStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 根据value类型处理
	var responseStr string
	switch v := value.(type) {
	case []byte:
		// 直接字节数组
		responseStr = string(v)
	case string:
		// 字符串
		responseStr = v
	default:
		// 其他类型（如*CacheEntry），序列化为JSON
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		responseStr = string(jsonBytes)
	}
	
	// 构建文档
	now := time.Now()
	doc := map[string]interface{}{
		"key":           key,
		"response":      responseStr,
		"timestamp":     now.Format(time.RFC3339),
		"last_accessed": now.Format(time.RFC3339),
		"access_count":  0,
	}

	// 设置 TTL
	if ttl > 0 {
		doc["ttl"] = now.Add(ttl).Format(time.RFC3339)
	}

	// 索引文档
	if err := k.client.IndexDocument(ctx, k.index, key, doc, "true"); err != nil {
		logger.LogError("failed to set key in elasticsearch", err,
			logger.GetField("key", key),
			logger.GetField("index", k.index))
		return err
	}

	logger.Debug("Key set successfully",
		logger.GetField("key", key),
		logger.GetField("ttl", ttl))

	return nil
}

// Get 获取值
func (k *KVStore) Get(ctx context.Context, key string) (interface{}, error) {
	doc, err := k.client.GetDocument(ctx, k.index, key)
	if err != nil {
		if err == ErrDocumentNotFound {
			return nil, fmt.Errorf("cache miss")
		}
		return nil, err
	}

	// 检查 TTL
	source := doc.Source
	if ttl, ok := source["ttl"].(string); ok {
		ttlTime, err := time.Parse(time.RFC3339, ttl)
		if err != nil {
			logger.Warn("Failed to parse TTL",
				logger.GetField("key", key),
				logger.GetField("ttl_value", ttl),
				zap.Error(err))
		} else if time.Now().After(ttlTime) {
			// 过期，删除文档
			_ = k.Delete(ctx, key)
			logger.Info("Key expired and deleted",
				logger.GetField("key", key),
				logger.GetField("expired_at", ttlTime),
				logger.GetField("current_time", time.Now().Format(time.RFC3339)))
			return nil, fmt.Errorf("cache miss: expired")
		} else {
			logger.Debug("TTL check passed",
				logger.GetField("key", key),
				logger.GetField("expires_at", ttlTime),
				logger.GetField("remaining", time.Until(ttlTime)))
		}
	}

	// 更新访问统计
	if err := k.updateAccessStats(ctx, key); err != nil {
		logger.Warn("failed to update access stats", logger.GetField("key", key), zap.Error(err))
	}

	// 返回响应
	if response, ok := source["response"].(string); ok {
		logger.Debug("Key retrieved successfully", logger.GetField("key", key))
		return []byte(response), nil
	}

	return nil, fmt.Errorf("cache miss")
}

// GetString 获取字符串值
func (k *KVStore) GetString(ctx context.Context, key string) (string, error) {
	val, err := k.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return string(val.([]byte)), nil
}

// GetBytes 获取字节值
func (k *KVStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	val, err := k.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	return val.([]byte), nil
}

// Delete 删除键
func (k *KVStore) Delete(ctx context.Context, key string) error {
	// 使用 delete_by_query 删除文档
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"key.keyword": key,
			},
		},
	}

	_, err := k.client.DeleteByQuery(ctx, k.index, query)
	if err != nil {
		logger.LogError("failed to delete key from elasticsearch", err,
			logger.GetField("key", key),
			logger.GetField("index", k.index))
		return err
	}

	logger.Debug("Key deleted successfully", logger.GetField("key", key))
	return nil
}

// Exists 检查键是否存在
func (k *KVStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := k.client.GetDocument(ctx, k.index, key)
	if err != nil {
		if err == ErrDocumentNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Expire 设置过期时间
func (k *KVStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	// 使用 Update API 更新 TTL
	update := map[string]interface{}{
		"doc": map[string]interface{}{
			"ttl": time.Now().Add(ttl).Format(time.RFC3339),
		},
	}

	if err := k.client.UpdateByScript(ctx, k.index, key, update); err != nil {
		logger.LogError("failed to set expire time", err,
			logger.GetField("key", key),
			logger.GetField("ttl", ttl))
		return err
	}

	logger.Debug("Expire time set successfully",
		logger.GetField("key", key),
		logger.GetField("ttl", ttl))
	return nil
}

// TTL 获取剩余过期时间
func (k *KVStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	doc, err := k.client.GetDocument(ctx, k.index, key)
	if err != nil {
		if err == ErrDocumentNotFound {
			return -1, nil // 没有设置 TTL 或键不存在
		}
		return 0, err
	}

	source := doc.Source
	if ttl, ok := source["ttl"].(string); ok {
		ttlTime, err := time.Parse(time.RFC3339, ttl)
		if err != nil {
			return 0, err
		}

		remaining := time.Until(ttlTime)
		if remaining > 0 {
			return remaining, nil
		}

		// 已经过期
		_ = k.Delete(ctx, key)
		return 0, nil
	}

	return -1, nil // 没有设置 TTL
}

// SetBatch 批量设置
func (k *KVStore) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	// 构建 bulk request
	var buf bytes.Buffer
	now := time.Now()

	for key, value := range items {
		// index 操作元数据
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": k.index,
				"_id":    key,
			},
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		buf.Write(metaJSON)
		buf.WriteByte('\n')

		// 文档内容
		doc := map[string]interface{}{
			"key":           key,
			"response":      string(value.([]byte)),
			"timestamp":     now.Format(time.RFC3339),
			"last_accessed": now.Format(time.RFC3339),
			"access_count":  0,
		}

		if ttl > 0 {
			doc["ttl"] = now.Add(ttl).Format(time.RFC3339)
		}

		docJSON, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// 执行批量请求
	result, err := k.client.BulkRequest(ctx, &buf, "true")
	if err != nil {
		logger.LogError("failed to set batch keys", err,
			logger.GetField("count", len(items)))
		return err
	}

	// 检查错误
	if result.Errors {
		// 记录错误详情
		errorCount := 0
		for _, item := range result.Items {
			if item.Index.Status >= 400 {
				errorCount++
				logger.Warn("bulk set item failed",
					logger.GetField("id", item.Index.ID),
					logger.GetField("status", item.Index.Status),
					logger.GetField("error", item.Index.Error.Reason))
			}
		}

		logger.Warn("bulk set had errors",
			logger.GetField("total", len(items)),
			logger.GetField("errors", errorCount))
	} else {
		logger.Debug("Batch set successful", logger.GetField("count", len(items)))
	}

	return nil
}

// GetBatch 批量获取
func (k *KVStore) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	// 构建 mget request
	var buf bytes.Buffer
	buf.WriteString(`{"docs":[`)

	for i, key := range keys {
		if i > 0 {
			buf.WriteString(",")
		}
		doc := map[string]interface{}{
			"_index": k.index,
			"_id":    key,
		}
		docJSON, _ := json.Marshal(doc)
		buf.Write(docJSON)
	}

	buf.WriteString(`]}`)

	// 执行请求
	result, err := k.client.MgetRequest(ctx, &buf)
	if err != nil {
		logger.LogError("failed to get batch keys", err,
			logger.GetField("count", len(keys)))
		return nil, err
	}

	// 解析结果
	results := make(map[string]interface{})
	for _, doc := range result.Docs {
		if doc.Found {
			source := doc.Source

			// 检查 TTL
			if ttl, ok := source["ttl"].(string); ok {
				ttlTime, err := time.Parse(time.RFC3339, ttl)
				if err == nil && time.Now().After(ttlTime) {
					continue // 已过期，跳过
				}
			}

			// 返回响应
			if response, ok := source["response"].(string); ok {
				results[doc.ID] = []byte(response)
			}
		}
	}

	logger.Debug("Batch get successful",
		logger.GetField("requested", len(keys)),
		logger.GetField("found", len(results)))

	return results, nil
}

// DeleteBatch 批量删除
func (k *KVStore) DeleteBatch(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// 使用 delete_by_query 批量删除
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{
				"key.keyword": keys,
			},
		},
	}

	result, err := k.client.DeleteByQuery(ctx, k.index, query)
	if err != nil {
		logger.LogError("failed to delete batch keys", err,
			logger.GetField("count", len(keys)))
		return err
	}

	logger.Debug("Batch delete successful",
		logger.GetField("requested", len(keys)),
		logger.GetField("deleted", result.Deleted))

	return nil
}

// Keys 获取匹配模式的所有键
func (k *KVStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	// 如果pattern是"*",直接查询所有文档
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 10000,
		"_source": false, // 不需要返回文档内容,只需要ID
	}

	logger.Info("Searching keys in elasticsearch",
		logger.GetField("index", k.index),
		logger.GetField("pattern", pattern))

	result, err := k.client.SearchRequest(ctx, k.index, query)
	if err != nil {
		logger.LogError("Failed to search keys", err, logger.GetField("index", k.index))
		return nil, err
	}

	logger.Info("Search result",
		logger.GetField("index", k.index),
		logger.GetField("hits_count", len(result.Hits.Hits)),
		logger.GetField("total_value", result.Hits.Total.Value))

	// 提取键 (从文档ID中获取)
	keys := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		keys = append(keys, hit.ID)
	}

	logger.Debug("Keys retrieved successfully",
		logger.GetField("pattern", pattern),
		logger.GetField("count", len(keys)),
		logger.GetField("total_hits", result.Hits.Total.Value))

	return keys, nil
}

// GetAll 获取所有键值对(包括已过期的)
func (k *KVStore) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 10000,
	}

	logger.Debug("Getting all values",
		logger.GetField("index", k.index),
		logger.GetField("pattern", pattern))

	result, err := k.client.SearchRequest(ctx, k.index, query)
	if err != nil {
		logger.LogError("Failed to search all values", err, logger.GetField("index", k.index))
		return nil, err
	}

	// 返回所有文档的response字段
	values := make(map[string][]byte)
	for _, hit := range result.Hits.Hits {
		source := hit.Source
		if response, ok := source["response"].(string); ok {
			values[hit.ID] = []byte(response)
		}
	}

	logger.Debug("All values retrieved successfully",
		logger.GetField("pattern", pattern),
		logger.GetField("count", len(values)),
		logger.GetField("total_hits", result.Hits.Total.Value))

	return values, nil
}

// Count 获取匹配模式的键总数
func (k *KVStore) Count(ctx context.Context, pattern string) (int64, error) {
	// 使用size=0的搜索查询来获取总数,比_count API更通用
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 0, // 只需要总数,不需要返回文档
		"track_total_hits": true,
	}

	result, err := k.client.SearchRequest(ctx, k.index, query)
	if err != nil {
		logger.LogError("Failed to count keys", err, logger.GetField("index", k.index))
		return 0, err
	}

	count := int64(result.Hits.Total.Value)
	logger.Debug("Keys counted", logger.GetField("pattern", pattern), logger.GetField("count", count))

	return count, nil
}

// FlushDB 清空数据库
func (k *KVStore) FlushDB(ctx context.Context) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	_, err := k.client.DeleteByQuery(ctx, k.index, query)
	if err != nil {
		logger.LogError("failed to flush database", err, logger.GetField("index", k.index))
		return err
	}

	logger.Info("Database flushed", logger.GetField("index", k.index))
	return nil
}

// Close 关闭连接
func (k *KVStore) Close() error {
	// Elasticsearch 客户端是共享的，不需要在这里关闭
	return nil
}

// GetStoreInfo 获取存储后端信息
func (k *KVStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "elasticsearch",
	}
}

// updateAccessStats 更新访问统计（异步）
func (k *KVStore) updateAccessStats(ctx context.Context, key string) error {
	// 使用脚本更新访问计数和最后访问时间
	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.access_count += params.count; ctx._source.last_accessed = params.time",
			"params": map[string]interface{}{
				"count": 1,
				"time":  time.Now().Format(time.RFC3339),
			},
		},
	}

	// 不等待结果，避免阻塞主流程
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = k.client.UpdateByScript(ctx, k.index, key, script)
	}()

	return nil
}
