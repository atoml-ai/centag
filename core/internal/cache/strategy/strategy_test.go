package strategy

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/embedding"
	"centag/core/pkg/storage"
)

// mockEmbeddingService 模拟嵌入服务
type mockEmbeddingService struct {
    dimension int
}

func (m *mockEmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
    // 简单模拟：根据文本内容生成固定维度的向量
    vec := make([]float32, m.dimension)
    for i := range vec {
        vec[i] = float32(len(text)+i) * 0.01
    }
    return vec, nil
}

func (m *mockEmbeddingService) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
    results := make([][]float32, len(texts))
    for i, text := range texts {
        vec, err := m.GetEmbedding(ctx, text)
        if err != nil {
            return nil, err
        }
        results[i] = vec
    }
    return results, nil
}

func (m *mockEmbeddingService) GetDimension() int {
    return m.dimension
}

func (m *mockEmbeddingService) GetProviderInfo() embedding.ProviderInfo {
    return embedding.ProviderInfo{
        Provider:  "mock",
        Model:     "mock-embedding",
        Dimension: m.dimension,
    }
}

// mockVectorStore 模拟向量存储
type mockVectorStore struct {
    vectors []storage.Vector
}

func (m *mockVectorStore) Insert(ctx context.Context, vectors []storage.Vector) error {
    m.vectors = append(m.vectors, vectors...)
    return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
    var results []storage.SearchResult
    for _, v := range m.vectors {
        // 简单模拟相似度计算
        score := float32(0.9) // 模拟高相似度
        results = append(results, storage.SearchResult{
            ID:       v.ID,
            Score:    score,
            Metadata: v.Metadata,
        })
    }
    
    // 按分数排序并返回 topK
    if len(results) > topK {
        results = results[:topK]
    }
    return results, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, ids []string) error {
    remaining := make([]storage.Vector, 0)
    idMap := make(map[string]bool)
    for _, id := range ids {
        idMap[id] = true
    }
    for _, v := range m.vectors {
        if !idMap[v.ID] {
            remaining = append(remaining, v)
        }
    }
    m.vectors = remaining
    return nil
}

func (m *mockVectorStore) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
    var result []storage.Vector
    idMap := make(map[string]bool)
    for _, id := range ids {
        idMap[id] = true
    }
    for _, v := range m.vectors {
        if idMap[v.ID] {
            result = append(result, v)
        }
    }
    return result, nil
}

func (m *mockVectorStore) Update(ctx context.Context, vectors []storage.Vector) error {
    for i, v := range m.vectors {
        for _, newV := range vectors {
            if v.ID == newV.ID {
                m.vectors[i] = newV
                break
            }
        }
    }
    return nil
}

func (m *mockVectorStore) CreateCollection(ctx context.Context, collection string, dimension int) error {
    return nil
}

func (m *mockVectorStore) DropCollection(ctx context.Context, collection string) error {
    return nil
}

func (m *mockVectorStore) CollectionExists(ctx context.Context, collection string) (bool, error) {
    return true, nil
}

func (m *mockVectorStore) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
    return &storage.CollectionInfo{Name: collection, Dimension: 1536}, nil
}

func (m *mockVectorStore) ListCollections(ctx context.Context) ([]string, error) {
    return []string{"test-collection"}, nil
}

func (m *mockVectorStore) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	entries := make([]storage.VectorEntry, len(m.vectors))
	for i, v := range m.vectors {
		entries[i] = storage.VectorEntry{
			ID:       v.ID,
			Metadata: v.Metadata,
		}
	}
	return entries, int64(len(entries)), nil
}

func (m *mockVectorStore) Clear(ctx context.Context, collection string) error {
    m.vectors = nil
    return nil
}

func (m *mockVectorStore) Count(ctx context.Context, collection string) (int64, error) {
	return int64(len(m.vectors)), nil
}

func (m *mockVectorStore) Close() error {
	return nil
}

func (m *mockVectorStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{Type: "mock"}
}

func (m *mockVectorStore) GetDefaultCollection() string {
	return "test-collection"
}

func TestExactStrategy(t *testing.T) {
    // TODO: 添加测试
}

