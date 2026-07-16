package postgresql

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"centag/core/pkg/database/pgconn"
	"centag/core/pkg/storage"
)

// Config PostgreSQL 配置
type Config struct {
	// 连接配置
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`

	// 连接池配置
	MaxConnLifetime int   `yaml:"max_conn_lifetime"`  // 秒
	MaxConnIdleTime int   `yaml:"max_conn_idle_time"` // 秒
	MaxConns        int32 `yaml:"max_conns"`
	MinConns        int32 `yaml:"min_conns"`

	// KV 存储配置
	KVTable string `yaml:"kv_table"`

	// 向量存储配置
	VectorTable     string `yaml:"vector_table"`
	VectorDimension int    `yaml:"vector_dimension"`
	IndexType       string `yaml:"index_type"` // hnsw, ivfflat
}

// Plugin PostgreSQL 存储插件
type Plugin struct {
	config      *Config
	pool        *pgxpool.Pool
	kvStore     *KVStore
	vectorStore *VectorStore
	mu          sync.RWMutex
}

// NewPlugin 创建 PostgreSQL 插件
func NewPlugin(config interface{}) (storage.StoragePlugin, error) {
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 使用 pgconn.Manager 获取连接池
	manager := pgconn.NewManagerWithConfig(&pgconn.Config{
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		Password:        cfg.Password,
		Database:        cfg.Database,
		SSLMode:         cfg.SSLMode,
		MaxConnLifetime: cfg.MaxConnLifetime,
		MaxConnIdleTime: cfg.MaxConnIdleTime,
		MaxConns:        int(cfg.MaxConns),
		MinConns:        int(cfg.MinConns),
	})

	pool, err := manager.GetPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 尝试自动初始化表结构（Docker 部署时表通常已存在，失败时记录警告不阻断启动）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := ensureTables(ctx, pool, cfg); err != nil {
		// 打印警告但不阻断启动：表可能已由 Docker init 脚本创建
		fmt.Printf("[WARN] postgresql plugin: ensureTables warning (tables may already exist): %v\n", err)
	}

	// 初始化存储
	plugin := &Plugin{
		config:      cfg,
		pool:        pool,
		kvStore:     NewKVStore(pool, cfg.KVTable),
		vectorStore: NewVectorStore(pool, cfg.VectorTable, cfg.VectorDimension),
	}

	return plugin, nil
}

// ensureTables 确保所有必要的表结构存在。
// 非致命：CREATE EXTENSION 需要超级用户权限，在 Docker 环境中扩展通常已由 initdb 脚本安装。
// 若无权限，检查扩展是否已存在；若已存在则继续，若不存在则返回错误。
func ensureTables(ctx context.Context, pool *pgxpool.Pool, cfg *Config) error {
	// 尝试启用 pgvector 扩展（需要超级用户权限）
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		// 无权创建扩展时，检查扩展是否已经安装
		var count int
		if qErr := pool.QueryRow(ctx,
			"SELECT count(*) FROM pg_extension WHERE extname = 'vector'").Scan(&count); qErr != nil || count == 0 {
			return fmt.Errorf("pgvector extension not installed and cannot be created (need superuser): %w", err)
		}
		// 扩展已存在（由 Docker 或 DBA 预装），忽略权限错误
		fmt.Printf("[INFO] postgresql plugin: pgvector extension already installed, skipping CREATE EXTENSION\n")
	}

	// 尝试启用 pg_trgm 扩展（用于文本相似度预筛，无需 Embedding API）
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm"); err != nil {
		var count int
		if qErr := pool.QueryRow(ctx,
			"SELECT count(*) FROM pg_extension WHERE extname = 'pg_trgm'").Scan(&count); qErr != nil || count == 0 {
			// pg_trgm 不可用不致命，文本预筛功能将被跳过，降级到纯向量搜索
			fmt.Printf("[WARN] postgresql plugin: pg_trgm extension not available, text pre-filter disabled: %v\n", err)
		} else {
			fmt.Printf("[INFO] postgresql plugin: pg_trgm extension already installed\n")
		}
	}

	// 创建 KV 缓存表
	kvTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key VARCHAR(512) PRIMARY KEY,
			value JSONB NOT NULL,
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`, cfg.KVTable)
	if _, err := pool.Exec(ctx, kvTable); err != nil {
		return fmt.Errorf("failed to create kv table: %w", err)
	}

	// KV 表过期索引
	kvIdx := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS %s_expires_at_idx ON %s (expires_at) WHERE expires_at IS NOT NULL`,
		cfg.KVTable, cfg.KVTable)
	if _, err := pool.Exec(ctx, kvIdx); err != nil {
		return fmt.Errorf("failed to create kv index: %w", err)
	}

	// 创建向量缓存表
	vectorTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(512) PRIMARY KEY,
			vector vector(%d) NOT NULL,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`, cfg.VectorTable, cfg.VectorDimension)
	if _, err := pool.Exec(ctx, vectorTable); err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}

	// 向量 HNSW 索引（创建失败不致命，表可能已有其他索引）
	vectorIdx := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS %s_vector_idx ON %s USING hnsw (vector vector_cosine_ops) WITH (m = 16, ef_construction = 200)`,
		cfg.VectorTable, cfg.VectorTable)
	if _, err := pool.Exec(ctx, vectorIdx); err != nil {
		fmt.Printf("[WARN] postgresql plugin: failed to create vector HNSW index (may already exist): %v\n", err)
	}

	// pg_trgm GIN 索引（用于 metadata->>'request' 的文本相似度快速检索）
	// 依赖 pg_trgm 扩展；若扩展不存在则创建失败，文本预筛将在运行时降级到向量搜索
	trgmIdx := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS %s_request_trgm_idx ON %s USING gin ((metadata->>'request') gin_trgm_ops)`,
		cfg.VectorTable, cfg.VectorTable)
	if _, err := pool.Exec(ctx, trgmIdx); err != nil {
		fmt.Printf("[WARN] postgresql plugin: failed to create trgm index (pg_trgm may not be installed): %v\n", err)
	}

	return nil
}

