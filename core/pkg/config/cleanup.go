package config

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CleanupResult 描述一次卸载数据清理的执行结果。
type CleanupResult struct {
	Driver     string // 实际生效的元数据库驱动（sqlite | postgresql | ""）
	Cleaned    bool   // 是否执行了实际清理
	Skipped    bool   // 是否跳过（驱动非 PG / 无配置文件走默认 sqlite）
	SkipReason string // 跳过原因（Skipped=true 时有值）
	Error      error  // 清理失败错误（读配置失败 / 连接/执行失败）
}

// CleanupDeploymentData 是卸载时清理部署级数据的跨平台入口。
//
// 仅当 centag.conf 明确配置 db_driver=postgresql 时才连接 PostgreSQL，
// 并 DROP 目标数据库 public schema 下的所有表（级联），保留库本身与扩展
// （迁移脚本在重装后会重建表结构）。SQLite 或未配置时直接跳过，绝不误连。
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

	if dep.DBDriver != "postgresql" {
		return CleanupResult{
			Driver:     dep.DBDriver,
			Skipped:    true,
			SkipReason: "元数据库驱动非 postgresql，跳过数据库清理",
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	cleanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dsn := buildCleanupPGDSN(dep)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return CleanupResult{
			Driver: dep.DBDriver,
			Error:  fmt.Errorf("连接 PostgreSQL 失败: %w", err),
		}
	}
	defer db.Close()

	if err := db.PingContext(cleanCtx); err != nil {
		return CleanupResult{
			Driver: dep.DBDriver,
			Error:  fmt.Errorf("ping PostgreSQL 失败: %w", err),
		}
	}

	const dropSQL = `DO $$ DECLARE r RECORD; BEGIN FOR r IN SELECT tablename FROM pg_tables WHERE schemaname='public' LOOP EXECUTE format('DROP TABLE IF EXISTS %I CASCADE', r.tablename); END LOOP; END $$;`
	if _, err := db.ExecContext(cleanCtx, dropSQL); err != nil {
		return CleanupResult{
			Driver: dep.DBDriver,
			Error:  fmt.Errorf("清理 PostgreSQL public schema 失败: %w", err),
		}
	}

	return CleanupResult{
		Driver:  dep.DBDriver,
		Cleaned: true,
	}
}

// buildCleanupPGDSN 基于 DeploymentConfig 构建 PostgreSQL 连接串。
// sslmode 默认 disable，避免 pgx 因空值报 "sslmode is invalid"。
func buildCleanupPGDSN(dep DeploymentConfig) string {
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
	db := dep.PGDB
	if db == "" {
		db = "centag"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, user, dep.PGPassword, db)
}
