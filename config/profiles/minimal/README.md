# Minimal Profile — 轻量单机 / 代理服务

Centag 最简发行版：文件配置、无数据库驱动，提供 OpenAI 兼容的 LLM 代理服务。

## 特性

- ✅ OpenAI / Anthropic / Ollama 后端支持
- ✅ 直连模式、路由模式、透明模式（不注入 system prompt）
- ✅ 业务插件：仅 `router`
- ✅ 配置与后端存文件（YAML），无 SQLite / PostgreSQL
- ❌ 无前端管理界面（使用 config-generator）
- ❌ 无团队管理功能 / 无外置中间件默认依赖

> 发行版插件矩阵见 [`docs/guide/dist-profiles.md`](../../../docs/guide/dist-profiles.md)。
## 快速开始

### 方式一：本地运行

```bash
# 1. 配置环境变量
cp .env.example .env
vim .env  # 填入 API Keys

# 2. 启动服务
./start.sh dist run minimal

# 3. 测试
curl http://localhost:20060/health
```

### 方式二：Docker 运行

```bash
# 1. 构建镜像
./start.sh dist docker-build minimal

# 2. 运行容器
./start.sh dist docker-run minimal

# 3. 测试
curl http://localhost:20060/health
```

### 方式三：Docker Compose

```bash
# 1. 进入 minimal profile 目录
cd config/profiles/minimal

# 2. 配置环境变量
cp .env.example .env
vim .env

# 3. 启动服务
docker compose up -d

# 4. 查看日志
docker compose logs -f
```

## 配置说明

### 后端配置

编辑 `initdata/initial-backends.yaml` 文件：

```yaml
backends:
  - id: openai
    api_key: "${OPENAI_API_KEY}"  # 在 .env 中设置
    enabled: true
```

### 流水线配置

运行时会合并加载：

1. 全局 `config/initdata/pipeline-templates/common/`（含智能调度、路由模式等）
2. Profile 覆盖 `initdata/pipeline-templates/common/`

Profile 内置（`initdata/pipeline-templates/common/`，渠道打包自包含）：

- `01-direct-backend.yaml` - 直连模式
- `14-transparent-proxy.yaml` - 透明模式（系统默认）
- `15-raw-forward.yaml` - 原始 HTTP 转发（需 Target-URL / hostproxy）
- `router-mode.yaml` - 路由模式（`#r` / `X-Pipeline-ID: router-mode`）
- `smart-scheduling.yaml` - 智能调度（`#s`，依赖 ScheduleBackend hook）

开发运行时仍会与全局 `config/initdata/pipeline-templates/common/` 合并；上述两份已同步进 profile，保证 fnOS 等只带包内 initdata 时也可用。

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPENAI_API_KEY` | OpenAI API Key | - |
| `ANTHROPIC_API_KEY` | Anthropic API Key | - |
| `OLLAMA_ENABLED` | 启用 Ollama | false |
| `SERVER_PORT` | 服务端口 | 20060 |

## API 使用

### 直连模式

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Authorization: Bearer pc-admin-key-change-me" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### 路由模式

```bash
curl -X POST http://localhost:20060/v1/chat/completions \
  -H "Authorization: Bearer pc-admin-key-change-me" \
  -H "Content-Type: application/json" \
  -H "X-Pipeline-ID: router-mode" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "写一个 Python 快速排序"}]
  }'
```

## 与 personal 的区别

| 功能 | minimal | personal |
|------|:-------:|:-------:|
| LLM 后端 | ✅ | ✅ |
| 路由模式 | ✅ | ✅ |
| 前端管理 | ❌ | ✅ |
| 动态配置 | ❌ | ✅ |
| 存储插件 | ❌ | ✅ |
| 团队管理 | ❌ | ❌ |

## 故障排查

### 服务无法启动

```bash
# 查看日志
docker compose logs centag

# 检查配置
cat .env
```

### API Key 错误

确保 `.env` 文件中已正确设置 API Key：

```bash
OPENAI_API_KEY=sk-your-actual-key
```
