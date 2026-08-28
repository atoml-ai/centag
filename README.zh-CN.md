# Centag

> 🚀 **Visual‑DAG LLM Proxy Hub**
Centag 是自托管 LLM 网关/代理枢纽，区别于普通转发代理：支持**可视化拖拽DAG流水线编排**，搭配 `centag wrap` 零代码进程代理，不用修改业务一行代码，即可实现路由、鉴权、缓存、校验、故障转移、业务策略编排，内置完整 WebUI。

普通 LLM 网关只做请求转发；Centag 把业务策略变成可编辑的可视化流水线。

✅ 开箱即用 OpenAI‑兼容接口
✅ 可视化流水线编辑器（DAG）
✅ centag‑wrap：进程级无侵入代理，无需改代码
✅ 多 Key 轮询、故障降级、限流、请求校验、缓存
✅ 完整 WebUI，可观测全链路请求
✅ Docker / npm 快速部署

---

## Why Centag vs LiteLLM / 其他 LLM Gateway

| Feature | Centag | LiteLLM‑Proxy |
|---|---|---|
| OpenAI兼容代理 | ✅ | ✅ |
| 多key负载均衡/故障转移 | ✅ | ✅ |
| WebUI管理面板 | ✅ | 基础 |
| **可视化DAG流水线编排** | ✅ | ❌ |
| 进程级零侵入 wrap 代理 | ✅ | ❌ |
| 请求中间件链式编排 | ✅ | 需代码开发 |
| 缓存、守卫、改写节点可视化配置 | ✅ | 代码/配置文件 |

> 如果你只需要简单多key转发，Centag 也可以直接用；如果你需要给LLM请求叠加复杂业务策略，Centag 是专门为此设计。

---

## ⚡ Quick Start（5分钟跑通）
```bash
# npm
npm install -g centag
centag serve

# or docker
docker run -p 8000:8000 atomlai/centag


## 我们解决什么问题

一般的 LLM「中转站」只做一件事：把请求原样转发出去。Key 挂了要人换，模型选错要人改，换 Agent 又要重新配一遍——策略散落在各个工具里，网关本身没有策略。

**Centag 不是中转站，而是可编排的代理中枢：** 后端池、容错降级、场景路由、计量计费都收敛到同一条流水线里，Agent 侧几乎无感。

| 能力 | 你得到什么 |
|---|---|
| **后端大模型池管理** | OpenAI、Anthropic、智谱、Ollama 及任意兼容端点统一纳管；多 Key、多后端在 Web 上一处配置 |
| **自动容错 · 匹配 · 降级** | Key 限流自动轮转；后端故障自动切换；按模型能力与负载匹配最优出口，服务持续可用 |
| **模型路由** | 按问题类型实时切换后端大模型；即使在同一会话同一个任务内也可智能动态换模，客户端无需改配 |
| **Agent 场景切换** | 编码、问答等场景各用一条流水线——换场景等于换策略，Agent 无感 |
| **快速接入 Agent** | 常用 Agent 支持一键写入配置；也可用 `centag wrap` 进程代理零改动接入；尚未一键适配的提供 Web UI 配置指引。支持列表持续扩充 |
| **System Prompt 策略** | 对客户端 system prompt 支持透传、追加、替换——可保留 Agent 原有人设，也可按场景叠加规范或统一覆盖，流水线级灵活配置 |
| **计量与计费** | Token 与费用按请求、后端、模型可追踪，成本一目了然 |
| **高性能无损接入** | 透明转发与 SSE 透传，协议兼容、低开销，尽量不改写上游语义 |

---

## 核心优势

### 可视化流水线编排

中转站只会转发。**Centag 让你设计请求的完整生命周期**——在画布上拖拽编排 DAG，流水线就是策略。

**16 种内置节点**，自由组合：

| 节点 | Kind | 作用 |
|------|------|------|
| Generator | `llm.generate` | 调用任意 LLM 后端生成内容 |
| Router | `route.decide` | 按意图、关键词或 LLM 分类分支 |
| Scheduler | `scheduling.decide` | 跨后端智能调度与匹配 |
| Transparent Forward | `proxy.transparent_forward` | 原始 HTTP 代理（SSE 透传） |
| Aggregator | `aggregate.merge` | 并行多生成器合并 / 投票 / 择优 |
| Reviewer | `quality.review` | 评分审核上游回答 |
| Memory | `memory.query` | 云记忆 / 本地向量召回上下文 |
| Audit | `audit.safety` | 内容审核与安全过滤 |
| Token Usage | `metrics.token_usage` | Token 消耗与成本追踪 |
| Cache | `cache.access` | 缓存读写（精确 / 语义 / 混合） |
| Processor | `content.transform` | 内容变换与后处理 |
| Tool Call | `inject.tool_call` | 注入 Function Calling 工具 |
| Prompt Ops | `prompt.ops` | 用户 Prompt 预处理 |
| Output Post-ops | `prompt.postprocess` | 输出后处理 |
| Loop Controller | — | 循环控制，支持迭代工作流 |
| Plugin Node | *(远程 / 业务)* | 自定义节点，HTTP 或 Go SDK 接入 |

**流水线 = 策略。** 换场景 → 换流水线 → Agent 不改一行代码。

| 场景 | 流水线示例 |
|------|-----------|
| 编码助手 | 路由 → 编码专用模型 → 代码审查 |
| 智能调度 | 意图识别 → 模型能力匹配 → 容错降级 |
| 企业合规 | 安全审核 → 生成 → PII 脱敏 → 合规审计 |
| 客服 / RAG | 记忆或检索召回 → 生成 → 质量审核 |

### 统一后端与 Key 池

| 能力 | 说明 |
|------|------|
| **多后端管理** | 主流厂商与 OpenAI 兼容端点，Web 上统一管理 |
| **API Key 池化** | 每后端多 Key；限流或宕机时自动轮转 |
| **自动容错与降级** | Key 失败 → 下一 Key；后端故障 → 下一后端 |
| **智能匹配** | 权重、优先级、模型能力匹配，选最优出口 |
| **成本追踪** | 按请求、后端、模型统计 Token 与费用 |

### 快速接入 Agent — 三种方式

把 Agent 接到 Centag，不必改业务代码。按适配程度任选其一：

| 方式 | 适用 | 说明 |
|------|------|------|
| **一键写入配置** | 已适配的常用 Agent | Web UI 一键写入 Base URL / API Key 等，即连即用 |
| **centag wrap 进程代理** | 希望零改配置 | 进程级透明代理，不改 Agent 配置与代码即可把流量导到 Centag |
| **UI 配置指引** | 尚未一键适配的 Agent | 页面内逐步说明如何手动指向网关，照做即可 |

常用 Agent 持续适配增加；未覆盖的也可先用指引或 wrap 接入。

```bash
# 启动 Centag
centag

