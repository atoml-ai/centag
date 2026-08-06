package strategy

import (
    "context"
    "fmt"
    "centag/core/pkg/embedding"
    "centag/core/pkg/storage"
)

// SemanticStrategy 语义匹配策略
type SemanticStrategy struct {
    embeddingService embedding.EmbeddingService
    vectorStore      storage.VectorStore
    kvStore          storage.KVStore
    config           *SemanticConfig
}

type SemanticConfig struct {
    StorageName       string
    VectorStorageName string
    Threshold         float32  // 相似度阈值
    TopK              int      // 返回前K个
    Model             string   // 嵌入模型
}

// NewSemanticStrategy 创建语义匹配策略
func NewSemanticStrategy(config *SemanticConfig) *SemanticStrategy {
    return &SemanticStrategy{
        config: config,
    }
}

func (s *SemanticStrategy) Name() string {
    return "semantic"
}

func (s *SemanticStrategy) SupportsSemantic() bool {
    return true
}

func (s *SemanticStrategy) SetEmbeddingService(svc embedding.EmbeddingService) {
    s.embeddingService = svc
}

func (s *SemanticStrategy) SetVectorStore(store storage.VectorStore) {
    s.vectorStore = store
}

func (s *SemanticStrategy) SetKVStore(store storage.KVStore) {
    s.kvStore = store
}

func (s *SemanticStrategy) Configure(config map[string]interface{}) error {
    if storageName, ok := config["storage_name"].(string); ok {
        s.config.StorageName = storageName
    }
    if vectorStorageName, ok := config["vector_storage_name"].(string); ok {
        s.config.VectorStorageName = vectorStorageName
    }
    if threshold, ok := config["threshold"].(float64); ok {
        s.config.Threshold = float32(threshold)
    }
    if topK, ok := config["top_k"].(float64); ok {
        s.config.TopK = int(topK)
    }
    if model, ok := config["model"].(string); ok {
        s.config.Model = model
    }
    return nil
}

func (s *SemanticStrategy) Read(ctx context.Context, query string, opts ReadOptions) (*Result, error) {
    if s.embeddingService == nil {
        return nil, fmt.Errorf("EmbeddingService not initialized")
    }
    if s.vectorStore == nil {
        return nil, fmt.Errorf("VectorStore not initialized")
    }

    threshold := opts.Threshold
    if threshold <= 0 && s.config != nil {
        threshold = s.config.Threshold
    }
    if threshold <= 0 {
        threshold = 0.8
    }
    topK := opts.TopK
    if topK <= 0 && s.config != nil {
        topK = s.config.TopK
    }
    if topK <= 0 {
        topK = 5
    }
    
    // 1. 生成查询的嵌入向量
    queryVector, err := s.embeddingService.GetEmbedding(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }
    
    // 2. 搜索相似向量
    results, err := s.vectorStore.Search(ctx, queryVector, topK, nil)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }
    
    // 3. 选择最佳匹配（必须达到阈值）
    if len(results) == 0 {
        return &Result{Hit: false}, nil
    }
    
    best := results[0]
    if best.Score < threshold {
        return &Result{Hit: false, Score: float64(best.Score), Key: best.ID}, nil
    }
    
    // 4. 从 KVStore 获取完整响应
    if s.kvStore != nil {
        data, err := s.kvStore.GetBytes(ctx, best.ID)
        if err == nil && data != nil {
            return &Result{
                Hit:     true,
                Content: string(data),
                Key:     best.ID,
                Score:   float64(best.Score),
            }, nil
        }
    }
    
    return &Result{
        Hit:     true,
        Content: fmt.Sprintf("%v", best.Metadata["response"]),
        Key:     best.ID,
        Score:   float64(best.Score),
    }, nil
}

func (s *SemanticStrategy) Write(ctx context.Context, entry *Entry, opts WriteOptions) error {
    if s.embeddingService == nil {
        return fmt.Errorf("EmbeddingService not initialized")
    }
    if s.vectorStore == nil {
        return fmt.Errorf("VectorStore not initialized")
    }
    requestText := entry.Request
    if requestText == "" {
        return fmt.Errorf("semantic write requires non-empty Request (query text for embedding)")
    }
    
    // 1. 生成嵌入向量
    vector, err := s.embeddingService.GetEmbedding(ctx, requestText)
    if err != nil {
        return fmt.Errorf("failed to embed request: %w", err)
    }
    
    // 2. 存储到向量存储
    metadata := map[string]interface{}{
        "request":  requestText,
        "response": entry.Response,
        "expires":  entry.ExpiresAt,
    }
    
    err = s.vectorStore.Insert(ctx, []storage.Vector{{
        ID:      entry.Key,
        Vector:  vector,
        Metadata: metadata,
    }})
    if err != nil {
        return fmt.Errorf("vector store insert failed: %w", err)
    }
    
    // 3. 同步存储到 KVStore (可选)
    if s.kvStore != nil && opts.GenerateEmbedding {
        data := []byte(entry.Response)
        _ = s.kvStore.Set(ctx, entry.Key, data, opts.TTL)
    }
    
    return nil
}

func (s *SemanticStrategy) Delete(ctx context.Context, key string) error {
    if s.vectorStore != nil {
        _ = s.vectorStore.Delete(ctx, []string{key})
    }
    if s.kvStore != nil {
        _ = s.kvStore.Delete(ctx, key)
    }
    return nil
}
