# PostgreSQL 存储初始化说明

本目录包含 PostgreSQL 数据库的初始化配置和说明文档。

## 快速开始

### 1. 启动 PostgreSQL 服务

```bash
cd /path/to/centag/docker

# 使用 Docker Compose 启动
docker-compose up -d postgresql

# 或使用 profile 启动
docker-compose --profile postgresql up -d
```

### 2. 查看服务状态

```bash
# 检查容器状态
docker-compose ps postgresql

# 查看日志
docker-compose logs -f postgresql
```

### 3. 连接到数据库

```bash
# 使用 psql 客户端连接
docker-compose exec postgresql psql -U postgres -d centag

# 或从宿主机连接（需要安装 psql）
psql -h localhost -p 5432 -U postgres -d centag
```

### 4. 验证表结构

```sql
-- 查看所有表
\dt

-- 查看 KV 缓存表结构
\d kv_cache

-- 查看向量存储表结构
\d vector_cache

-- 查看问答对表结构
\d qa_pairs
```

## 配置说明

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| POSTGRES_USER | postgres | 数据库用户名 |
| POSTGRES_PASSWORD | - | 数据库密码（必须设置） |
| POSTGRES_DB | centag | 数据库名称 |
| POSTGRES_HOST | postgresql | 数据库主机名（Docker 内部） |
| POSTGRES_PORT | 5432 | 数据库端口 |
| POSTGRES_MAX_CONNS | 20 | 最大连接数 |
| POSTGRES_MIN_CONNS | 5 | 最小连接数 |
| POSTGRES_MAX_CONN_LIFETIME | 3600 | 连接最大生命周期（秒） |
| POSTGRES_MAX_CONN_IDLE_TIME | 600 | 连接最大空闲时间（秒） |
| POSTGRES_VECTOR_DIMENSION | 1024 | 向量维度 |
| POSTGRES_INDEX_TYPE | hnsw | 索引类型 |

### 配置文件示例

在 `config/secrets/.env` 中添加：

```bash
POSTGRES_ENABLED=true
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password
POSTGRES_DB=centag
POSTGRES_PORT=5432
POSTGRES_MAX_CONNS=20
POSTGRES_MIN_CONNS=5
POSTGRES_VECTOR_DIMENSION=1024
POSTGRES_INDEX_TYPE=hnsw
```

## 数据表说明

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

### qa_pairs
问答对存储表，用于相似度检索。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 自增 ID（主键） |
| question | TEXT | 问题 |
| answer | TEXT | 答案 |
| question_embedding | vector(1024) | 问题向量 |
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

## 维护函数

### cleanup_expired_kv()
清理过期的 KV 缓存数据。

```sql
SELECT cleanup_expired_kv();
```

### get_kv_cache_stats()
获取 KV 缓存统计信息。

```sql
SELECT * FROM get_kv_cache_stats();
```

### get_vector_cache_stats()
获取向量缓存统计信息。

```sql
SELECT * FROM get_vector_cache_stats();
```

### get_qa_pairs_stats()
获取问答对统计信息。

```sql
SELECT * FROM get_qa_pairs_stats();
```

## 测试向量功能

### 1. 测试 pgvector 扩展

```sql
-- 测试 pgvector 扩展是否正常
SELECT vector_dims('[1,2,3]'::vector);

-- 测试余弦相似度
SELECT 1 - (vector('[1,2,3]') <=> vector('[1,2,3.1]')) as similarity;
```

### 2. 测试向量插入

```sql
-- 插入测试向量
INSERT INTO vector_cache (id, vector, metadata)
VALUES (
    'test-1',
    '[1,2,3]'::vector,
    '{"test": true}'::jsonb
);
```

### 3. 测试向量检索

```sql
-- 查询最相似的向量
SELECT
    id,
    1 - (vector <=> '[1,2,3]'::vector) as similarity,
    metadata
FROM vector_cache
ORDER BY vector <=> '[1,2,3]'::vector
LIMIT 5;
```

### 4. 测试问答对检索

```sql
-- 插入测试问答对
INSERT INTO qa_pairs (question, answer, question_embedding)
VALUES (
    '如何使用 PostgreSQL？',
    'PostgreSQL 是一个强大的开源关系数据库...',
    ARRAY[0.1, 0.2, 0.3]::vector  -- 这里应该是实际的向量
);

-- 查询相似问题
SELECT
    question,
    answer,
    1 - (question_embedding <=> ARRAY[0.1, 0.2, 0.3]::vector) as similarity
FROM qa_pairs
ORDER BY question_embedding <=> ARRAY[0.1, 0.2, 0.3]::vector
LIMIT 5;
```

