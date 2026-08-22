package server

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"centag/core/pkg/database"
)

var pgPlaceholderRe = regexp.MustCompile(`\$\d+`)

// agentSessionStore 内置 Agent 会话/消息持久化（agent_sessions / agent_messages，
// 040 迁移建表）。替换旧的内存 map：会话跨重启保留，并按 user_id 归属过滤。
type agentSessionStore struct {
	db      *sql.DB
	dialect database.Dialect
}

func newAgentSessionStore(db *sql.DB, driver string) *agentSessionStore {
	if db == nil {
		return nil
	}
	var d database.Dialect = &database.SQLiteDialect{}
	if driver == "postgresql" || driver == "postgres" {
		d = &database.PostgreSQLDialect{}
	}
	return &agentSessionStore{db: db, dialect: d}
}

// q 将查询中的 $n 占位符改写为当前方言占位符（SQLite 为 ?）。
func (s *agentSessionStore) q(query string) string {
	n := 0
	return pgPlaceholderRe.ReplaceAllStringFunc(query, func(m string) string {
		n++
		return s.dialect.Placeholder(n)
	})
}

const agentSessionColumns = `id, user_id, tenant_id, title, skill, backend_id, model, status, created_at, updated_at`

func (s *agentSessionStore) Create(ctx context.Context, sess *AgentSession) error {
	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO agent_sessions (id, user_id, tenant_id, title, skill, backend_id, model, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`),
		sess.ID, sess.UserID, sess.TenantID, sess.Title, sess.Skill, sess.BackendID, sess.Model, sess.Status, sess.CreatedAt, sess.UpdatedAt)
	return err
}

// Get 返回会话；不存在时返回 (nil, nil)。
func (s *agentSessionStore) Get(ctx context.Context, id string) (*AgentSession, error) {
	row := s.db.QueryRowContext(ctx, s.q(`SELECT `+agentSessionColumns+` FROM agent_sessions WHERE id = $1`), id)
	return scanAgentSession(row)
}

// List 按 updated_at 倒序返回会话；viewerID >= 0 且 includeAll=false 时仅返回
// 该用户本人与共享（user_id=0）会话。
func (s *agentSessionStore) List(ctx context.Context, viewerID int64, includeAll bool) ([]*AgentSession, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if includeAll || viewerID < 0 {
		rows, err = s.db.QueryContext(ctx, s.q(`SELECT `+agentSessionColumns+` FROM agent_sessions ORDER BY updated_at DESC`))
	} else {
		rows, err = s.db.QueryContext(ctx, s.q(`SELECT `+agentSessionColumns+` FROM agent_sessions WHERE user_id = $1 OR user_id = 0 ORDER BY updated_at DESC`), viewerID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*AgentSession, 0)
	for rows.Next() {
		sess, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// UpdateRuntimeFields 更新 backend/model 并刷新 updated_at。
func (s *agentSessionStore) UpdateRuntimeFields(ctx context.Context, id, backendID, model string) error {
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE agent_sessions SET backend_id = $1, model = $2, updated_at = $3 WHERE id = $4`),
		backendID, model, time.Now(), id)
	return err
}

// Delete 删除会话及其消息。
func (s *agentSessionStore) Delete(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, s.q(`DELETE FROM agent_messages WHERE session_id = $1`), id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, s.q(`DELETE FROM agent_sessions WHERE id = $1`), id)
	return err
}

// SetStatus 更新会话状态（cancel 等）。
func (s *agentSessionStore) SetStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE agent_sessions SET status = $1, updated_at = $2 WHERE id = $3`), status, time.Now(), id)
	return err
}

// AppendMessage 写入消息并累加会话 message_count / updated_at。
func (s *agentSessionStore) AppendMessage(ctx context.Context, m *AgentMessage) error {
	now := time.Now()
	if _, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO agent_messages (id, session_id, role, content, skill, tool_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`),
		m.ID, m.SessionID, m.Role, m.Content, m.Skill, m.ToolName, m.CreatedAt); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE agent_sessions SET message_count = message_count + 1, updated_at = $1 WHERE id = $2`), now, m.SessionID)
	return err
}

// ListMessages 按时间正序返回会话消息；会话不存在时返回 (nil, false, nil)。
func (s *agentSessionStore) ListMessages(ctx context.Context, sessionID string) ([]*AgentMessage, bool, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT 1 FROM agent_sessions WHERE id = $1`), sessionID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	rows, err := s.db.QueryContext(ctx, s.q(`
		SELECT id, session_id, role, content, skill, tool_name, created_at
		FROM agent_messages WHERE session_id = $1 ORDER BY created_at ASC, id ASC`), sessionID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := make([]*AgentMessage, 0)
	for rows.Next() {
		m := &AgentMessage{}
		var createdAt any
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Skill, &m.ToolName, &createdAt); err != nil {
			return nil, false, err
		}
		m.CreatedAt = parseAgentTime(s.dialect, createdAt)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanAgentSession(row rowScanner) (*AgentSession, error) {
	sess := &AgentSession{}
	var createdAt, updatedAt any
	err := row.Scan(&sess.ID, &sess.UserID, &sess.TenantID, &sess.Title, &sess.Skill,
		&sess.BackendID, &sess.Model, &sess.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sess.CreatedAt = parseAgentTime(nil, createdAt)
	sess.UpdatedAt = parseAgentTime(nil, updatedAt)
	return sess, nil
}

// parseAgentTime 兼容两种驱动返回：PG 为 time.Time，SQLite 为字符串。
func parseAgentTime(dialect database.Dialect, v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	if s, ok := v.(string); ok && dialect != nil {
		if t, err := dialect.ParseTime(s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
