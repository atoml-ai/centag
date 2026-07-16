# Proxy Claw 缓存指南

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

LLM Proxy提供多种缓存策略，旨在减少重复请求的延迟，提升系统吞吐量。

### 缓存类型

1. **精确匹配缓存**：基于请求的精确匹配
2. **语义缓存**：基于向量相似度的语义匹配
3. **混合缓存**：结合精确匹配和语义匹配

### 缓存优势

- **降低延迟**：缓存命中时响应时间从数秒降至毫秒级
- **减少成本**：减少对LLM的API调用
- **提升吞吐量**：相同请求可并发处理

## 缓存策略

### 1. 精确匹配缓存

**原理**：基于请求的精确匹配，包括模型、messages、temperature等参数。

**适用场景**：
- 重复的完全相同请求
- 对响应一致性要求高的场景

**配置**：
```yaml
cache:
  enabled: true
  strategy: exact
  ttl: 3600  # 缓存有效期（秒）
  max_entries: 10000  # 最大缓存条目数
```

**优点**：
- 速度快
- 一致性高
- 实现简单

**缺点**：
- 无法识别语义相似但文本不同的请求

### 2. 语义缓存

**原理**：使用向量嵌入和相似度搜索，识别语义相似的请求。

**适用场景**：
- 用户提问方式多样化的场景
- 需要提升命中率的场景

**配置**：
```yaml
cache:
  enabled: true
  strategy: semantic
  semantic_cache:
    enabled: true
    threshold: 0.85  # 相似度阈值（0-1）
    top_k: 3  # 返回最相似的K个结果
  embedding:
    provider: ollama  # 嵌入服务提供商
    model: nomic-embed-text  # 嵌入模型
    base_url: http://localhost:21434
```

**优点**：
- 命中率高
- 可识别语义相似请求

**缺点**：
- 需要额外的嵌入服务
- 响应时间略长
- 可能返回不完全相关的结果

### 3. 混合缓存

**原理**：先尝试精确匹配，未命中时再尝试语义匹配。

**配置**：
```yaml
cache:
  enabled: true
  strategy: hybrid
  exact_cache:
    ttl: 3600
  semantic_cache:
    enabled: true
    threshold: 0.85
```

**推荐场景**：
- 对命中率和响应速度都有要求的场景
- 大多数请求相似度较高的场景

## 配置方法

### 基本配置

编辑缓存配置（旧版 `config.yaml` 已归档至 `../../archive/deprecated/configs/config.yaml`）：

```yaml
cache:
  enabled: true
  strategy: hybrid  # exact | semantic | hybrid

  # 精确缓存配置
  exact_cache:
    ttl: 3600  # 缓存时间（秒）
    max_entries: 10000

  # 语义缓存配置
  semantic_cache:
    enabled: true
    threshold: 0.85  # 相似度阈值（0.7-0.9）
    top_k: 3
    enable_auto_embedding: true  # 自动生成嵌入

  # 嵌入服务配置
  embedding:
    provider: ollama  # ollama | openai | remote
    model: nomic-embed-text
    base_url: http://localhost:21434
    api_key: ""  # OpenAI需要
```

### 环境变量配置

```bash
export CACHE_ENABLED=true
export CACHE_STRATEGY=hybrid
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
```

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
