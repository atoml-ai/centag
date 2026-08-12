package database

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pre035Schema matches the token_usage / token_usage_daily shape expected by the
// codebase before migrations 035 (cost_input_price / cost_output_price) and
// 036 (group_id) are applied.
const pre035Schema = `
CREATE TABLE token_usage (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	api_key_id INTEGER,
	backend_id TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt_tokens INTEGER DEFAULT 0,
	completion_tokens INTEGER DEFAULT 0,
	total_tokens INTEGER DEFAULT 0,
	request_id TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	tenant_id TEXT,
	cost_usd REAL DEFAULT 0,
	input_cost REAL DEFAULT 0,
	output_cost REAL DEFAULT 0,
	revenue_usd REAL DEFAULT 0,
	revenue_input_price REAL DEFAULT 0,
	revenue_output_price REAL DEFAULT 0,
	pricing_rule_id INTEGER,
	success INTEGER NOT NULL DEFAULT 1,
	dept_tag TEXT,
	agent_type TEXT
);
CREATE TABLE token_usage_daily (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	backend_id TEXT NOT NULL,
	model TEXT NOT NULL,
	agent_type TEXT,
	date DATE NOT NULL,
	total_prompt_tokens INTEGER DEFAULT 0,
	total_completion_tokens INTEGER DEFAULT 0,
	total_tokens INTEGER DEFAULT 0,
	total_cost_usd REAL DEFAULT 0,
	total_revenue_usd REAL DEFAULT 0,
	request_count INTEGER DEFAULT 0,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, backend_id, model, agent_type, date)
);
`

func TestMigration035036_AddColumns(t *testing.T) {
	db := initTestSQLiteDB(t)

	_, err := db.Exec(pre035Schema)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO token_usage (user_id, backend_id, model, cost_usd) VALUES (1, 'b1', 'm1', 0.5)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO token_usage_daily (user_id, backend_id, model, agent_type, date, total_cost_usd) VALUES (1, 'b1', 'm1', 'chat', '2026-08-12', 0.5)`)
	require.NoError(t, err)

	sql035, err := os.ReadFile("migrations/035_token_usage_cost_prices.sqlite.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(sql035))
	require.NoError(t, err, "apply 035")

	sql036, err := os.ReadFile("migrations/036_token_usage_group_id.sqlite.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(sql036))
	require.NoError(t, err, "apply 036")

	// Pre-existing rows must be preserved with the new columns defaulted.
	var costIn, costOut float64
	var groupID sql.NullString
	err = db.QueryRow(`SELECT cost_input_price, cost_output_price, group_id FROM token_usage WHERE user_id = 1`).
		Scan(&costIn, &costOut, &groupID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, costIn)
	assert.Equal(t, 0.0, costOut)
	assert.False(t, groupID.Valid)

	err = db.QueryRow(`SELECT cost_input_price, cost_output_price, group_id FROM token_usage_daily WHERE user_id = 1`).
		Scan(&costIn, &costOut, &groupID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, costIn)
	assert.Equal(t, 0.0, costOut)
	assert.False(t, groupID.Valid)

	// New columns must be writable.
	_, err = db.Exec(`
		INSERT INTO token_usage (user_id, backend_id, model, cost_usd, cost_input_price, cost_output_price, group_id)
		VALUES (2, 'b2', 'm2', 1.0, 1.5, 2.5, 'g_1')
	`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO token_usage_daily (user_id, backend_id, model, agent_type, date, total_cost_usd, cost_input_price, cost_output_price, group_id)
		VALUES (2, 'b2', 'm2', 'chat', '2026-08-12', 1.0, 1.5, 2.5, 'g_1')
	`)
	require.NoError(t, err)

	err = db.QueryRow(`SELECT cost_input_price, group_id FROM token_usage WHERE user_id = 2`).
		Scan(&costIn, &groupID)
	require.NoError(t, err)
	assert.Equal(t, 1.5, costIn)
	assert.True(t, groupID.Valid)
	assert.Equal(t, "g_1", groupID.String)
}

