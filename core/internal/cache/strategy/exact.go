package strategy

import (
    "context"
    "fmt"
    "centag/core/pkg/storage"
)

// ExactStrategy 精确匹配策略
type ExactStrategy struct {
    kvStore storage.KVStore
    config  *ExactConfig
}

type ExactConfig struct {
    StorageName string
}

// NewExactStrategy 创建精确匹配策略
func NewExactStrategy(config *ExactConfig) *ExactStrategy {
    return &ExactStrategy{
        config: config,
    }
}

func (s *ExactStrategy) Name() string {
    return "exact"
}

func (s *ExactStrategy) SupportsSemantic() bool {
    return false
}

func (s *ExactStrategy) SetKVStore(store storage.KVStore) {
    s.kvStore = store
}

func (s *ExactStrategy) Configure(config map[string]interface{}) error {
    if storageName, ok := config["storage_name"].(string); ok {
        s.config.StorageName = storageName
    }
    return nil
}

func (s *ExactStrategy) Read(ctx context.Context, query string, opts ReadOptions) (*Result, error) {
    if s.kvStore == nil {
        return nil, fmt.Errorf("KVStore not initialized")
    }
    
    data, err := s.kvStore.GetBytes(ctx, query)
    if err != nil {
        return &Result{Hit: false}, nil
    }
    
    return &Result{
        Hit:     true,
        Content: string(data),
        Key:     query,
    }, nil
}

func (s *ExactStrategy) Write(ctx context.Context, entry *Entry, opts WriteOptions) error {
    if s.kvStore == nil {
        return fmt.Errorf("KVStore not initialized")
    }
    
    data := []byte(entry.Response)
    return s.kvStore.Set(ctx, entry.Key, data, opts.TTL)
}

func (s *ExactStrategy) Delete(ctx context.Context, key string) error {
    if s.kvStore == nil {
        return fmt.Errorf("KVStore not initialized")
    }
    
    return s.kvStore.Delete(ctx, key)
}