func TestSemanticStrategy(t *testing.T) {
    t.Run("NewSemanticStrategy initializes correctly", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        
        if strategy.Name() != "semantic" {
            t.Errorf("expected strategy name 'semantic', got '%s'", strategy.Name())
        }
        
        if !strategy.SupportsSemantic() {
            t.Error("expected semantic strategy to support semantic matching")
        }
    })
    
    t.Run("Configure updates config correctly", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        
        newConfig := map[string]interface{}{
            "storage_name":       "new-storage",
            "vector_storage_name": "new-vector",
            "threshold":          float64(0.90),
            "top_k":              float64(10),
            "model":              "new-model",
        }
        
        err := strategy.Configure(newConfig)
        if err != nil {
            t.Fatalf("failed to configure strategy: %v", err)
        }
        
        if config.StorageName != "new-storage" {
            t.Errorf("expected storage_name 'new-storage', got '%s'", config.StorageName)
        }
        
        if config.VectorStorageName != "new-vector" {
            t.Errorf("expected vector_storage_name 'new-vector', got '%s'", config.VectorStorageName)
        }
        
        if config.Threshold != 0.90 {
            t.Errorf("expected threshold 0.90, got %f", config.Threshold)
        }
        
        if config.TopK != 10 {
            t.Errorf("expected top_k 10, got %d", config.TopK)
        }
        
        if config.Model != "new-model" {
            t.Errorf("expected model 'new-model', got '%s'", config.Model)
        }
    })
    
    t.Run("Read fails without embedding service", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        
        _, err := strategy.Read(context.Background(), "test query", ReadOptions{})
        if err == nil {
            t.Error("expected error when embedding service not initialized")
        }
    })
    
    t.Run("Write fails without embedding service", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        
        entry := &Entry{
            Key:       "test-key",
            Request:   "test request",
            Response:  "test response",
            Timestamp: time.Now(),
            ExpiresAt: time.Now().Add(time.Hour),
        }
        
        err := strategy.Write(context.Background(), entry, WriteOptions{})
        if err == nil {
            t.Error("expected error when embedding service not initialized")
        }
    })
    
    t.Run("SemanticStrategy read/write with mock services", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        
        // 注入模拟服务
        embeddingSvc := &mockEmbeddingService{dimension: 1536}
        vectorStore := &mockVectorStore{}
        
        strategy.SetEmbeddingService(embeddingSvc)
        strategy.SetVectorStore(vectorStore)
        
        // 测试写入
        entry := &Entry{
            Key:       "test-key-1",
            Request:   "什么是机器学习",
            Response:  "机器学习是人工智能的一个分支...",
            Timestamp: time.Now(),
            ExpiresAt: time.Now().Add(time.Hour),
        }
        
        err := strategy.Write(context.Background(), entry, WriteOptions{})
        if err != nil {
            t.Fatalf("failed to write with semantic strategy: %v", err)
        }
        
        // 测试读取
        result, err := strategy.Read(context.Background(), "机器学习是什么", ReadOptions{TopK: 5})
        if err != nil {
            t.Fatalf("failed to read with semantic strategy: %v", err)
        }
        
        if !result.Hit {
            t.Error("expected cache hit for similar query")
        }
        
        if result.Score < 0.85 {
            t.Errorf("expected score >= 0.85, got %f", result.Score)
        }
        
        t.Logf("Semantic read/write test passed: hit=%v, score=%f", result.Hit, result.Score)
    })
    
    t.Run("SemanticStrategy returns miss for empty store", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        embeddingSvc := &mockEmbeddingService{dimension: 1536}
        vectorStore := &mockVectorStore{} // 空存储
        
        strategy.SetEmbeddingService(embeddingSvc)
        strategy.SetVectorStore(vectorStore)
        
        result, err := strategy.Read(context.Background(), "不存在的查询", ReadOptions{TopK: 5})
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        
        if result.Hit {
            t.Error("expected cache miss for empty store")
        }
        
        t.Logf("Semantic miss test passed: hit=%v", result.Hit)
    })
    
    t.Run("SemanticStrategy delete works correctly", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        vectorStore := &mockVectorStore{}
        
        strategy.SetVectorStore(vectorStore)
        
        // 先插入
        vectorStore.Insert(context.Background(), []storage.Vector{
            {ID: "delete-me", Vector: []float32{0.1, 0.2, 0.3}},
        })
        
        // 删除
        err := strategy.Delete(context.Background(), "delete-me")
        if err != nil {
            t.Fatalf("failed to delete: %v", err)
        }
        
        // 验证已删除
        vectors, _ := vectorStore.Get(context.Background(), []string{"delete-me"})
        if len(vectors) > 0 {
            t.Error("expected vector to be deleted")
        }
        
        t.Log("Semantic delete test passed")
    })
}

