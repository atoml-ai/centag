# Dist Profiles（发行版构建规格）

## 概述

Dist Profile 是 Centag 的**编译时插件子集**。能否真正注册，取决于两层同时满足：

1. **`_ import`**：`dist/<name>/main.go`（或本地 `cmd/centag/main.go`）把插件包链进二进制  
2. **`-tags`**：带 `//go:build xxx` 的 `register.go` 必须命中对应 tag，才会执行 `init()` 注册  

> 数据库 / 存储插件目前**没有** build tag，只要 import 就会注册。  
> 后端 / 协议 / 业务插件的 `register.go` **有** build tag，缺 tag 时包可能编进二进制，但**不会注册**。

### 产品定位（重要）

| 发行版 | 定位 | 二进制插件 | 部署默认依赖 |
|--------|------|------------|--------------|
| **minimal** | 轻量单机 / CLI | 精简（无 DB、仅 router） | 文件配置，无中间件 |
| **gateway** | **个人全功能** | 与 team **对齐** | **默认内置 SQLite**；可通过配置连接外部 PG / 向量 / Redis 等 |
| **team** | **团队版** | 与 gateway **对齐** | **中间件单独部署**（PG、向量化等），应用连外部服务；可多租户 / HA |

> gateway 与 team 的差别**主要不在二进制裁剪**，而在 **Config Profile / 部署默认依赖**（见 `config/profiles/`）。  
> 两者都编入完整业务插件与 sqlite+postgresql 驱动；gateway 默认用 sqlite，team 默认连外部 PG。

与之对应的是 **Config Profile**（`config/profiles/<name>/`）：部署蓝图（compose + manifest），在运行时决定行为。

## 构建入口一览

| 构建方式 | 入口 | tags 来源 | 产出 |
|---------|------|-----------|------|
| `./start.sh dist build <name>` | `dist/<name>/main.go` | `start.sh` → `_get_dist_tags()` | `bin/server/centag-<name>` |
| `./start.sh dist docker-build <name>` | 同上（`Dockerfile.dist`） | 同上，经 `BUILD_TAGS` ARG | Docker 镜像 |
| `./start.sh build be` / `make build` / `run be` / `debug` | `cmd/centag/main.go` | `Makefile` → `BUILD_TAGS`（默认对齐 gateway/team 全功能） | `bin/server/centag` |

## 总览对照表

| 项 | minimal | gateway | team | 本地 `cmd/centag` |
|----|---------|---------|------|----------------------|
| 入口 | `dist/minimal/main.go` | `dist/gateway/main.go` | `dist/team/main.go` | `cmd/centag/main.go` |
| 构建命令 | `./start.sh dist build minimal` | `./start.sh dist build gateway` | `./start.sh dist build team` | `./start.sh build be` / `make build` |
| 插件集合 | 精简 | **全功能** | **全功能（同 gateway）** | 全功能（同 gateway/team） |
| 默认 DB（部署） | 无（文件配置） | SQLite | 外部 PostgreSQL | 视本地 `.env` |
| 前端（Docker） | 否（config-generator） | 是 | 是 | 本地另跑 `build fe` |
| 目标场景 | 轻量单机 / CLI | 个人完整版 | 团队 / 多租户 | 本地开发调试 |

## Build Tags 矩阵

来源：`start.sh` 中 `_get_dist_tags()`（gateway/team 共用 `_FULL_FEATURE_TAGS`）；本地默认见 `Makefile` 的 `BUILD_TAGS`。

| Tag | minimal | gateway | team | 本地 cmd |
|-----|:-------:|:-------:|:----:|:--------:|
| `minimal` | ✅ | — | — | — |
| `protocol_openai` | ✅ | ✅ | ✅ | ✅ |
| `protocol_anthropic` | ✅ | ✅ | ✅ | ✅ |
| `protocol_gemini` | — | ✅ | ✅ | ✅ |
| `protocol_openairesponses` | — | ✅ | ✅ | ✅ |
| `backend_openai` | ✅ | ✅ | ✅ | ✅ |
| `backend_ollama` | ✅ | ✅ | ✅ | ✅ |
| `backend_anthropic` | ✅ | ✅ | ✅ | ✅ |
| `backend_gemini` | — | ✅ | ✅ | ✅ |
| `backend_azure` | — | ✅ | ✅ | ✅ |
| `business_router` | ✅ | ✅ | ✅ | ✅ |
| `business_optimizer` | — | ✅ | ✅ | ✅ |
| `business_reviewer` | — | ✅ | ✅ | ✅ |
| `business_summarizer` | — | ✅ | ✅ | ✅ |
| `business_translator` | — | ✅ | ✅ | ✅ |
| `business_question_splitter` | — | ✅ | ✅ | ✅ |
| `business_answer_synthesizer` | — | ✅ | ✅ | ✅ |
| `business_tasktype_detector` | — | ✅ | ✅ | ✅ |
| `business_mem0` | — | ✅ | ✅ | ✅ |
| `business_pi_agent` | — | ✅ | ✅ | ✅ |
| `business_pii_redactor` | — | ✅ | ✅ | ✅ |
| `business_geo_router` | — | ✅ | ✅ | ✅ |
| `business_rag_retrieval` | — | ✅ | ✅ | ✅ |

