# Pi Sandbox 操作手册

Pi Sandbox 提供 **Pi Coding Agent 的 HTTP 网关**，暴露 OpenAI 兼容之外的会话/Prompt API，供 Centag 或外部系统调用。

**特点**：完全独立，不依赖 middleware 或 Mem0。

## 目录结构

```
services/pi-sandbox/
├── docker-compose.yml      # 生产部署（单容器）
├── Dockerfile
├── go-client/              # pi-server 源码
├── pi-config/
│   ├── models.json         # 自定义 Provider/Model
│   └── settings.json       # 默认 provider/model
├── docker/                 # 开发：双容器模式
│   ├── Dockerfile.pi
│   ├── Dockerfile.pi-source
│   └── docker-compose.yml
└── scripts/
    ├── pi-docker.sh        # 轻量 dev 容器
    └── manage.sh           # 双容器管理
```

## 启动方式

### 方式 A：生产（推荐）

在 stack 根目录：

```bash
./start.sh start pi-sandbox
# 简写
./start.sh start pi
```

未配置 LLM API Key 时，`start.sh` 会在启动前弹出交互向导（支持 OpenAI、Kimi、Anthropic 等），自动写入 `.env`。

| 入口 | URL |
|------|-----|
| 健康检查 | http://localhost:20062/api/health |
| API 索引 | http://localhost:20062/ |
| Web UI | http://localhost:20062/ui/ |

### 方式 B：双容器开发

```bash
cd services/pi-sandbox
./scripts/manage.sh build
./scripts/manage.sh start
```

HTTP 网关端口：**8080**（非 20062）。

### 方式 C：轻量 dev 容器

直接跑 `pi` CLI，无 HTTP 网关：

```bash
cd services/pi-sandbox
./scripts/pi-docker.sh build
./scripts/pi-docker.sh start
./scripts/pi-docker.sh run              # 交互 TUI
./scripts/pi-docker.sh print "你好"     # 非交互

# 从本地 pi 源码构建
PI_SOURCE_DIR=/path/to/pi ./scripts/pi-docker.sh build --source
```

## 环境变量

在 stack 根目录 `.env` 配置（见 `.env.example`）：

| 变量 | 说明 | 默认 |
|------|------|------|
| `PI_SANDBOX_PORT` | 宿主机端口 | 20062 |
| `PI_PACKAGE` | npm 包名 | `@earendil-works/pi-coding-agent` |
| `PI_VERSION` | 包版本 | 0.78.1 |
| `PI_SKIP_VERSION_CHECK` | 跳过版本检查 | 1 |
| `OPENAI_API_KEY` / `KIMI_API_KEY` 等 | LLM 密钥 | — |

## Provider 配置

编辑 `services/pi-sandbox/pi-config/models.json`：

```json
{
  "providers": {
    "kimi": {
      "apiKey": "$KIMI_API_KEY",
      "models": [{ "id": "kimi-for-coding", "name": "Kimi for Coding" }]
    }
  }
}
```

**禁止**写入明文密钥。修改后：

```bash
./start.sh restart pi-sandbox
```

## HTTP API 用法

```bash
BASE=http://localhost:20062

# 1. 创建会话
curl -sS -X POST "$BASE/api/sessions" \
  -H 'Content-Type: application/json' \
  -d '{"id":"demo","args":["--provider","bigmodel","--model","glm-4-flash"]}'

# 2. 发送消息
curl -sS -X POST "$BASE/api/prompt" \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"demo","message":"用一句话介绍 Centag"}'

# 3. 获取事件
curl -sS "$BASE/api/sessions/demo/events?since=0&limit=100"
```

### 一键验证脚本

```bash
bash services/pi-sandbox/pi-config/smoke-test.sh
bash services/pi-sandbox/pi-config/webui-test.sh
```

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/health` | 健康检查 |
| GET | `/api/status` | 运行状态 |
| GET/POST | `/api/sessions` | 列表 / 创建会话 |
| GET | `/api/sessions/{id}` | 会话详情 |
| GET | `/api/sessions/{id}/events` | 事件轮询 |
| POST | `/api/prompt` | 发送 prompt |

## 与 Centag 集成

`centag` 主仓库的 `agent-memory` Profile 引用本服务包：

- 双容器：`services/pi-sandbox/docker/` + `go-client/`
- 配置挂载：`services/pi-sandbox/pi-config/`

详见主仓库 `profiles/agent-memory/docker-compose.yaml`。

## 故障排查

| 现象 | 处理 |
|------|------|
| 构建慢 | 首次需拉 Node 镜像并 `npm install -g pi` |
| 无模型响应 | 检查 `.env` API Key；`docker logs centag-pi-sandbox` |
| Provider 未找到 | 检查 `models.json` 挂载与 `$VAR` 格式 |
| `pi: command not found` | 确认镜像使用 `@earendil-works/pi-coding-agent` |