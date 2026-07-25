# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

---

## ⚡ 你的 LLM 代理中枢 — 流水线即策略

**别再逐个 Agent 配置了。** Centag 用一个网关统一管理所有后端、API Key 和代理策略——全部以可视化、可替换的流水线串联起来。

![Centag Architecture Flow](docs/assets/readme/hero-architecture.png)

**一个网关，统一所有后端。流水线就是你的策略，Agent 只管写代码。**

---

## 🎯 我们解决什么问题

> **你肯定遇到过：**
>
> - 五个 Agent 各自配了同一个 API Key——一个 Key 挂了，全线中断。
> - 想给对话加个合规审核？得重写每个 Agent 的代码。
> - 在编码和翻译场景之间切换？每个工具重新配一遍。
>
> **Centag 让这些问题全部消失。**

| 你真正需要的 | Centag 怎么做 |
|---|---|
| **快速切换后端** | 所有后端统一管理——Web 上一键切换，Agent 侧零改动 |
| **自动容错 + Key 池** | 多 Key 轮转；限流或宕机时自动换 Key，服务不断 |
| **按场景定制策略** | 编码、翻译、合规各建一条流水线——换场景等于换策略，Agent 无感 |
| **用量与成本可视化** | Token 和费用追踪，每笔消耗一目了然 |

---

## ⭐ 核心优势

### 🎨 可视化流水线编排 — 独有能力

大多数 LLM Proxy 只做路由转发。**Centag 让你 *设计* 请求的完整生命周期**——在可视化画布上拖拽编排 DAG。

![Pipeline Architecture — Visual DAG Orchestration](docs/assets/readme/pipeline-canvas.png)

**16 种内置节点**，自由组合：

| 节点 | Kind | 作用 |
|------|------|------|
| 🤖 Generator | `llm.generate` | 调用任意 LLM 后端生成内容——核心生成节点 |
| 🔄 Processor | `content.transform` | 翻译、摘要、优化内容 |
| 🛡️ Reviewer | `quality.review` | 评分审核上游回答 |
| 🔀 Router | `route.decide` | 按意图、关键词或 LLM 分类分支 |
| ⚖️ Aggregator | `aggregate.merge` | 并行多生成器输出合并/投票/择优 |
| 🧠 Memory | `memory.query` | 从云记忆 / 本地向量召回上下文 |
| 🔒 Audit | `audit.safety` | 内容审核与安全过滤 |
| 💰 Token Usage | `metrics.token_usage` | Token 消耗与成本追踪 |
| 📦 Cache | `cache.access` | 缓存读写（精确/语义/混合） |
| ⏱️ Scheduler | `scheduling.decide` | 跨后端智能调度 |
| 🔌 Transparent Forward | `proxy.transparent_forward` | 原始 HTTP 代理（SSE 透传） |
| 🛠️ Tool Call | `inject.tool_call` | 注入 Function Calling 工具 |
| ✂️ Prompt Ops | `prompt.ops` | 用户 Prompt 预处理 |
| 📝 Output Post-ops | `prompt.postprocess` | 输出后处理 |
| 🔄 Loop Controller | — | 循环控制，支持迭代工作流 |
| 🔌 Plugin Node | *(远程/业务)* | 你的自定义节点，通过 HTTP 或 Go SDK 接入 |

**流水线 = 策略。** 换场景 → 换流水线 → Agent 不改一行代码。

| 场景 | 流水线 |
|------|--------|
| 🧑‍💻 编码助手 | 路由 → 编码专用模型 → 代码审查 |
| 🌐 翻译场景 | 生成 → 翻译 → 格式校验 |
| 🏢 企业合规 | 安全审核 → 生成 → PII 脱敏 → 合规审计 |
| 🤖 客服场景 | 记忆召回 → 生成 → 多语言翻译 |

---

### 🧩 开放插件生态 — 无限扩展

Centag 的流水线节点**不是封闭的**。你可以通过三种方式扩展：

![Plugin Ecosystem — Extend Everything](docs/assets/readme/node-plugins.png)

**能力抽象而非实现绑定**——你声明"需要什么"，不关心"怎么实现"：

| 能力 | 含义 | 典型实现 |
|------|------|---------|
| `memory` | 记忆召回/存储/搜索 | 云记忆、远程 HTTP、本地向量 |
| `token` | Token 优化 | 智能截断、语义摘要 |
| `prompt` | 提示词处理 | 模板引擎、动态优化 |
| `security` | 安全与合规 | 内容审核、PII 脱敏 |
| `router` | 意图分类与路由 | 意图驱动、负载均衡 |
| `monitor` | 成本/延迟/质量追踪 | 成本分析、质量评估 |

