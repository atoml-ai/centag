# Elasticsearch 存储插件

Centag 的 Elasticsearch 统一存储插件，同时支持 KV 存储和向量存储。

## 特性

- ✅ **统一存储**: 单一存储系统同时支持 KV 和向量存储
- ✅ **高性能**: 原生 HNSW 索引，支持高效的向量搜索
- ✅ **灵活配置**: 支持精确匹配和语义匹配
- ✅ **TTL 管理**: 支持文档级别的过期时间
- ✅ **批处理**: 支持批量操作提升性能
- ✅ **监控完善**: 访问统计和健康检查

## 快速开始

### 1. 启动 Elasticsearch

```bash
./scripts/start-elasticsearch.sh
```

### 2. 配置存储

复制并修改配置文件：

```bash
cp archive/deprecated/configs/storage-elasticsearch.json.example archive/deprecated/configs/storage.json
```

### 3. 启动服务

```bash
./centag --config archive/deprecated/configs/config.yaml
```

## 配置示例

```json
{
  "storages": [
    {
      "name": "elasticsearch",
      "type": "elasticsearch",
      "enabled": true,
      "description": "Elasticsearch 统一存储",
      "config": {
        "addresses": ["http://localhost:29200"],
        "username": "elastic",
        "password": "changeme",
        "exact_index": "cache_exact_index",
        "semantic_index": "cache_semantic_index",
        "vector_dimension": 1536,
        "enable_tls": false,
        "enable_compress": true,
        "request_timeout": 30
      }
    }
  ],
  "default_kv": "elasticsearch",
  "default_vector": "elasticsearch"
}
```

## 性能对比

| 操作 | Redis + ChromaDB | Elasticsearch | 说明 |
|------|-----------------|---------------|------|
| KV Set | 50k QPS | 10k QPS | Redis 5x |
| KV Get | 100k QPS | 20k QPS | Redis 5x |
| Vector Search | 100 QPS | 120 QPS | ES 更快 |
| Vector Search P99 | 15ms | 10ms | ES 更快 |

**结论**: Elasticsearch 在向量搜索上更快，KV 虽然慢 5 倍但 20k QPS 仍然足够。

## 成本对比

| 指标 | Redis + ChromaDB | Elasticsearch | 节省 |
|------|-----------------|---------------|------|
| 节点数 | 6 | 3 | 50% |
| 内存 | 50GB | 16GB | 68% |
| 磁盘 | 350GB | 300GB | 14% |

**总体成本**: 节省 30-40%

## 文档

- **使用指南**: `docs/ElasticSearch插件使用指南.md`
- **集成指南**: `docs/ElasticSearch插件集成指南.md`
- **技术方案**: `docs/ElasticSearch缓存优化方案.md`

## 测试

```bash
# 运行测试
go test -v ./plugins/storage/elasticsearch/

# 运行性能测试
go test -bench=. -benchmem ./plugins/storage/elasticsearch/
```

## API

### KV Store

```go
Set(ctx, key, value, ttl) error
Get(ctx, key) (interface{}, error)
GetBytes(ctx, key) ([]byte, error)
Delete(ctx, key) error
Exists(ctx, key) (bool, error)
Expire(ctx, key, ttl) error
TTL(ctx, key) (time.Duration, error)
SetBatch(ctx, items, ttl) error
GetBatch(ctx, keys) (map[string]interface{}, error)
DeleteBatch(ctx, keys) error
Keys(ctx, pattern) ([]string, error)
FlushDB(ctx) error
```

### Vector Store

```go
Insert(ctx, vectors) error
Search(ctx, query, topK, filter) ([]SearchResult, error)
Delete(ctx, ids) error
Get(ctx, ids) ([]Vector, error)
Update(ctx, vectors) error
CreateCollection(ctx, collection, dimension) error
DropCollection(ctx, collection) error
CollectionExists(ctx, collection) (bool, error)
GetCollection(ctx, collection) (*CollectionInfo, error)
ListCollections(ctx) ([]string, error)
```

## 故障排查

### 连接失败

```bash
# 检查 Elasticsearch 是否运行
curl http://localhost:29200/_cluster/health

# 检查日志
./start.sh docker logs centag-es
```

### 索引不存在

插件会自动创建索引，如果失败可以手动创建：

```bash
curl -X PUT http://localhost:29200/cache_exact_index
curl -X PUT http://localhost:29200/cache_semantic_index
```

### 向量维度不匹配

检查配置中的 `vector_dimension` 是否与嵌入服务返回的向量维度一致。

## 许可证

与 Centag 主项目保持一致。
