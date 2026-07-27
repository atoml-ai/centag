# deploy/docker/ — Docker 配置

> 面向 Agent：本目录包含 **Centag 应用镜像** 的构建与 Compose 编排。

## 目录职责

Docker 镜像构建、`docker-compose.yaml`（**仅 `centag` 服务**）、调试用 override。

**PostgreSQL、Redis、Elasticsearch、Ollama、Mem0 等中间件** 已迁移到子项目 **`deploy/stack/`**，请勿在本目录重复编排长期依赖栈。

## 核心文件

| 文件 | 用途 |
|------|------|
| `Dockerfile.dist` | 统一镜像构建（personal / minimal / team 共用） |
| `docker-compose.yaml` | 仅启动 `centag` 容器（默认 personal） |
| `docker-compose.prod.yaml` | 生产/Drone 部署（预构建统一镜像 + extra_hosts） |
| `docker-compose.debug.yaml` | 本地调试 override（挂载 `~/.centag/lib/<edition>`） |

## 常用命令

```bash
# 构建并运行 personal 单容器
./start.sh docker build personal
./start.sh docker up

# 查看日志 / 停止
./start.sh docker logs
./start.sh docker down
```

## 持久化

所有数据集中在一个目录 `deploy/docker/data/`，与二进制安装 `lib/<edition>/` 对齐：

```
data/
├── storage/      # SQLite 数据库、配置文件
├── logs/         # 应用日志
├── plugins/      # 插件
└── bin/certs/    # MITM CA 证书
```

首次启动自动创建。备份只需复制 `data/` 目录。

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
