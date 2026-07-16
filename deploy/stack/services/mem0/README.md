# Mem0 记忆服务

Mem0 Server 服务包。运行时需要 **middleware** 中的 PostgreSQL、Qdrant、Neo4j、Ollama（由 `start.sh` 自动拉起）。

## 快速启动

```bash
# 在 centag-stack 根目录
cp .env.example .env
./start.sh start mem0
```

访问 http://localhost:20061/health

完整操作手册：[docs/mem0-guide.md](../../docs/mem0-guide.md)