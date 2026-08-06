package config

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CleanupResult 描述一次卸载数据清理的执行结果。
type CleanupResult struct {
	Driver     string // 实际生效的元数据库驱动（sqlite | postgresql | ""）
	Cleaned    bool   // 是否执行了实际清理（或目标已不存在的幂等成功）
	Skipped    bool   // 是否跳过（未知驱动等）
	SkipReason string // 跳过原因（Skipped=true 时有值）
	Error      error  // 清理失败错误（读配置失败 / 连接/执行失败）
}

var safePGIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// CleanupDeploymentData 是卸载时清理部署级数据的跨平台入口。
//
//   - postgresql：连接维护库 postgres，终止目标库连接后 DROP DATABASE（整个库，
//     而非仅 public schema 表）。目标库名取自 centag.conf 的 pg_db（默认 centag）。
//   - sqlite：删除整个 SQLite 库文件（含 -wal/-shm 旁路文件）。路径优先
//     SQLITE_PATH，否则 ${dataDir}/centag.db。
//
// 读取/解析 centag.conf 失败时返回 Error（非 Skipped），便于卸载脚本以非零退出码告警。
// dataDir 为空时使用 ResolveDataDir() 自动解析（与 centag.conf 落盘目录一致）。
func CleanupDeploymentData(ctx context.Context, dataDir string) CleanupResult {
	if dataDir == "" {
		dataDir = ResolveDataDir()
	}
	dep, err := LoadDeploymentConfigFrom(dataDir)
	if err != nil {
		return CleanupResult{
			Error: fmt.Errorf("读取部署配置失败: %w", err),
		}
	}

	switch dep.DBDriver {
	case "postgresql":
		return cleanupPostgreSQLDatabase(ctx, dep)
	case "sqlite", "":
		return cleanupSQLiteDatabase(dataDir)
	default:
		return CleanupResult{
			Driver:     dep.DBDriver,
			Skipped:    true,
			SkipReason: fmt.Sprintf("未知元数据库驱动 %q，跳过数据库清理", dep.DBDriver),
		}
	}
}

func cleanupPostgreSQLDatabase(ctx context.Context, dep DeploymentConfig) CleanupResult {
	if ctx == nil {
		ctx = context.Background()
	}
	cleanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dbName := dep.PGDB
	if dbName == "" {
		dbName = "centag"
	}
	quoted, err := quotePGIdent(dbName)
	if err != nil {
		return CleanupResult{
			Driver: "postgresql",
			Error:  fmt.Errorf("非法 PostgreSQL 库名: %w", err),
		}
	}
	switch dbName {
	case "postgres", "template0", "template1":
		return CleanupResult{
			Driver: "postgresql",
			Error:  fmt.Errorf("拒绝删除 PostgreSQL 系统库 %q", dbName),
		}
	}

	// DROP DATABASE 不能连在目标库上执行；依次尝试维护库 postgres / template1。
	db, adminDB, err := openCleanupAdminDB(cleanCtx, dep)
	if err != nil {
		return CleanupResult{
			Driver: "postgresql",
			Error:  err,
		}
	}
	defer db.Close()

	// 先断开占用连接，再 DROP DATABASE。
	termSQL := `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`
	if _, err := db.ExecContext(cleanCtx, termSQL, dbName); err != nil {
		return CleanupResult{
			Driver: "postgresql",
			Error:  fmt.Errorf("终止 PostgreSQL 目标库连接失败（维护库=%s）: %w", adminDB, err),
		}
	}

	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoted)
	if _, err := db.ExecContext(cleanCtx, dropSQL); err != nil {
		return CleanupResult{
			Driver: "postgresql",
			Error:  fmt.Errorf("删除 PostgreSQL 数据库 %s 失败（维护库=%s）: %w", dbName, adminDB, err),
		}
	}

	return CleanupResult{
		Driver:  "postgresql",
		Cleaned: true,
	}
}

// openCleanupAdminDB 连接到可用于 DROP DATABASE 的维护库。
func openCleanupAdminDB(ctx context.Context, dep DeploymentConfig) (*sql.DB, string, error) {
	var errs []string
	for _, adminDB := range []string{"postgres", "template1"} {
		dsn := buildCleanupPGDSN(dep, adminDB)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: open: %v", adminDB, err))
			continue
		}
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			errs = append(errs, fmt.Sprintf("%s: ping: %v", adminDB, err))
			continue
		}
		return db, adminDB, nil
	}
	return nil, "", fmt.Errorf("连接 PostgreSQL 维护库失败（已试 postgres/template1）: %s", strings.Join(errs, "; "))
}

func cleanupSQLiteDatabase(dataDir string) CleanupResult {
	path := resolveCleanupSQLitePath(dataDir)
	if path == "" {
		return CleanupResult{
			Driver:     "sqlite",
			Skipped:    true,
			SkipReason: "无法解析 SQLite 库路径，跳过数据库清理",
		}
	}

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		err := os.Remove(p)
		if err == nil || os.IsNotExist(err) {
			// 文件本就不存在也视为幂等成功（卸载可重复执行）。
			continue
		}
		return CleanupResult{
			Driver: "sqlite",
			Error:  fmt.Errorf("删除 SQLite 文件 %s 失败: %w", p, err),
		}
	}

	return CleanupResult{
		Driver:  "sqlite",
		Cleaned: true,
	}
}

// resolveCleanupSQLitePath 解析卸载时应删除的 SQLite 库文件路径。
// 优先 SQLITE_PATH（与运行时 bootstrap 一致）；否则使用 ${dataDir}/centag.db（fnOS 默认）。
func resolveCleanupSQLitePath(dataDir string) string {
	if env := os.Getenv("SQLITE_PATH"); env != "" {
		return resolvePathRelativeToExecutable(env)
	}
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "centag.db")
}

// quotePGIdent 校验并双引号包裹 PostgreSQL 标识符，防止注入。
func quotePGIdent(name string) (string, error) {
	if !safePGIdent.MatchString(name) {
		return "", fmt.Errorf("%q", name)
	}
	return `"` + name + `"`, nil
}

// buildCleanupPGDSN 基于 DeploymentConfig 构建 PostgreSQL 连接串。
// dbname 为实际连接库（卸载时应连 postgres 维护库，而非待删目标库）。
// sslmode 默认 disable，避免 pgx 因空值报 "sslmode is invalid"。
func buildCleanupPGDSN(dep DeploymentConfig, dbname string) string {
	host := dep.PGHost
	if host == "" {
		host = "localhost"
	}
	port := dep.PGPort
	if port == "" {
		port = "5432"
	}
	user := dep.PGUser
	if user == "" {
		user = "postgres"
	}
	if dbname == "" {
		dbname = "postgres"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, user, dep.PGPassword, dbname)
}
