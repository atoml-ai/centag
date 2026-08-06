# Centag 缓存指南

本文档详细介绍LLM Proxy的缓存机制、配置和使用方法。

## 目录

- [概述](#概述)
- [缓存策略](#缓存策略)
- [配置方法](#配置方法)
- [使用指南](#使用指南)
- [性能优化](#性能优化)
- [监控和诊断](#监控和诊断)
- [测试](#测试)

## 概述

LLM Proxy 将「存储/召回后端」与「命中策略」分层（v0.3.3）：

| 层 | 配置 | 说明 |
|----|------|------|
| **召回后端** | `cache.backend` | `exact`（S1，默认）/ `semantic`（S2 插件）/ `external`（S3 插件） |
| **命中策略** | `cache.hit_strategies` | 查库前的查询改写链，如 `normalize`、`expand`、自定义插件名 |
| **叠加（高级）** | `cache.allow_backend_stacking` | 默认 `false`；为 `true` 时 exact miss 后再查 semantic（旧 hybrid） |

旧字段 `cache.strategy` 仍兼容：加载时归一到 `backend`（`hybrid` → `exact` + 可选 stacking）。

### 缓存优势

- **降低延迟**：缓存命中时响应时间从数秒降至毫秒级
- **减少成本**：减少对LLM的API调用
- **提升吞吐量**：相同请求可并发处理

### 管理台

Web 侧栏「缓存」→ `/cache` 为统一数据管理台（数据 / 统计 / 配置）。会话页可通过「查看关联缓存」深链到 `/cache?session_id=`。

## 召回后端与命中策略

### 1. S1 Exact KV（默认）

**原理**：基于请求 key 的精确匹配（模型、messages、temperature 等）。

**配置**：
```yaml
cache:
  enabled: true
  backend: exact          # 默认
  hit_strategies: [normalize, expand]
  default_ttl: 3600
```

**优点**：快、一致性高、配好 KV/存储即可开。  
**缺点**：无法识别语义相似但文本不同的请求（可用 hit_strategies 提升命中）。

### 2. S2 语义向量（插件扩展）

**原理**：向量嵌入 + 相似度搜索。与 S1 **默认可互斥**（改 `backend: semantic`）。

```yaml
cache:
  enabled: true
  backend: semantic
  hit_strategies: [normalize]
  semantic:
    threshold: 0.85
    top_k: 5
    enable_auto_embedding: true
embedding:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:21434
```

### 3. S3 外部召回（插件扩展）

实现 `CacheRecallBackend` 契约，配置 `backend: external` 与 `cache.external.plugin`。可用于缓存短路或 `#rag` 提示增强（见 pipeline 模板）。

### 4. 旧 hybrid / stacking

不再默认开启。若需「exact miss → semantic」：

```yaml
cache:
  backend: exact
  allow_backend_stacking: true
```

或配置遗留 `strategy: hybrid`（归一时会打开 stacking 告警路径，见 `NormalizeCacheConfig`）。

## 配置方法

### 基本配置

```yaml
cache:
  enabled: true
  backend: exact                 # exact | semantic | external
  hit_strategies: [normalize, expand]
  allow_backend_stacking: false
  default_ttl: 3600
  semantic:
    threshold: 0.85
    top_k: 5
    enable_auto_embedding: true

embedding:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:21434
```

### 环境变量配置

```bash
export CACHE_ENABLED=true
export CACHE_STRATEGY=exact   # 兼容旧名；优先使用配置文件 backend
export CACHE_TTL=3600
export SEMANTIC_THRESHOLD=0.85
```

## 使用指南

### 查看缓存统计

```bash
# 获取缓存统计
curl http://localhost:20060/api/v1/cache/stats

# 查看缓存命中率
curl http://localhost:20060/api/v1/monitor/dashboard | python3 -c "import sys, json; d=json.load(sys.stdin); print(f\"命中率: {d['cache']['hit_rate_percent']:.2f}%\")"
```

### 查看缓存列表

```bash
# 查看所有缓存条目
curl http://localhost:20060/api/v1/cache/list

# 查看特定缓存
curl http://localhost:20060/api/v1/cache/info -X POST \
  -H "Content-Type: application/json" \
  -d '{"key":"your-cache-key"}'
```

### 清空缓存

```bash
# 清空所有缓存
curl -X POST http://localhost:20060/api/v1/cache/clear

# 删除特定缓存
curl -X DELETE http://localhost:20060/api/v1/cache/entry \
  -H "Content-Type: application/json" \
  -d '{"key":"your-cache-key"}'

# 多维列表（管理台）
curl "http://localhost:20060/api/v1/cache/list?type=all&session_id=sess-1&model=qwen&q=hello&from=2026-08-01&to=2026-08-06&page=1&size=20"

# 单条详情
curl "http://localhost:20060/api/v1/cache/entry?key=your-cache-key&type=exact"
```

列表筛选参数：`type`、`session_id`、`model`、`q`、`from`/`to`、`storage`、`save_only`、分页。新写入条目尽力带 `session_id` / `model` / `cache_type`（依赖请求头 `X-Session-ID` 等）。

### 启用/禁用缓存

```bash
# 查看缓存状态
curl http://localhost:20060/api/v1/cache/enabled

# 启用缓存
curl -X POST http://localhost:20060/api/v1/cache/toggle \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}'

# 禁用缓存
curl -X POST http://localhost:20060/api/v1/cache/toggle \
  -H "Content-Type: application/json" \
  -d '{"enabled":false}'
```

## 性能优化

### 调整TTL

**原则**：
- 短期数据（如实时信息）：TTL设置为60-300秒
- 知识类数据（如概念解释）：TTL设置为3600-86400秒
- 永久数据（如定义）：TTL设置为7天以上

**示例**：
```yaml
cache:
  exact_cache:
    ttl: 3600  # 1小时
```

### 调整相似度阈值

**阈值选择**：
- 0.90-0.95：严格匹配，准确率高但命中率低
- 0.85-0.90：平衡点，推荐值
- 0.80-0.85：宽松匹配，命中率高但准确率略低

**调整建议**：
1. 从0.85开始
2. 观察命中率和准确率
3. 根据实际效果微调

### 调整缓存大小

```yaml
cache:
  exact_cache:
    max_entries: 10000  # 根据内存调整
```

**内存估算**：
- 每个缓存条目约1-2KB
- 10000条目约10-20MB内存

## 监控和诊断

### 缓存统计指标

**关键指标**：
- `hit_rate_percent`：缓存命中率
- `hits`：命中次数
- `misses`：未命中次数
- `total_entries`：缓存条目数
- `evictions`：缓存淘汰次数

**健康标准**：
- 命中率 >= 60%：良好
- 命中率 >= 80%：优秀
- 淘汰次数过高：考虑增加max_entries

### 诊断命令

```bash
# 查看实时监控
curl -s http://localhost:20060/api/v1/monitor/dashboard | python3 -m json.tool

# 查看缓存详情
curl http://localhost:20060/api/v1/cache/stats | python3 -m json.tool

# 测试缓存
./test/simple-cache-test.sh
```

### 常见问题

**Q: 缓存命中率低怎么办？**

A:
1. 检查是否启用了语义缓存
2. 调低相似度阈值
3. 增加TTL时间
4. 检查缓存键生成是否正确

**Q: 缓存占用内存过高？**

A:
1. 减少max_entries
2. 缩短TTL
3. 启用缓存淘汰策略

**Q: 语义缓存响应慢？**

A:
1. 检查嵌入服务性能
2. 使用更快的嵌入模型
3. 考虑使用精确缓存+语义缓存的混合模式

## 测试

### 快速测试

```bash
# 运行缓存测试
./test/simple-cache-test.sh

# 运行E2E测试（包含缓存）
./test/test-question-split-e2e.sh
```

### 缓存策略测试

```bash
# 测试不同缓存策略
./test/test-cache-strategy.sh
```

### 测试报告模板

参见 `cache-test-report-template.md` 了解如何编写测试报告。

## 最佳实践

1. **开发环境**：使用精确缓存，快速验证
2. **生产环境**：使用混合缓存，平衡性能和准确率
3. **监控**：定期查看缓存统计，及时调整参数
4. **测试**：部署前后都运行测试脚本验证功能
5. **日志**：启用缓存相关日志，便于问题排查

## 相关文档

- [测试指南](../../archive/deprecated/docs/guide/Testing-Guide.md)（已归档）
- [问题拆分指南](../../archive/deprecated/docs/processor/Question-Processor-Guide.md)（已归档）
- [模型扩展框架](Model-Extension.md)（文档已移除）
