# 基础中间件

可选的基础设施服务包，各组件可单独启动，彼此无强依赖。

服务定义复用 `../compose/fragments/`；`docker-compose.yml` 通过 Compose `include` 合并 fragments 与 `../compose/overrides/stack-middleware.yaml`（容器名、端口、profiles）。Profile（如 agent-memory）与 Mem0 同样复用 fragments，各自有独立 override。

## 快速启动

```bash
# 在 centag-stack 根目录
./start.sh start base      # PG + Redis + ES + Qdrant + Neo4j
./start.sh start pg        # 仅 PostgreSQL
./start.sh start ol        # 仅 Ollama
```

完整操作手册：[docs/middleware-guide.md](../docs/middleware-guide.md)