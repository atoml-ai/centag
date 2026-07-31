# Centag 发行版组装指南

## 概述

Centag 支持通过发行版（distribution）自定义包含的插件组合。每个发行版是一个独立的 Go module，通过 `_ import` 触发插件注册。

## 现有发行版

> 以下为发行版概览（完整构建/注册矩阵见闭源仓内部文档）。

| 发行版 | 会注册的插件（概览） | 与部署的关系 | 适用场景 |
|--------|----------------------|----------------|----------|
| `dist/minimal/` | 3 后端 + 2 协议 + router（无 DB） | 文件配置，无中间件 | 轻量单机 / CLI |
| `dist/personal/` | **全功能 29 插件**（5 后端 + 4 协议 + 13 业务 + 7 存储/DB） | **默认 SQLite**；可配置连外部中间件 | 个人全功能 |
| `dist/team/` | **与 personal 相同** | **中间件单独部署**（PG / 向量等） | 团队 / 多租户 |
| `cmd/centag`（本地 `make build`） | 与 personal/team 相同 | 视本地 `.env` | 本地开发调试 |

> personal 与 team **二进制插件集合对齐**；差别在 Config Profile 的默认依赖，不在插件裁剪。

## 创建新发行版

### 步骤 1：创建目录

```bash
mkdir -p dist/mydist
cd dist/mydist
```

### 步骤 2：创建 go.mod

```go
module centag/dist/mydist

go 1.25.0

require (
    centag/core v0.0.0
    centag/plugins/backend/openai v0.0.0
    centag/plugins/database/sqlite v0.0.0
)

replace (
    centag/core => ../../core
    centag/plugins/backend/openai => ../../plugins/backend/openai
    centag/plugins/database/sqlite => ../../plugins/database/sqlite
)
```

### 步骤 3：创建 main.go

```go
package main

import (
    "centag/core/pkg/entrypoint"

    // 触发插件注册
    _ "centag/plugins/backend/openai"
    _ "centag/plugins/backend/ollama"
    _ "centag/plugins/database/sqlite"
)

func main() {
    entrypoint.Run("dev", "unknown")
}
```

### 步骤 4：构建

```bash
go mod tidy
go build -o centag-mydist .
```

## 可用插件列表

### 后端插件
- `plugins/backend/openai` — OpenAI API
- `plugins/backend/ollama` — Ollama 本地模型
- `plugins/backend/anthropic` — Anthropic Claude
- `plugins/backend/gemini` — Google Gemini
- `plugins/backend/azure` — Azure OpenAI

### 协议插件
- `plugins/protocol/openai` — OpenAI 协议（Chat Completions）
- `plugins/protocol/anthropic` — Anthropic Messages
- `plugins/protocol/gemini` — Gemini Generate Content
- `plugins/protocol/openairesponses` — OpenAI Responses API

### 存储插件
- `plugins/storage/redis` — Redis 缓存
- `plugins/storage/postgresql` — PostgreSQL 向量存储
- `plugins/storage/elasticsearch` — Elasticsearch 搜索
- `plugins/storage/chroma` — ChromaDB 向量存储
- `plugins/storage/file` — 文件存储

### 数据库插件
- `plugins/database/sqlite` — SQLite（嵌入式）
- `plugins/database/postgresql` — PostgreSQL

### 业务插件
- 外部业务插件仓 `business/router` — 请求路由
- 外部业务插件仓 `business/optimizer` — 请求优化
- 外部业务插件仓 `business/reviewer` — 内容审核
- 外部业务插件仓 `business/summarizer` — 摘要生成
- 外部业务插件仓 `business/translator` — 翻译
- 外部业务插件仓 `business/question_splitter` — 问题拆分
- 外部业务插件仓 `business/answer_synthesizer` — 答案合成
- 外部业务插件仓 `business/tasktype_detector` — 任务类型检测
- 外部业务插件仓 `business/mem0` — 记忆存储
- 外部业务插件仓 `business/pi_agent` — PI 代理
- 外部业务插件仓 `business/pii_redactor` — PII 脱敏
- 外部业务插件仓 `business/geo_router` — 地理路由
- 外部业务插件仓 `business/rag_retrieval` — RAG 检索

## 构建脚本

推荐：

```bash
./start.sh build minimal
./start.sh build personal
# Team：仅在 centag-pro → ./scripts/build-team.sh
```

或在各 `dist/<name>/`（minimal|personal）下使用已带 tags 的 `./build.sh`。勿省略 `-tags`，否则带 `//go:build` 的 `register.go` 不会编进二进制。

```bash
#!/bin/bash
# 示例：构建开源发行版（委托 start.sh，确保 tags 正确）
set -e
for dist in minimal personal; do
    ./start.sh build "$dist"
done
```

## 依赖管理

所有插件通过 `replace` 指令引用本地 core 模块：

```go
replace centag/core => ../../core
```

发布时替换为版本号：

```go
replace centag/core => github.com/marmotcai/centag/core v1.0.0
```
