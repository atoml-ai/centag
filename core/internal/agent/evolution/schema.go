// Package evolution 自进化范式宿主适配层（self-evolution-paradigm / v0.3.5 T2）。
//
// 范式资产在 edgeag `pkg/agentcore/evolution`（宿主无关）；本包是第一宿主
// 实现：agent_evolution_log 仓储 + 操作面状态机（propose/dryrun/apply/
// cancel/rollback）+ confirm 闭环 + learning 归档。
//
// 表 agent_evolution_log 的 DDL 正本在 `core/pkg/database/migrations/046_*`，
// 本包 EnsureSchema 仅兜底幂等建表（fail-safe，防裸库直接注入）。
package evolution

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// ensureOnce 兜底幂等建表只跑一次（并发安全）。
var ensureOnce sync.Once

// schemaDDL 幂等建表（与 046 迁移文件保持同字段；时间列统一 TEXT 以保证
// sqlite/postgresql 同构）。
const schemaDDL = `
CREATE TABLE IF NOT EXISTS agent_evolution_log (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    target TEXT NOT NULL,
    proposal TEXT NOT NULL,
    status TEXT NOT NULL,
    applied_at TEXT,
    effect_measure TEXT,
    rollback_of TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
)
`

// schemaIdxObj 幂等索引。
var schemaIdxObjs = []string{
	`CREATE INDEX IF NOT EXISTS idx_agent_evolution_log_session ON agent_evolution_log(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_agent_evolution_log_status ON agent_evolution_log(status)`,
}

// ensure Store 级幂等建表入口。
func (s *Store) ensure(ctx context.Context) error { return EnsureSchema(ctx, s.db) }

// driverFromDB 显式驱动名归一化（sqlite/postgresql，其余回退 sqlite）。
// 注：modernc sqlite 不暴露 Driver().Name()，驱动名的一致性由调用方显式传入
// （handler 持有 database.DriverName()）。
func driverFromDB(driver string) string {
	if driver == "postgresql" {
		return "postgresql"
	}
	return "sqlite"
}

// EnsureSchema 幂等建表 + 索引（TC-DB-EVO-001：重复调用不报错）。
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("evolution: db 不能为空")
	}
	stmts := append([]string{strings.TrimSpace(schemaDDL)}, schemaIdxObjs...)
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("evolution: ensure schema: %w", err)
		}
	}
	return nil
}
