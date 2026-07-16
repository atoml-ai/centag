package storage

import (
	"context"
	"time"
)

// FullTextSearchStore 支持文本相似度检索的可选接口。
// 不是所有存储后端都需要实现此接口；语义缓存通过类型断言检测后端是否具备此能力。
//
// 当前实现：
//   - PostgreSQL：基于 pg_trgm 三元组相似度，天然 0-1 分数
//   - Elasticsearch：基于 BM25 全文检索，经 tanh(score/10) 归一化到 0-1
//
// 未实现该接口的后端（Redis、ChromaDB 等）会在类型断言时返回 false，
// 语义缓存自动降级到纯向量搜索，调用方无需感知。
type FullTextSearchStore interface {
	// SearchByText 基于文本相似度检索缓存候选集，无需 Embedding API 调用。
	// 返回的 Score 字段已归一化到 [0, 1]，各后端实现负责保证此约定。
	// query: 原始查询文本
	// topK: 最多返回条数
	// minScore: 最低归一化相似度阈值，低于此值的结果被过滤
	SearchByText(ctx context.Context, query string, topK int, minScore float32) ([]SearchResult, error)
}

// VectorStore 向量存储接口
type VectorStore interface {
	// Insert 插入向量
	Insert(ctx context.Context, vectors []Vector) error

	// Search 搜索最相似的向量
	Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]SearchResult, error)

	// Delete 删除向量
	Delete(ctx context.Context, ids []string) error

	// Get 获取向量
	Get(ctx context.Context, ids []string) ([]Vector, error)

	// Update 更新向量
	Update(ctx context.Context, vectors []Vector) error

	// CreateCollection 创建集合
	CreateCollection(ctx context.Context, collection string, dimension int) error

	// DropCollection 删除集合
	DropCollection(ctx context.Context, collection string) error

	// CollectionExists 检查集合是否存在
	CollectionExists(ctx context.Context, collection string) (bool, error)

	// GetCollection 获取集合信息
	GetCollection(ctx context.Context, collection string) (*CollectionInfo, error)

	// ListCollections 列出所有集合
	ListCollections(ctx context.Context) ([]string, error)

	// ListAll 列出集合中所有文档（用于缓存管理）。
	// 每个插件实现必须归一化返回 []VectorEntry，不能使用后端特有的字段结构。
	ListAll(ctx context.Context, collection string, limit int, offset int) ([]VectorEntry, int64, error)

	// GetStoreInfo 获取存储后端信息
	GetStoreInfo() StoreInfo

	// GetDefaultCollection 获取默认集合名称
	GetDefaultCollection() string

	// Close 关闭连接
	Close() error
}

// StoreInfo 存储后端信息
type StoreInfo struct {
	Type string // elasticsearch, milvus, qdrant, etc.
}

// VectorEntry ListAll 返回的标准化条目。
// 每个插件的 ListAll 实现必须将后端特有结构归一化到此结构，
// 确保调用方无需关心底层存储实现。
type VectorEntry struct {
	// ID 向量的唯一标识（即缓存 key）
	ID string `json:"id"`
	// Metadata 业务元数据，包含 request、response、model、timestamp 等
	Metadata map[string]interface{} `json:"metadata"`
	// CreatedAt 条目创建时间
	CreatedAt time.Time `json:"created_at"`
}

// Vector 向量数据
type Vector struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Score    float32                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CollectionInfo 集合信息
type CollectionInfo struct {
	Name       string `json:"name"`
	Dimension  int    `json:"dimension"`
	Count      int64  `json:"count"`
	IndexType  string `json:"index_type"`
	MetricType string `json:"metric_type"`
}

// MetricType 距离度量类型
type MetricType string

const (
	MetricTypeL2       MetricType = "L2"        // 欧氏距离
	MetricTypeIP       MetricType = "IP"        // 内积
	MetricTypeCosine   MetricType = "COSINE"    // 余弦相似度
	MetricTypeHamming  MetricType = "HAMMING"   // 汉明距离
	MetricTypeJaccard  MetricType = "JACCARD"   // Jaccard距离
)

// IndexType 索引类型
type IndexType string

const (
	IndexTypeFlat       IndexType = "FLAT"       // 暴力搜索
	IndexTypeIVFFlat    IndexType = "IVFFLAT"    // IVFFlat索引
	IndexTypeIVFPQ      IndexType = "IVFPQ"      // IVFPQ索引
	IndexTypeHNSW       IndexType = "HNSW"       // HNSW索引
	IndexTypeDISKANN    IndexType = "DISKANN"    // DiskANN索引
)
