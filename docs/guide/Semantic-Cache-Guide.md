# 语义缓存使用指南

## 概述

语义缓存是基于向量相似度的缓存机制,可以识别语义相似的查询,即使查询文本不完全相同也能命中缓存。相比精确匹配缓存,语义缓存可以显著提升缓存命中率。

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                    Cache Manager                      │
│                                                         │
│  ┌──────────────┐      ┌──────────────┐              │
│  │ Exact Cache  │      │Semantic Cache│              │
│  │  (Hash Match)│      │(Vector Match)│              │
│  └──────────────┘      └──────────────┘              │
│         │                     │                         │
│         └─────────┬───────────┘                         │
│                   │                                      │
│         ┌─────────▼───────────┐                         │
│         │   Embedding Service  │                         │
│         │  (OpenAI / Ollama) │                         │
│         └─────────────────────┘                         │
└─────────────────────────────────────────────────────────┘
```

## 功能特性

### 向量化服务

支持两种嵌入服务提供商:

#### 1. OpenAI Embedding API
- **优势**: 高质量、免部署、易集成
- **成本**: 调用成本较高
- **适用**: 生产环境、高精度需求
- **模型**:
  - `text-embedding-3-small`: 1536维,性价比高
  - `text-embedding-3-large`: 3072维,高质量
  - `text-embedding-ada-002`: 1536维,兼容性

#### 2. Ollama (本地)
- **优势**: 免费、本地部署、可控
- **成本**: 需要GPU资源
- **适用**: 开发环境、隐私要求高
- **模型**:
  - `nomic-embed-text`: 768维,轻量级
  - `all-minilm`: 轻量级,快速
  - `bge-m3`: 多语言,高质量

### 相似度计算

支持多种距离度量:

1. **余弦相似度 (Cosine)**
   - 范围: -1 到 1
   - 用途: 文本相似度
   - 推荐使用

2. **欧氏距离 (Euclidean)**
   - 范围: 0 到 ∞
   - 用途: 几何距离
   - 需要归一化

3. **点积 (Dot Product)**
   - 范围: -∞ 到 ∞
   - 用途: 已归一化向量
   - 计算最快

### 缓存策略

支持三种缓存策略:

1. **精确匹配 (Exact)**
   - 基于 Prompt Hash
   - 完全匹配才命中
   - 快速、可靠

2. **语义匹配 (Semantic)**
   - 基于向量相似度
   - 阈值可配置
   - 提升命中率

3. **混合策略 (Hybrid)**
   - 先精确匹配
   - 失败后语义匹配
   - 最佳性能

## 配置

### 1. 环境变量

```bash
# OpenAI API Key
export OPENAI_API_KEY=your_api_key_here

# Ollama 服务地址 (默认: http://localhost:21434)
export OLLAMA_BASE_URL=http://localhost:21434
```

### 2. 配置文件

编辑 `embedding.yaml`（旧版已归档至 `../../archive/deprecated/configs/embedding.yaml`）：

```yaml
# 使用 Ollama
provider: ollama
model:
  ollama:
    name: nomic-embed-text
    base_url: http://localhost:21434

# 使用 OpenAI
# provider: openai
# model:
#   openai:
#     name: text-embedding-3-small
#     base_url: https://api.openai.com/v1
#     api_key: ${OPENAI_API_KEY}
```

### 3. 语义缓存配置

```yaml
cache:
  # 相似度阈值 (0.7-0.95)
  threshold: 0.85

  # Top-K 检索 (1-10)
  top_k: 5

  # 距离类型 (cosine, euclidean)
  distance_type: cosine

  # 自动向量化
  auto_embedding: true
```

## 使用示例

### 1. 初始化 OpenAI 嵌入服务

```go
import (
    "centag/internal/embedding"
)

config := &embedding.EmbeddingConfig{
    Provider: "openai",
    Model:    "text-embedding-3-small",
    BaseURL:  "https://api.openai.com/v1",
    APIKey:   os.Getenv("OPENAI_API_KEY"),
}

service, err := embedding.NewOpenAIEmbeddingService(config)
if err != nil {
    log.Fatal(err)
}
```

### 2. 初始化 Ollama 嵌入服务

```go
import (
    "centag/internal/embedding"
)

config := &embedding.EmbeddingConfig{
    Provider: "ollama",
    Model:    "nomic-embed-text",
    BaseURL:  "http://localhost:21434",
}

service, err := embedding.NewOllamaEmbeddingService(config)
if err != nil {
    log.Fatal(err)
}
```

### 3. 初始化语义缓存

```go
import (
    "centag/internal/cache"
)

semanticConfig := &cache.SemanticCacheConfig{
    CacheConfig: cache.CacheConfig{
        Enabled:         true,
        DefaultTTL:      3600 * time.Second,
        CleanupInterval: 5 * time.Minute,
        MaxSize:         1000,
    },
    Threshold:           0.85,
    TopK:                 5,
    DistanceType:         "cosine",
    EnableAutoEmbedding:  true,
}

semanticCache, err := cache.NewSemanticCache(semanticConfig, embeddingService)
if err != nil {
    log.Fatal(err)
}
```

### 4. 配置缓存管理器

```go
import (
    "centag/internal/cache"
)

// 创建管理器
cacheConfig := &cache.CacheConfig{
    Enabled: true,
    DefaultTTL: 3600 * time.Second,
}

