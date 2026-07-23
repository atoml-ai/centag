# Centag — Agent 入口（Harness 取向）

本文件面向 **AI 编码智能体与人类协作者**：用仓库内可验证事实描述背景、常见任务、当前优先级与约束。细节以代码与 `docs/` 为准；执行中的任务状态见 `docs/versions/`（按版本归档），需求版本索引见 `docs/versions/README.md`。

> **Skill 分层总指引**在仓库根目录 **[AGENT.md](../../AGENT.md)**（业务正本 vs Agent 交互入口）。本文件是 Harness 详细总纲。

---

## 1. 项目是什么

**Centag**（Go）是高性能 **LLM 反向代理 / 网关**：统一 OpenAI 兼容入口、多后端调度、缓存（精确 + 语义）、插件化协议/后端/存储、Gin HTTP 服务、Vue3 Web 管理端（`web/` → 构建到 `var/static/`）。

- **Go 模块**: `centag`，`go 1.25.0`（见根 `go.mod` / `go.work`；子模块可声明更低的 `go` 版本，如 `apps/wrap` 为 `1.23.7`）。
- **主进程入口**: `cmd/centag/main.go`；迁移: `cmd/migrate`；其它入口见 `cmd/README.md`。
- **业务代码**: 主要在 `internal/`；可选实现与注册在 `plugins/`。

---

## 1.1 行为准则

### 不确定就问，不要猜

这是最重要的行为准则，优先级高于一切：

- **理解模糊时必须反问**：如果用户的需求描述不够清晰、有歧义、或可以有多种理解方式，必须先向用户提问确认，不得自行假设后直接执行
- **上下文不足时必须追问**：如果缺少做出正确决策所需的关键信息（如：业务规则、边界条件、技术约束、部署环境差异等），必须先问清楚再动手
- **有多种可能时必须列出选项**：如果一个问题有多种合理的实现方式，必须列出选项让用户选择，不得擅自选择其中一种
- **禁止想当然**：不得基于猜测编造业务规则、接口行为、数据结构或产品逻辑。不知道的就说不知道，然后问用户

### 透明度协议（对用户公开 AI 的工作上下文）

每次回复的**开头**，必须声明当前加载了哪些额外规范、Skill 或文档：

- 加载了额外规范时：`📋 额外生效的规范：harness-workflow — 研发工作流规范`
- 加载了 Skill 时：`🔧 加载 Skill：quality-gate — 门禁检查`
- 读取了关键文档时：`📖 读取上下文：docs/harness/ARCHITECTURE.md — 获取系统架构信息`
- 什么都没额外加载时：`ℹ️ 当前仅基础规范生效，未加载额外规则或 Skill。`

### 上下文声明（多需求多分支协作）

AI 在执行任务前，**必须确认用户当前的工作上下文**：

1. **主动询问**：如果用户没有说明，AI 必须先问："你当前在做哪个版本/哪个需求？你在哪个分支上？"
2. **自动推断**：如果能从对话上下文或已打开文件推断，则无需反复询问
3. **锁定上下文后**：自动关联对应的执行计划文档

---

## 2. 接口边界与约束（必须严格遵守）

> 完整接口清单见 **[INTERFACES.md](INTERFACES.md)**。

Centag 的核心定位是"LLM 代理 / 网关"，不是"面向业务场景的 REST 服务平台"。因此所有新能力都必须遵守以下约束：

- 不新增面向 Agent 的业务专用接口；
- 统一使用标准的大模型协议接口作为外部入口：OpenAI 兼容的 `/v1/chat/completions`、Anthropic 兼容的 `/v1/messages`，必要时再扩展 `/v1/embeddings`；
- 垂直领域能力必须通过 Pipeline / 场景包 / 配置来表达，而不是通过新增业务路由暴露；
- 需要切换场景时，通过请求头、配置或默认规则选择 Pipeline，例如 `X-Pipeline-ID`、`X-Proxy-Mode` 或相应的配置绑定；
- 管理类接口（如 `/api/v1/admin/*`、`/api/v1/pipelines/*`、`/api/v1/registry/plugins`）可保留为运维与管理能力，但不应被当作面向普通 Agent 的业务接口。