# wrap 方式示例——不改 Agent 配置
centag wrap run -- opencode

# 自检
centag wrap doctor
```

### 开放插件生态

流水线节点可扩展：Go SDK 本地插件，或任意语言的远程 HTTP 插件。

```go
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

远程插件约定：

```
GET  /.well-known/centag-node-plugin.json   →  自动发现
POST /validate                               →  配置校验
POST /execute                                →  执行节点
```

---

## 快速上手

```bash
# 1. 安装（任选其一）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# 或
npm install -g @atomlai/centag

# 2. 启动
centag

# 3. 打开 Web UI → http://localhost:20060 → 添加第一个后端

# 4. 接入 Agent（一键写入配置，或 wrap 零改动）
centag wrap run -- opencode
```

搞定。流量经 Centag：共享后端池、自动容错、模型路由、成本可视。

> **默认登录：** 用户名 `admin` —— 首次启动的初始化向导中自行设置密码（无预置密码）。也可在首次启动前通过 `LLM_PROXY_ADMIN_PASSWORD` 预置。

### 其他安装方式

<details>
<summary>npm（不改全局路径）</summary>

```bash
npx --yes @atomlai/centag
```
</details>

<details>
<summary>离线 / 内网安装</summary>

```bash
npm install -g @atomlai/centag-offline
```
</details>

<details>
<summary>Docker（从源码）</summary>

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 按需修改密钥
./start.sh docker build personal                     # 构建镜像
./start.sh docker up personal                        # 启动容器
```

管理界面：http://localhost:20060 · 停止：`./start.sh docker down`

所有持久化数据在 `deploy/docker/data/` 目录（首次启动自动创建）。

<details>
<summary>原生 Docker 命令（替代方式）</summary>

```bash
# 构建
docker build -t centag-personal:latest \
  --build-arg DIST_NAME=personal \
  --build-arg INCLUDE_FRONTEND=true \
  -f deploy/docker/Dockerfile.dist .

