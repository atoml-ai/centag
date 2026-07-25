# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

---

## ⚡ Your LLM Proxy Hub — Pipelines as Strategy

**Stop wiring each Agent separately.** Centag gives you one gateway to manage every backend, API key, and proxy strategy — all wired together as visual, swappable pipelines.

![Centag Architecture Flow](docs/assets/readme/hero-architecture.png)

**One gateway. All backends. Pipelines are your strategy. Agents just write code.**

---

## 🎯 The Problem We Solve

> **You've been there:**
>
> - Five Agents, each configured with the same API key — one key dies, everything stops.
> - Need a content audit for compliance? Rewrite every Agent's code.
> - Switching between coding and translation scenarios? Reconfigure everything.
>
> **Centag eliminates all of that.**

| Your Real Need | Centag Does This |
|---|---|
| **Switch backends instantly** | Manage all backends in one place — toggle in Web UI, zero Agent-side changes |
| **Auto failover + Key pools** | Rotate multiple keys per backend; when one is rate-limited, the next takes over seamlessly |
| **Strategy per scenario** | Build a pipeline for coding, another for translation, another for security — swap without touching Agent code |
| **Usage & cost visibility** | Token and cost tracking so you always know what you're spending |

---

## ⭐ Why Centag

### 🎨 Visual Pipeline Orchestration — The Killer Feature

Most LLM proxies just route requests. **Centag lets you *design* the request lifecycle** as a visual DAG on a drag-and-drop canvas.

![Pipeline Architecture — Visual DAG Orchestration](docs/assets/readme/pipeline-canvas.png)

**16 built-in node types** you can combine freely:

| Node | Kind | What It Does |
|------|------|--------------|
| 🤖 Generator | `llm.generate` | Call any LLM backend — the core generation node |
| 🔄 Processor | `content.transform` | Translate, summarize, optimize content |
| 🛡️ Reviewer | `quality.review` | Score and audit upstream answers |
| 🔀 Router | `route.decide` | Branch by intent, keyword, or LLM classification |
| ⚖️ Aggregator | `aggregate.merge` | Merge, vote, or pick the best from parallel generators |
| 🧠 Memory | `memory.query` | Recall context from cloud memory / local vectors |
| 🔒 Audit | `audit.safety` | Content moderation and safety filtering |
| 💰 Token Usage | `metrics.token_usage` | Track token consumption and costs |
| 📦 Cache | `cache.access` | Read/write cache (exact, semantic, or hybrid) |
| ⏱️ Scheduler | `scheduling.decide` | Smart scheduling across backends |
| 🔌 Transparent Forward | `proxy.transparent_forward` | Raw HTTP proxy with SSE passthrough |
| 🛠️ Tool Call | `inject.tool_call` | Inject function-calling tools |
| ✂️ Prompt Ops | `prompt.ops` | User prompt preprocessing |
| 📝 Output Post-ops | `prompt.postprocess` | Output post-processing |
| 🔄 Loop Controller | — | Loop control for iterative workflows |
| 🔌 Plugin Node | *(remote / business)* | Your custom node via HTTP or Go SDK |

**Pipeline = Strategy.** Switch scenario → switch pipeline → Agent unchanged.

| Scenario | Pipeline |
|----------|----------|
| 🧑‍💻 Coding assistant | Router → Code-specialized model → Code review |
| 🌐 Translation | Generator → Translator → Format check |
| 🏢 Enterprise compliance | Security audit → Generator → PII redact → Compliance audit |
| 🤖 Customer support | Memory recall → Generator → Multi-language translation |

---

### 🧩 Open Plugin Ecosystem — Extend Everything

Centag's pipeline nodes are **not closed**. You can extend with three levels of plugins:

![Plugin Ecosystem — Extend Everything](docs/assets/readme/node-plugins.png)

**Capability abstractions** — you declare *what* you need, not *how* to implement:

| Capability | What It Means | Examples |
|-----------|---------------|----------|
| `memory` | Recall / store / search context | Cloud memory, remote HTTP, local vectors |
| `token` | Token optimization | Smart truncation, semantic summarization |
| `prompt` | Prompt processing | Template engine, dynamic enhancement |
| `security` | Safety and compliance | Content moderation, PII redaction |
| `router` | Intent classification and routing | Intent-based, load-balanced |
| `monitor` | Cost / latency / quality tracking | Cost analysis, quality evaluation |

**Write your own plugin in minutes:**

```go
// Implement this interface — that's it
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

Or deploy a **remote plugin** as any HTTP service:

```
GET  /.well-known/centag-node-plugin.json   →  auto-discovered
POST /validate                               →  config check
POST /execute                                →  run the node
```

---

### 🚀 centag wrap — Zero-Invasion Agent Access

Your Agent works exactly as before. **Centag just routes the traffic.**

```bash
# Start Centag
centag

# Launch your Agent through Centag — that's all you need
centag wrap run -- opencode

