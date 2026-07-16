package storage

import (
	"context"
	"time"
)

// KVStore 键值存储接口
type KVStore interface {
	// Set 设置键值对
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Get 获取值
	Get(ctx context.Context, key string) (interface{}, error)

	// GetBytes 获取字节值
	GetBytes(ctx context.Context, key string) ([]byte, error)

	// GetString 获取字符串值
	GetString(ctx context.Context, key string) (string, error)

	// Delete 删除键
	Delete(ctx context.Context, key string) error

	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// Expire 设置过期时间
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL 获取剩余过期时间
	TTL(ctx context.Context, key string) (time.Duration, error)

	// SetBatch 批量设置
	SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// GetBatch 批量获取
	GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error)

	// DeleteBatch 批量删除
	DeleteBatch(ctx context.Context, keys []string) error

	// Keys 获取匹配的键
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Count 获取键的总数
	Count(ctx context.Context, pattern string) (int64, error)

	// GetAll 获取所有键值对(包括已过期的)
	GetAll(ctx context.Context, pattern string) (map[string][]byte, error)

	// FlushDB 清空数据库
	FlushDB(ctx context.Context) error

	// Close 关闭连接
	Close() error

	// GetStoreInfo 获取存储后端信息
	GetStoreInfo() StoreInfo
}

// KVItem 键值项
type KVItem struct {
	Key   string
	Value interface{}
	TTL   time.Duration
}