// envOrDefault 读取环境变量，不存在时返回默认值
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseConfig 解析配置。仅使用传入的 config 参数，不使用环境变量覆盖，
// 以保证 WebUI 保存的配置与运行时一致（环境变量仅用于首次 seed 写入 DB 和未配置时的默认值）。
func parseConfig(config interface{}) (*Config, error) {
	// 默认值优先从环境变量读取，确保即使用户未在 UI 中填写也能正常连接
	defaultPort := 5432
	if p := os.Getenv("PG_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			defaultPort = v
		}
	}

	cfg := &Config{
		Host:            envOrDefault("PG_HOST", "localhost"),
		Port:            defaultPort,
		User:            envOrDefault("PG_USER", "postgres"),
		Password:        envOrDefault("PG_PASSWORD", ""),
		Database:        envOrDefault("PG_DATABASE", "centag"),
		SSLMode:         envOrDefault("PG_SSL_MODE", "disable"),
		MaxConnLifetime: 3600,
		MaxConnIdleTime: 600,
		MaxConns:        20,
		MinConns:        5,
		KVTable:         "kv_cache",
		VectorTable:     "vector_cache",
		VectorDimension: 1024,
		IndexType:       "hnsw",
	}

	// 从配置填充（仅非空值覆盖默认值，确保 UI 未填写时回退到环境变量）
	if m, ok := config.(map[string]interface{}); ok {
		if v, ok := m["host"].(string); ok && v != "" {
			cfg.Host = v
		}
		if v, ok := m["port"].(int); ok && v > 0 {
			cfg.Port = v
		}
		if v, ok := m["port"].(float64); ok && v > 0 {
			cfg.Port = int(v)
		}
		if v, ok := m["user"].(string); ok && v != "" {
			cfg.User = v
		}
		if v, ok := m["password"].(string); ok && v != "" {
			cfg.Password = v
		}
		if v, ok := m["database"].(string); ok && v != "" {
			cfg.Database = v
		}
		if v, ok := m["ssl_mode"].(string); ok && v != "" {
			cfg.SSLMode = v
		}
		if v, ok := m["max_conn_lifetime"].(int); ok {
			cfg.MaxConnLifetime = v
		}
		if v, ok := m["max_conn_lifetime"].(float64); ok {
			cfg.MaxConnLifetime = int(v)
		}
		if v, ok := m["max_conn_idle_time"].(int); ok {
			cfg.MaxConnIdleTime = v
		}
		if v, ok := m["max_conn_idle_time"].(float64); ok {
			cfg.MaxConnIdleTime = int(v)
		}
		if v, ok := m["max_conns"].(int); ok {
			cfg.MaxConns = int32(v)
		}
		if v, ok := m["max_conns"].(float64); ok {
			cfg.MaxConns = int32(v)
		}
		if v, ok := m["min_conns"].(int); ok {
			cfg.MinConns = int32(v)
		}
		if v, ok := m["min_conns"].(float64); ok {
			cfg.MinConns = int32(v)
		}
		if v, ok := m["kv_table"].(string); ok {
			cfg.KVTable = v
		}
		if v, ok := m["vector_table"].(string); ok {
			cfg.VectorTable = v
		}
		if v, ok := m["vector_dimension"].(int); ok {
			cfg.VectorDimension = v
		}
		if v, ok := m["vector_dimension"].(float64); ok {
			cfg.VectorDimension = int(v)
		}
		if v, ok := m["index_type"].(string); ok {
			cfg.IndexType = v
		}
	}

	return cfg, nil
}

