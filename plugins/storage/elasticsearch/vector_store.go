package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"
)

// VectorStore 向量存储实现（基于 Elasticsearch）
type VectorStore struct {
	client    *Client
	index     string
	dimension int
}

// NewVectorStore 创建向量存储
func NewVectorStore(client *Client, index string, dimension int) *VectorStore {
	return &VectorStore{
		client:    client,
		index:     index,
		dimension: dimension,
	}
}

// Insert 插入向量
func (v *VectorStore) Insert(ctx context.Context, vectors []storage.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	// 构建 bulk request
	var buf bytes.Buffer
	now := time.Now()

	for _, vec := range vectors {
		// index 操作元数据
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": v.index,
				"_id":    vec.ID,
			},
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal meta: %w", err)
		}
		buf.Write(metaJSON)
		buf.WriteByte('\n')

		// 文档内容
		doc := map[string]interface{}{
			"key":                vec.ID,
			"embedding":          convertVector(vec.Vector),
			"embedding_dimension": v.dimension,
			"timestamp":          now.Format(time.RFC3339),
			"last_accessed":      now.Format(time.RFC3339),
			"access_count":       0,
		}

		// 合并元数据
		for key, value := range vec.Metadata {
			// 保留常用字段
			if key != "key" && key != "embedding" && key != "embedding_dimension" {
				doc[key] = value
			}
		}

		docJSON, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("failed to marshal doc: %w", err)
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// 执行批量请求
	result, err := v.client.BulkRequest(ctx, &buf, "true")
	if err != nil {
		logger.LogError("failed to insert vectors", err,
			logger.GetField("count", len(vectors)),
			logger.GetField("index", v.index))
		return err
	}

	// 检查错误
	if result.Errors {
		errorCount := 0
		for _, item := range result.Items {
			if item.Index.Status >= 400 {
				errorCount++
				logger.Warn("bulk insert item failed",
					logger.GetField("id", item.Index.ID),
					logger.GetField("status", item.Index.Status),
					logger.GetField("error", item.Index.Error.Reason))
			}
		}

		logger.Warn("bulk insert had errors",
			logger.GetField("total", len(vectors)),
			logger.GetField("errors", errorCount))
	} else {
		logger.Debug("Vectors inserted successfully", logger.GetField("count", len(vectors)))
	}

	return nil
}

// Search 向量搜索（KNN）
func (v *VectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
	if len(query) != v.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", v.dimension, len(query))
	}

	// 构建 KNN 查询 - 使用顶层 knn 参数（Elasticsearch 8.x+ 要求）
	queryMap := map[string]interface{}{
		"size": topK,
		"knn": map[string]interface{}{
			"field":           "embedding",
			"query_vector":    convertVector(query),
			"k":               topK,
			"num_candidates":  100,
		},
	}

	// 如果有过滤条件，需要添加到 knn.filter 中
	if len(filter) > 0 {
		knnQuery := queryMap["knn"].(map[string]interface{})
		filters := []map[string]interface{}{}

		for key, value := range filter {
			filters = append(filters, map[string]interface{}{
				"term": map[string]interface{}{
					key: value,
				},
			})
		}

		if len(filters) > 0 {
			knnQuery["filter"] = filters
		}
	}

	// 执行搜索
	result, err := v.client.SearchRequest(ctx, v.index, queryMap)
	if err != nil {
		logger.LogError("vector search failed", err,
			logger.GetField("index", v.index),
			logger.GetField("topK", topK))
		return nil, err
	}

	// 解析结果
	searchResults := make([]storage.SearchResult, 0, len(result.Hits.Hits))

	for _, hit := range result.Hits.Hits {
		source := hit.Source

		// 提取向量
		var embedding []float32
		if emb, ok := source["embedding"].([]interface{}); ok {
			embedding = make([]float32, v.dimension)
			for i, val := range emb {
				embedding[i] = float32(val.(float64))
			}
		}

		// ES KNN 搜索返回的 _score 是相似度分数（0-1）
		score := float32(hit.Score)

		searchResults = append(searchResults, storage.SearchResult{
			ID:       hit.ID,
			Score:    score,
			Vector:   embedding,
			Metadata: source,
		})

		// 异步更新访问统计
		go v.updateAccessStats(hit.ID)
	}

	logger.Debug("Vector search completed",
		logger.GetField("topK", topK),
		logger.GetField("results", len(searchResults)))

	return searchResults, nil
}

