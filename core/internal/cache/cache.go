package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	evalplugin "centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/storage"
	"centag/core/pkg/processor"
)

// UnmarshalCacheEntry attempts to deserialize a cache entry using multiple strategies:
// 1. Direct JSON
// 2. Base64-encoded JSON (JSON string wrapper)
// 3. Raw base64-encoded JSON
func UnmarshalCacheEntry(data []byte) (*CacheEntry, error) {
	var lastErr error

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err == nil {
		return &entry, nil
	} else {
		lastErr = fmt.Errorf("direct unmarshal failed: %w", err)
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		decoded, err2 := base64.StdEncoding.DecodeString(strings.TrimSpace(str))
		if err2 == nil {
			if err3 := json.Unmarshal(decoded, &entry); err3 == nil {
				return &entry, nil
			} else {
				lastErr = fmt.Errorf("unmarshal after base64 decode failed: %w", err3)
			}
		} else {
			lastErr = fmt.Errorf("base64 decode failed: %w", err2)
		}
	} else {
		lastErr = fmt.Errorf("unmarshal as string failed: %w", err)
	}

	trimmedData := strings.TrimSpace(string(data))
	decoded, err4 := base64.StdEncoding.DecodeString(trimmedData)
	if err4 == nil {
		err5 := json.Unmarshal(decoded, &entry)
		if err5 == nil {
			return &entry, nil
		}
		lastErr = fmt.Errorf("unmarshal after direct base64 decode failed: %w", err5)
	} else {
		lastErr = fmt.Errorf("direct base64 decode failed: %w", err4)
	}

	return nil, fmt.Errorf("failed to unmarshal cache entry from data (length: %d): %w", len(data), lastErr)
}

// EvaluationResult 评估结果
// 定义在cache包以便被CacheManager接口引用
type EvaluationResult struct {
	ShouldCache bool                   `json:"should_cache"`
	Score       float64                `json:"score"`
	Labels      []string               `json:"labels"`
	Details     map[string]interface{} `json:"details"`
}

// Cache 缓存接口
type Cache interface {
	// Get 获取缓存
	Get(ctx context.Context, key string) (*CacheEntry, error)

	// Set 设置缓存
	Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error

	// Delete 删除缓存
	Delete(ctx context.Context, key string) error

	// Exists 检查缓存是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// Clear 清空所有缓存
	Clear(ctx context.Context) error

	// Stats 获取缓存统计
	Stats(ctx context.Context) (*CacheStats, error)
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key            string                 `json:"key"`
	Request        string                 `json:"request"`    // 原始请求(用于日志和调试)
	Response       string                 `json:"response"`   // 响应内容(非流式)或合并后的完整响应(流式)
	Metadata       map[string]interface{} `json:"metadata"`   // 元数据(如模型名称、参数等)
	Timestamp      time.Time              `json:"timestamp"`  // 创建时间
	ExpiresAt      time.Time              `json:"expires_at"` // 过期时间
	TokensUsed     int                    `json:"tokens_used"` // 消耗的Token数
	StorageBackend string                 `json:"storage_backend,omitempty"` // 存储后端名称

	// 流式请求相关字段
	IsStream   bool                   `json:"is_stream"`   // 是否为流式请求
	StreamData []StreamChunk          `json:"stream_data"` // 流式分块数据(可选)
}

// StreamChunk 流式数据块
type StreamChunk struct {
	Content      string `json:"content"`       // 内容片段
	Done         bool   `json:"done"`          // 是否完成
	FinishReason string `json:"finish_reason"` // 完成原因
}

// CacheStats 缓存统计(引用自metrics.go)
// 已移至 internal/cache/metrics.go

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled    bool          `json:"enabled"`     // 是否启用缓存
	DefaultTTL time.Duration `json:"default_ttl"` // 默认TTL
	MaxSize    int64         `json:"max_size"`    // 最大缓存条目数(0表示无限制)
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
}

// ExactCache 精确匹配缓存(基于Prompt Hash)
type ExactCache interface {
	Cache
}

// SemanticCache 语义缓存(基于向量相似度)
type SemanticCache interface {
	Cache
	// SearchByQuery 根据查询搜索相似缓存
	SearchByQuery(ctx context.Context, query string, threshold float32, topK int) ([]*CacheEntry, error)
	// InsertWithEmbedding 使用嵌入向量插入缓存
	InsertWithEmbedding(ctx context.Context, entry *CacheEntry, embedding []float32) error
}

// CacheKeyGenerator 缓存键生成器
type CacheKeyGenerator interface {
	// GenerateKey 生成缓存键
	GenerateKey(request interface{}) (string, error)
}

// CacheStrategy 缓存策略
type CacheStrategy string

const (
	// CacheStrategyExact 精确匹配策略
	CacheStrategyExact CacheStrategy = "exact"
	// CacheStrategySemantic 语义匹配策略
	CacheStrategySemantic CacheStrategy = "semantic"
	// CacheStrategyHybrid 混合策略(先精确,再语义)
	CacheStrategyHybrid CacheStrategy = "hybrid"
)

// CacheManager 缓存管理器
type CacheManager interface {
	// Get 获取缓存
	Get(ctx context.Context, key string) (*CacheEntry, error)
	// Set 设置缓存
	Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error
	// Delete 删除缓存
	Delete(ctx context.Context, key string) error
	// Clear 清空缓存
	Clear(ctx context.Context) error
	// Stats 获取统计
	Stats() *CacheStats
	// Hit 记录缓存命中
	Hit()
	// Miss 记录缓存未命中
	Miss()

	// GetQASplitter 获取QA拆分器（用于流水线QA拆分）
	GetQASplitter() *processor.QASplitter
	// GetSemanticCache 获取语义缓存对象
	GetSemanticCache() *SemanticCacheImpl
	// GetSemanticCacheStore 获取语义缓存存储（用于流水线QA拆分存储）
	GetSemanticCacheStore() storage.KVStore
	// ShouldEvaluateCache 检查是否需要评估缓存
	ShouldEvaluateCache() bool
	// EvaluateCacheEntry 评估缓存条目是否值得缓存
	EvaluateCacheEntry(ctx context.Context, question, answer string, historyMessages []evalplugin.Message) (*EvaluationResult, error)
}

// SemanticCacheInterface 语义缓存接口(避免与结构体名冲突)
type SemanticCacheInterface interface {
	Cache
	// SearchByQuery 根据查询搜索相似缓存
	SearchByQuery(ctx context.Context, query string, threshold float32, topK int) ([]*CacheEntry, error)
	// InsertWithEmbedding 使用嵌入向量插入缓存
	InsertWithEmbedding(ctx context.Context, entry *CacheEntry, embedding []float32) error
}