**几分钟写一个插件：**

```go
// 实现这个接口——就这么简单
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

或部署一个 **远程插件服务**（任意语言的 HTTP 服务即可）：

```
GET  /.well-known/centag-node-plugin.json   →  自动发现
POST /validate                               →  配置校验
POST /execute                                →  执行节点
```

---

### 🚀 centag wrap — 零侵入接入 Agent

你的 Agent 完全照常工作。**Centag 只是把流量导了进来。**

```bash
# 启动 Centag
centag

# 用 wrap 启动 Agent——仅此一步
centag wrap run -- opencode

# 自检
centag wrap doctor
```

不改配置文件，不改环境变量，不改 Agent 代码。`centag wrap` 是进程级代理，透明地注入路由。

---

### 🔑 统一后端与 Key 管理

| 能力 | 说明 |
|------|------|
| **多后端管理** | OpenAI、Anthropic、智谱、Ollama、任意 OpenAI 兼容端点——Web 上统一管理 |
| **API Key 池化** | 每个后端多个 Key，限流或宕机时自动轮转 |
| **自动容错** | Key 失败 → 换下一个 Key；后端故障 → 换下一个后端 |
| **智能调度** | 权重、优先级、模型能力匹配，选最优后端 |
| **成本追踪** | 按请求、按后端、按模型统计 Token 消耗和费用 |

---

## 🚀 快速上手

```bash
# 1. 安装（任选其一）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# 或
npm install -g @atomlai/centag

# 2. 启动
centag

# 3. 打开 Web UI → http://localhost:20060 → 添加第一个后端

# 4. 接入 Agent（零配置）
centag wrap run -- opencode
```

搞定。你的 Agent 流量现在走 Centag 了——共享后端、自动容错、成本可视。

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
./start.sh docker up                                 # 默认 personal 容器
```

管理界面：http://localhost:20060 · 停止：`./start.sh docker down`
</details>

---

## 📸 截图

| 流水线画布 | Agent 接入 |
|-----------|-----------|
| ![Pipeline Canvas](docs/assets/readme/pipeline-canvas.png) | ![Agent Setup](docs/assets/readme/agent-setup.png) |

| 仪表盘 | 节点插件 |
|--------|---------|
| ![Dashboard](docs/assets/readme/dashboard.png) | ![Node Plugins](docs/assets/readme/node-plugins.png) |

---

## 🧩 17+ 代理模式 — 开箱即用

Centag 内置多种场景化流水线模板：

| 模式 | 快捷码 | 说明 |
|------|--------|------|
| 🧠 智能调度 | (默认) | 基于模型兼容性和后端负载智能路由 |
| 📡 透明代理 | `#t` | 原样转发客户端请求——不注入 system prompt |
| 🎯 直连后端 | `#d` | 固定出口 + 托管 system prompt |
| 🔄 容错 | `#f` | 跨后端自动降级 |
| 🛡️ 审核 | `#a` | 生成 → 质量审核 → 反馈 |
| ⚡ 优化 | `#o` | 生成 → 内容优化 |
| 🔀 路由 | `#r` | 意图感知的多分支路由 |
| 🌐 翻译 | `#l` | 生成 → 翻译为目标语言 |
| ⚖️ 聚合 | `#ag` | 并行多模型生成 → 合并结果 |
| 🔒 安全防火墙 | `#sec` | 安全审核 → 生成 → PII 脱敏 |
| 📚 RAG 网关 | `#rag` | 缓存优先的检索增强生成 |
| 🌍 地理路由 | `#geo` | 基于规则的区域路由 |
| 🤖 Pi Agent | `#pi` | 代码任务 → 沙箱；问答 → LLM |
| 💬 多语言客服 | `#cs` | 记忆召回 → 生成 → 翻译 |
| 📞 CI/CD Webhook | — | 从外部系统触发流水线 |

**自定义流水线**才是 Centag 真正的闪光点——在画布上设计你自己的 DAG。

---

## 📚 文档

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
| 架构 | [docs/architecture/](docs/architecture/) |
| 安全 | [docs/security/](docs/security/) |

---

## 💬 反馈与支持

有问题或建议？请提 [GitHub Issues](https://github.com/atoml-ai/centag/issues)，或发邮件 **centag@atoml.com**。

---

## 📄 许可证

MIT License（开源发行版：`minimal` / `personal`）
