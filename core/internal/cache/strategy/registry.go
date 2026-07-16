package strategy

import (
	"fmt"
	"centag/core/pkg/embedding"
	"centag/core/pkg/storage"
)

// Registry 策略注册表
type Registry struct {
	strategies map[string]Strategy
}

var globalRegistry *Registry

func init() {
	globalRegistry = &Registry{
		strategies: make(map[string]Strategy),
	}
	// 注意: 内置策略需要在 storage/embedding 就绪后由 server.go 调用 RegisterBuiltinStrategies 注册
}

// GetRegistry 获取全局注册表
func GetRegistry() *Registry {
	return globalRegistry
}

// Register 注册策略
func (r *Registry) Register(strategy Strategy) error {
	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	name := strategy.Name()
	if _, exists := r.strategies[name]; exists {
		return fmt.Errorf("strategy '%s' already registered", name)
	}

	r.strategies[name] = strategy
	return nil
}

// Get 获取策略
func (r *Registry) Get(name string) (Strategy, error) {
	strategy, exists := r.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy '%s' not found", name)
	}
	return strategy, nil
}

// ListAll 列出所有策略
func (r *Registry) ListAll() []string {
	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		names = append(names, name)
	}
	return names
}

// RegisterBuiltinStrategies 注册内置策略（精确/语义/混合）
// 需要在 storage/embedding 就绪后调用（通常在 server.go 初始化阶段）
func (r *Registry) RegisterBuiltinStrategies(
	kvStore storage.KVStore,
	vectorStore storage.VectorStore,
	embeddingService embedding.EmbeddingService,
) error {
	// 注册精确匹配策略
	exactConfig := &ExactConfig{
		StorageName: "default-kv",
	}
	exactStrategy := NewExactStrategy(exactConfig)
	exactStrategy.SetKVStore(kvStore)

	if err := r.Register(exactStrategy); err != nil {
		return fmt.Errorf("failed to register exact strategy: %w", err)
	}

	// 注册语义匹配策略
	semanticConfig := &SemanticConfig{
		StorageName:       "default-kv",
		VectorStorageName: "default-vector",
		Threshold:         0.85,
		TopK:              5,
		Model:             "text-embedding-3-small",
	}
	semanticStrategy := NewSemanticStrategy(semanticConfig)
	semanticStrategy.SetKVStore(kvStore)
	semanticStrategy.SetVectorStore(vectorStore)
	semanticStrategy.SetEmbeddingService(embeddingService)

	if err := r.Register(semanticStrategy); err != nil {
		return fmt.Errorf("failed to register semantic strategy: %w", err)
	}

	// 注册混合策略
	hybridStrategy := NewHybridStrategy(exactStrategy, semanticStrategy)
	if err := r.Register(hybridStrategy); err != nil {
		return fmt.Errorf("failed to register hybrid strategy: %w", err)
	}

	return nil
}
