package strategy

import (
    "context"
    "time"
)

// Strategy 缓存策略接口
type Strategy interface {
    // 基础信息
    Name() string                    // 策略名称
    SupportsSemantic() bool          // 是否支持语义搜索
    
    // 缓存操作
    Read(ctx context.Context, query string, opts ReadOptions) (*Result, error)
    Write(ctx context.Context, entry *Entry, opts WriteOptions) error
    Delete(ctx context.Context, key string) error
    
    // 配置
    Configure(config map[string]interface{}) error
}

// Result 缓存读取结果
type Result struct {
    Hit               bool
    Content           string
    Key               string
    Score             float64  // 相似度分数
    SourceStrategy    string   // 命中的策略来源 (exact/semantic)
}

// ReadOptions 读取选项
type ReadOptions struct {
    Threshold   float32  // 语义搜索阈值
    TopK        int      // 返回前K个结果
    StorageName string   // 存储后端名称
}

// WriteOptions 写入选项
type WriteOptions struct {
    TTL               time.Duration
    StorageName       string
    GenerateEmbedding bool     // 是否生成嵌入向量
}

// Entry 缓存条目
type Entry struct {
    Key       string
    Request   string
    Response  string
    Metadata  map[string]interface{}
    Timestamp time.Time
    ExpiresAt time.Time
}
