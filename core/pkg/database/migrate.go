package database

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"centag/core/pkg/logger"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migratorInfof 在 centag 主进程（已 Init logger）中写入 zap；独立运行 cmd/migrate 时回退到标准输出。
func migratorInfof(template string, args ...interface{}) {
	if logger.Logger != nil {
		logger.Sugar.Infof(template, args...)
		return
	}
	fmt.Printf(template+"\n", args...)
}

// Migration 表示一个数据库迁移
type Migration struct {
	Version     string
	Name        string
	Description string
	Created     time.Time
	UpSQL       string
	DownSQL     string
}

// Migrator 数据库迁移器
type Migrator struct {
	db     *sql.DB
	dbType string // "postgresql" 或 "sqlite"
}

// NewMigrator 创建迁移器
func NewMigrator(db *sql.DB, dbType string) *Migrator {
	return &Migrator{db: db, dbType: dbType}
}

// Migrate 执行所有未执行的迁移
func (m *Migrator) Migrate() error {
	dbName := m.dbType
	if dbName == "" {
		dbName = "postgresql"
	}
	migratorInfof("[MIGRATE] 开始执行数据库 Schema 迁移（%s）", dbName)

	// 创建迁移记录表
	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// 获取已执行的迁移版本
	executed, err := m.getExecutedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// 获取所有迁移文件
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	if len(migrations) == 0 {
		migratorInfof("[MIGRATE] 未找到迁移文件")
		return nil
	}

	migratorInfof("[MIGRATE] 共 %d 个迁移版本，已执行 %d 个", len(migrations), len(executed))

	// 执行未执行的迁移
	applied := 0
	for _, migration := range migrations {
		if contains(executed, migration.Version) {
			continue
		}

		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}

		applied++
		migratorInfof("[MIGRATE] 已应用迁移: %s - %s", migration.Version, migration.Name)
	}

	if applied == 0 {
		migratorInfof("[MIGRATE] 数据库 Schema 已是最新，无需新迁移")
	} else {
		migratorInfof("[MIGRATE] 本轮新应用迁移 %d 条，Schema 更新完成", applied)
	}

	return nil
}

// Rollback 回滚最后一个迁移
func (m *Migrator) Rollback() error {
	migratorInfof("[MIGRATE] 正在回滚上一版迁移…")

	// 获取已执行的迁移（按时间倒序）
	executed, err := m.getExecutedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get executed migrations: %w", err)
	}

	if len(executed) == 0 {
		return fmt.Errorf("no migrations to rollback")
	}

	// 获取最后一个迁移版本
	lastVersion := executed[len(executed)-1]

	// 加载迁移
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// 找到对应的迁移
	var target *Migration
	for _, mig := range migrations {
		if mig.Version == lastVersion {
			target = &mig
			break
		}
	}

	if target == nil {
		return fmt.Errorf("migration %s not found", lastVersion)
	}

	if target.DownSQL == "" {
		return fmt.Errorf("migration %s has no down migration", lastVersion)
	}

	// 执行回滚
	if err := m.executeSQL(target.DownSQL); err != nil {
		return fmt.Errorf("failed to execute down migration: %w", err)
	}

	// 从记录表中删除
	if err := m.removeMigrationRecord(lastVersion); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	migratorInfof("[MIGRATE] 已回滚迁移: %s - %s", target.Version, target.Name)
	return nil
}

// Status 显示迁移状态
func (m *Migrator) Status() error {
	fmt.Println("[MIGRATE] Migration Status:")
	fmt.Println(strings.Repeat("=", 80))

	executed, err := m.getExecutedMigrations()
	if err != nil {
		return err
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return err
	}

	fmt.Printf("%-10s %-40s %-10s\n", "Version", "Name", "Status")
	fmt.Println(strings.Repeat("-", 80))

	for _, mig := range migrations {
		status := "⏳ Pending"
		if contains(executed, mig.Version) {
			status = "✅ Applied"
		}
		fmt.Printf("%-10s %-40s %-10s\n", mig.Version, mig.Name, status)
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total: %d migrations, %d applied, %d pending\n",
		len(migrations), len(executed), len(migrations)-len(executed))

	return nil
}

