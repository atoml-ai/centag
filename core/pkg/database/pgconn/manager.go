package pgconn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.uber.org/zap"

	"centag/core/pkg/logger"
)

// Manager PostgreSQL 连接管理器
// 提供统一的连接管理，支持 database/sql 和 pgxpool 两种连接方式
type Manager struct {
	config    *Config
	db        *sql.DB
	pool      *pgxpool.Pool
	mu        sync.RWMutex
	initOnce  sync.Once
	initErr   error
}

// NewManager 创建连接管理器
// 从环境变量读取配置
func NewManager() *Manager {
	config := &Config{
		Host:            envFirst("PG_HOST", "POSTGRES_HOST"),
		Port:            envIntFirst(5432, "PG_PORT", "POSTGRES_PORT"),
		User:            envFirst("PG_USER", "POSTGRES_USER"),
		Password:        envFirst("PG_PASSWORD", "POSTGRES_PASSWORD"),
		Database:        envFirst("PG_DATABASE", "POSTGRES_DB"),
		SSLMode:         envFirstOrDefault("disable", "PG_SSLMODE", "POSTGRES_SSLMODE"),
		MaxOpenConns:    envIntFirst(20, "PG_MAX_OPEN_CONNS"),
		MaxIdleConns:    envIntFirst(5, "PG_MAX_IDLE_CONNS"),
		ConnMaxLifetime: time.Hour,
		MaxConnLifetime: envIntFirst(3600, "PG_MAX_CONN_LIFETIME"),
		MaxConnIdleTime: envIntFirst(600, "PG_MAX_CONN_IDLE_TIME"),
		MaxConns:        envIntFirst(20, "PG_MAX_CONNS"),
		MinConns:        envIntFirst(5, "PG_MIN_CONNS"),
	}

	return &Manager{config: config}
}

// NewManagerWithConfig 创建连接管理器（使用指定配置）
func NewManagerWithConfig(config *Config) *Manager {
	return &Manager{config: config}
}

// GetConfig 获取连接配置（从环境变量读取）
// 采用 check-lock-check 模式避免 data race
func (m *Manager) GetConfig() *Config {
	// 第一次检查（读锁）
	m.mu.RLock()
	if m.config != nil {
		cfg := m.config
		m.mu.RUnlock()
		return cfg
	}
	m.mu.RUnlock()

	// 加锁初始化
	m.mu.Lock()
	defer m.mu.Unlock()

	// 第二次检查（写锁）
	if m.config != nil {
		return m.config
	}

	m.config = &Config{
		Host:            envFirst("PG_HOST", "POSTGRES_HOST"),
		Port:            envIntFirst(5432, "PG_PORT", "POSTGRES_PORT"),
		User:            envFirst("PG_USER", "POSTGRES_USER"),
		Password:        envFirst("PG_PASSWORD", "POSTGRES_PASSWORD"),
		Database:        envFirst("PG_DATABASE", "POSTGRES_DB"),
		SSLMode:         envFirstOrDefault("disable", "PG_SSLMODE", "POSTGRES_SSLMODE"),
		MaxOpenConns:    envIntFirst(20, "PG_MAX_OPEN_CONNS"),
		MaxIdleConns:    envIntFirst(5, "PG_MAX_IDLE_CONNS"),
		ConnMaxLifetime: time.Hour,
		MaxConnLifetime: envIntFirst(3600, "PG_MAX_CONN_LIFETIME"),
		MaxConnIdleTime: envIntFirst(600, "PG_MAX_CONN_IDLE_TIME"),
		MaxConns:        envIntFirst(20, "PG_MAX_CONNS"),
		MinConns:        envIntFirst(5, "PG_MIN_CONNS"),
	}

	return m.config
}

// GetSQLDB 获取 database/sql 连接
func (m *Manager) GetSQLDB() (*sql.DB, error) {
	m.mu.RLock()
	if m.db != nil {
		db := m.db
		m.mu.RUnlock()
		return db, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil {
		return m.db, nil
	}

	if m.config == nil {
		m.config = m.GetConfig()
	}

	db, err := sql.Open("pgx", m.config.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(m.config.MaxOpenConns)
	db.SetMaxIdleConns(m.config.MaxIdleConns)
	db.SetConnMaxLifetime(m.config.ConnMaxLifetime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	m.db = db
	logger.Info("PostgreSQL SQL connection established",
		zap.String("host", m.config.Host),
		zap.Int("port", m.config.Port),
		zap.String("database", m.config.Database))

	return m.db, nil
}

// GetPool 获取 pgxpool 连接池
func (m *Manager) GetPool() (*pgxpool.Pool, error) {
	m.mu.RLock()
	if m.pool != nil {
		pool := m.pool
		m.mu.RUnlock()
		return pool, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pool != nil {
		return m.pool, nil
	}

	if m.config == nil {
		m.config = m.GetConfig()
	}

	// 构建连接字符串
	dsn := m.config.DSN()

	// 创建连接池配置
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// 强制使用IPv4连接，避免macOS上IPv6解析问题
	poolConfig.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{}
		return dialer.DialContext(ctx, "tcp4", addr)
	}

	// 设置连接池参数
	if m.config.MaxConns > 0 {
		poolConfig.MaxConns = int32(m.config.MaxConns)
	}
	if m.config.MinConns > 0 {
		poolConfig.MinConns = int32(m.config.MinConns)
	}
	if m.config.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = time.Duration(m.config.MaxConnLifetime) * time.Second
	}
	if m.config.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = time.Duration(m.config.MaxConnIdleTime) * time.Second
	}

	// 创建连接池
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	m.pool = pool
	logger.Info("PostgreSQL pool connection established",
		zap.String("host", m.config.Host),
		zap.Int("port", m.config.Port),
		zap.String("database", m.config.Database),
		zap.Int32("max_conns", poolConfig.MaxConns))

	return m.pool, nil
}

// Close 关闭所有连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if m.db != nil {
		if err := m.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close SQL database: %w", err))
		}
		m.db = nil
	}

	if m.pool != nil {
		m.pool.Close()
		m.pool = nil
	}

	return errors.Join(errs...)
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.db != nil {
		return m.db.PingContext(ctx)
	}

	if m.pool != nil {
		return m.pool.Ping(ctx)
	}

	return fmt.Errorf("no database connection established")
}

// IsInitialized 检查是否已初始化连接
func (m *Manager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db != nil || m.pool != nil
}
