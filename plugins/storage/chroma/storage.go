package chroma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"centag/core/pkg/storage"

	"go.uber.org/zap"
)

// Config ChromaDB配置
type Config struct {
	Addr       string        // 服务器地址，如 localhost:28000
	Collection string        // 集合名称
	Timeout    time.Duration // 请求超时时间
	Token      string        // 认证Token
}

// Storage ChromaDB存储插件
type Storage struct {
	config     *Config
	httpClient *http.Client
	logger     *zap.Logger
	baseURL    string
}

// NewStorage 创建ChromaDB存储实例
func NewStorage(cfg *Config, logger *zap.Logger) (*Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}
	if cfg.Collection == "" {
		cfg.Collection = "llm_cache" // 默认集合名称
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second // 默认超时30秒
	}

	baseURL := fmt.Sprintf("http://%s/api/v1", cfg.Addr)

	return &Storage{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger:  logger,
		baseURL: baseURL,
	}, nil
}

// Connect 连接到ChromaDB
func (s *Storage) Connect(ctx context.Context) error {
	s.logger.Info("connecting to ChromaDB",
		zap.String("addr", s.config.Addr),
		zap.String("collection", s.config.Collection),
	)

	// 检查心跳
	if err := s.heartbeat(ctx); err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	// 创建集合（如果不存在）
	if err := s.createCollection(ctx); err != nil {
		return fmt.Errorf("create collection failed: %w", err)
	}

	s.logger.Info("ChromaDB connected successfully")
	return nil
}

// Close 关闭连接
func (s *Storage) Close() error {
	s.httpClient.CloseIdleConnections()
	s.logger.Info("ChromaDB connection closed")
	return nil
}

// InsertRequest 插入请求
type InsertRequest struct {
	IDs        []string         `json:"ids"`        // 文档ID列表
	Embeddings [][]float64      `json:"embeddings"` // 向量列表
	Metadatas  []map[string]any `json:"metadatas"`  // 元数据列表
	Documents  []string         `json:"documents"`  // 文档内容列表
}

// InsertResponse 插入响应
type InsertResponse struct {
	IDs      []string `json:"ids,omitempty"`
	Duration float64  `json:"duration,omitempty"`
	Message  string   `json:"message,omitempty"`
}

