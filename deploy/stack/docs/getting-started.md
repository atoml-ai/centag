# 快速开始

## 前置条件

- Docker & Docker Compose
- 2GB+ 可用内存
- 已初始化 git submodule（在 centag 根目录：`git submodule update --init`）

## 1. 初始化配置

```bash
cd centag-stack
cp .env.example .env
```

按需编辑 `.env`：

| 场景 | 必填变量 |
|------|----------|
| 仅中间件 | `POSTGRES_PASSWORD`（向导可自动生成） |
| Mem0 | 上述 + `MEM0_JWT_SECRET`、`MEM0_ADMIN_API_KEY` |
| Pi Sandbox | 至少一个 LLM Key，如 `OPENAI_API_KEY` 或 `KIMI_API_KEY` |

也可运行向导自动生成密钥：

```bash
./start.sh w
```

向导选项说明：

| 选项 | 内容 |
|------|------|
| 1 | 基础中间件 |
| 2 | Mem0 完整栈（含依赖中间件） |
| 3 | 全部服务 |
| **4** | **仅 Pi Sandbox（独立，无需中间件）** |
| 5 | 仅 PostgreSQL |
| 6 | 自定义选择 |

## 2. 按场景启动

### 在 centag 主仓库（推荐）

与 Profile 共用同一编排库，无需 `cd centag-stack`：

```bash
cd /path/to/centag

# 场景化一键（自动 stack ensure + 应用层）
./start.sh profile personal up
./start.sh profile cached up
./start.sh profile agent-memory up

# 仅管理中间件
./start.sh stack ensure postgresql ollama
./start.sh stack ensure postgresql qdrant ollama mem0
./start.sh stack status
```

详见 centag 文档：[`docs/guide/deployment-profiles-and-stack.md`](../../docs/guide/deployment-profiles-and-stack.md)。

### 在 centag-stack 子模块内

`start.sh` 为薄入口，与 `./start.sh stack` 加载同一套 `lib/stack.sh`：

```bash
cd centag-stack
./start.sh start base
./start.sh start mem0
./start.sh status
```

### 仅基础中间件

```bash
./start.sh stack start base    # 在 centag 根目录
# 或
cd centag-stack && ./start.sh start base
```

### Mem0 记忆服务

```bash
./start.sh stack start mem0    # centag 根目录
curl http://localhost:20061/health
```

`start mem0` 会自动：确保共享网络 → 拉起缺失的 PG/Qdrant/Neo4j/Ollama → 启动 Mem0。

### Pi Agent 网关（独立，无需 middleware）

```bash
./start.sh stack start pi-sandbox
curl http://localhost:20062/api/health
```

### 全部服务

```bash
./start.sh stack start all
```

## 3. 验证

```bash
./start.sh stack health
./start.sh stack status
```

## 4. 停止与清理

```bash
./start.sh stack stop all          # 停止，保留数据卷
./start.sh stack clean mem0        # 清理 Mem0 容器与镜像
./start.sh stack clean all         # 清理全部（含数据卷）
```

（在 `centag-stack/` 目录下可使用等价命令 `./start.sh stop all` 等。）

## 5. Compose 结构说明

中间件服务定义位于 `compose/fragments/`，`middleware/docker-compose.yml` 通过 Compose `include` 合并 fragments 与 `compose/overrides/stack-middleware.yaml`。Mem0、agent-memory embedded 同样复用 fragments。

## 下一步

- [Pi Sandbox 操作手册](pi-sandbox-guide.md)
- [Mem0 操作手册](mem0-guide.md)
- [中间件说明](middleware-guide.md)
- [架构设计](ARCHITECTURE.md)