// TestMigrateVersionOverlapSQLite 复现「版本倒挂」：库已执行 037（widen_api_key_prefix）
// 但缺失 035/036（后提交），且 035/036 的列已由其他通道（如 centag-pro EnsureSchema）
// 写入。迁移器重跑 035/036 时必须因列已存在而跳过，而非报 duplicate column name。
func TestMigrateVersionOverlapSQLite(t *testing.T) {
	db := initTestSQLiteDB(t)

	// 先按 001-037 全链迁移，得到完整 schema。
	m := NewMigrator(db, "sqlite")
	require.NoError(t, m.Migrate(), "apply full migration chain")

	// 模拟旧库状态：清空并只保留 001-034 与 037 的记录（035/036 缺失）。
	_, err := db.Exec(`DELETE FROM schema_migrations`)
	require.NoError(t, err)
	for _, v := range []string{
		"001", "002", "003", "004", "005", "006", "007", "008", "010", "011",
		"014", "015", "016", "017", "018", "019", "020", "021", "022", "023",
		"024", "025", "026", "027", "028", "029", "030", "031", "032", "033",
		"034", "037",
	} {
		_, err = db.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, 'seed')`, v)
		require.NoError(t, err)
	}

	// 重新跑迁移器：不应报 duplicate column name，且 035/036 被补记。
	require.NoError(t, m.Migrate(), "version-overlap replay must not fail")

	for _, v := range []string{"035", "036"} {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, v).Scan(&n))
		assert.Equal(t, 1, n, "migration %s should be recorded after overlap replay", v)
	}
	// 列仍然存在且可写。
	cols := sqliteColumns(t, db, "token_usage")
	assert.Contains(t, cols, "cost_input_price")
	assert.Contains(t, cols, "cost_output_price")
	assert.Contains(t, cols, "group_id")
}

// TestColumnExists_SQLiteAndPG 覆盖 columnExists 的判定逻辑：
// SQLite 走 pragma_table_info，PostgreSQL 走 information_schema。
func TestColumnExists(t *testing.T) {
	db := initTestSQLiteDB(t)
	_, err := db.Exec(`CREATE TABLE foo (id INTEGER PRIMARY KEY, bar TEXT, baz INTEGER)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// 已存在的列 → skip=true
	skip, err := columnExists(tx, "sqlite", `ALTER TABLE foo ADD COLUMN bar TEXT`)
	require.NoError(t, err)
	assert.True(t, skip, "existing column bar should be skipped")

	// 不存在的列 → skip=false
	skip, err = columnExists(tx, "sqlite", `ALTER TABLE foo ADD COLUMN qux REAL DEFAULT 0`)
	require.NoError(t, err)
	assert.False(t, skip, "missing column qux should not be skipped")

	// 非 ADD COLUMN 语句 → skip=false
	skip, err = columnExists(tx, "sqlite", `CREATE INDEX IF NOT EXISTS idx_foo ON foo(id)`)
	require.NoError(t, err)
	assert.False(t, skip)

	// 带 COLUMN 关键字与大小写、带引号表名
	skip, err = columnExists(tx, "sqlite", `ALTER TABLE "foo" ADD COLUMN "bar" TEXT`)
	require.NoError(t, err)
	assert.True(t, skip)

	// PostgreSQL 分支：正则解析路径与 SQLite 共用，这里仅验证解析不误伤
	// （真实 PG 的 information_schema 查询不在本机测试环境执行）。
	table, column, ok := parseAddColumn(`ALTER TABLE foo ADD COLUMN bar TEXT`)
	assert.True(t, ok)
	assert.Equal(t, "foo", table)
	assert.Equal(t, "bar", column)
}

func TestMigrateFullChainSQLite_Includes035036(t *testing.T) {
	db := initTestSQLiteDB(t)

	m := NewMigrator(db, "sqlite")
	require.NoError(t, m.Migrate(), "apply full migration chain")

	// Verify 035/036 landed on both tables.
	for _, table := range []string{"token_usage", "token_usage_daily"} {
		cols := sqliteColumns(t, db, table)
		assert.Contains(t, cols, "cost_input_price", table)
		assert.Contains(t, cols, "cost_output_price", table)
		assert.Contains(t, cols, "group_id", table)
	}
	assert.Contains(t, sqliteColumns(t, db, "token_usage_daily"), "total_revenue_usd")

	// The upsert used by the token-usage service must succeed end-to-end.
	_, err := db.Exec(`
		INSERT INTO token_usage
			(user_id, backend_id, model, prompt_tokens, completion_tokens, total_tokens, request_id,
			 tenant_id, cost_usd, input_cost, output_cost, cost_input_price, cost_output_price,
			 revenue_usd, revenue_input_price, revenue_output_price, pricing_rule_id, success, dept_tag, agent_type, group_id)
		VALUES
			(1, 'b1', 'm1', 100, 200, 300, 'req-1',
			 't_1', 0.5, 0.2, 0.3, 1.5, 2.5,
			 1.0, 2.0, 3.0, 42, 1, 'ops', 'chat', 'g_1')
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO token_usage_daily
			(user_id, backend_id, model, agent_type, date, total_prompt_tokens, total_completion_tokens,
			 total_tokens, total_cost_usd, cost_input_price, cost_output_price, total_revenue_usd,
			 request_count, group_id)
		VALUES
			(1, 'b1', 'm1', 'chat', '2026-08-12', 100, 200, 300,
			 0.5, 1.5, 2.5, 1.0, 1, 'g_1')
	`)
	require.NoError(t, err)
}

func sqliteColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	return cols
}
