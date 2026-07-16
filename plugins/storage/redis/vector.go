package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisVectorStore Redis向量存储
type RedisVectorStore struct {
	client *redis.Client
}

// NewVectorStore 创建向量存储
func NewVectorStore(client *redis.Client) *RedisVectorStore {
	return &RedisVectorStore{client: client}
}

// Insert 插入向量
func (vs *RedisVectorStore) Insert(ctx context.Context, vectors []storage.Vector) error {
	collectionVectors := make(map[string][]storage.Vector)
	for _, vec := range vectors {
		collection := "default"
		if colName, ok := vec.Metadata["collection"].(string); ok {
			collection = colName
		}
		collectionVectors[collection] = append(collectionVectors[collection], vec)
	}

	for collection, vecs := range collectionVectors {
		keyPrefix := fmt.Sprintf("vector:%s:", collection)

		for _, vec := range vecs {
			key := keyPrefix + vec.ID

			vectorData := make(map[string]interface{})
			vectorData["vector"] = vec.Vector
			vectorData["metadata"] = vec.Metadata

			data, err := json.Marshal(vectorData)
			if err != nil {
				logger.Error("Failed to marshal vector", zap.String("id", vec.ID), zap.Error(err))
				continue
			}

			if err := vs.client.HSet(ctx, key, "data", data).Err(); err != nil {
				return fmt.Errorf("failed to insert vector %s: %w", vec.ID, err)
			}
		}
	}

	logger.Debug("Vectors inserted", zap.Int("count", len(vectors)))
	return nil
}

// Search 搜索最相似的向量
func (vs *RedisVectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
	collection := "default"
	if colName, ok := filter["collection"].(string); ok {
		collection = colName
	}

	pattern := fmt.Sprintf("vector:%s:*", collection)
	keys, err := vs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get vector keys: %w", err)
	}

	var vectors []*storage.Vector
	for _, key := range keys {
		data, err := vs.client.HGet(ctx, key, "data").Result()
		if err != nil {
			if err != redis.Nil {
				logger.Warn("Failed to get vector data", zap.String("key", key), zap.Error(err))
			}
			continue
		}

		var vectorData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &vectorData); err != nil {
			logger.Warn("Failed to unmarshal vector data", zap.String("key", key), zap.Error(err))
			continue
		}

		vectorList := vectorData["vector"].([]interface{})
		vec := make([]float32, len(vectorList))
		for i, v := range vectorList {
			vec[i] = float32(v.(float64))
		}

		metadata, _ := vectorData["metadata"].(map[string]interface{})

		id := strings.TrimPrefix(key, fmt.Sprintf("vector:%s:", collection))

		vectors = append(vectors, &storage.Vector{
			ID:       id,
			Vector:   vec,
			Metadata: metadata,
		})
	}

	results := vs.searchAndSort(vectors, query, topK, filter)
	return results, nil
}

// Delete 删除向量
func (vs *RedisVectorStore) Delete(ctx context.Context, ids []string) error {
	patterns := []string{"vector:default:*", "vector:*:*"}

	var keys []string
	for _, pattern := range patterns {
		matchedKeys, err := vs.client.Keys(ctx, pattern).Result()
		if err != nil {
			logger.Warn("Failed to get keys by pattern", zap.String("pattern", pattern), zap.Error(err))
			continue
		}
		keys = append(keys, matchedKeys...)
	}

	for _, key := range keys {
		for _, id := range ids {
			if strings.HasSuffix(key, ":"+id) {
				if err := vs.client.Del(ctx, key).Err(); err != nil {
					logger.Warn("Failed to delete vector", zap.String("key", key), zap.Error(err))
				}
			}
		}
	}

	logger.Debug("Vectors deleted", zap.Int("count", len(ids)))
	return nil
}

// Get 获取向量
func (vs *RedisVectorStore) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
	var result []storage.Vector
	patterns := []string{"vector:default:*", "vector:*:*"}

	for _, pattern := range patterns {
		keys, err := vs.client.Keys(ctx, pattern).Result()
		if err != nil {
			continue
		}

		for _, key := range keys {
			for _, id := range ids {
				if strings.HasSuffix(key, ":"+id) {
					data, err := vs.client.HGet(ctx, key, "data").Result()
					if err != nil {
						continue
					}

					var vectorData map[string]interface{}
					if err := json.Unmarshal([]byte(data), &vectorData); err != nil {
						continue
					}

					vectorList := vectorData["vector"].([]interface{})
					vec := make([]float32, len(vectorList))
					for i, v := range vectorList {
						vec[i] = float32(v.(float64))
					}

					metadata, _ := vectorData["metadata"].(map[string]interface{})

					result = append(result, storage.Vector{
						ID:       id,
						Vector:   vec,
						Metadata: metadata,
					})
				}
			}
		}
	}

	return result, nil
}

// Update 更新向量
func (vs *RedisVectorStore) Update(ctx context.Context, vectors []storage.Vector) error {
	return vs.Insert(ctx, vectors)
}

// CreateCollection 创建集合
func (vs *RedisVectorStore) CreateCollection(ctx context.Context, collection string, dimension int) error {
	metaKey := fmt.Sprintf("vector:collection:%s", collection)

	metadata := map[string]interface{}{
		"name":       collection,
		"dimension":  dimension,
		"index_type": "HNSW",
		"metric_type": "COSINE",
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal collection metadata: %w", err)
	}

	if err := vs.client.Set(ctx, metaKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	logger.Info("Collection created",
		zap.String("name", collection),
		zap.Int("dimension", dimension))

	return nil
}

// DropCollection 删除集合
func (vs *RedisVectorStore) DropCollection(ctx context.Context, collection string) error {
	metaKey := fmt.Sprintf("vector:collection:%s", collection)
	vs.client.Del(ctx, metaKey)

	pattern := fmt.Sprintf("vector:%s:*", collection)
	keys, err := vs.client.Keys(ctx, pattern).Result()
	if err != nil {
		logger.Warn("Failed to get vector keys", zap.Error(err))
	}

	if len(keys) > 0 {
		if err := vs.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete vectors: %w", err)
		}
	}

	logger.Info("Collection dropped", zap.String("name", collection))
	return nil
}

