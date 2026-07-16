# PostgreSQL 存储插件

PostgreSQL 存储插件为 centag 提供统一的 KV 存储和向量存储能力，基于 PostgreSQL 和 pgvector 扩展实现。

## 功能特性

### 1. KV 存储
- ✅ 支持 JSONB 格式的值存储
- ✅ 支持 TTL 过期时间
- ✅ 批量操作支持
- ✅ 自动更新时间戳

### 2. 向量存储
- ✅ 支持 1024 维向量（可配置）
- ✅ 使用 HNSW 索引实现高性能检索
- ✅ 支持 Cosine 相似度计算
- ✅ 支持元数据过滤
- ✅ 批量插入和查询

### 3. RAG 文档存储（预留）
- ⏳ 预留接口，支持未来扩展

## 快速开始

### 1. 安装依赖

```bash
go get github.com/jackc/pgx/v5
go get github.com/pgvector/pgvector-go
```

### 2. 配置存储

在 `config.yaml` 中添加 PostgreSQL 存储配置：

```yaml
storages:
  - name: postgresql-main
    type: postgresql
    enabled: true
    config:
      host: localhost
      port: 5432
      user: postgres
      password: your_password
      database: centag
      ssl_mode: disable
      max_conn_lifetime: 3600
      max_conn_idle_time: 600
      max_conns: 20
      min_conns: 5
      kv_table: kv_cache
      vector_table: vector_cache
      vector_dimension: 1024
      index_type: hnsw
    description: PostgreSQL存储配置
    is_default: true

default_storage: postgresql-main
```

### 3. 启动 PostgreSQL

```bash
# 使用 Docker Compose 启动
cd docker
docker-compose up -d postgresql

# 查看日志
docker-compose logs -f postgresql
```

### 4. 初始化数据库

数据库会在首次启动时自动初始化，包括：
- 创建所有必要的表
- 创建向量索引
- 创建触发器和函数

库表结构由 **`cmd/migrate`** 迁移维护；勿再使用已移除的 `docker/postgresql/` 目录。

## 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| host | string | localhost | 数据库主机 |
| port | int | 5432 | 数据库端口 |
| user | string | postgres | 数据库用户名 |
| password | string | - | 数据库密码（必须设置） |
| database | string | centag | 数据库名称 |
| ssl_mode | string | disable | SSL 模式：disable/allow/prefer/require |
| max_conn_lifetime | int | 3600 | 连接最大生命周期（秒） |
| max_conn_idle_time | int | 600 | 连接最大空闲时间（秒） |
| max_conns | int | 20 | 最大连接数 |
| min_conns | int | 5 | 最小连接数 |
| kv_table | string | kv_cache | KV 存储表名 |
| vector_table | string | vector_cache | 向量存储表名 |
| vector_dimension | int | 1024 | 向量维度 |
| index_type | string | hnsw | 索引类型：hnsw/ivfflat |

## API 使用示例

### KV 存储

```go
// 获取 KVStore
kvStore, err := manager.GetKVStore("postgresql-main")

// 设置 KV
err = kvStore.Set(ctx, "mykey", map[string]interface{}{
    "value": "data",
    "ttl": 3600,
}, time.Hour)

// 获取 KV
value, err := kvStore.Get(ctx, "mykey")

// 批量设置
items := map[string]interface{}{
    "key1": "value1",
    "key2": "value2",
}
err = kvStore.SetBatch(ctx, items, time.Hour)

// 批量获取
keys := []string{"key1", "key2"}
values, err := kvStore.GetBatch(ctx, keys)
```

### 向量存储

```go
// 获取 VectorStore
vectorStore, err := manager.GetVectorStore("postgresql-main")

// 插入向量
vectors := []storage.Vector{
    {
        ID:        "doc1",
        Embedding: []float32{0.1, 0.2, 0.3, ...},
        Metadata:  map[string]interface{}{"category": "tech"},
    },
}
err = vectorStore.Insert(ctx, vectors)

// 搜索向量
query := []float32{0.1, 0.2, 0.3, ...}
results, err := vectorStore.Search(ctx, query, 10, map[string]interface{}{
    "category": "tech",
})

// 获取向量
vectors, err := vectorStore.Get(ctx, []string{"doc1"})
```

## 数据库表结构

### kv_cache
KV 缓存表，用于精确匹配缓存。

