package conversation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type sqlDialect int

const (
	dialectSQLite sqlDialect = iota
	dialectPostgres
)

// SQLStore implements Store on SQLite or PostgreSQL.
type SQLStore struct {
	db      *sql.DB
	dialect sqlDialect
}

// NewSQLStore creates a SQL-backed conversation store.
func NewSQLStore(db *sql.DB, d sqlDialect) *SQLStore {
	return &SQLStore{db: db, dialect: d}
}

func (s *SQLStore) rebind(q string) string {
	if s.dialect != dialectPostgres {
		return q
	}
	// convert ? placeholders to $1,$2,...
	var b strings.Builder
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

func (s *SQLStore) EnsureSession(ctx context.Context, sess *Session) (*Session, error) {
	if sess == nil {
		return nil, fmt.Errorf("session is nil")
	}
	now := time.Now().UTC()
	if sess.ID == "" {
		sess.ID = newID("s_")
	}
	if sess.Category == "" {
		sess.Category = "general"
	}
	if existing, err := s.GetSession(ctx, sess.ID); err == nil && existing != nil {
		return existing, nil
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	q := s.rebind(`INSERT INTO conversation_sessions
		(id, user_id, tenant_id, title, category, pipeline_id, proxy_mode, message_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q,
		sess.ID, sess.UserID, sess.TenantID, sess.Title, sess.Category,
		sess.PipelineID, sess.ProxyMode, sess.MessageCount, sess.CreatedAt, sess.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return sess, nil
}

func (s *SQLStore) AppendMessage(ctx context.Context, m *Message) error {
	if m == nil || m.SessionID == "" {
		return fmt.Errorf("invalid message")
	}
	if m.ID == "" {
		m.ID = newID("m_")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	q := s.rebind(`INSERT INTO conversation_messages
		(id, session_id, role, content, request_id, model, backend, pipeline_id,
		 input_tokens, output_tokens, latency_ms, status_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if _, err := tx.ExecContext(ctx, q,
		m.ID, m.SessionID, m.Role, m.Content, m.RequestID, m.Model, m.Backend, m.PipelineID,
		m.InputTokens, m.OutputTokens, m.LatencyMs, m.StatusCode, m.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	if m.Role == "user" {
		uq := s.rebind(`UPDATE conversation_sessions SET message_count = message_count + 1, updated_at = ?,
			title = CASE WHEN title = '' OR title IS NULL THEN ? ELSE title END WHERE id = ?`)
		if _, err := tx.ExecContext(ctx, uq, m.CreatedAt, truncate(m.Content, 80), m.SessionID); err != nil {
			return err
		}
	} else {
		uq := s.rebind(`UPDATE conversation_sessions SET message_count = message_count + 1, updated_at = ? WHERE id = ?`)
		if _, err := tx.ExecContext(ctx, uq, m.CreatedAt, m.SessionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) ListSessions(ctx context.Context, q ListSessionsQuery) ([]*Session, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	var conds []string
	var args []interface{}
	if q.UserID != 0 {
		conds = append(conds, "user_id = ?")
		args = append(args, q.UserID)
	}
	if q.TenantID != "" {
		conds = append(conds, "tenant_id = ?")
		args = append(args, q.TenantID)
	}
	if q.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, q.Category)
	}
	if !q.Since.IsZero() {
		conds = append(conds, "updated_at >= ?")
		args = append(args, q.Since)
	}
	if !q.Until.IsZero() {
		conds = append(conds, "updated_at <= ?")
		args = append(args, q.Until)
	}
	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}
	query := fmt.Sprintf(`SELECT id, user_id, tenant_id, title, category, pipeline_id, proxy_mode,
		message_count, created_at, updated_at FROM conversation_sessions WHERE %s
		ORDER BY updated_at DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLStore) GetSession(ctx context.Context, id string) (*Session, error) {
	q := s.rebind(`SELECT id, user_id, tenant_id, title, category, pipeline_id, proxy_mode,
		message_count, created_at, updated_at FROM conversation_sessions WHERE id = ?`)
	row := s.db.QueryRowContext(ctx, q, id)
	sess, err := scanSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *SQLStore) ListMessages(ctx context.Context, sessionID string, q PageQuery) ([]*Message, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	query := s.rebind(`SELECT id, session_id, role, content, request_id, model, backend, pipeline_id,
		input_tokens, output_tokens, latency_ms, status_code, created_at
		FROM conversation_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, query, sessionID, limit, q.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Message
	for rows.Next() {
		var m Message
		var created interface{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.RequestID, &m.Model, &m.Backend,
			&m.PipelineID, &m.InputTokens, &m.OutputTokens, &m.LatencyMs, &m.StatusCode, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = asTime(created)
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *SQLStore) ListCategories(ctx context.Context, userID int64, tenantID string) ([]string, error) {
	var conds []string
	var args []interface{}
	if userID != 0 {
		conds = append(conds, "user_id = ?")
		args = append(args, userID)
	}
	if tenantID != "" {
		conds = append(conds, "tenant_id = ?")
		args = append(args, tenantID)
	}
	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}
	q := s.rebind(fmt.Sprintf(`SELECT DISTINCT category FROM conversation_sessions WHERE %s ORDER BY category`, where))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if c == "" {
			c = "general"
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// deleteSessionConds builds the WHERE conditions (and args) selecting sessions to delete.
func deleteSessionConds(q DeleteSessionsQuery) ([]string, []interface{}) {
	var conds []string
	var args []interface{}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+strings.Repeat("?,", len(q.IDs)-1)+"?)")
		for _, id := range q.IDs {
			args = append(args, id)
		}
		return conds, args
	}
	if q.UserID != 0 {
		conds = append(conds, "user_id = ?")
		args = append(args, q.UserID)
	}
	if q.TenantID != "" {
		conds = append(conds, "tenant_id = ?")
		args = append(args, q.TenantID)
	}
	if q.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, q.Category)
	}
	if !q.Since.IsZero() {
		conds = append(conds, "updated_at >= ?")
		args = append(args, q.Since)
	}
	if !q.Until.IsZero() {
		conds = append(conds, "updated_at <= ?")
		args = append(args, q.Until)
	}
	return conds, args
}

func (s *SQLStore) DeleteSessions(ctx context.Context, q DeleteSessionsQuery) (int64, error) {
	conds, args := deleteSessionConds(q)
	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}
	idQuery := s.rebind(fmt.Sprintf(`SELECT id FROM conversation_sessions WHERE %s`, where))
	rows, err := s.db.QueryContext(ctx, idQuery, args...)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	in := strings.Repeat("?,", len(ids)-1) + "?"
	msgArgs := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		msgArgs = append(msgArgs, id)
	}
	_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM conversation_messages WHERE session_id IN (`+in+`)`), msgArgs...)
	if err != nil {
		return 0, fmt.Errorf("delete messages: %w", err)
	}
	res, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM conversation_sessions WHERE id IN (`+in+`)`), msgArgs...)
	if err != nil {
		return 0, fmt.Errorf("delete sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return int64(len(ids)), tx.Commit()
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *SQLStore) DeleteMessages(ctx context.Context, q DeleteMessagesQuery) (int64, error) {
	var conds []string
	var args []interface{}
	if q.SessionID != "" {
		conds = append(conds, "session_id = ?")
		args = append(args, q.SessionID)
	}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+strings.Repeat("?,", len(q.IDs)-1)+"?)")
		for _, id := range q.IDs {
			args = append(args, id)
		}
	} else if q.Role != "" {
		conds = append(conds, "role = ?")
		args = append(args, q.Role)
	}
	if len(conds) == 0 {
		return 0, nil
	}
	where := strings.Join(conds, " AND ")
	query := s.rebind(`DELETE FROM conversation_messages WHERE ` + where)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if q.SessionID != "" && n > 0 {
		uq := s.rebind(`UPDATE conversation_sessions SET message_count = (SELECT COUNT(*) FROM conversation_messages WHERE session_id = ?) WHERE id = ?`)
		_, _ = s.db.ExecContext(ctx, uq, q.SessionID, q.SessionID)
	}
	return n, nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanSession(row scannable) (*Session, error) {
	var sess Session
	var created, updated interface{}
	err := row.Scan(&sess.ID, &sess.UserID, &sess.TenantID, &sess.Title, &sess.Category,
		&sess.PipelineID, &sess.ProxyMode, &sess.MessageCount, &created, &updated)
	if err != nil {
		return nil, err
	}
	sess.CreatedAt = asTime(created)
	sess.UpdatedAt = asTime(updated)
	return &sess, nil
}

func asTime(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed
			}
		}
	case []byte:
		return asTime(string(t))
	}
	return time.Time{}
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var out []*Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}