# 运行
docker run -d --name centag \
  --env-file config/secrets/.env \
  -e CENTAG_EDITION=personal \
  -e LLM_PROXY_DB_DRIVER=sqlite \
  -e SQLITE_PATH=/app/storage/centag.db \
  -e LLM_PROXY_LOG_OUTPUT=both \
  -e LLM_PROXY_LOG_FORMAT=console \
  -p 20060:20060 \
  -v $(pwd)/deploy/docker/data/storage:/app/storage \
  -v $(pwd)/deploy/docker/data/logs:/app/logs \
  centag-personal:latest

# 停止并删除
docker stop centag && docker rm centag
```

</details>
</details>

---

## 截图

<p align="center">
  <strong>仪表盘</strong><br/>
  <img src="docs/assets/readme/screenshot-dashboard.png" alt="仪表盘" width="900" />
</p>

<p align="center">
  <strong>流水线可视化编辑器</strong><br/>
  <img src="docs/assets/readme/screenshot-pipeline-visual-editor.png" alt="流水线可视化编辑器" width="900" />
</p>

<p align="center">
  <strong>Agent 配置</strong><br/>
  <img src="docs/assets/readme/screenshot-agent-config.png" alt="Agent 配置" width="900" />
</p>

<p align="center">
  <strong>Token 用量与计费</strong><br/>
  <img src="docs/assets/readme/screenshot-token-usage.png" alt="Token 用量与计费" width="900" />
</p>

---

## 代理模式 — 开箱即用

内置多种场景化流水线模板（可按 `#` 快捷码切换）：

| 模式 | 快捷码 | 说明 |
|------|--------|------|
| 智能调度 | (默认) | 基于模型兼容性与后端负载智能路由 |
| 透明代理 | `#t` | 原样转发——高性能无损，不注入 system prompt |
| 直连后端 | `#d` | 固定出口 + 托管 system prompt |
| 容错 | `#f` | 跨后端自动降级 |
| 路由 | `#r` | 意图感知的多分支路由（场景 / 模型自动切换） |
| 审核 | `#a` | 生成 → 质量审核 → 反馈 |
| 优化 | `#o` | 生成 → 内容优化 |
| 聚合 | `#ag` | 并行多模型生成 → 合并结果 |
| 安全防火墙 | `#sec` | 安全审核 → 生成 → PII 脱敏 |
| RAG 网关 | `#rag` | 缓存优先的检索增强生成 |
| 地理路由 | `#geo` | 基于规则的区域路由 |
| Pi Agent | `#pi` | 代码任务 → 沙箱；问答 → LLM |
| CI/CD Webhook | — | 从外部系统触发流水线 |

真正的闪光点是**自定义流水线**——在画布上设计你自己的 DAG。

---

## 文档

| 主题 | 链接 |
|------|------|
| 完整文档索引 | [docs/README.md](docs/README.md) |
| 流水线插件标准 | [docs/guide/pipeline-plugin-standard.md](docs/guide/pipeline-plugin-standard.md) |
| Processor 插件指南 | [docs/guide/processor-plugins.md](docs/guide/processor-plugins.md) |
| 流水线变量参考 | [docs/guide/pipeline-variables.md](docs/guide/pipeline-variables.md) |
| 代理模式 | [docs/guide/proxy-modes.md](docs/guide/proxy-modes.md) |
| 后端配置 | [docs/guide/backend-configuration.md](docs/guide/backend-configuration.md) |
| 本机代理 / wrap | [docs/guide/system-proxy-egress.md](docs/guide/system-proxy-egress.md) |
| 环境变量 | [docs/guide/environment-variables.md](docs/guide/environment-variables.md) |
| API 参考 | [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) |
| 安全 | [docs/security/](docs/security/) |

---

## 反馈与支持

有问题或建议？请提 [GitHub Issues](https://github.com/atoml-ai/centag/issues)，或发邮件 **centag@atoml.com**。

---

## 参与贡献

欢迎开发者加入，一起开发与维护 Centag。无论是修 bug、加功能、写文档，还是适配更多 Agent，都欢迎通过 [Pull Request](https://github.com/atoml-ai/centag/pulls) 或 [Issues](https://github.com/atoml-ai/centag/issues) 参与进来。

---

## 许可证

MIT License（开源发行版：`minimal` / `personal`）