// CollectionExists 检查集合是否存在
func (vs *RedisVectorStore) CollectionExists(ctx context.Context, collection string) (bool, error) {
	metaKey := fmt.Sprintf("vector:collection:%s", collection)
	exists, err := vs.client.Exists(ctx, metaKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check collection existence: %w", err)
	}

	return exists > 0, nil
}

// GetCollection 获取集合信息
func (vs *RedisVectorStore) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
	metaKey := fmt.Sprintf("vector:collection:%s", collection)
	data, err := vs.client.Get(ctx, metaKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("collection %s not found", collection)
		}
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collection metadata: %w", err)
	}

	pattern := fmt.Sprintf("vector:%s:*", collection)
	keys, err := vs.client.Keys(ctx, pattern).Result()
	if err != nil {
		keys = []string{}
	}

	return &storage.CollectionInfo{
		Name:       collection,
		Dimension:  int(metadata["dimension"].(float64)),
		Count:      int64(len(keys)),
		IndexType:  metadata["index_type"].(string),
		MetricType: metadata["metric_type"].(string),
	}, nil
}

// ListCollections 列出所有集合
func (vs *RedisVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	pattern := "vector:collection:*"
	keys, err := vs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return []string{}, fmt.Errorf("failed to list collections: %w", err)
	}

	var collections []string
	for _, key := range keys {
		collection := strings.TrimPrefix(key, "vector:collection:")
		collections = append(collections, collection)
	}

	return collections, nil
}

// ListAll 列出集合中所有文档
func (vs *RedisVectorStore) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	pattern := fmt.Sprintf("vector:%s:*", collection)
	keys, err := vs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get vector keys: %w", err)
	}

	total := int64(len(keys))

	// 应用分页
	start := offset
	if start > int(total) {
		start = int(total)
	}
	end := start + limit
	if end > int(total) {
		end = int(total)
	}

	if start >= end {
		return []storage.VectorEntry{}, total, nil
	}

	// Redis 存储格式：{vector: [...], metadata: {...}}
	// 归一化：ID = key suffix，Metadata = vectorData["metadata"]
	entries := make([]storage.VectorEntry, 0, end-start)
	for i := start; i < end; i++ {
		key := keys[i]
		data, err := vs.client.HGet(ctx, key, "data").Result()
		if err != nil {
			if err != redis.Nil {
				logger.Warn("Failed to get vector data", zap.String("key", key), zap.Error(err))
			}
			continue
		}

		var vectorData struct {
			Metadata map[string]interface{} `json:"metadata"`
		}
		if err := json.Unmarshal([]byte(data), &vectorData); err != nil {
			logger.Warn("Failed to unmarshal vector data", zap.String("key", key), zap.Error(err))
			continue
		}

		id := strings.TrimPrefix(key, fmt.Sprintf("vector:%s:", collection))
		metadata := vectorData.Metadata
		if metadata == nil {
			metadata = map[string]interface{}{}
		}

		// 从 metadata 中解析创建时间
		var createdAt time.Time
		switch v := metadata["timestamp"].(type) {
		case float64:
			if v > 0 {
				createdAt = time.Unix(int64(v), 0)
			}
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				createdAt = parsed
			}
		}

		entries = append(entries, storage.VectorEntry{
			ID:        id,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	logger.Debug("Vectors listed",
		zap.String("collection", collection),
		zap.Int64("total", total),
		zap.Int("returned", len(entries)))

	return entries, total, nil
}

// GetStoreInfo 获取存储后端信息
func (vs *RedisVectorStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "redis",
	}
}

// GetDefaultCollection 获取默认集合名称
func (vs *RedisVectorStore) GetDefaultCollection() string {
	return "default"
}

// Close 关闭连接
func (vs *RedisVectorStore) Close() error {
	return nil
}

// ============ 辅助函数 ============

func (vs *RedisVectorStore) searchAndSort(vectors []*storage.Vector, query []float32, topK int, filter map[string]interface{}) []storage.SearchResult {
	type scoredVector struct {
		vector *storage.Vector
		score  float32
	}

	var scores []scoredVector
	for _, vec := range vectors {
		if !matchesVectorFilter(vec, filter) {
			continue
		}

		score := cosineSimilarity(query, vec.Vector)
		scores = append(scores, scoredVector{
			vector: vec,
			score:  score,
		})
	}

	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[i].score < scores[j].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	resultCount := topK
	if resultCount > len(scores) {
		resultCount = len(scores)
	}

	results := make([]storage.SearchResult, resultCount)
	for i := 0; i < resultCount; i++ {
		results[i] = storage.SearchResult{
			ID:       scores[i].vector.ID,
			Vector:   scores[i].vector.Vector,
			Score:    scores[i].score,
			Metadata: scores[i].vector.Metadata,
		}
	}

	return results
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

func matchesVectorFilter(vec *storage.Vector, filter map[string]interface{}) bool {
	if len(filter) == 0 {
		return true
	}

	filterCopy := make(map[string]interface{})
	for k, v := range filter {
		if k != "collection" {
			filterCopy[k] = v
		}
	}

	if len(filterCopy) == 0 {
		return true
	}

	for key, expectedValue := range filterCopy {
		actualValue, ok := vec.Metadata[key]
		if !ok || actualValue != expectedValue {
			return false
		}
	}

	return true
}