// SearchByText 基于 BM25 全文检索进行文本匹配，无需 Embedding API 调用。
// 实现了 storage.FullTextSearchStore 接口。
//
// ES 动态映射已将顶层 request 字段索引为 text 类型（Insert 时 metadata 展开到文档顶层），
// 因此无需修改 Index Mapping 即可直接使用 BM25 match 查询。
//
// BM25 原始分数通过 tanh(score/10) 归一化到 0-1：
//   - score=5  → 0.46（弱匹配）
//   - score=10 → 0.76（中等匹配）
//   - score=15 → 0.91（强匹配，典型直接命中区间）
//   - score=20 → 0.96（近似完全匹配）
func (v *VectorStore) SearchByText(ctx context.Context, query string, topK int, minScore float32) ([]storage.SearchResult, error) {
	if query == "" {
		return []storage.SearchResult{}, nil
	}
	if topK <= 0 {
		topK = 5
	}

	// minimum_should_match 保证至少 30% 的词条命中，过滤完全不相关的结果
	queryMap := map[string]interface{}{
		"size": topK * 2, // 多取一些，Go 层归一化过滤后再截断到 topK
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"request": map[string]interface{}{
					"query":                query,
					"minimum_should_match": "30%",
				},
			},
		},
	}

	result, err := v.client.SearchRequest(ctx, v.index, queryMap)
	if err != nil {
		return nil, fmt.Errorf("BM25 text search failed: %w", err)
	}

	searchResults := make([]storage.SearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		// tanh(score/10) 将无界 BM25 分数单调映射到 (0, 1)
		normalizedScore := float32(math.Tanh(hit.Score / 10.0))
		if normalizedScore < minScore {
			continue
		}

		source := hit.Source

		// 提取向量（直接命中路径不需要，但保持接口一致性）
		var embedding []float32
		if emb, ok := source["embedding"].([]interface{}); ok {
			embedding = make([]float32, len(emb))
			for i, val := range emb {
				if f, ok := val.(float64); ok {
					embedding[i] = float32(f)
				}
			}
		}

		searchResults = append(searchResults, storage.SearchResult{
			ID:       hit.ID,
			Score:    normalizedScore,
			Vector:   embedding,
			Metadata: source,
		})

		if len(searchResults) >= topK {
			break
		}
	}

	logger.Debug("BM25 text search completed",
		logger.GetField("topK", topK),
		logger.GetField("results", len(searchResults)))

	return searchResults, nil
}

// Delete 删除向量
func (v *VectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建 bulk delete request
	var buf bytes.Buffer

	for _, id := range ids {
		meta := map[string]interface{}{
			"delete": map[string]interface{}{
				"_index": v.index,
				"_id":    id,
			},
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		buf.Write(metaJSON)
		buf.WriteByte('\n')
	}

	// 执行批量删除
	result, err := v.client.BulkRequest(ctx, &buf, "true")
	if err != nil {
		logger.LogError("failed to delete vectors", err,
			logger.GetField("count", len(ids)),
			logger.GetField("index", v.index))
		return err
	}

	// 检查错误
	if result.Errors {
		errorCount := 0
		for _, item := range result.Items {
			if item.Delete.Status >= 400 {
				errorCount++
				logger.Warn("bulk delete item failed",
					logger.GetField("id", item.Delete.ID),
					logger.GetField("status", item.Delete.Status))
			}
		}

		logger.Warn("bulk delete had errors",
			logger.GetField("total", len(ids)),
			logger.GetField("errors", errorCount))
	} else {
		logger.Debug("Vectors deleted successfully", logger.GetField("count", len(ids)))
	}

	return nil
}

// Get 获取向量
func (v *VectorStore) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
	if len(ids) == 0 {
		return []storage.Vector{}, nil
	}

	// 构建 mget request
	var buf bytes.Buffer
	buf.WriteString(`{"docs":[`)

	for i, id := range ids {
		if i > 0 {
			buf.WriteString(",")
		}
		doc := map[string]interface{}{
			"_index": v.index,
			"_id":    id,
		}
		docJSON, _ := json.Marshal(doc)
		buf.Write(docJSON)
	}

	buf.WriteString(`]}`)

	// 执行请求
	result, err := v.client.MgetRequest(ctx, &buf)
	if err != nil {
		logger.LogError("failed to get vectors", err,
			logger.GetField("count", len(ids)))
		return nil, err
	}

	// 解析结果
	vectors := make([]storage.Vector, 0, len(result.Docs))

	for _, doc := range result.Docs {
		if doc.Found {
			source := doc.Source

			// 提取向量
			var embedding []float32
			if emb, ok := source["embedding"].([]interface{}); ok {
				embedding = make([]float32, v.dimension)
				for i, val := range emb {
					embedding[i] = float32(val.(float64))
				}
			}

			vectors = append(vectors, storage.Vector{
				ID:       doc.ID,
				Vector:   embedding,
				Metadata: source,
			})
		}
	}

	logger.Debug("Vectors retrieved",
		logger.GetField("requested", len(ids)),
		logger.GetField("found", len(vectors)))

	return vectors, nil
}

