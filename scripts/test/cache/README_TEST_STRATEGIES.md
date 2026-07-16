# 缓存策略测试说明

## 概述

本测试脚本用于测试 Centag 的多种缓存存储策略,包括:
- **Elasticsearch**: 支持精确匹配(KV)和语义搜索(向量)
- **Redis**: 支持精确匹配(KV)
- **ChromaDB**: 支持语义搜索(向量)

## 测试脚本

### 1. 完整策略测试脚本

`test-cache-strategies.sh` - 测试所有存储策略的完整功能

#### 使用方法

```bash
# 测试所有存储策略
cd /home/caijun/workspaces/centag
bash scripts/test/cache/test-cache-strategies.sh

# 仅测试 Elasticsearch
TEST_MODE=elasticsearch-main bash scripts/test/cache/test-cache-strategies.sh

# 仅测试 Redis
TEST_MODE=redis bash scripts/test/cache/test-cache-strategies.sh

# 仅测试 ChromaDB
TEST_MODE=chroma bash scripts/test/cache/test-cache-strategies.sh
```

#### 测试内容

每个存储策略会测试以下功能:

1. **精确匹配缓存测试**
   - 简单问题精确命中
   - 代码问题精确命中
   - 流式请求精确命中

2. **语义匹配缓存测试**
   - 同义词语义命中
   - 问题改写语义命中
   - 流式请求语义命中

3. **边界情况测试**
   - 空响应处理
   - 超长文本处理

4. **性能测试**
   - 连续请求性能(10次)

#### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| PROXY_API | http://localhost:20060 | API地址 |
| MODEL | qwen2.5:1.5b | 模型名称 |
| TEMPERATURE | 0.7 | 温度参数 |
| MAX_TOKENS | 2000 | 最大token数 |
| TEST_MODE | all | 测试模式: all/elasticsearch-main/redis/chroma |

### 2. 流式语义缓存测试

`test_stream_semantic_cache.sh` - 专门测试流式请求的语义缓存

#### 使用方法

```bash
cd /home/caijun/workspaces/centag
bash scripts/test/cache/test_stream_semantic_cache.sh
```

#### 测试内容

1. 生成缓存
2. 相似问题命中测试
3. 不相关问题未命中测试
4. 响应内容验证
5. 精确匹配优先级测试

## 存储架构说明

### Elasticsearch

- **类型**: KV + 向量数据库
- **功能**:
  - 精确匹配: 使用 `cache_exact_index` 索引
  - 语义搜索: 使用 `cache_semantic_index` 索引和向量嵌入
- **特点**:
  - 全功能支持
  - 支持复杂的搜索和聚合
  - 适合生产环境

### Redis

- **类型**: KV存储
- **功能**:
  - 精确匹配: 使用 Redis 的 key-value 存储
- **特点**:
  - 高性能
  - 低延迟
  - 适合高频精确匹配场景
- **注意**: 仅支持精确匹配,不支持语义搜索

### ChromaDB

- **类型**: 向量数据库
- **功能**:
  - 语义搜索: 使用向量相似度搜索
- **特点**:
  - 专为向量搜索设计
  - 易于集成和部署
- **当前状态**: ⚠️ REST API 存在兼容性问题,建议谨慎使用

## 启动服务

### 方式1: 使用启动脚本

```bash
cd /home/caijun/workspaces/centag
./start.sh start
```

### 方式2: Docker 环境

```bash
cd /home/caijun/workspaces/centag/docker
docker-compose up -d
```

### 验证服务

```bash
curl http://localhost:20060/api/v1/monitor/dashboard
```

## 配置说明

当前配置位于 `bin/configs/config.yaml`:

```yaml
# 缓存配置
cache:
  enabled: true
  strategy: semantic
  semantic:
    threshold: 0.8  # 语义相似度阈值

# 默认存储
default_storage: elasticsearch-main

# 存储配置
storages:
  - name: elasticsearch-main
    type: elasticsearch
    enabled: true
    # ...

  - name: redis
    type: redis
    enabled: true
    # ...

  - name: chroma
    type: chroma
    enabled: true
    # ...
```

## 切换存储策略

### 通过 API 切换