# Health check
centag wrap doctor
```

No config file changes. No environment variable edits. No Agent-side code modifications. `centag wrap` is a process-level proxy that injects routing transparently.

---

### 🔑 Unified Backend & Key Management

| Feature | Details |
|---------|---------|
| **Multi-backend management** | OpenAI, Anthropic, 智谱, Ollama, any OpenAI-compatible endpoint — all from one Web UI |
| **API Key pooling** | Multiple keys per backend, auto-rotated when rate-limited or down |
| **Failover** | Automatic fallback: key fails → next key; backend fails → next backend |
| **Smart scheduling** | Weights, priorities, and model-capability matching for optimal routing |
| **Cost tracking** | Token usage and cost per request, per backend, per model |

---

## 🚀 Quickstart

```bash
# 1. Install (pick one)
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# or
npm install -g @atomlai/centag

# 2. Start
centag

# 3. Open Web UI → http://localhost:20060 → Add your first backend

# 4. Connect an Agent (zero config)
centag wrap run -- opencode
```

That's it. Your Agent traffic now flows through Centag with shared backends, failover, and cost visibility.

### Other install methods

<details>
<summary>npm (without changing global paths)</summary>

```bash
npx --yes @atomlai/centag
```
</details>

<details>
<summary>Offline / air-gapped</summary>

```bash
npm install -g @atomlai/centag-offline
```
</details>

<details>
<summary>Docker (from source)</summary>

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # edit secrets
./start.sh docker up                                 # default: personal
```

Admin UI: http://localhost:20060 · Stop: `./start.sh docker down`
</details>

---

## 📸 Screenshots

| Pipeline Canvas | Agent Setup |
|-----------------|-------------|
| ![Pipeline Canvas](docs/assets/readme/pipeline-canvas.png) | ![Agent Setup](docs/assets/readme/agent-setup.png) |

| Dashboard | Node Plugins |
|-----------|-------------|
| ![Dashboard](docs/assets/readme/dashboard.png) | ![Node Plugins](docs/assets/readme/node-plugins.png) |

---

## 🧩 17+ Proxy Modes — Ready to Use

Centag ships with battle-tested pipeline templates for common scenarios:

| Mode | Shortcut | What It Does |
|------|----------|--------------|
| 🧠 Smart Scheduling | (default) | Intelligent routing based on model compatibility and backend load |
| 📡 Transparent Proxy | `#t` | Pass through client requests as-is — no system prompt injection |
| 🎯 Direct Backend | `#d` | Fixed egress with managed system prompt |
| 🔄 Fallback | `#f` | Automatic failover across backends |
| 🛡️ Audit | `#a` | Generate → quality audit → feedback |
| ⚡ Optimize | `#o` | Generate → content optimization |
| 🔀 Router | `#r` | Intent-aware multi-branch routing |
| 🌐 Translate | `#l` | Generate → translate to target language |
| ⚖️ Aggregator | `#ag` | Parallel multi-model generation → merge results |
| 🔒 Security Firewall | `#sec` | Safety audit → generate → PII redaction |
| 📚 RAG Gateway | `#rag` | Cache-first retrieval-augmented generation |
| 🌍 Geo Routing | `#geo` | Rule-based region-to-backend routing |
| 🤖 Pi Agent | `#pi` | Code tasks → sandbox; Q&A → LLM |
| 💬 Multilingual Support | `#cs` | Memory recall → generate → translate |
| 📞 CI/CD Webhook | — | Trigger pipelines from external systems |

**Custom pipelines** are where Centag truly shines — design your own DAG on the canvas.

---

## 📚 Documentation

| Topic | Link |
|-------|------|
| Full docs index | [docs/README.md](docs/README.md) |
| Pipeline plugin standard | [docs/guide/pipeline-plugin-standard.md](docs/guide/pipeline-plugin-standard.md) |
| Processor plugins | [docs/guide/processor-plugins.md](docs/guide/processor-plugins.md) |
| Pipeline variables | [docs/guide/pipeline-variables.md](docs/guide/pipeline-variables.md) |
| Proxy modes | [docs/guide/proxy-modes.md](docs/guide/proxy-modes.md) |
| Backend configuration | [docs/guide/backend-configuration.md](docs/guide/backend-configuration.md) |
| Local proxy / wrap | [docs/guide/system-proxy-egress.md](docs/guide/system-proxy-egress.md) |
| Environment variables | [docs/guide/environment-variables.md](docs/guide/environment-variables.md) |
| API reference | [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) |
| Architecture | [docs/architecture/](docs/architecture/) |
| Security | [docs/security/](docs/security/) |

---

## 💬 Feedback & Support

Questions, suggestions, or issues? Open a [GitHub Issue](https://github.com/atoml-ai/centag/issues) or email **centag@atoml.com**.

---

## 📄 License

MIT License (open-source editions: `minimal` / `personal`)
