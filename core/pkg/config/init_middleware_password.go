package config

// DefaultInitMiddlewarePassword 在未通过环境变量覆盖时，作为 PostgreSQL / Redis /
// Elasticsearch / ChromaDB 等存储配置的默认口令，用于首次 seed 与 initdata SQLite，
// 便于与 docker-compose 及 secrets 对齐后开箱连接。生产环境请改为强口令并同步更新。
const DefaultInitMiddlewarePassword = "llmproxy-middleware-init"