// ensureMigrationsTable 创建迁移记录表
func (m *Migrator) ensureMigrationsTable() error {
	var query string
	if m.dbType == "sqlite" {
		// SQLite 语法
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version TEXT UNIQUE NOT NULL,
				name TEXT,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		// PostgreSQL 语法
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id SERIAL PRIMARY KEY,
				version VARCHAR(100) UNIQUE NOT NULL,
				name VARCHAR(255),
				applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	_, err := m.db.Exec(query)
	return err
}

// getExecutedMigrations 获取已执行的迁移版本列表
func (m *Migrator) getExecutedMigrations() ([]string, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// loadMigrations 加载所有迁移文件
func (m *Migrator) loadMigrations() ([]Migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// 根据 dbType 加载对应的迁移文件
		// 支持三种文件类型：
		// - *.common.sql - 通用迁移（两种数据库都适用）
		// - *.postgresql.sql - PostgreSQL 专属
		// - *.sqlite.sql - SQLite 专属
		isCommon := strings.Contains(name, "common")
		isPostgreSQL := strings.Contains(name, "postgresql")
		isSQLite := strings.Contains(name, "sqlite")

		// 如果没有指定 dbType 或为 postgresql，加载 common 和 postgresql
		// 如果为 sqlite，加载 common 和 sqlite
		shouldLoad := false
		if m.dbType == "" || m.dbType == "postgresql" {
			shouldLoad = isCommon || isPostgreSQL
		} else if m.dbType == "sqlite" {
			shouldLoad = isCommon || isSQLite
		}

		if !shouldLoad {
			continue
		}

		// 解析迁移文件
		migration, err := m.parseMigrationFile(name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", name, err)
		}

		migrations = append(migrations, migration)
	}

	// 按版本号排序
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseMigrationFile 解析迁移文件
func (m *Migrator) parseMigrationFile(filename string) (Migration, error) {
	content, err := migrationFS.ReadFile(path.Join("migrations", filename))
	if err != nil {
		return Migration{}, err
	}

	migration := Migration{
		Version: extractVersion(filename),
		Name:    extractName(filename),
	}

	contentStr := string(content)

	// 检查是否有 up/down 标记
	if strings.Contains(contentStr, "-- +migrate Up") {
		// 新格式：有 up/down 标记
		parts := strings.Split(contentStr, "-- +migrate")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "Up") {
				sql := strings.TrimSpace(strings.TrimPrefix(part, "Up"))
				migration.UpSQL = sql
			} else if strings.HasPrefix(part, "Down") {
				sql := strings.TrimSpace(strings.TrimPrefix(part, "Down"))
				migration.DownSQL = sql
			}
		}
	} else {
		// 旧格式：整个文件都是 up SQL
		migration.UpSQL = contentStr
		// 旧迁移不支持回滚
		migration.DownSQL = ""
	}

	return migration, nil
}

// applyMigration 应用单个迁移
func (m *Migrator) applyMigration(migration Migration) error {
	if migration.UpSQL == "" {
		return fmt.Errorf("migration has no up SQL")
	}

	// 在事务中执行
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 执行 Up SQL
	if err := m.executeSQLInTx(tx, migration.UpSQL); err != nil {
		return err
	}

	// 记录迁移
	insertQuery := `INSERT INTO schema_migrations (version, name) VALUES (?, ?)`
	if m.dbType != "sqlite" {
		insertQuery = `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`
	}

	_, err = tx.Exec(insertQuery, migration.Version, migration.Name)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// executeSQL 执行 SQL（支持多语句）
func (m *Migrator) executeSQL(sqlStr string) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := m.executeSQLInTx(tx, sqlStr); err != nil {
		return err
	}

	return tx.Commit()
}