### 接口变更控制（严格遵守）

所有接口变更（新增、修改、删除、路径迁移）必须遵循以下流程：

1. **提案阶段**：在技术方案文档中说明新增接口的必要性、路径设计、认证方式
2. **审查阶段**：确认该接口不属于"禁止新增"类别，且无法通过 Pipeline / 插件 / 配置替代
3. **实现阶段**：实现后必须同步更新 [INTERFACES.md](INTERFACES.md)
4. **文档同步**：`docs/api/` 下的 API 文档必须同步更新

**禁止新增的接口类型**：
- 面向特定业务场景的专用接口（如 `/api/v1/education/*`、`/api/v1/healthcare/*`）
- 绕过 Pipeline 直接暴露业务逻辑的接口
- 重复已有协议接口功能的包装接口（如 `/api/v1/openai/*` 与 `/v1/*` 重复，保留仅为兼容）

**允许的接口变更**：
- 协议兼容性扩展（如新增 `/v1/responses` 等新协议端点）
- 管理接口的功能增强（在 `/api/v1/admin/*` 或 `/api/v1/*` 下新增运维功能）
- 插件 / 流水线相关的管理接口（`/api/v1/pipelines/*`、`/api/v1/registry/*`）

### 接口分类概览

| 分类 | 路径前缀 | 说明 |
|------|---------|------|
| 代理接口 | `/v1/*`、`/api/v1/openai/*` | 协议层，核心对外能力 |
| 管理接口 | `/api/v1/*`、`/api/auth/*` | 运维管理，仅限内部使用 |
| 其它接口 | 见 [INTERFACES.md §3](INTERFACES.md#3-其它接口待整理) | 待评估定位 |

### 核心与插件能力边界

> 完整能力矩阵见 **[CAPABILITY_BOUNDARY.md](CAPABILITY_BOUNDARY.md)**。

Centag 采用 **薄核心 + 厚管道 + 可插拔实现** 的三层架构：

| 层 | 目录 | 职责 | 设计约束 |
|----|------|------|---------|
| **核心层** | `internal/` | 协议翻译、请求编排、基础设施 | 不含业务逻辑，不含具体技术实现 |
| **流水线层** | `internal/pipeline/` | 处理链编排、节点调度 | 节点通过 CapabilityBroker 调用能力 |
| **插件层** | `plugins/` | 具体实现（协议/后端/存储/业务） | 通过接口契约与核心交互 |

**核心层只做**：HTTP 路由、协议翻译、请求转发、调度框架、流水线编排、用户认证/额度、Token 用量、Prometheus 指标。

**流水线节点做**：LLM 调用、缓存、内容审核、翻译、摘要、记忆、问题拆分/合成、PII 脱敏、Token 优化——均为**可选、可配置**的处理步骤。

**插件层做**：具体协议实现（OpenAI/Anthropic）、具体后端实现（各厂商 API）、具体存储实现（Redis/PG）、业务处理逻辑。

**判断新功能该放哪层**：
1. 所有 LLM 网关都需要？→ 核心层
2. 可编排的处理步骤？→ 流水线节点
3. 具体技术/厂商实现？→ 插件

> **结论**：未来任何"教育 / 办公 / 医疗 / 客服"等垂直场景能力，都应通过 Pipeline 和场景配置接入，而不是新增独立业务接口。

---

## 3. 真源与端口（避免与旧文档冲突）

| 项目 | 真源 | 说明 |
|------|------|------|
| HTTP 默认端口 | `internal/config/bootstrap.go`（`LLM_PROXY_SERVER_PORT`，默认 **20060**）、`start.sh` 内 `BACKEND_PORT=20060` | 本地/脚本文档以 **20060** 为准。 |
| 启动脚本 | `./start.sh` | Docker 子命令为 **`docker up`** / `docker down`（**空格**，不是 `docker-up`）。 |
| 中间件与 Mem0 等依赖编排 | 子模块 **`deploy/stack/`**（`./deploy/stack/start.sh`） | 本仓库 `docker/docker-compose.yaml` **仅**编排 `centag` 应用容器；数据库/Redis/ES/Mem0 等在 stack 启动。 |
| 环境变量与密钥 | `config/secrets/`、`docs/guide/configuration.md`、`docs/ENV_VARIABLES.md` | 勿把密钥写入仓库。 |
| 后端种子 | 各 `config/profiles/<edition>/initdata/initial-backends.yaml`（Profile 优先；参考目录 `_shared/backends-catalog.yaml` 非 seed） | 调度与路由相关。 |

若外部 README 仍出现 `8060` 或 `docker-up`，视为历史笔误；改代码前以 **本表 + bootstrap.go + start.sh** 为准。

---

## 3. 研发工作流（Spec-Driven Development）

> 完整工作流文档见 `docs/harness/workflow/README.md`。

本项目遵循 **5 阶段 6 步骤 5 门禁** 的统一研发工作流（Step 1 内含强制「开发风险评估」环节；Step 6 为发版），覆盖从方案设计到 GitHub Release 的完整链路。

### 3.1 工作流全景

```
Phase 1              Phase 2             Phase 3              Phase 4              Phase 5
方案设计              任务规划             编码实现              质量交付              发版
──────────           ──────────          ──────────           ──────────           ──────────

Step 1: 方案设计      Step 2: 任务规划     Step 3: SDD 编码      Step 5: CR 审查      Step 6: 发版
  与确认             (拆解+风险映射)         实现
  + 开发风险评估
                                           Step 4: 单元测试
                                              补全

──────────           ──────────          ──────────           ──────────           ──────────
     GATE 1 ──────→     GATE 2 ────────→     GATE 3 ──────→     GATE 4 ──────→     GATE 5
                                                              (CR/发版许可)         (发版准出)
```

### 3.2 快捷别名（Step 别名路由）

用户可通过别名直接触发对应步骤，AI 自动加载所需 Skill 并检查前置门禁：

| 别名 | 步骤 | 加载内容 | 前置门禁 |
|------|------|---------|---------|
| `step1-design` / 方案设计 | Step 1: 方案设计与确认 | 技术方案规范 | 无 |
| `step1-risk` / 开发风险评估 | Step 1 强制环节（可单独补齐） | 开发风险评估模板 | 无 |
| `step2-plan` / 任务规划 / 拆任务 | Step 2: 任务规划 | 任务拆解规范 | Gate 1 |
| `step3-code` / 编码 / 开始写代码 | Step 3: SDD 编码实现 | 编码执行规范 | Gate 2 |
| `step4-test` / 补测试 | Step 4: 单元测试补全 | 单元测试规范 | 无（同 Phase） |
| `step5-review` / CR / 代码审查 | Step 5: CR 审查 | 审查规范 + Gate 4 人工确认 | Gate 3 |
| `step6-release` / 发版 / release | Step 6: 全流程发版（GitHub + npm + CI） | `step6-release/` 正本 | **Gate 4** |

### 3.3 门禁链

| 门禁 | 位置 | 核心条件 | 触发时机 |
|------|------|---------|---------|
| Gate 1 | Phase 1 → 2 | 技术方案 + 开发风险评估落盘 + Critical=0 | `step2-plan` |
| Gate 2 | Phase 2 → 3 | 任务计划落盘 + High 风险已映射任务 | `step3-code` |
| Gate 3 | Phase 3 → 4 | 全量测试通过 + 覆盖率达标 | `step5-review` |
| Gate 4 | Phase 4 → 5 | CR Critical=0 + 产物齐全 + **人工批准可发版** | Step 5 确认；`step6-release` 再检 |
| Gate 5 | 发版准出 | Release 资产 + 冒烟（或记录跳过） | Step 6 完成时 |

### 3.4 产物路径

```
docs/versions/<版本>/<需求>/
├── workflow_state.md         ← 工作流状态追踪（各步骤/门禁进度）
├── 技术方案.md               ← Step 1
├── 开发风险评估.md           ← Step 1（强制）
├── 任务计划.md               ← Step 2
├── 自测记录.md               ← Step 5
└── CR_报告.md                ← Step 5
```

> 版本索引见 `docs/versions/`，按版本汇总需求清单与发布追踪。

各阶段详细执行指南：

- [Phase 1: 方案设计](workflow/phase-1-design.md)
- [Phase 2: 任务规划](workflow/phase-2-plan.md)
- [Phase 3: 编码实现](workflow/phase-3-implement.md)
- [Phase 4: 质量交付](workflow/phase-4-deliver.md)
- [Phase 5: 发版](workflow/phase-5-release.md)
- [门禁检查清单](workflow/gate-checklist.md)

---

## 4. 常见开发场景（按需深入）

| 场景 | 从哪开始 | 常用命令 |
|------|-----------|----------|
| 只跑后端 | `internal/server/server.go`、`internal/config/` | `./start.sh run be` 或 `make run`（见 Makefile） |
| 前后端联调 | `web/`、`internal/server/` 静态路由 | `./start.sh debug` 或终端分别 `run be` / `run fe` |
| Docker 应用容器 | `docker/docker-compose.yaml`、`start.sh` 中 `docker_*` | `./start.sh docker up`（仅拉起 centag；依赖由 **deploy/stack** 提供） |
| 缓存 / 语义 | `internal/cache/`、`docs/cache/` 下专题 | 需 Redis/ES/Chroma 时先在 **deploy/stack** 启动对应服务并配置 `config/secrets/.env` 地址 |
| 调度 / 路由 | `internal/scheduler/`、`internal/router/`、`docs/scheduler-api.md` | 单测在对应 `*_test.go` |
| 插件 | `plugins/`、`internal/plugin/` | 注册多为空白 `_ import` |
| OpenClaw / 云记忆（已归档） | `archive/deprecated/agents/openclaw/README.md` | 见下文 **§4.2**（Gateway 端口、memory-local-bridge、云记忆文档索引）。 |
| Hermes Agent | `archive/deprecated/agents/hermes-agent/HERMES_AGENT_CENTAG_INTEGRATION.md` | Hermes 将 Centag 作为 **LLM + 记忆（云记忆）** 提供方；见 **§4.3**。 |

人类可读索引：`docs/README.md`（含 API、运维、架构子文档链接）。

### 4.1 代理模式（请求侧最重要）

客户端通过 **`X-Proxy-Mode`**（及配套头）控制走法；实现侧统一枚举见 `internal/proxymode/execution_mode.go`（`#d` / `#s` / `#m` 等与 Preset/Web 对齐）。与下列用户文档对应时，注意示例里的 **`localhost` 端口以 §2 真源（默认 20060）为准**，勿照抄旧文 `8060`。

| 模式（文档常用名） | 典型请求头 | 行为摘要 |
|-------------------|------------|----------|
| **透明模式（默认）** | 不传 `X-Proxy-Mode`，或 `transparent-proxy` / `#t` / `#tf` | **不注入** system prompt，尽可能原样透传；若客户端指定 model，则跨已启用后端松匹配（如 `mino2.5` ≈ `mino2.5 free`）并优先该后端，未指定/未命中回落系统默认。初始化默认见 `config.DefaultSystemPipelineID`。 |
| **直连后端** | `X-Proxy-Mode: direct-backend`（`#d`） | 节点配置的后端/模型（`{{system.default_*}}` → 系统默认）+ **注入**网关 system prompt（覆盖客户端 system）；**不**按客户端 model 跨后端选路。 |
| **系统调度 / 智能调度** | `smart-scheduling` / `system-scheduling` / `#s` | 按模型与后端能力、权重与负载做路由与调度（`ModeSystemScheduling`）；策略相对透明/直连定位暂不变。 |
| **原始转发** | `X-Proxy-Mode: raw-forward`（`#raw`）+ `X-Target-URL` / hostproxy | 高级 HTTP 透传；非聊天默认路径。 |
| **聚合** | `X-Proxy-Mode: aggregator-mode` | 多模型并行生成 → 聚合结果（`ModeAggregator` / `#ag`）。 |
| **路由** | `X-Proxy-Mode: router-mode` | 意图/关键词路由决策 → 条件分支处理（`ModeRouter` / `#r`；`#c` 已合并至 `#r`）。 |
| **翻译** | `X-Proxy-Mode: translate-mode` | 生成 → 翻译两阶段处理（`ModeTranslate` / `#l`）。 |

**扩展（调度链 / 预设里常见，非上述三者但同属执行模式）**：**模型匹配**（`model-matching` / `#m`）、**降级**（`fallback` / `#f`）、**审核**（`audit-mode` / `#a`）、**优化**（`optimize-mode` / `#o`）、**Mem0记忆**（`mem0-memory` / `#mem0`）、**自定义**（`#custom`）— 与 Pipeline / Preset / Web「代理模式」页联动，细节见 `docs/guide/proxy-modes.md`、`docs/guide/mode-behavior-matrix.md`、以及 `internal/proxy/types.go` 等。

### 4.2 OpenClaw 与云记忆

- **OpenClaw Gateway（已归档）**：配置唯一源与渲染流程见 **`archive/deprecated/agents/openclaw/README.md`**。端口以 **`archive/deprecated/agents/openclaw/README.md` 为准**：**系统级** `openclaw system` → **18789**（`~/.openclaw/`）；**项目级** `openclaw project` → **18790**（`./archive/deprecated/agents/openclaw/.openclaw/`）。`deploy` / `project-sync` / `run` 与官方向导、**`OPENCLAW_CONFIG_PATH`** 的区分见该 README「你只要记住三件事」与 **Control UI / token** 章节。
- **云记忆（库内记忆 + bridge 联调）**：Centag 侧 DB/pgvector、嵌入维度、管理端 Key 与 **`memory-local-bridge`** 对齐方式见 **`archive/deprecated/agents/openclaw/docs/CLOUD_MEMORY_CONFIG_AND_PROMPTS.md`**；速查规则摘要见 **`archive/deprecated/agents/openclaw/plugins/memory-local-bridge/store/main/memory/centag-rules.md`**（其中「云记忆」一节）。插件与 OpenClaw 栈目录：**`archive/deprecated/agents/openclaw/plugins/`**。

### 4.3 Hermes Agent

- 部署与配置 Hermes 将 **模型** 与 **记忆（云记忆）** 的 `provider` 指向 Centag（OpenAI 兼容 `base_url`、API Key、记忆 API 等）见 **`archive/deprecated/agents/hermes-agent/HERMES_AGENT_CENTAG_INTEGRATION.md`**。默认与本仓库 HTTP 端口一致时为 **`http://localhost:20060/v1`**（以实际 `LLM_PROXY_SERVER_PORT` 为准）。

---

## 5. 当前进度与优先级（仓库内结论）

汇总自近期 pipeline/plugin 改造与 2026-05-10 代码复核：

- **状态**: Phase A/B 主体已落地：`CapabilityBroker` 主链路接入、`DBPluginRegistryStore` 持久化、远程节点并发/生命周期治理、模式模板映射与 `inputs` 表达式增强均已有实现和测试覆盖。
- **2026-05-21 新增**: `question_splitter`、`answer_synthesizer`、`tasktype_detector` 已迁移到 BusinessPlugin 体系（与 optimizer / translator / reviewer / summarizer 一致），Handler 层插件优先 + fallback 回退已落地，见 `docs/guide/processor-plugins.md`。
- **P0（建议优先）**:
  - 保持代理模式 `mode -> pipeline_id` 与内置/initdata 模板 ID 一致；新增或改名模板时同步 `internal/proxy/pipeline_mode.go`、`internal/pipeline/config.go`、`config/initdata/pipeline-templates/` 与 `docs/guide/mode-behavior-matrix.md`。
  - 插件/流水线进度与待办以对应版本的 `docs/versions/` 目录为准；processor 插件化迁移已完成。
- **P1**:
  - Phase C 增量：模板编辑器与权限/版本提示细化；执行历史节点级审计摘要（若产品需要）；远程响应签名链路等。
  - Phase D 扩展：发现协议、制品分发、Loader 真实运行时、多租户配额等（最小闭环已完成：市场表 `plugin_market_*`、`/api/v1/registry/plugins`、CLI publish）。
  - **Pi Agent 集成**：将 `deploy/stack/services/pi-sandbox` 中已实现的 Pi Agent 沙盒能力接入 Centag 核心（代理模式、Pipeline 模板、业务插件、WebUI）。技术文档见 `docs/guide/pi-agent.md`。分支：`feature/pi-agent-integration`。

**进行中 / 可认领任务** 请写在 `docs/versions/` 对应版本目录下。

---

## 6. 下一步（给后续 Agent 的默认 checklist）

1. **先读版本目录**：`docs/versions/` 下查找对应版本的需求。
2. **确认工作流阶段**：若任务属于研发工作流，先读 `docs/harness/workflow/README.md` 确认当前 Phase，检查 `workflow_state.md`，通过 step 别名（`step1-design` → `step6-release`）进入对应环节。
3. 阅读与本任务相关的 `docs/versions/` 条目（若有）。
4. 小步修改：`internal/` 保持清晰分层；新增行为配测试（`go test`，见 CI）。
5. API 或配置行为变更：同步 `docs/api/` 或 `docs/guide/` 中对应页。
6. **更新 workflow_state**：每个 Step 完成后更新 `workflow_state.md`。
7. 提交前本地对齐 CI：`make test`、`make lint`、`make harness-check`，`web/` 下 `npm run lint:ci`。
8. 不扩大范围：避免无关格式化与无关文件的重命名。

---

## 7. 结构约束（防熵增）

→ 目录以根 [`README.md`](../../README.md) 与本仓库实际树为准；本地构建/运行产物默认与安装布局一致（`~/.centag/`）。

**根目录速查**（Centag）：

| 目录 | 用途 |
|------|------|
| `cmd/` `core/` `plugins/` `sdk/` | Go 模块核心 |
| `web/` | Vue 管理端 |
| `apps/launcher/` | 可选桌面启动器（L1：菜单/托盘+浏览器；独立 go.mod，不入 go.work） |
| `apps/wrap/` | 本机 PAC/CA 一键工具；独立 go.mod，**已加入**根 `go.work`，并由 `centag wrap` 子命令嵌入主二进制（见 `docs/guide/system-proxy-egress.md`） |
| `config/initdata/` `config/profiles/` `config/secrets/` | 种子数据、场景 Profile、本地密钥 |
| `deploy/docker/` `deploy/stack/` `deploy/fnos/` | 容器编排与 NAS 打包 |
| `scripts/` | 开发/运维脚本与集成测试 |
| `docs/` `docs/harness/` | 人类文档与 Agent 规范 |
| `docs/versions/` | 版本归档（新需求产物） |
| `dist/` | 发行版入口源码（minimal / personal / team） |
| `~/.centag/` | 本地构建与正式安装共用根：`bin/` PATH、`lib/<edition>/` 二进制与 static、`var/` 非发布中间物 |
| `bin/`（仓库内） | 遗留/兼容路径（勿再作为默认输出；gitignore） |

- **`cmd/` / `dist/`**: 仅入口与组装，避免写复杂业务逻辑。
- **`core/`**: 核心业务；依赖方向应 **向内**，避免循环依赖。
- **`plugins/`**: 可插拔实现（protocol / backend / database / storage）；business 插件外置。
- **勿** 在测试中硬编码本机绝对路径或依赖个人机器上的运行时数据库。

CI 与脚本侧会做 **轻量卫生检查**（`scripts/check-harness-hygiene.sh`）：关键文档存在、根目录无遗留旧路径、`go list` 排除 `web/node_modules`。

---

## 8. Harness Engineering 规范文档

本项目遵循 Harness Engineering 规范，提供以下核心文档供 Agent 参考：

| 文档 | 用途 | 何时读取 |
|------|------|----------|
| **AGENTS.md**（本文件） | 项目总纲、入口、真源、工作流 | 首次进入项目 |
| **ARCHITECTURE.md** | 架构约束、分层结构、模块边界 | 修改架构、添加模块 |
| **CAPABILITY_BOUNDARY.md** | 能力边界矩阵、扩展点、分层判断 | 新功能该放哪层 |
| **CONVENTIONS.md** | 编码规范、命名规则、文件组织 | 编写新代码 |
| **PATTERNS.md** | 设计模式、代码模板 | 实现新功能 |
| **ANTI-PATTERNS.md** | 反模式、禁止行为 | 代码审查、重构 |

**阅读顺序建议**：
1. 新任务：先读 AGENTS.md → 相关 ARCHITECTURE.md → PATTERNS.md
2. 编码时：参考 CONVENTIONS.md
3. 提交前：检查 ANTI-PATTERNS.md

### 8.1 Skill 分层（强制）

> 根目录 **[AGENT.md](../../AGENT.md)** 为全仓库 Agent 入口指引；本节是 Harness 细节索引。

| 层级 | 路径 | 职责 |
|------|------|------|
| 总指引 | `/AGENT.md` | 标明正本与入口关系、交接契约 |
| **业务正本** | `docs/harness/skills/` | 唯一业务定义：步骤、判定、脚本、映射 |
| **交互入口** | `.cursor/`、`.opencode/`、… | 仅触发词 + 该 Agent 控件收参 + 交接正本 |

入口收参完成后，**必须**加载 `docs/harness/skills/` 正本执行；禁止在 Agent 目录复制业务步骤。明细见 [skills/README.md](skills/README.md)。

### 9.1 Cursor 规则（`.cursor/rules/*.mdc` — 仅交互）

| 规则文件 | 类型 | 用途 |
|---------|------|------|
| `harness-ask-ui.mdc` | `alwaysApply: true` | 固定选项必须用 AskQuestion |
| `centag-wizard-test.mdc` | 按需 | 向导测试收参 → `skills/centag-wizard-test.md` |
| `centag-pipeline-test.mdc` | 按需 | 流水线测试收参 → `skills/centag-pipeline-test.md` |
| `centag-admin-e2e.mdc` | 按需 | 管理 E2E 收参 → `skills/centag-admin-e2e.md` |
| `centag-ui-browser-test.mdc` | 按需 | WebUI 浏览器验收收参 → `skills/centag-ui-browser-test.md`（强制 Browser MCP） |
| `harness-baseline.mdc` / `harness-workflow.mdc` | 待补 | 基础规范 / step 路由 |
| `centag-core.mdc` / `centag-deploy.mdc` | 待补 | 核心操作 / 部署交互入口 |
| `step6-release.mdc` | 按需 | Step 6 发版收参 → `skills/step6-release/` |

### 9.2 技能正本映射（`docs/harness/skills/`）

| 触发场景 | 技能正本 |
|---------|---------|
| 方案设计与确认（含开发风险评估） | `skills/step1-design/SKILL.md` |
| 任务规划 | `skills/step2-plan/SKILL.md` |
| 编码实现 | `skills/step3-code/SKILL.md` |
| 单元测试补全 | `skills/step4-test/SKILL.md` |
| CR 审查 | `skills/step5-review/SKILL.md` |
| 发版（Step 6） | `skills/step6-release/SKILL.md` + `procedure.md`（渠道：GitHub / npm / CI） |
| 门禁检查 | `skills/quality-gate/SKILL.md` |
| 核心操作（构建/运行/调试） | `skills/centag-core.md`（待补） |
| 向导式部署 | `skills/centag-deploy.md`（待补） |
| GitHub Release（兼容旧名） | `skills/centag-release.md` → 重定向至 Step 6 |
| 流水线模式测试 | `skills/centag-pipeline-test.md` |
| 向导式全面测试 | `skills/centag-wizard-test.md` |
| 管理功能端到端测试（HTTP） | `skills/centag-admin-e2e.md` |
| WebUI 浏览器自动化验收 | `skills/centag-ui-browser-test.md` |

### 9.3 工作流 Step 别名 → 执行指引映射

| 用户说 | 加载 Skill | 执行指引 |
|-------|-----------|---------|
| `step1-design` / 方案设计 | `skills/step1-design/SKILL.md` | `workflow/phase-1-design.md` |
| `step1-risk` / 开发风险评估 | `skills/step1-design/SKILL.md` | `templates/开发风险评估模板.md` |
| `step2-plan` / 任务规划 | `skills/step2-plan/SKILL.md` | Gate 1 → `workflow/phase-2-plan.md` |
| `step3-code` / 编码 | `skills/step3-code/SKILL.md` | Gate 2 → `workflow/phase-3-implement.md` |
| `step4-test` / 补测试 | `skills/step4-test/SKILL.md` | `workflow/phase-3-implement.md` §Step 4 |
| `step5-review` / CR | `skills/step5-review/SKILL.md` | Gate 3 → `workflow/phase-4-deliver.md`（确认 Gate 4） |
| `step6-release` / 发版 | `skills/step6-release/SKILL.md` | Gate 4 → `workflow/phase-5-release.md` + `procedure.md` |
| 门禁检查 | `skills/quality-gate/SKILL.md` | `workflow/gate-checklist.md` |

| 规范文件 | 说明 | 何时触发 |
|---------|------|---------|
| `/AGENT.md` | Skill 分层与交接契约 | 任意 Agent 进入项目 |
| `.cursor/rules/harness-ask-ui.mdc` | AskQuestion 优先 | Cursor 会话始终 |
| `.cursor/rules/centag-*-test.mdc` | 测试类交互入口 | 触发对应测试词 |
| `docs/harness/workflow/` | 工作流各阶段执行指南 | 进入对应 Phase 时 |
| `docs/harness/workflow/gate-checklist.md` | 门禁检查清单 | 跨 Phase 过渡时 |
| `docs/harness/templates/` | 文档模板 | 需要创建标准化产物时 |
| `docs/versions/` | 版本索引与需求清单 | 版本规划或发布时 |

---

## 10. 故障与自检

- 测试：`make test` 或 `scripts/ci-go-packages.sh | xargs go test -count=1`。
- Lint：`make lint`（需安装 `golangci-lint`，版本建议与 `.github/workflows/ci.yml` 一致）。
- 合并门禁：见 GitHub Actions workflow `ci.yml`。
- Harness 卫生检查：`make harness-check` 或 `bash scripts/check-harness-hygiene.sh`。

## 11. 文档维护

若本文件过长，新增内容优先进入 `docs/versions/` 或专题 `docs/`，此处只保留 **索引与不变量**。

**Harness 规范文档更新规则**：
- 修改架构 → 同步更新 ARCHITECTURE.md
- 添加新模式 → 更新 PATTERNS.md
- 发现反模式 → 更新 ANTI-PATTERNS.md
- 代码规范变更 → 更新 CONVENTIONS.md
- 工作流变更 → 同步更新 workflow/ 目录下对应文件
- 版本规划 → 在 docs/versions/ 下创建/更新版本 README
