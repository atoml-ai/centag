// Package config provides configuration management for centag.
//
// Configuration is split into two tiers:
//
//  1. Bootstrap config (this file) – the minimal settings needed to open the
//     database.  Sourced exclusively from environment variables so that there
//     are zero file dependencies at startup.
//
//  2. Runtime config (db_loader.go) – all other settings, stored in the
//     database and loaded after the DB connection is established.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// BootstrapConfig contains only the settings that must be known before the
// database is available.  Every other setting lives in the database.
type BootstrapConfig struct {
	Server ServerConfig
	Log    LogConfig
	DB     DBBootstrapConfig
}

// DBBootstrapConfig describes how to open the metadata database.
type DBBootstrapConfig struct {
	// Driver selects the database plugin, e.g. "postgresql" or "sqlite".
	// Use "auto" to automatically detect based on environment variables.
	// When set to a specific driver (e.g., "postgresql"), it will still
	// fallback to "sqlite" if the primary driver fails to connect.
	Driver string

	// Path is the file-system path used for logs (relative to executable).
	Path string

	// DSN is the connection string for the database.
	DSN string
}

// LoadBootstrap reads the bootstrap configuration from environment variables.
// No files are ever read.  Every setting falls back to a safe default when
// the corresponding variable is absent or empty.
//
// Environment variables (all optional):
//
//	LLM_PROXY_SERVER_PORT      default 20060
//	LLM_PROXY_SERVER_HOST      default "0.0.0.0"
//	LLM_PROXY_SERVER_MODE      default "release"  (gin mode)
//	LLM_PROXY_LOG_LEVEL        default "info"
//	LLM_PROXY_LOG_FORMAT       default "json"
//	LLM_PROXY_LOG_OUTPUT       default "both" (stdout + file; daemon/launcher may set "file")
//	LLM_PROXY_LOG_PATH         default "./logs"
//	LLM_PROXY_LOG_FILENAME     default "centag.log"
//	LLM_PROXY_LOG_COMPRESS     default true
//	LLM_PROXY_DB_DRIVER        default "auto" (auto-detect: postgresql or sqlite)
//	LLM_PROXY_DB_DSN           default "" (built from PG_* or SQLITE_PATH env vars)
//	CENTAG_EDITION default "team" ("personal" for desktop / single-user)
//
// 相对路径（如 ./data/centag.db、./logs）会按可执行文件所在目录解析，
// 这样无论从项目根还是 bin/ 启动，data/logs 都会落在可执行文件同目录下（如 bin/data、bin/logs）。
//
// Note: Even when Driver is set to "postgresql", the database.Init function will
// automatically fallback to "sqlite" if PostgreSQL connection fails.
func LoadBootstrap() *BootstrapConfig {
	logPath := envStr("LLM_PROXY_LOG_PATH", "./logs")

	// 默认使用 SQLite；可通过 LLM_PROXY_DB_DRIVER 环境变量覆盖（如 "postgresql" 或 "auto"）
	// 注意：即使设置了 postgresql，database.Init 也会在连接失败时降级到 sqlite
	driver := envStr("LLM_PROXY_DB_DRIVER", "sqlite")

	// 构建 DSN
	dsn := buildDSN(driver)

	// SQLite 路径（供数据库插件使用，与 DSN 保持一致）
	var dbPath string
	if driver == "sqlite" || driver == "auto" {
		dbPath = resolvePathRelativeToExecutable(envFirst("SQLITE_PATH", "./storage/centag.db"))
	}

	return &BootstrapConfig{
		Server: ServerConfig{
			Port:        envInt("LLM_PROXY_SERVER_PORT", 20060),
			Host:        envStr("LLM_PROXY_SERVER_HOST", "0.0.0.0"),
			Mode:        envStr("LLM_PROXY_SERVER_MODE", "release"),
			ExternalURL: envStr("LLM_PROXY_EXTERNAL_URL", ""),
			Edition:     envStr("CENTAG_EDITION", "team"),
		},
		Log: LogConfig{
			Level:  envStr("LLM_PROXY_LOG_LEVEL", "info"),
			Format: envStr("LLM_PROXY_LOG_FORMAT", "json"),
			Output: envStr("LLM_PROXY_LOG_OUTPUT", "both"),
			File: FileLogConfig{
				Path:       resolvePathRelativeToExecutable(logPath),
				Filename:   envStr("LLM_PROXY_LOG_FILENAME", "centag.log"),
				MaxSize:    envInt("LLM_PROXY_LOG_MAX_SIZE", 0),
				MaxBackups: envInt("LLM_PROXY_LOG_MAX_BACKUPS", 0),
				MaxAge:     envInt("LLM_PROXY_LOG_MAX_AGE", 0),
				Compress:   envBool("LLM_PROXY_LOG_COMPRESS", true),
			},
		},
		DB: DBBootstrapConfig{
			Driver: driver,
			Path:   dbPath,
			DSN:    dsn,
		},
	}
}

// buildDSN 根据数据库类型构建连接字符串
func buildDSN(driver string) string {
	switch driver {
	case "postgresql":
		return buildPGDSN()
	case "sqlite":
		return buildSQLiteDSN()
	default:
		return ""
	}
}

// buildPGDSN 从环境变量构建 PostgreSQL DSN
func buildPGDSN() string {
	host := envFirst("PG_HOST", "POSTGRES_HOST", "localhost")
	port := envFirst("PG_PORT", "POSTGRES_PORT", "5432")
	user := envFirst("PG_USER", "POSTGRES_USER", "postgres")
	password := envFirst("PG_PASSWORD", "POSTGRES_PASSWORD", "")
	dbname := envFirst("PG_DATABASE", "POSTGRES_DB", "centag")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
}

// buildSQLiteDSN 从环境变量构建 SQLite DSN
func buildSQLiteDSN() string {
	path := resolvePathRelativeToExecutable(envFirst("SQLITE_PATH", "./storage/centag.db"))
	return fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL&_cache_size=-64000&_busy_timeout=5000", path)
}

// resolvePathRelativeToExecutable 将相对路径解析为相对于当前可执行文件所在目录的绝对路径。
// 绝对路径原样返回。这样从项目根执行 ./bin/centag 或从 bin 执行 ./centag，data/logs 都在 bin 下。
func resolvePathRelativeToExecutable(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	execPath, err := os.Executable()
	if err != nil {
		return path
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, path)
}

// ResolvePathRelativeToExecutable 将相对路径解析为相对于可执行文件目录的绝对路径。
// 供其它包（如 handler/server）统一复用，避免在项目根目录误创建运行时目录。
func ResolvePathRelativeToExecutable(path string) string {
	return resolvePathRelativeToExecutable(path)
}

// ── env helpers ──────────────────────────────────────────────────────────────

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// envFirst 返回第一个非空的环境变量
func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	if len(keys) > 0 {
		return keys[len(keys)-1]
	}
	return ""
}
