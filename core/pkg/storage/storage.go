package storage

import (
	"context"
)

// StoragePlugin 存储插件接口（由插件系统实现）
type StoragePlugin interface {
	KVStore() (KVStore, error)
	VectorStore() (VectorStore, error)
	KnowledgeStore() (KnowledgeDataStore, error)
	StorageType() StorageType
	HealthCheck(ctx context.Context) error
	// GetDefaultConfig 获取默认配置（由插件实现，Server层调用）
	GetDefaultConfig() map[string]interface{}
}

// StorageType 存储类型
type StorageType string

const (
	StorageTypeRedis       StorageType = "redis"
	StorageTypeMilvus      StorageType = "milvus"
	StorageTypeChroma      StorageType = "chroma"
	StorageTypePostgresql  StorageType = "postgresql"
	StorageTypeElasticsearch StorageType = "elasticsearch"
	StorageTypeFile          StorageType = "file"
)

// StorageConfigItem 存储配置项
type StorageConfigItem struct {
	Name        string                 `json:"name"`
	Type        StorageType            `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Description string                 `json:"description"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Storages  []StorageConfigItem `json:"storages"`
	DefaultKV string              `json:"default_kv"`
}