| 字段 | 类型 | 说明 |
|------|------|------|
| key | VARCHAR(512) | 缓存键（主键） |
| value | JSONB | 缓存值 |
| expires_at | TIMESTAMPTZ | 过期时间（NULL = 永不过期） |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |

### vector_cache
向量存储表，用于语义匹配缓存。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(512) | 向量 ID（主键） |
| vector | vector(1024) | 向量数据 |
| metadata | JSONB | 元数据 |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |

### rag_documents
RAG 文档存储表（预留）。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(512) | 文档 ID（主键） |
| content | TEXT | 文档内容 |
| embedding | vector(1024) | 文档向量 |
| metadata | JSONB | 元数据 |
| chunk_id | VARCHAR(512) | 分块 ID |
| chunk_index | INTEGER | 分块索引 |
| document_id | VARCHAR(512) | 原始文档 ID |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |

## 性能优化

### 1. 连接池配置

```yaml
config:
  max_conns: 20        # 最大连接数（推荐：20-50）
  min_conns: 5         # 最小连接数（推荐：5-10）
  max_conn_lifetime: 3600   # 连接最大生命周期（秒）
  max_conn_idle_time: 600    # 连接最大空闲时间（秒）
```

### 2. 索引优化

HNSW 索引参数在 `schema.sql` 中配置：

```sql
CREATE INDEX ... USING hnsw (vector vector_cosine_ops)
WITH (m = 16, ef_construction = 200);
```

- **m**: 每个节点的连接数（默认 16）
  - 更大 = 更好性能，更多内存
  - 推荐范围：16-32

- **ef_construction**: 构建时的候选列表大小（默认 200）
  - 更大 = 更好索引质量，更慢构建
  - 推荐范围：200-400

### 3. 查询优化

- 使用批量操作减少网络往返
- 合理设置 topK 参数
- 利用元数据过滤减少搜索范围

## 维护操作

### 清理过期数据

```sql
-- 清理过期的 KV 数据
SELECT cleanup_expired_kv();
```

### 查看统计信息

```sql
-- KV 缓存统计
SELECT * FROM get_kv_cache_stats();

-- 向量缓存统计
SELECT * FROM get_vector_cache_stats();

```

### 更新统计信息

```sql
ANALYZE kv_cache;
ANALYZE vector_cache;
```

## 测试连接

### 前端测试

1. 打开存储管理页面
2. 点击"添加存储"
3. 选择类型"PostgreSQL"
4. 填写连接信息
5. 点击"测试连接"按钮

### 后端测试

```bash
# 使用 psql 测试连接
docker-compose exec postgresql psql -U postgres -d centag

# 测试 pgvector 扩展
SELECT vector_dims('[1,2,3]'::vector);

# 测试向量相似度
SELECT 1 - (vector('[1,2,3]') <=> vector('[1,2,3.1]')) as similarity;
```

## 故障排查

### 连接失败

1. 检查 PostgreSQL 服务是否运行
   ```bash
   docker-compose ps postgresql
   ```

2. 检查网络连接
   ```bash
   docker-compose exec postgresql pg_isready -U postgres
   ```

3. 检查日志
   ```bash
   docker-compose logs postgresql
   ```

### 索引性能问题

```sql
-- 检查索引使用情况
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read
FROM pg_stat_user_indexes
WHERE tablename IN ('kv_cache', 'vector_cache');
```

### 内存占用高

```sql
-- 查看表大小
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size(tablename::regclass)) as size
FROM pg_tables
WHERE tablename IN ('kv_cache', 'vector_cache');
```

## 与其他存储的对比

| 特性 | PostgreSQL | Elasticsearch | Redis |
|------|-----------|---------------|-------|
| KV 存储 | ✅ JSONB | ✅ JSON | ✅ String/Hash |
| 向量存储 | ✅ pgvector | ✅ Dense Vector | ❌ |
| ACID 事务 | ✅ | ❌ | ❌ |
| 复杂查询 | ✅ SQL | ✅ DSL | ❌ |
| 全文检索 | ✅ GIN | ✅ Inverted Index | ❌ |
| 部署复杂度 | 中 | 高 | 低 |
| 学习成本 | 低 | 中 | 低 |
| 资源占用 | 中 | 高 | 低 |
| 单一数据库 | ✅ | ✅ | ❌ |

## 许可证

本插件遵循项目主许可证。