func TestHybridStrategy(t *testing.T) {
    // TODO: 添加测试
}

func TestRegistry(t *testing.T) {
    registry := GetRegistry()
    
    // 测试注册
    exactConfig := &ExactConfig{StorageName: "test"}
    exact := NewExactStrategy(exactConfig)
    if err := registry.Register(exact); err != nil {
        t.Errorf("failed to register exact strategy: %v", err)
    }
    
    // 测试获取
    strategy, err := registry.Get("exact")
    if err != nil {
        t.Errorf("failed to get exact strategy: %v", err)
    }
    if strategy.Name() != "exact" {
        t.Errorf("expected strategy name 'exact', got '%s'", strategy.Name())
    }
    
    // 测试重复注册
    if err := registry.Register(exact); err == nil {
        t.Error("expected error on duplicate registration")
    }
    
    // 测试获取不存在的策略
    _, err = registry.Get("nonexistent")
    if err == nil {
        t.Error("expected error for nonexistent strategy")
    }
}

func TestEntry(t *testing.T) {
    entry := &Entry{
        Key:       "test-key",
        Request:   "test request",
        Response:  "test response",
        Timestamp: time.Now(),
        ExpiresAt: time.Now().Add(time.Hour),
    }
    
    if entry.Key != "test-key" {
        t.Errorf("expected key 'test-key', got '%s'", entry.Key)
    }
}

func TestResult(t *testing.T) {
    result := &Result{
        Hit:     true,
        Content: "test content",
        Key:     "test-key",
        Score:   0.95,
    }
    
    if !result.Hit {
        t.Error("expected hit to be true")
    }
    if result.Score != 0.95 {
        t.Errorf("expected score 0.95, got %f", result.Score)
    }
}

func TestCacheStrategyPluginIntegration(t *testing.T) {
    t.Run("GetCacheStrategy returns correct strategy", func(t *testing.T) {
        registry := GetRegistry()
        
        // 注册语义策略
        semanticConfig := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        semantic := NewSemanticStrategy(semanticConfig)
        if err := registry.Register(semantic); err != nil {
            t.Logf("semantic strategy already registered: %v", err)
        }
        
        // 获取策略
        strategy, err := registry.Get("semantic")
        if err != nil {
            t.Fatalf("failed to get semantic strategy: %v", err)
        }
        
        if strategy.Name() != "semantic" {
            t.Errorf("expected strategy name 'semantic', got '%s'", strategy.Name())
        }
        
        if !strategy.SupportsSemantic() {
            t.Error("expected semantic strategy to support semantic matching")
        }
        
        t.Logf("Cache strategy plugin integration test passed: strategy=%s", strategy.Name())
    })
}

func TestSemanticStrategyFallback(t *testing.T) {
    t.Run("Read returns error when vector store is nil", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        strategy.SetEmbeddingService(&mockEmbeddingService{dimension: 1536})
        // 不设置 VectorStore
        
        _, err := strategy.Read(context.Background(), "test query", ReadOptions{})
        if err == nil {
            t.Error("expected error when vector store not initialized")
        }
        
        t.Logf("Fallback test passed: error=%v", err)
    })
    
    t.Run("Write returns error when vector store is nil", func(t *testing.T) {
        config := &SemanticConfig{
            StorageName:       "test",
            VectorStorageName: "test-vector",
            Threshold:         0.85,
            TopK:              5,
            Model:             "text-embedding-3-small",
        }
        
        strategy := NewSemanticStrategy(config)
        strategy.SetEmbeddingService(&mockEmbeddingService{dimension: 1536})
        // 不设置 VectorStore
        
        entry := &Entry{
            Key:       "test-key",
            Request:   "test request",
            Response:  "test response",
            Timestamp: time.Now(),
            ExpiresAt: time.Now().Add(time.Hour),
        }
        
        err := strategy.Write(context.Background(), entry, WriteOptions{})
        if err == nil {
            t.Error("expected error when vector store not initialized")
        }
        
        t.Logf("Fallback test passed: error=%v", err)
    })
}
