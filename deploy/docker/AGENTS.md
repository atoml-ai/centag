# deploy/docker/ — Docker 配置

> 面向 Agent：本目录包含 **Centag 应用镜像** 的构建与 Compose 编排。

## 目录职责

Docker 镜像构建、`docker-compose.yaml`（**仅 `centag` 服务**）、调试用 override。

**PostgreSQL、Redis、Elasticsearch、Ollama、Mem0 等中间件** 已迁移到子项目 **`deploy/stack/`**，请勿在本目录重复编排长期依赖栈。

## 核心文件

| 文件 | 用途 |
|------|------|
| `Dockerfile` | 主镜像构建（合并前后端） |
| `Dockerfile.backend` | 后端单独构建 |
| `Dockerfile.frontend` | 前端单独构建 |
| `docker-compose.yaml` | 仅启动 `centag` 容器 |
| `docker-compose.prod.yaml` | 生产环境（预构建镜像 + 前端） |
| `docker-compose.debug.yaml` | 本地调试 override（挂载 `bin/centag`） |

## 常用命令

```bash
# 启动应用容器（需 config/secrets/.env 中已配置 PG_HOST 等指向可达的数据库）
./start.sh docker up

# 调试模式（macOS / Linux 自动选 override）
./start.sh docker debug

# 查看日志 / 停止
./start.sh docker logs
./start.sh docker down
```

## 约束

- ❌ **禁止**在 Dockerfile 中硬编码密钥
- ❌ **禁止**提交 `config/secrets/.env`
- ✅ 运行时仅使用 **`config/secrets/.env`**（不再维护 `docker/.env`）
- ✅ 中间件真源：**`deploy/stack`**

## 相关文档

- 子项目：`../deploy/stack/README.md`
- 部署：`../docs/operations/deployment.md`（若与 stack 冲突以 stack 为准）
- 根目录：`../docs/harness/AGENTS.md`

---

*最后更新：2026-05-10*
