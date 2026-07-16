package pipeline

import (
	"context"
	"fmt"

	"centag/core/pkg/storage"
)

// StorageManagerAdapter 将 storage.Manager 适配为 StorageProvider
type StorageManagerAdapter struct {
	manager *storage.Manager
}

// NewStorageManagerAdapter 创建存储管理器适配器
func NewStorageManagerAdapter(manager *storage.Manager) *StorageManagerAdapter {
	return &StorageManagerAdapter{manager: manager}
}

// GetStorage 根据命名空间获取存储（适配为 pipeline.Storage 接口）
func (a *StorageManagerAdapter) GetStorage(namespace string) (Storage, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("storage manager not configured")
	}

	// 获取存储插件
	plugin, err := a.manager.GetStorage(namespace)
	if err != nil {
		return nil, fmt.Errorf("get storage %s failed: %w", namespace, err)
	}

	// 获取 KVStore
	kvStore, err := plugin.KVStore()
	if err != nil {
		return nil, fmt.Errorf("get KVStore from storage %s failed: %w", namespace, err)
	}

	return &kvStoreAdapter{kvStore: kvStore}, nil
}

// kvStoreAdapter 将 storage.KVStore 适配为 pipeline.Storage
type kvStoreAdapter struct {
	kvStore storage.KVStore
}

func (a *kvStoreAdapter) Read(ctx context.Context, key string) ([]byte, error) {
	return a.kvStore.GetBytes(ctx, key)
}

func (a *kvStoreAdapter) Write(ctx context.Context, key string, value []byte) error {
	return a.kvStore.Set(ctx, key, value, 0) // 0 表示不过期
}

func (a *kvStoreAdapter) Delete(ctx context.Context, key string) error {
	return a.kvStore.Delete(ctx, key)
}