## 插件编译 / 注册矩阵

图例：

- ✅ = `main.go` 有 `_ import`，且（无 tag 或 tag 已开）→ **会注册**
- — = 未 import

### 后端（需 `backend_*` tag）

| 插件包 | Tag | minimal | gateway | team | 本地 cmd |
|--------|-----|:-------:|:-------:|:----:|:--------:|
| `plugins/backend/openai` | `backend_openai` | ✅ | ✅ | ✅ | ✅ |
| `plugins/backend/ollama` | `backend_ollama` | ✅ | ✅ | ✅ | ✅ |
| `plugins/backend/anthropic` | `backend_anthropic` | ✅ | ✅ | ✅ | ✅ |
| `plugins/backend/gemini` | `backend_gemini` | — | ✅ | ✅ | ✅ |
| `plugins/backend/azure` | `backend_azure` | — | ✅ | ✅ | ✅ |

### 协议（需 `protocol_*` tag）

| 插件包 | Tag | minimal | gateway | team | 本地 cmd |
|--------|-----|:-------:|:-------:|:----:|:--------:|
| `plugins/protocol/openai` | `protocol_openai` | ✅ | ✅ | ✅ | ✅ |
| `plugins/protocol/anthropic` | `protocol_anthropic` | ✅ | ✅ | ✅ | ✅ |
| `plugins/protocol/gemini` | `protocol_gemini` | — | ✅ | ✅ | ✅ |
| `plugins/protocol/openairesponses` | `protocol_openairesponses` | — | ✅ | ✅ | ✅ |

### 数据库（无 tag，import 即注册）

| 插件包 | minimal | gateway | team | 本地 cmd | 说明 |
|--------|:-------:|:-------:|:----:|:--------:|------|
| `plugins/database/sqlite` | — | ✅ | ✅ | ✅ | gateway **部署默认**用它 |
| `plugins/database/postgresql` | — | ✅ | ✅ | ✅ | team **部署默认**用它；gateway 可配置切换 |

### 存储（无 tag，import 即注册）

| 插件包 | minimal | gateway | team | 本地 cmd |
|--------|:-------:|:-------:|:----:|:--------:|
| `plugins/storage/redis` | — | ✅ | ✅ | ✅ |
| `plugins/storage/postgresql` | — | ✅ | ✅ | ✅ |
| `plugins/storage/elasticsearch` | — | ✅ | ✅ | ✅ |
| `plugins/storage/chroma` | — | ✅ | ✅ | ✅ |
| `plugins/storage/file` | — | ✅ | ✅ | ✅ |

> 存储驱动编进二进制 ≠ 默认启动对应中间件。gateway 默认可不启 Redis/向量等；team / 相关 Config Profile 默认依赖外部部署。

### 业务（需 `business_*` tag）

| 插件包 | Tag | minimal | gateway | team | 本地 cmd |
|--------|-----|:-------:|:-------:|:----:|:--------:|
| `plugins/business/router` | `business_router` | ✅ | ✅ | ✅ | ✅ |
| `plugins/business/optimizer` | `business_optimizer` | — | ✅ | ✅ | ✅ |
| `plugins/business/reviewer` | `business_reviewer` | — | ✅ | ✅ | ✅ |
| `plugins/business/summarizer` | `business_summarizer` | — | ✅ | ✅ | ✅ |
| `plugins/business/translator` | `business_translator` | — | ✅ | ✅ | ✅ |
| `plugins/business/question_splitter` | `business_question_splitter` | — | ✅ | ✅ | ✅ |
| `plugins/business/answer_synthesizer` | `business_answer_synthesizer` | — | ✅ | ✅ | ✅ |
| `plugins/business/tasktype_detector` | `business_tasktype_detector` | — | ✅ | ✅ | ✅ |
| `plugins/business/mem0` | `business_mem0` | — | ✅ | ✅ | ✅ |
| `plugins/business/pi_agent` | `business_pi_agent` | — | ✅ | ✅ | ✅ |
| `plugins/business/pii_redactor` | `business_pii_redactor` | — | ✅ | ✅ | ✅ |
| `plugins/business/geo_router` | `business_geo_router` | — | ✅ | ✅ | ✅ |
| `plugins/business/rag_retrieval` | `business_rag_retrieval` | — | ✅ | ✅ | ✅ |