```bash
# 切换到 Elasticsearch
curl -X PUT http://localhost:20060/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{"default_storage": "elasticsearch-main"}'

# 切换到 Redis
curl -X PUT http://localhost:20060/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{"default_storage": "redis"}'

# 切换到 ChromaDB
curl -X PUT http://localhost:20060/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{"default_storage": "chroma"}'
```

### 通过配置文件切换

编辑 `bin/configs/config.yaml`:

```yaml
default_storage: elasticsearch-main  # 或 redis / chroma
```

然后重启服务。

## 查看缓存状态

```bash
# 查看监控面板
curl http://localhost:20060/api/v1/monitor/dashboard

# 查看缓存统计
curl http://localhost:20060/api/v1/monitor/cache

# 列出缓存条目
curl http://localhost:20060/api/v1/cache/list

# 查看当前配置
curl http://localhost:20060/api/v1/config
```

## 清空缓存

```bash
# 清空所有缓存
curl -X DELETE http://localhost:20060/api/v1/cache

# 删除特定缓存条目
curl -X POST http://localhost:20060/api/v1/cache/entry \
  -H "Content-Type: application/json" \
  -d '{"key": "your_cache_key"}'
```

## 调整语义阈值

```bash
# 设置新的语义阈值
curl -X POST http://localhost:20060/api/v1/cache/semantic/threshold \
  -H "Content-Type: application/json" \
  -d '{"value": 0.85}'

# 查看当前阈值
curl http://localhost:20060/api/v1/cache/semantic/threshold
```

## 已知问题

### ChromaDB REST API 问题

- **问题描述**: ChromaDB 的 REST API 在当前版本中存在兼容性问题
- **症状**: API 调用返回 405 Method Not Allowed
- **影响**: ChromaDB 存储策略可能无法正常工作
- **解决方案**:
  1. 等待 ChromaDB 官方修复 REST API
  2. 考虑使用 ChromaDB 的 Python 客户端
  3. 暂时使用 Elasticsearch 或 Redis 作为存储后端

### 配置 API 字段名不统一

- **问题描述**: JSON 响应中的字段名大小写不一致
- **影响**: 脚本解析配置时可能出错
- **解决方案**: 脚本已更新以处理不同的字段名格式

## 测试最佳实践

1. **测试前准备**
   - 确保服务正常运行
   - 确保所有依赖服务(Elasticsearch, Redis, ChromaDB)已启动
   - 清空旧缓存以确保测试准确性

2. **测试顺序**
   - 先测试 Elasticsearch(功能最全)
   - 再测试 Redis(高性能)
   - 最后测试 ChromaDB(如果可用)

3. **结果分析**
   - 关注命中率和响应时间
   - 检查日志中的错误信息
   - 验证缓存内容的正确性

4. **性能优化**
   - 根据实际场景选择合适的存储策略
   - 调整语义阈值以平衡命中率和准确性
   - 监控缓存大小和内存使用

## 日志查看

```bash
# 查看实时日志
tail -f logs/centag.log

# 过滤缓存相关日志
tail -f logs/centag.log | grep -i cache

# 查看语义搜索日志
tail -f logs/centag.log | grep -i semantic

# 查看命中/未命中日志
tail -f logs/centag.log | grep -E "HIT|MISS"
```

## 故障排查

### 服务无法启动

```bash
# 检查端口占用
lsof -i :20060

# 检查配置文件
./bin/centag --config ./bin/configs/config.yaml --check

# 查看详细日志
./bin/centag --config ./bin/configs/config.yaml --verbose
```

### 缓存不工作

```bash
# 检查缓存是否启用
curl http://localhost:20060/api/v1/config | jq '.cache.Enabled'

# 检查存储配置
curl http://localhost:20060/api/v1/config | jq '.storages'

# 测试存储连接
curl -X POST http://localhost:20060/api/v1/cache/check \
  -H "Content-Type: application/json" \
  -d '{"storage": "elasticsearch-main"}'
```

### 语义搜索不命中

```bash
# 检查相似度阈值
curl http://localhost:20060/api/v1/cache/semantic/threshold

# 测试相似度计算
curl -X POST http://localhost:20060/api/v1/cache/semantic/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "你的问题",
    "topk": 5
  }'
```

## 贡献

如有问题或改进建议,请提交 Issue 或 Pull Request。
