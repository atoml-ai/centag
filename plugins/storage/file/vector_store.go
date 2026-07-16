package file

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/storage"
)

// vectorCollection 向量集合持久化结构
type vectorCollection struct {
	Name      string          `json:"name"`
	Dimension int             `json:"dimension"`
	Vectors   []storedVector  `json:"vectors"`
}

// storedVector 持久化的向量条目
type storedVector struct {
	ID        string                 `json:"id"`
	Vector    []float32              `json:"vector"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// VectorStore 文件实现的向量存储
type VectorStore struct {
	mu                sync.RWMutex
	vectorDir         string
	defaultCollection string
	collections       map[string]*vectorCollection // 内存中的集合缓存
}

// NewVectorStore 创建新的文件向量存储
func NewVectorStore(vectorDir, defaultCollection string) (*VectorStore, error) {
	store := &VectorStore{
		vectorDir:         vectorDir,
		defaultCollection: defaultCollection,
		collections:       make(map[string]*vectorCollection),
	}

	return store, nil
}

// collectionFilePath 获取集合文件路径
func (s *VectorStore) collectionFilePath(collection string) string {
	return strings.TrimRight(s.vectorDir, "/") + "/" + collection + ".json"
}

// loadCollection 从文件加载集合
func (s *VectorStore) loadCollection(collection string) (*vectorCollection, error) {
	filePath := s.collectionFilePath(collection)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var coll vectorCollection
	if err := json.Unmarshal(data, &coll); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collection %s: %w", collection, err)
	}

	return &coll, nil
}

// saveCollection 持久化集合到文件
func (s *VectorStore) saveCollection(coll *vectorCollection) error {
	data, err := json.MarshalIndent(coll, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal collection %s: %w", coll.Name, err)
	}

	filePath := s.collectionFilePath(coll.Name)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write collection file: %w", err)
	}

	return nil
}

// getOrLoadCollection 获取或加载集合
func (s *VectorStore) getOrLoadCollection(name string) (*vectorCollection, error) {
	// 先检查内存缓存
	if coll, ok := s.collections[name]; ok {
		return coll, nil
	}

	// 从文件加载
	coll, err := s.loadCollection(name)
	if err != nil {
		return nil, err
	}
	if coll != nil {
		s.collections[name] = coll
	}

	return coll, nil
}

// cosineSimilarity 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// Insert 插入向量
func (s *VectorStore) Insert(ctx context.Context, vectors []storage.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	collName := s.defaultCollection
	coll, err := s.getOrLoadCollection(collName)
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}
	if coll == nil {
		return fmt.Errorf("collection %s does not exist", collName)
	}

	now := time.Now()
	for _, vec := range vectors {
		// 检查是否已存在
		found := false
		for i, existing := range coll.Vectors {
			if existing.ID == vec.ID {
				coll.Vectors[i] = storedVector{
					ID:        vec.ID,
					Vector:    vec.Vector,
					Metadata:  vec.Metadata,
					CreatedAt: existing.CreatedAt,
					UpdatedAt: now,
				}
				found = true
				break
			}
		}
		if !found {
			coll.Vectors = append(coll.Vectors, storedVector{
				ID:        vec.ID,
				Vector:    vec.Vector,
				Metadata:  vec.Metadata,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}

	return s.saveCollection(coll)
}

// Search 搜索最相似的向量
func (s *VectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collName := s.defaultCollection
	coll, err := s.getOrLoadCollection(collName)
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}
	if coll == nil {
		return nil, fmt.Errorf("collection %s does not exist", collName)
	}

	// 计算所有向量的余弦相似度
	type scored struct {
		vec   storedVector
		score float32
	}

	var results []scored
	for _, stored := range coll.Vectors {
		// 简单的过滤器支持
		if len(filter) > 0 {
			if !matchesFilter(stored.Metadata, filter) {
				continue
			}
		}

		score := cosineSimilarity(query, stored.Vector)
		results = append(results, scored{vec: stored, score: score})
	}

	// 按相似度降序排列
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// 取 topK
	if topK > len(results) {
		topK = len(results)
	}

	var searchResults []storage.SearchResult
	for i := 0; i < topK; i++ {
		searchResults = append(searchResults, storage.SearchResult{
			ID:       results[i].vec.ID,
			Vector:   results[i].vec.Vector,
			Score:    results[i].score,
			Metadata: results[i].vec.Metadata,
		})
	}

	return searchResults, nil
}

// Delete 删除向量
func (s *VectorStore) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	collName := s.defaultCollection
	coll, err := s.getOrLoadCollection(collName)
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}
	if coll == nil {
		return fmt.Errorf("collection %s does not exist", collName)
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	var remaining []storedVector
	for _, stored := range coll.Vectors {
		if !idSet[stored.ID] {
			remaining = append(remaining, stored)
		}
	}

	coll.Vectors = remaining
	return s.saveCollection(coll)
}

// Get 获取向量
func (s *VectorStore) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collName := s.defaultCollection
	coll, err := s.getOrLoadCollection(collName)
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}
	if coll == nil {
		return nil, fmt.Errorf("collection %s does not exist", collName)
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	var result []storage.Vector
	for _, stored := range coll.Vectors {
		if idSet[stored.ID] {
			result = append(result, storage.Vector{
				ID:       stored.ID,
				Vector:   stored.Vector,
				Metadata: stored.Metadata,
			})
		}
	}

	return result, nil
}

// Update 更新向量
func (s *VectorStore) Update(ctx context.Context, vectors []storage.Vector) error {
	return s.Insert(ctx, vectors)
}

// CreateCollection 创建集合
func (s *VectorStore) CreateCollection(ctx context.Context, collection string, dimension int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在
	if _, ok := s.collections[collection]; ok {
		return nil
	}

	coll, err := s.loadCollection(collection)
	if err != nil {
		return err
	}
	if coll != nil {
		s.collections[collection] = coll
		return nil
	}

	coll = &vectorCollection{
		Name:      collection,
		Dimension: dimension,
		Vectors:   make([]storedVector, 0),
	}

	s.collections[collection] = coll
	return s.saveCollection(coll)
}

// DropCollection 删除集合
func (s *VectorStore) DropCollection(ctx context.Context, collection string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.collections, collection)

	filePath := s.collectionFilePath(collection)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove collection file: %w", err)
	}

	return nil
}

// CollectionExists 检查集合是否存在
func (s *VectorStore) CollectionExists(ctx context.Context, collection string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.collections[collection]; ok {
		return true, nil
	}

	filePath := s.collectionFilePath(collection)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// GetCollection 获取集合信息
func (s *VectorStore) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll, err := s.getOrLoadCollection(collection)
	if err != nil {
		return nil, err
	}
	if coll == nil {
		return nil, fmt.Errorf("collection %s does not exist", collection)
	}

	return &storage.CollectionInfo{
		Name:      coll.Name,
		Dimension: coll.Dimension,
		Count:     int64(len(coll.Vectors)),
		IndexType: "brute_force",
		MetricType: string(storage.MetricTypeCosine),
	}, nil
}

// ListCollections 列出所有集合
func (s *VectorStore) ListCollections(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.vectorDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read vector directory: %w", err)
	}

	var collections []string
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			collName := strings.TrimSuffix(name, ".json")
			if !seen[collName] {
				collections = append(collections, collName)
				seen[collName] = true
			}
		}
	}

	// 也包括仅存在于内存中的集合
	for name := range s.collections {
		if !seen[name] {
			collections = append(collections, name)
		}
	}

	return collections, nil
}

// ListAll 列出集合中所有文档
func (s *VectorStore) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll, err := s.getOrLoadCollection(collection)
	if err != nil {
		return nil, 0, err
	}
	if coll == nil {
		return nil, 0, fmt.Errorf("collection %s does not exist", collection)
	}

	total := int64(len(coll.Vectors))

	// 应用分页
	start := offset
	if start > len(coll.Vectors) {
		start = len(coll.Vectors)
	}
	end := start + limit
	if end > len(coll.Vectors) {
		end = len(coll.Vectors)
	}

	var entries []storage.VectorEntry
	for _, stored := range coll.Vectors[start:end] {
		entries = append(entries, storage.VectorEntry{
			ID:        stored.ID,
			Metadata:  stored.Metadata,
			CreatedAt: stored.CreatedAt,
		})
	}

	return entries, total, nil
}

// GetStoreInfo 获取存储后端信息
func (s *VectorStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "file",
	}
}

// GetDefaultCollection 获取默认集合名称
func (s *VectorStore) GetDefaultCollection() string {
	return s.defaultCollection
}

// Close 关闭连接
func (s *VectorStore) Close() error {
	return nil
}

// matchesFilter 检查元数据是否匹配过滤条件
func matchesFilter(metadata map[string]interface{}, filter map[string]interface{}) bool {
	if metadata == nil {
		return len(filter) == 0
	}

	for k, v := range filter {
		mv, ok := metadata[k]
		if !ok {
			return false
		}
		// 简单相等比较
		if fmt.Sprintf("%v", mv) != fmt.Sprintf("%v", v) {
			return false
		}
	}

	return true
}