// stripLeadingLineComments 去掉语句块开头的空行与整行 `--` 注释。
// 按分号切分后，常见片段以「-- 表说明」开头再接 CREATE；若整段 HasPrefix("--") 会误跳过整段 DDL。
func stripLeadingLineComments(stmt string) string {
	lines := strings.Split(stmt, "\n")
	start := 0
	for start < len(lines) {
		t := strings.TrimSpace(lines[start])
		if t == "" {
			start++
			continue
		}
		if strings.HasPrefix(t, "--") {
			start++
			continue
		}
		break
	}
	if start >= len(lines) {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

// executeSQLInTx 在事务中执行 SQL
func (m *Migrator) executeSQLInTx(tx *sql.Tx, sqlStr string) error {
	// 分割 SQL 语句（按分号分割，但要注意注释中的分号）
	statements := splitSQLStatements(sqlStr)

	for _, stmt := range statements {
		stmt = stripLeadingLineComments(stmt)
		if stmt == "" {
			continue
		}

		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %w\nSQL: %s", err, stmt)
		}
	}

	return nil
}

// removeMigrationRecord 从记录表中删除迁移
func (m *Migrator) removeMigrationRecord(version string) error {
	query := `DELETE FROM schema_migrations WHERE version = ?`
	if m.dbType != "sqlite" {
		query = `DELETE FROM schema_migrations WHERE version = $1`
	}

	_, err := m.db.Exec(query, version)
	return err
}

// 辅助函数

// extractVersion 从文件名提取版本号
func extractVersion(filename string) string {
	// 格式：001_name.postgresql.sql -> 001
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		return matches[1]
	}
	return "unknown"
}

// extractName 从文件名提取名称
func extractName(filename string) string {
	// 格式：001_add_indexes.postgresql.sql -> add_indexes
	// 格式：001_add_indexes.sqlite.sql -> add_indexes
	// 格式：001_add_indexes.common.sql -> add_indexes
	re := regexp.MustCompile(`^\d+_(.+?)\.(postgresql|sqlite|common)\.sql$`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		return matches[1]
	}
	return filename
}

// contains 检查切片是否包含某项
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// skipDollarQuoted 若 s[start] 为 PostgreSQL 美元引用起始（$tag$ 或 $$），返回闭合后的下标；否则返回 start。
func skipDollarQuoted(s string, start int) int {
	if start >= len(s) || s[start] != '$' {
		return start
	}
	if start+1 < len(s) && s[start+1] == '$' {
		rest := s[start+2:]
		idx := strings.Index(rest, "$$")
		if idx < 0 {
			return start
		}
		return start + 2 + idx + 2
	}
	j := start + 1
	for j < len(s) && s[j] != '$' {
		j++
	}
	if j >= len(s) {
		return start
	}
	tag := s[start+1 : j]
	close := "$" + tag + "$"
	rest := s[j+1:]
	idx := strings.Index(rest, close)
	if idx < 0 {
		return start
	}
	return j + 1 + idx + len(close)
}

// splitSQLStatements 分割 SQL 语句（行注释与 PostgreSQL $$ 美元引用内的分号不拆句）
func splitSQLStatements(sql string) []string {
	var statements []string
	var b strings.Builder
	s := sql
	i := 0
	for i < len(s) {
		// 行注释 -- 直到换行
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			for i < len(s) && s[i] != '\n' {
				b.WriteByte(s[i])
				i++
			}
			continue
		}

		if s[i] == '$' {
			end := skipDollarQuoted(s, i)
			if end > i {
				b.WriteString(s[i:end])
				i = end
				continue
			}
		}

		if s[i] == ';' {
			statements = append(statements, strings.TrimSpace(b.String()))
			b.Reset()
			i++
			continue
		}

		b.WriteByte(s[i])
		i++
	}
	if tail := strings.TrimSpace(b.String()); tail != "" {
		statements = append(statements, tail)
	}
	return statements
}
