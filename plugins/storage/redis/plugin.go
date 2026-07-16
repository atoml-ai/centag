package redis

import (
	"context"
	"fmt"

	"centag/core/pkg/storage"
)

// Plugin Redis存储插件
type Plugin struct {
	storage *Storage
	config  *Config
}

// NewPlugin 创建Redis插件
func NewPlugin(config *Config) (*Plugin, error) {
	storageInstance, err := New(config)
	if err != nil {
		return nil, err
	}

	return &Plugin{
		storage: storageInstance,
		config:  config,
	}, nil
}

// StorageType 返回存储类型
func (p *Plugin) StorageType() storage.StorageType {
	return storage.StorageTypeRedis
}

// HealthCheck 健康检查
func (p *Plugin) HealthCheck(ctx context.Context) error {
	if p.storage == nil {
		return fmt.Errorf("redis storage not initialized")
	}
	return p.storage.HealthCheck(ctx)
}

// DetailedHealthCheck 详细健康检查
func (p *Plugin) DetailedHealthCheck(ctx context.Context) (*HealthStatus, error) {
	if p.storage == nil {
		return nil, fmt.Errorf("redis storage not initialized")
	}
	return p.storage.DetailedHealthCheck(ctx)
}

// GetPoolStatus 获取连接池状态
func (p *Plugin) GetPoolStatus(ctx context.Context) (*PoolStatus, error) {
	if p.storage == nil {
		return nil, fmt.Errorf("redis storage not initialized")
	}
	return p.storage.GetPoolStatus(ctx)
}

// Stats 获取统计信息
func (p *Plugin) Stats(ctx context.Context) (*Stats, error) {
	if p.storage == nil {
		return nil, fmt.Errorf("redis storage not initialized")
	}
	return p.storage.Stats(ctx)
}

// KVStore 获取KV存储
func (p *Plugin) KVStore() (storage.KVStore, error) {
	if p.storage == nil {
		return nil, fmt.Errorf("redis storage not initialized")
	}
	return p.storage, nil
}

// VectorStore 获取向量存储
func (p *Plugin) VectorStore() (storage.VectorStore, error) {
	return p.storage.VectorStore()
}

// KnowledgeStore 获取知识库存储（Redis 不支持知识库存储）
func (p *Plugin) KnowledgeStore() (storage.KnowledgeDataStore, error) {
	return nil, fmt.Errorf("Redis does not support knowledge storage")
}

// GetDefaultConfig 获取默认配置
func (p *Plugin) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"addr":            "localhost:26379",
		"password":         "",
		"db":              0,
		"pool_size":       10,
		"min_idle_conns":  2,
		"max_retries":     3,
		"dial_timeout":    5,
		"read_timeout":    3,
		"write_timeout":   3,
		"pool_timeout":    4,
	}
}

// init 在包初始化时注册插件工厂
func init() {
	storage.RegisterPlugin(storage.StorageTypeRedis, func(config map[string]interface{}) (storage.StoragePlugin, error) {
		redisConfig := &Config{
			Addr:         getStr(config, "addr", "localhost:26379"),
			Password:     getStr(config, "password", ""),
			DB:           getInt(config, "db", 0),
			PoolSize:     getInt(config, "pool_size", 10),
			MinIdleConns: getInt(config, "min_idle_conns", 2),
			MaxRetries:   getInt(config, "max_retries", 3),
			DialTimeout:  getInt(config, "dial_timeout", 5),
			ReadTimeout:  getInt(config, "read_timeout", 3),
			WriteTimeout: getInt(config, "write_timeout", 3),
			PoolTimeout:  getInt(config, "pool_timeout", 4),
		}
		return NewPlugin(redisConfig)
	})
}

func getStr(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getInt(m map[string]interface{}, key string, defaultValue int) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}