## 数量汇总（按「会注册」计）

| 类别 | minimal | gateway | team | 本地 cmd |
|------|--------:|--------:|-----:|---------:|
| 后端 | 3 | 5 | 5 | 5 |
| 协议 | 2 | 4 | 4 | 4 |
| 数据库 | 0 | 2 | 2 | 2 |
| 存储 | 0 | 5 | 5 | 5 |
| 业务 | 1（仅 router） | 13 | 13 | 13 |
| **合计** | **6** | **29** | **29** | **29** |

## 各发行版说明

### minimal

- 入口：`dist/minimal/main.go`
- tags：`minimal` + 协议/后端三件套 + `business_router`
- 含：三后端 + 两协议 + **唯一业务插件 router**
- 不含：任何数据库驱动、全部 storage、除 router 外业务插件（文件配置，无 DB）
- 对应 Config Profile：`config/profiles/minimal/`

### gateway（个人全功能）

- 入口：`dist/gateway/main.go`
- tags：与 team 相同的 `_FULL_FEATURE_TAGS`
- 插件：全后端 / 协议 / DB 驱动 / storage / **全部 13 个业务插件**
- **部署默认**：`LLM_PROXY_DB_DRIVER=sqlite`，单容器即可；需要时改配置连接外部 PG / Redis / 向量等
- 对应 Config Profile：`config/profiles/gateway/`

### team（团队版）

- 入口：`dist/team/main.go`
- tags / 插件集合：**与 gateway 对齐**
- **部署默认**：外部 PostgreSQL、向量等中间件单独部署；`CENTAG_EDITION=team`、多租户 / 可选 HA
- 对应 Config Profile：`config/profiles/team/`

### 本地 `cmd/centag`

- 用于 `make build` / `./start.sh build be` / `run be` / `debug`
- 默认 tags 与插件集合对齐 gateway/team 全功能

## Docker / CLI

### Dockerfile 选择

| Config Profile | Dockerfile | DIST_NAME | INCLUDE_FRONTEND | 默认 DB |
|----------------|------------|-----------|------------------|---------|
| `minimal` | `deploy/docker/Dockerfile.dist` | `minimal` | `false` | 无 |
| `gateway` | `deploy/docker/Dockerfile.dist` | `gateway` | `true` | SQLite |
| `team` | `deploy/docker/Dockerfile.dist` | `team` | `true` | 外部 PostgreSQL |

### 构建参数

| ARG | 默认 | 说明 |
|-----|------|------|
| `DIST_NAME` | `minimal` | 对应 `dist/<name>/` |
| `BUILD_TAGS` | （由 start.sh 注入） | gateway/team 共用全功能 tags |
| `INCLUDE_FRONTEND` | `true` | minimal 通常为 `false` |
| `INITDATA_ARCHIVE` | `false` | 是否注入 initdata zip |
| `VERSION` / `BUILD_TIME` | `dev` / `unknown` | CI 覆盖 |

### CLI

```bash
./start.sh dist build        <minimal|gateway|team>     # 编译发行版二进制
./start.sh build be                                     # 本地开发二进制（cmd/centag）
./start.sh dist docker-build <name> [--initdata <zip>]  # 构建 Docker 镜像
./start.sh dist docker-run   <name> [--initdata <zip>]  # 运行容器
```

> 勿使用已废弃写法 `./start.sh build dist …`；发行版构建走独立子命令 `dist`。
## 如何新增 Dist Profile

1. 创建 `dist/<name>/`，编写 `main.go`（`_ import` 所需插件）
2. 初始化 `dist/<name>/go.mod`（replace 指向本地 core/plugins）
3. 在 `start.sh` 的 `_get_dist_tags()` 增加对应 tags（全功能可复用 `_FULL_FEATURE_TAGS`）
4. （可选）创建 `config/profiles/<name>/` 定义**部署默认依赖**
5. 更新本文件的矩阵表

## 约束

1. **业务能力由二进制决定，默认依赖由 Profile 决定**：gateway/team 插件对齐；SQLite vs 外部 PG 等是部署选择  
2. **import 与 tag 必须对齐**：尤其是带 `//go:build` 的后端/协议/业务插件  
3. **`Dockerfile.dist` 是 dist 镜像唯一构建入口**；本地日常开发走 `cmd/centag` + `Makefile`  
4. 每个 Config Profile 应映射到唯一 Dist Profile，并写清默认中间件依赖  