manager, err := cache.NewManager(cacheConfig)
if err != nil {
    log.Fatal(err)
}

// 设置语义缓存
manager.SetSemanticCache(semanticCache)

// 设置嵌入服务
manager.SetEmbeddingService(embeddingService)

// 设置语义缓存配置
manager.SetSemanticConfig(semanticConfig)

// 设置混合策略
manager.SetStrategy(cache.CacheStrategyHybrid)
```

### 5. 使用语义搜索

```go
// 写入缓存 (自动向量化)
entry := &cache.CacheEntry{
    Key:      "unique_key",
    Request:  "什么是人工智能?",
    Response: "人工智能是计算机科学的一个分支...",
    Metadata: map[string]interface{}{
        "model": "gpt-4",
    },
}

err := manager.Set(ctx, entry.Key, entry, 3600*time.Second)

// 精确匹配查询
entry, err := manager.Get(ctx, "unique_key")

// 语义搜索
entries, err := manager.SearchByQuery(ctx, "AI是什么?", 0.85, 5)
for _, e := range entries {
    similarity := e.Metadata["similarity_score"].(float32)
    fmt.Printf("相似度: %.2f, 答案: %s\n", similarity, e.Response)
}
```

## 性能优化

### 1. 批量向量化

```go
// 批量获取嵌入向量,减少API调用
texts := []string{"问题1", "问题2", "问题3"}
embeddings, err := embeddingService.GetBatchEmbeddings(ctx, texts)
```

### 2. 阈值调优

```yaml
# 高精度: 仅高相似度才匹配
threshold: 0.95

# 高召回: 更多匹配但可能降低质量
threshold: 0.75

# 平衡: 推荐值
threshold: 0.85
```

### 3. 缓存大小控制

```yaml
# 控制缓存大小,避免内存溢出
cache:
  max_size: 1000
  cleanup_interval: 300  # 5分钟清理一次
```

### 4. 模型选择

| 场景 | 推荐模型 | 维度 | 特点 |
|------|----------|------|------|
| 开发测试 | nomic-embed-text | 768 | 快速、小体积 |
| 生产环境 | text-embedding-3-small | 1536 | 高性价比 |
| 高质量 | text-embedding-3-large | 3072 | 最高质量 |
| 多语言 | bge-m3 | 1024 | 多语言支持 |

## 监控和统计

### 查看缓存统计

```go
stats := manager.Stats()
fmt.Printf("命中率: %.2f%%\n", stats.HitRate*100)
fmt.Printf("精确匹配命中: %d\n", stats.Hits[string(cache.CacheStrategyExact)])
fmt.Printf("语义匹配命中: %d\n", stats.Hits[string(cache.CacheStrategySemantic)])
```

### 监控指标

- **命中率**: 目标 > 70%
- **平均相似度**: 目标 > 0.85
- **向量化延迟**: 目标 < 100ms
- **缓存大小**: 监控内存使用

## 故障排查

### 1. 嵌入服务连接失败

```bash
# 检查 OpenAI API Key
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models

# 检查 Ollama 服务
curl http://localhost:21434/api/tags
```

### 2. 向量化超时

```yaml
# 增加超时时间
timeout: 60

# 或使用更快的模型
model:
  ollama:
    name: all-minilm  # 更小更快
```

### 3. 命中率过低

```yaml
# 降低阈值
threshold: 0.75

# 增加 Top-K
top_k: 10

# 检查向量质量
# 确保使用合适的嵌入模型
```

### 4. 内存占用过高

```yaml
# 减少缓存大小
max_size: 500

# 减少TTL
default_ttl: 1800

# 增加清理频率
cleanup_interval: 60
```

## 测试

### 运行测试脚本

```bash
# 功能测试
bash test/semantic-cache-test.sh

# 性能测试 (需要配置嵌入服务)
bash test/semantic-cache-perf-test.sh
```

### 手动测试

```bash
# 1. 启动服务
./start.sh

# 2. 发送测试请求
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "什么是人工智能?"}]
  }'

# 3. 发送语义相似的请求
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "AI是什么?"}]
  }'

# 4. 检查缓存统计
curl http://localhost:8080/api/cache/stats
```

## 最佳实践

### 1. 策略选择

- **精确匹配**: 适合精确查询、参数化场景
- **语义匹配**: 适合自然语言、问答场景
- **混合策略**: 通用场景,推荐使用

### 2. 阈值设置

- **开发环境**: 0.75-0.80 (高召回)
- **测试环境**: 0.80-0.85 (平衡)
- **生产环境**: 0.85-0.90 (高精度)

### 3. 模型选择

- **快速原型**: Ollama + nomic-embed-text
- **生产环境**: OpenAI + text-embedding-3-small
- **高质量**: OpenAI + text-embedding-3-large

### 4. 成本优化

- 使用批量向量化
- 合理设置缓存大小和TTL
- 优先精确匹配,缓存未命中再语义匹配

## 参考资料

- [OpenAI Embedding API](https://platform.openai.com/docs/guides/embeddings)
- [Ollama Embeddings](https://ollama.com/blog/embedding-models)
- [语义搜索最佳实践](https://www.pinecone.io/learn/semantic-search/)
- [向量数据库选型](https://www.anthropic.com/index/vector-search-how-it-works)