// insertRaw 插入向量数据（内部方法）
func (s *Storage) insertRaw(ctx context.Context, ids []string, embeddings [][]float64, metadatas []map[string]any, documents []string) error {
	if len(ids) != len(embeddings) {
		return fmt.Errorf("ids and embeddings length mismatch: %d vs %d", len(ids), len(embeddings))
	}

	url := fmt.Sprintf("%s/collections/%s/add", s.baseURL, s.config.Collection)

	req := InsertRequest{
		IDs:        ids,
		Embeddings: embeddings,
		Metadatas:  metadatas,
		Documents:  documents,
	}

	resp, err := s.doRequest(ctx, "POST", url, req)
	if err != nil {
		return fmt.Errorf("insert request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("insert failed with status %d: %s", resp.StatusCode, string(body))
	}

	s.logger.Debug("vectors inserted",
		zap.Int("count", len(ids)),
		zap.String("collection", s.config.Collection),
	)

	return nil
}

// Insert 实现VectorStore接口
func (s *Storage) Insert(ctx context.Context, vectors []storage.Vector) error {
	ids := make([]string, len(vectors))
	embeddings := make([][]float64, len(vectors))
	metadatas := make([]map[string]any, len(vectors))
	documents := make([]string, len(vectors))

	for i, v := range vectors {
		ids[i] = v.ID
		// 转换 float32 到 float64
		embeddings[i] = make([]float64, len(v.Vector))
		for j, val := range v.Vector {
			embeddings[i][j] = float64(val)
		}
		metadatas[i] = v.Metadata
	}

	return s.insertRaw(ctx, ids, embeddings, metadatas, documents)
}

// SearchRequest 搜索请求
type SearchRequest struct {
	QueryEmbeddings []float64      `json:"query_embeddings"`  // 查询向量
	NResults        int            `json:"n_results"`         // 返回结果数量
	Where           map[string]any `json:"where,omitempty"`   // 过滤条件
	Include         []string       `json:"include,omitempty"` // 包含的字段，如 ["documents", "metadatas", "distances"]
}

// SearchResult 搜索结果
type SearchResult struct {
	IDs       []string         `json:"ids"`       // 文档ID列表
	Documents []string         `json:"documents"` // 文档内容列表
	Metadatas []map[string]any `json:"metadatas"` // 元数据列表
	Distances [][]float64      `json:"distances"` // 距离列表
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results []SearchResult `json:"results"` // 搜索结果
}

// searchRaw 搜索相似向量（内部方法）
func (s *Storage) searchRaw(ctx context.Context, queryEmbedding []float64, topK int) ([]string, []map[string]any, []float64, error) {
	if topK <= 0 {
		topK = 5 // 默认返回5个结果
	}

	url := fmt.Sprintf("%s/collections/%s/query", s.baseURL, s.config.Collection)

	req := SearchRequest{
		QueryEmbeddings: queryEmbedding,
		NResults:        topK,
		Include:         []string{"documents", "metadatas", "distances"},
	}

	resp, err := s.doRequest(ctx, "POST", url, req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// v2 API 返回格式可能不同，需要兼容处理
	if len(searchResp.Results) == 0 || len(searchResp.Results[0].IDs) == 0 {
		return nil, nil, nil, nil // 没有结果
	}

	result := searchResp.Results[0]

	s.logger.Debug("vector search completed",
		zap.Int("returned", len(result.IDs)),
		zap.Int("top_k", topK),
	)

	return result.IDs, result.Metadatas, result.Distances[0], nil
}

// Search 实现VectorStore接口
func (s *Storage) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
	// 转换 float32 到 float64
	queryFloat64 := make([]float64, len(query))
	for i, val := range query {
		queryFloat64[i] = float64(val)
	}

	ids, metadatas, distances, err := s.searchRaw(ctx, queryFloat64, topK)
	if err != nil {
		return nil, err
	}

	results := make([]storage.SearchResult, len(ids))
	for i, id := range ids {
		results[i] = storage.SearchResult{
			ID:       id,
			Score:    float32(distances[i]),
			Metadata: metadatas[i],
		}
	}

	return results, nil
}

// Delete 删除向量（内部方法）
func (s *Storage) deleteRaw(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/delete", s.baseURL, s.config.Collection)

	req := map[string]any{
		"ids": ids,
	}

	resp, err := s.doRequest(ctx, "POST", url, req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	s.logger.Debug("vectors deleted",
		zap.Int("count", len(ids)),
	)

	return nil
}

// Delete 实现VectorStore接口
func (s *Storage) Delete(ctx context.Context, ids []string) error {
	return s.deleteRaw(ctx, ids)
}

// Get 实现VectorStore接口
func (s *Storage) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
	// ChromaDB没有直接的get API，需要通过search实现
	return nil, fmt.Errorf("Get operation not supported by ChromaDB")
}

// Update 实现VectorStore接口
func (s *Storage) Update(ctx context.Context, vectors []storage.Vector) error {
	// ChromaDB的update操作需要delete + insert
	return fmt.Errorf("Update operation not supported, use Delete + Insert instead")
}

// CreateCollection 实现VectorStore接口
func (s *Storage) CreateCollection(ctx context.Context, collection string, dimension int) error {
	// ChromaDB不支持预先指定维度
	return fmt.Errorf("CreateCollection not supported by ChromaDB")
}

// DropCollection 实现VectorStore接口
func (s *Storage) DropCollection(ctx context.Context, collection string) error {
	return fmt.Errorf("DropCollection not supported by ChromaDB")
}

// CollectionExists 实现VectorStore接口
func (s *Storage) CollectionExists(ctx context.Context, collection string) (bool, error) {
	return false, fmt.Errorf("CollectionExists not supported by ChromaDB")
}

// GetCollection 实现VectorStore接口
func (s *Storage) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
	return nil, fmt.Errorf("GetCollection not supported by ChromaDB")
}

// ListCollections 实现VectorStore接口
func (s *Storage) ListCollections(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("ListCollections not supported by ChromaDB")
}

// ListAll 列出集合中所有文档
func (s *Storage) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	// ChromaDB 暂不支持查询所有文档
	return []storage.VectorEntry{}, 0, nil
}

// GetStoreInfo 获取存储后端信息
func (s *Storage) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "chromadb",
	}
}

// GetDefaultCollection 获取默认集合名称
func (s *Storage) GetDefaultCollection() string {
	return s.config.Collection
}

// HealthCheck 健康检查
func (s *Storage) HealthCheck(ctx context.Context) error {
	return s.heartbeat(ctx)
}

// GetDefaultConfig 获取默认配置
func (s *Storage) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"addr":       "localhost:28000",
		"collection": "llm_cache",
		"timeout":    30,
		"token":      "",
	}
}

// heartbeat 心跳检测
func (s *Storage) heartbeat(ctx context.Context) error {
	// ChromaDB v2 API使用/api/v2/heartbeat
	url := fmt.Sprintf("http://%s/api/v2/heartbeat", s.config.Addr)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status %d", resp.StatusCode)
	}

	return nil
}

// createCollection 创建集合
func (s *Storage) createCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections", s.baseURL)

	req := map[string]any{
		"name":     s.config.Collection,
		"metadata": map[string]string{"description": "Centag Vector Storage"},
	}

	resp, err := s.doRequest(ctx, "POST", url, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 201表示创建成功，如果是409表示已存在
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		s.logger.Debug("collection created", zap.String("name", s.config.Collection))
		return nil
	}

	// 检查是否是集合已存在
	if resp.StatusCode == http.StatusConflict {
		s.logger.Info("collection already exists", zap.String("name", s.config.Collection))
		return nil
	}

	return fmt.Errorf("create collection failed with status %d", resp.StatusCode)
}

// doRequest 执行HTTP请求
func (s *Storage) doRequest(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request failed: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 添加认证token（如果配置了）
	if s.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.Token)
	}

	return s.httpClient.Do(req)
}
