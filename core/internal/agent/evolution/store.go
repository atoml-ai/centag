package evolution

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Store agent_evolution_log 仓储。
type Store struct {
	db     *sql.DB
	driver string
}

// NewStore 构造仓储。
func NewStore(db *sql.DB, driver string) *Store { return &Store{db: db, driver: driver} }

// ph 数据库占位符：postgresql $n；sqlite/其他 ?。
func (s *Store) ph(n int) string {
	if s.driver == "postgresql" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// Insert 写入一条新提案记录（status=proposed；时间戳纳秒精度）。
func (s *Store) Insert(ctx context.Context, row *LogRow) error {
	const base = `INSERT INTO agent_evolution_log
(id, session_id, target, proposal, status, applied_at, effect_measure, rollback_of, created_at, updated_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`
	q := fmt.Sprintf(base,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10))
	_, err := s.db.ExecContext(ctx, q, row.ID, row.SessionID, row.Target, string(row.Proposal),
		row.Status, row.AppliedAt, row.Effect, row.RollbackOf, now(), now())
	if err != nil {
		return fmt.Errorf("insert evolution log: %w", err)
	}
	return nil
}

// Get 取回一条提案记录。
func (s *Store) Get(ctx context.Context, id string) (*LogRow, error) {
	q := fmt.Sprintf(`SELECT id, session_id, target, proposal, status,
applied_at, effect_measure, rollback_of FROM agent_evolution_log WHERE id = %s`, s.ph(1))
	row := s.db.QueryRowContext(ctx, q, id)
	return s.scan(row)
}

// LatestByTarget 取该目标最近一条记录（confirm 闭环时拒绝并发提案冲突）。
func (s *Store) LatestByTarget(ctx context.Context, target string) (*LogRow, error) {
	q := fmt.Sprintf(`SELECT id, session_id, target, proposal, status,
applied_at, effect_measure, rollback_of
FROM agent_evolution_log WHERE target = %s ORDER BY created_at DESC, id DESC LIMIT 1`, s.ph(1))
	row := s.db.QueryRowContext(ctx, q, target)
	r, err := s.scan(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("提案 target=%s 不存在", target)
	}
	return r, err
}

// UpdateStatus 更新状态（applied_at/rollback_of 可空传 ""）。
func (s *Store) UpdateStatus(ctx context.Context, id, status, appliedAt, rollbackOf string) error {
	q := fmt.Sprintf(`UPDATE agent_evolution_log
SET status = %s, applied_at = %s, rollback_of = %s, updated_at = %s
WHERE id = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5))
	_, err := s.db.ExecContext(ctx, q, status, appliedAt, rollbackOf, now(), id)
	if err != nil {
		return fmt.Errorf("update evolution log: %w", err)
	}
	return nil
}

// StoreEffect 更新效果快照字段。
func (s *Store) StoreEffect(ctx context.Context, id, effect string) error {
	q := fmt.Sprintf(`UPDATE agent_evolution_log
SET effect_measure = %s, updated_at = %s WHERE id = %s`, s.ph(1), s.ph(2), s.ph(3))
	_, err := s.db.ExecContext(ctx, q, effect, now(), id)
	if err != nil {
		return fmt.Errorf("update evolution effect: %w", err)
	}
	return nil
}

// ListBySession 会话审计回看记录（降序，limit 默认 50）。
func (s *Store) ListBySession(ctx context.Context, sessionID string, limit int) ([]*LogRow, error) {
	if limit <= 0 {
		limit = 50
	}
	q := fmt.Sprintf(`SELECT id, session_id, target, proposal, status,
applied_at, effect_measure, rollback_of
FROM agent_evolution_log WHERE session_id = %s ORDER BY created_at DESC, id DESC LIMIT %d`, s.ph(1), limit)
	rows, err := s.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list evolution log: %w", err)
	}
	defer rows.Close()
	var out []*LogRow
	for rows.Next() {
		r, err := s.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scan(r scanner) (*LogRow, error) {
	var m LogRow
	var appliedAt, effect, rollbackOf sql.NullString
	if err := r.Scan(&m.ID, &m.SessionID, &m.Target, &m.Proposal, &m.Status,
		&appliedAt, &effect, &rollbackOf); err != nil {
		return nil, err
	}
	m.AppliedAt = appliedAt.String
	m.Effect = effect.String
	m.RollbackOf = rollbackOf.String
	return &m, nil
}

// now RFC3339Nano 时间戳（UTC，纳秒精度保证排序稳定）。
func now() string { return timeNow().Format(time.RFC3339Nano) }
