package storage

import (
	"context"
	"time"
)

// DataType 数据类型
type DataType string

const (
	DataTypeKV        DataType = "kv"        // 键值存储
	DataTypeVector    DataType = "vector"    // 向量存储
	DataTypeKnowledge DataType = "knowledge" // 知识库存储
)

// DataDocument 数据文档（统一表示）
type DataDocument struct {
	ID         string                 `json:"id"`
	Key        string                 `json:"key,omitempty"`        // KV 存储的键
	Content    string                 `json:"content"`              // 文档内容
	Metadata   map[string]interface{} `json:"metadata"`             // 元数据
	Embedding  []float32              `json:"embedding,omitempty"`  // 向量（向量存储和知识库使用）
	Collection string                 `json:"collection,omitempty"` // 集合/命名空间
	DataType   DataType               `json:"data_type"`            // 数据类型
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// DataQuery 数据查询（统一表示）
type DataQuery struct {
	Query      string                 `json:"query"`                // 查询内容
	Key        string                 `json:"key,omitempty"`        // KV 查询的键
	Collection string                 `json:"collection,omitempty"` // 集合/命名空间
	TopK       int                    `json:"top_k,omitempty"`      // 向量/知识库查询的 TopK
	Threshold  float32                `json:"threshold,omitempty"`  // 向量/知识库查询的阈值
	Filter     map[string]interface{} `json:"filter,omitempty"`     // 过滤条件
	DataType   DataType               `json:"data_type"`            // 数据类型
}

// DataResult 数据查询结果
type DataResult struct {
	Document *DataDocument `json:"document"`
	Score    float32       `json:"score"` // 相似度分数（向量/知识库查询）
}

// DataStore 基础数据存储接口（通用方法）
type DataStore interface {
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭连接
	Close() error

	// GetStoreInfo 获取存储信息
	GetStoreInfo() StoreInfo

	// GetDataType 获取支持的数据类型
	GetDataType() DataType
}

// KVDataStore KV 数据存储接口（精确匹配）
type KVDataStore interface {
	DataStore

	// Set 设置键值对
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Get 获取值
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete 删除键
	Delete(ctx context.Context, key string) error

	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)
}

// VectorDataStore 向量数据存储接口（语义搜索）
type VectorDataStore interface {
	DataStore

	// Store 存储向量数据
	Store(ctx context.Context, doc *DataDocument) error

	// Search 搜索相似向量
	Search(ctx context.Context, query []float32, topK int, threshold float32) ([]*DataResult, error)

	// Delete 删除向量数据
	DeleteVector(ctx context.Context, docID string) error

	// ListCollections 列出所有集合
	ListCollections(ctx context.Context) ([]string, error)
}

// KnowledgeDataStore 知识库存储接口（混合检索）
type KnowledgeDataStore interface {
	DataStore

	// Store 存储知识文档
	Store(ctx context.Context, doc *DataDocument) error

	// Retrieve 检索相关文档
	Retrieve(ctx context.Context, query string, topK int, filter map[string]interface{}) ([]*DataResult, error)

	// Delete 删除知识文档
	DeleteKnowledge(ctx context.Context, docID string) error

	// ListCollections 列出所有集合
	ListCollections(ctx context.Context) ([]string, error)
}