// StorageType 返回存储类型
func (p *Plugin) StorageType() storage.StorageType {
	return storage.StorageTypePostgresql
}

// KVStore 返回 KV 存储接口
func (p *Plugin) KVStore() (storage.KVStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.kvStore, nil
}

// VectorStore 返回向量存储接口
func (p *Plugin) VectorStore() (storage.VectorStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.vectorStore, nil
}

// KnowledgeStore 获取知识库存储（PostgreSQL 不支持知识库存储）
func (p *Plugin) KnowledgeStore() (storage.KnowledgeDataStore, error) {
	return nil, fmt.Errorf("PostgreSQL does not support knowledge storage")
}

// HealthCheck 健康检查
func (p *Plugin) HealthCheck(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close 关闭连接
func (p *Plugin) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pool != nil {
		p.pool.Close()
	}

	return nil
}

// GetDefaultConfig 获取默认配置（仅用于 UI 新建时的占位，不用于运行时连接）
func (p *Plugin) GetDefaultConfig() map[string]interface{} {
	defaultPort := 5432
	if p := os.Getenv("PG_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			defaultPort = v
		}
	}
	return map[string]interface{}{
		"host":                envOrDefault("PG_HOST", "localhost"),
		"port":                defaultPort,
		"user":                envOrDefault("PG_USER", "postgres"),
		"password":            envOrDefault("PG_PASSWORD", ""),
		"database":            envOrDefault("PG_DATABASE", "centag"),
		"ssl_mode":            envOrDefault("PG_SSL_MODE", "disable"),
		"max_conn_lifetime":   3600,
		"max_conn_idle_time":  600,
		"max_conns":           20,
		"min_conns":           5,
		"kv_table":            "kv_cache",
		"vector_table":        "vector_cache",
		"vector_dimension":    1024,
		"index_type":          "hnsw",
	}
}

// init 注册 PostgreSQL 插件
func init() {
	storage.RegisterPlugin(storage.StorageTypePostgresql, func(config map[string]interface{}) (storage.StoragePlugin, error) {
		return NewPlugin(config)
	})
}