// Update 更新向量
func (v *VectorStore) Update(ctx context.Context, vectors []storage.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	// 构建 bulk update request
	var buf bytes.Buffer

	for _, vec := range vectors {
		// update 操作元数据
		meta := map[string]interface{}{
			"update": map[string]interface{}{
				"_index": v.index,
				"_id":    vec.ID,
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
			"doc": map[string]interface{}{
				"embedding": convertVector(vec.Vector),
			},
		}

		docJSON, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// 执行批量更新
	result, err := v.client.BulkRequest(ctx, &buf, "true")
	if err != nil {
		logger.LogError("failed to update vectors", err,
			logger.GetField("count", len(vectors)))
		return err
	}

	// 检查错误
	if result.Errors {
		errorCount := 0
		for _, item := range result.Items {
			if item.Update.Status >= 400 {
				errorCount++
				logger.Warn("bulk update item failed",
					logger.GetField("id", item.Update.ID),
					logger.GetField("status", item.Update.Status))
			}
		}

		logger.Warn("bulk update had errors",
			logger.GetField("total", len(vectors)),
			logger.GetField("errors", errorCount))
	} else {
		logger.Debug("Vectors updated successfully", logger.GetField("count", len(vectors)))
	}

	return nil
}

// CreateCollection 创建集合（ES 中对应 Index）
func (v *VectorStore) CreateCollection(ctx context.Context, collection string, dimension int) error {
	// 使用当前索引的配置
	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type": "keyword",
				},
				"embedding": map[string]interface{}{
					"type":       "dense_vector",
					"dims":       dimension,
					"index":      true,
					"similarity": "cosine",
					"index_options": map[string]interface{}{
						"type":             "hnsw",
						"m":                16,
						"ef_construction":  200,
					},
				},
				"timestamp": map[string]interface{}{
					"type": "date",
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"dynamic": true,
				},
			},
		},
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
			"refresh_interval":   "1s",
		},
	}

	if err := v.client.CreateIndex(ctx, collection, mapping); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	logger.Info("Collection created successfully",
		logger.GetField("name", collection),
		logger.GetField("dimension", dimension))

	return nil
}

// DropCollection 删除集合
func (v *VectorStore) DropCollection(ctx context.Context, collection string) error {
	if err := v.client.DeleteIndex(ctx, collection); err != nil {
		return fmt.Errorf("failed to drop collection: %w", err)
	}

	logger.Info("Collection dropped successfully", logger.GetField("name", collection))
	return nil
}

// CollectionExists 检查集合是否存在
func (v *VectorStore) CollectionExists(ctx context.Context, collection string) (bool, error) {
	exists, err := v.client.IndexExists(ctx, collection)
	if err != nil {
		return false, fmt.Errorf("failed to check collection: %w", err)
	}
	return exists, nil
}

// GetCollection 获取集合信息
func (v *VectorStore) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
	// 使用 _count API 获取文档数量
	_ = map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	result, err := v.client.SearchRequest(ctx, collection, map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	info := &storage.CollectionInfo{
		Name:      collection,
		Dimension: v.dimension,
		Count:     int64(result.Hits.Total.Value),
	}

	logger.Debug("Collection info retrieved",
		logger.GetField("name", collection),
		logger.GetField("count", info.Count))

	return info, nil
}

// ListAll 列出集合中所有文档
func (v *VectorStore) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	queryMap := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": limit,
		"from": offset,
		"sort": []map[string]interface{}{
			{"timestamp": map[string]interface{}{"order": "desc"}},
		},
	}

	result, err := v.client.SearchRequest(ctx, collection, queryMap)
	if err != nil {
		logger.LogError("failed to list all vectors", err,
			logger.GetField("index", collection),
			logger.GetField("limit", limit),
			logger.GetField("offset", offset))
		return nil, 0, err
	}

	// ES 文档是扁平结构：业务字段（request、response、model 等）直接在文档顶层。
	// 归一化：ID = hit.ID，Metadata = 文档顶层字段（排除原始向量以节省内存）。
	entries := make([]storage.VectorEntry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		metadata := make(map[string]interface{}, len(hit.Source))
		var createdAt time.Time
		for k, val := range hit.Source {
			if k == "embedding" {
				continue // 跳过原始向量数据
			}
			metadata[k] = val
			if k == "timestamp" {
				if ts, ok := val.(string); ok {
					if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
						createdAt = parsed
					}
				}
			}
		}
		entries = append(entries, storage.VectorEntry{
			ID:        hit.ID,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	total := int64(result.Hits.Total.Value)
	logger.Debug("Vectors listed",
		logger.GetField("index", collection),
		logger.GetField("total", total),
		logger.GetField("returned", len(entries)))

	return entries, total, nil
}

// ListCollections 列出所有集合
func (v *VectorStore) ListCollections(ctx context.Context) ([]string, error) {
	// 这里简化处理，直接返回当前索引
	return []string{v.index}, nil
}

// GetStoreInfo 获取存储后端信息
func (v *VectorStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "elasticsearch",
	}
}

// GetDefaultCollection 获取默认集合名称
func (v *VectorStore) GetDefaultCollection() string {
	return v.index
}

// Close 关闭连接
func (v *VectorStore) Close() error {
	// Elasticsearch 客户端是共享的，不需要在这里关闭
	return nil
}

// updateAccessStats 更新访问统计（异步）
func (v *VectorStore) updateAccessStats(id string) {
	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.access_count += params.count; ctx._source.last_accessed = params.time",
			"params": map[string]interface{}{
				"count": 1,
				"time":  time.Now().Format(time.RFC3339),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = v.client.UpdateByScript(ctx, v.index, id, script)
}

// convertVector 转换向量格式（[]float32 -> []interface{}）
func convertVector(vector []float32) []interface{} {
	result := make([]interface{}, len(vector))
	for i, v := range vector {
		result[i] = float64(v)
	}
	return result
}