## 性能优化

### 1. HNSW 索引参数

HNSW 索引参数请在迁移 SQL 或 DBA 脚本中设置（原 `docker/postgresql/schema.sql` 已随中间件目录移除）：

```sql
-- 当前配置
CREATE INDEX ... USING hnsw (vector vector_cosine_ops)
WITH (m = 16, ef_construction = 200);
```

- **m**: 每个节点的连接数（默认 16）
  - 更大 = 更好性能，更多内存
  - 推荐范围：16-32

- **ef_construction**: 构建时的候选列表大小（默认 200）
  - 更大 = 更好索引质量，更慢构建
  - 推荐范围：200-400

### 2. 连接池优化

根据应用负载调整连接池大小：

```yaml
# config/secrets/.env
POSTGRES_MAX_CONNS=20      # 推荐值：20-50
POSTGRES_MIN_CONNS=5       # 推荐值：5-10
POSTGRES_MAX_CONN_LIFETIME=3600  # 1小时
POSTGRES_MAX_CONN_IDLE_TIME=600  # 10分钟
```

### 3. 定期维护

```sql
-- 清理过期数据
SELECT cleanup_expired_kv();

-- 更新表统计信息
ANALYZE kv_cache;
ANALYZE vector_cache;
ANALYZE qa_pairs;

-- 重建索引（可选，仅在必要时）
REINDEX TABLE vector_cache;
REINDEX TABLE qa_pairs;
```

## 备份与恢复

### 备份

```bash
# 备份整个数据库
docker-compose exec postgresql pg_dump -U postgres centag > backup.sql

# 仅备份数据
docker-compose exec postgresql pg_dump -U postgres --data-only centag > data.sql

# 仅备份表结构
docker-compose exec postgresql pg_dump -U postgres --schema-only centag > schema.sql
```

### 恢复

```bash
# 恢复整个数据库
cat backup.sql | docker-compose exec -T postgresql psql -U postgres centag

# 恢复数据（需要先恢复表结构）
cat data.sql | docker-compose exec -T postgresql psql -U postgres centag
```

## 故障排查

### 1. 容器无法启动

```bash
# 查看日志
docker-compose logs postgresql

# 检查端口占用
netstat -tulpn | grep 5432

# 检查数据卷
docker volume ls | grep postgresql
```

### 2. 连接失败

```bash
# 测试连接
docker-compose exec postgresql pg_isready -U postgres

# 检查网络
docker network inspect centag_centag-network

# 检查环境变量
docker-compose config | grep POSTGRES
```

### 3. 索引性能问题

```sql
-- 检查索引使用情况
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename IN ('kv_cache', 'vector_cache', 'qa_pairs')
ORDER BY idx_scan DESC;

-- 查看索引大小
SELECT
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_indexes
WHERE tablename IN ('kv_cache', 'vector_cache', 'qa_pairs');
```

### 4. 内存占用高

```sql
-- 查看表大小
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size(tablename::regclass)) as total_size,
    pg_size_pretty(pg_relation_size(tablename::regclass)) as table_size,
    pg_size_pretty(pg_total_relation_size(tablename::regclass) - pg_relation_size(tablename::regclass)) as index_size
FROM pg_tables
WHERE tablename IN ('kv_cache', 'vector_cache', 'qa_pairs')
ORDER BY pg_total_relation_size(tablename::regclass) DESC;
```

## 安全建议

1. **使用强密码**：确保 `POSTGRES_PASSWORD` 使用强密码
2. **限制网络访问**：不要将 5432 端口暴露到公网
3. **定期备份**：设置自动备份策略
4. **更新密码**：定期更改数据库密码
5. **审计日志**：启用 PostgreSQL 审计日志

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

## 迁移指南

### 从 Redis 迁移

1. 导出 Redis 数据：
```bash
redis-cli --rdb redis-dump.rdb
```

2. 编写迁移脚本，将 Redis 数据导入 PostgreSQL
3. 更新应用配置，切换到 PostgreSQL

### 从 Elasticsearch 迁移

1. 使用 Elasticsearch API 导出数据
2. 转换数据格式适配 PostgreSQL
3. 使用批量插入导入数据
4. 更新应用配置，切换到 PostgreSQL

## 支持

如有问题，请参考：
- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [pgvector GitHub](https://github.com/pgvector/pgvector)
- 项目 Issues
