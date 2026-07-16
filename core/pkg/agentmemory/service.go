// Package agentmemory persists OpenClaw / Web UI 云记忆到应用数据库（PostgreSQL 含 pgvector；SQLite 仅文档 + 关键词搜索）。
package agentmemory

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"centag/core/pkg/embedding"
)

// VectorDimension 须与 migrations/005_agent_memory.postgresql.sql 中 vector(N) 一致。
const VectorDimension = 1024

// Service 在 database.Get().GetDB() 可用时由 MemoryHandler 注入；nil 表示回退磁盘 MEMORY_STORE_ROOT。
type Service struct {
	db       *sql.DB
	postgres bool
	embed    embedding.EmbeddingService
}

// NewService driver 为 database.Manager.DriverName()，如 "postgresql"。
func NewService(db *sql.DB, driver string, embed embedding.EmbeddingService) *Service {
	if db == nil {
		return nil
	}
	d := strings.ToLower(strings.TrimSpace(driver))
	return &Service{
		db:       db,
		postgres: strings.Contains(d, "postgres"),
		embed:    embed,
	}
}

func parseUserID(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

func vectorToPgString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', 8, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// PutDoc 写入或覆盖文档并（PostgreSQL 且 embedding 可用时）重建向量分块。
func (s *Service) PutDoc(ctx context.Context, userID, namespace, path, content string) (vectors int, err error) {
	return s.putDocInternal(ctx, userID, namespace, path, content, true)
}

// PutDocWithoutIndex 写入或覆盖文档，但不重建向量分块（供异步索引队列使用）。
func (s *Service) PutDocWithoutIndex(ctx context.Context, userID, namespace, path, content string) (int, error) {
	return s.putDocInternal(ctx, userID, namespace, path, content, false)
}

func (s *Service) putDocInternal(ctx context.Context, userID, namespace, path, content string, rebuildIndex bool) (vectors int, err error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user_id: %w", err)
	}
	if namespace == "" {
		namespace = "main"
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, fmt.Errorf("path required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if s.postgres {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO agent_memory_docs (user_id, namespace, path, content, content_rev, deleted_at)
			VALUES ($1, $2, $3, $4, 1, NULL)
			ON CONFLICT (user_id, namespace, path) DO UPDATE SET
				content = EXCLUDED.content,
				content_rev = agent_memory_docs.content_rev + 1,
				updated_at = CURRENT_TIMESTAMP,
				deleted_at = NULL
		`, uid, namespace, path, content)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO agent_memory_docs (user_id, namespace, path, content, content_rev, deleted_at)
			VALUES (?, ?, ?, ?, 1, NULL)
			ON CONFLICT(user_id, namespace, path) DO UPDATE SET
				content = excluded.content,
				content_rev = content_rev + 1,
				updated_at = CURRENT_TIMESTAMP,
				deleted_at = NULL
		`, uid, namespace, path, content)
	}
	if err != nil {
		return 0, err
	}

	vectors = 0
	if rebuildIndex && s.postgres && s.embed != nil {
		if err := s.deleteChunksTx(ctx, tx, uid, namespace, path); err != nil {
			return 0, err
		}
		n, err := s.insertChunksTx(ctx, tx, uid, namespace, path, content)
		if err != nil {
			return 0, err
		}
		vectors = n
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return vectors, nil
}

// ReindexDoc 重建单个文档分块（仅 PostgreSQL）。
func (s *Service) ReindexDoc(ctx context.Context, userID, namespace, path string) (int, error) {
	if !s.postgres || s.embed == nil {
		return 0, nil
	}
	uid, err := parseUserID(userID)
	if err != nil {
		return 0, err
	}
	if namespace == "" {
		namespace = "main"
	}
	content, _, err := s.GetDocWithUpdatedAt(ctx, userID, namespace, path)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.deleteChunksTx(ctx, tx, uid, namespace, path); err != nil {
		return 0, err
	}
	nv, err := s.insertChunksTx(ctx, tx, uid, namespace, path, content)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nv, nil
}

func (s *Service) deleteChunksTx(ctx context.Context, tx *sql.Tx, uid int64, namespace, path string) error {
	if s.postgres {
		_, err := tx.ExecContext(ctx, `DELETE FROM agent_memory_chunks WHERE user_id = $1 AND namespace = $2 AND path = $3`, uid, namespace, path)
		return err
	}
	return nil
}

func (s *Service) insertChunksTx(ctx context.Context, tx *sql.Tx, uid int64, namespace, path, full string) (int, error) {
	lines := strings.Split(full, "\n")
	chunkSize := 10
	indexed := 0
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunkText := strings.TrimSpace(strings.Join(lines[i:end], "\n"))
		if chunkText == "" || strings.HasPrefix(chunkText, "# ") {
			continue
		}
		emb, err := s.embed.GetEmbedding(ctx, chunkText)
		if err != nil || len(emb) == 0 {
			continue
		}
		if len(emb) != VectorDimension {
			return indexed, fmt.Errorf("embedding dim %d != %d (check model vs migration)", len(emb), VectorDimension)
		}
		vecStr := vectorToPgString(emb)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO agent_memory_chunks (user_id, namespace, path, chunk_index, line_start, line_end, chunk_text, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::vector)
		`, uid, namespace, path, indexed+1, i+1, end, chunkText, vecStr)
		if err != nil {
			return indexed, err
		}
		indexed++
	}
	return indexed, nil
}

// AppendDoc 读取现有正文后追加并 PutDoc。
func (s *Service) AppendDoc(ctx context.Context, userID, namespace, path, appendContent string) (vectors int, err error) {
	return s.appendDocInternal(ctx, userID, namespace, path, appendContent, true)
}

// AppendDocWithoutIndex 读取现有正文后追加，并跳过向量重建。
func (s *Service) AppendDocWithoutIndex(ctx context.Context, userID, namespace, path, appendContent string) (int, error) {
	return s.appendDocInternal(ctx, userID, namespace, path, appendContent, false)
}

func (s *Service) appendDocInternal(ctx context.Context, userID, namespace, path, appendContent string, rebuildIndex bool) (vectors int, err error) {
	cur, err := s.GetDoc(ctx, userID, namespace, path)
	if err != nil {
		return s.putDocInternal(ctx, userID, namespace, path, appendContent, rebuildIndex)
	}
	var newContent string
	if cur != "" && !strings.HasSuffix(cur, "\n") {
		newContent = cur + "\n\n" + appendContent
	} else {
		newContent = cur + appendContent
	}
	return s.putDocInternal(ctx, userID, namespace, path, newContent, rebuildIndex)
}

// GetDoc 返回正文；不存在返回 ("", sql.ErrNoRows)。
func (s *Service) GetDoc(ctx context.Context, userID, namespace, path string) (string, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return "", err
	}
	if namespace == "" {
		namespace = "main"
	}
	var content string
	if s.postgres {
		err = s.db.QueryRowContext(ctx, `
			SELECT content FROM agent_memory_docs
			WHERE user_id = $1 AND namespace = $2 AND path = $3 AND deleted_at IS NULL
		`, uid, namespace, path).Scan(&content)
	} else {
		err = s.db.QueryRowContext(ctx, `
			SELECT content FROM agent_memory_docs
			WHERE user_id = ? AND namespace = ? AND path = ? AND deleted_at IS NULL
		`, uid, namespace, path).Scan(&content)
	}
	return content, err
}

// GetDocWithUpdatedAt 返回正文与 updated_at；不存在返回 ("", zero, sql.ErrNoRows)。
func (s *Service) GetDocWithUpdatedAt(ctx context.Context, userID, namespace, path string) (string, time.Time, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return "", time.Time{}, err
	}
	if namespace == "" {
		namespace = "main"
	}
	var content string
	var updatedAt time.Time
	if s.postgres {
		err = s.db.QueryRowContext(ctx, `
			SELECT content, updated_at FROM agent_memory_docs
			WHERE user_id = $1 AND namespace = $2 AND path = $3 AND deleted_at IS NULL
		`, uid, namespace, path).Scan(&content, &updatedAt)
	} else {
		var updatedRaw string
		err = s.db.QueryRowContext(ctx, `
			SELECT content, updated_at FROM agent_memory_docs
			WHERE user_id = ? AND namespace = ? AND path = ? AND deleted_at IS NULL
		`, uid, namespace, path).Scan(&content, &updatedRaw)
		if err == nil {
			layouts := []string{
				"2006-01-02 15:04:05.999999999-07:00",
				"2006-01-02 15:04:05.999999999",
				"2006-01-02 15:04:05",
				time.RFC3339Nano,
				time.RFC3339,
			}
			for _, layout := range layouts {
				if ts, parseErr := time.Parse(layout, updatedRaw); parseErr == nil {
					updatedAt = ts
					break
				}
			}
		}
	}
	return content, updatedAt, err
}

// DeleteDoc 硬删除文档与分块。
func (s *Service) DeleteDoc(ctx context.Context, userID, namespace, path string) error {
	uid, err := parseUserID(userID)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "main"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if s.postgres {
		_, _ = tx.ExecContext(ctx, `DELETE FROM agent_memory_chunks WHERE user_id = $1 AND namespace = $2 AND path = $3`, uid, namespace, path)
		res, err := tx.ExecContext(ctx, `DELETE FROM agent_memory_docs WHERE user_id = $1 AND namespace = $2 AND path = $3`, uid, namespace, path)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return sql.ErrNoRows
		}
	} else {
		res, err := tx.ExecContext(ctx, `DELETE FROM agent_memory_docs WHERE user_id = ? AND namespace = ? AND path = ?`, uid, namespace, path)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}

// ListPaths 列出当前用户+namespace 下未删除文档的 path（相对路径，正斜杠）。
func (s *Service) ListPaths(ctx context.Context, userID, namespace string) ([]string, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = "main"
	}
	var rows *sql.Rows
	if s.postgres {
		rows, err = s.db.QueryContext(ctx, `
			SELECT path FROM agent_memory_docs
			WHERE user_id = $1 AND namespace = $2 AND deleted_at IS NULL
			ORDER BY path
		`, uid, namespace)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT path FROM agent_memory_docs
			WHERE user_id = ? AND namespace = ? AND deleted_at IS NULL
			ORDER BY path
		`, uid, namespace)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, strings.ReplaceAll(p, "\\", "/"))
	}
	return out, rows.Err()
}

// SearchResult 与 memory_handler MemorySearchResult 对齐字段。
type SearchResult struct {
	Path      string
	Content   string
	Score     float64
	StartLine int
	EndLine   int
}

// Search 先尝试 pgvector；否则在文档正文上关键词匹配（SQLite 或无命中时）。
func (s *Service) Search(ctx context.Context, userID, namespace, query string, limit int) ([]SearchResult, string, error) {
	if limit <= 0 {
		limit = 8
	}
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, "", err
	}
	if namespace == "" {
		namespace = "main"
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, "", fmt.Errorf("empty query")
	}

	if s.postgres && s.embed != nil {
		qemb, err := s.embed.GetEmbedding(ctx, q)
		if err == nil && len(qemb) == VectorDimension {
			vecStr := vectorToPgString(qemb)
			rows, err := s.db.QueryContext(ctx, `
				SELECT path, chunk_text, line_start, line_end,
				       (1 - (embedding <=> $1::vector))::float8 AS score
				FROM agent_memory_chunks
				WHERE user_id = $2 AND namespace = $3
				ORDER BY embedding <=> $1::vector
				LIMIT $4
			`, vecStr, uid, namespace, limit)
			if err == nil {
				defer rows.Close()
				var out []SearchResult
				for rows.Next() {
					var path, chunk string
					var ls, le int
					var score float64
					if err := rows.Scan(&path, &chunk, &ls, &le, &score); err != nil {
						return nil, "", err
					}
					out = append(out, SearchResult{Path: path, Content: chunk, Score: score, StartLine: ls, EndLine: le})
				}
				if err := rows.Err(); err != nil {
					return nil, "", err
				}
				if len(out) > 0 {
					return out, "vector", nil
				}
			}
		}
	}

	// 关键词：扫描文档表
	var rows *sql.Rows
	if s.postgres {
		like := "%" + strings.ReplaceAll(strings.ReplaceAll(q, "%", "\\%"), "_", "\\_") + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT path, content FROM agent_memory_docs
			WHERE user_id = $1 AND namespace = $2 AND deleted_at IS NULL
			  AND content ILIKE $3 ESCAPE '\'
			ORDER BY path
			LIMIT $4
		`, uid, namespace, like, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT path, content FROM agent_memory_docs
			WHERE user_id = ? AND namespace = ? AND deleted_at IS NULL
			  AND INSTR(LOWER(content), LOWER(?)) > 0
			ORDER BY path
			LIMIT ?
		`, uid, namespace, q, limit)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []SearchResult
	ql := strings.ToLower(q)
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			return nil, "", err
		}
		lines := strings.Split(content, "\n")
		first := -1
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), ql) {
				first = i + 1
				break
			}
		}
		if first < 0 && strings.Contains(strings.ToLower(content), ql) {
			first = 1
		}
		if first < 0 {
			continue
		}
		lo, hi := first-1, first+4
		if lo < 1 {
			lo = 1
		}
		if hi > len(lines) {
			hi = len(lines)
		}
		snippet := strings.Join(lines[lo-1:hi], "\n")
		runes := []rune(snippet)
		if len(runes) > 2000 {
			snippet = string(runes[:2000])
		}
		out = append(out, SearchResult{
			Path: path, Content: snippet, Score: 0.55, StartLine: lo, EndLine: hi,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, "keyword", rows.Err()
}

// ReindexAllDocs 重建某用户+namespace 下所有文档的分块（仅 PostgreSQL）。
func (s *Service) ReindexAllDocs(ctx context.Context, userID, namespace string) (docs int, vectors int, err error) {
	if !s.postgres || s.embed == nil {
		return 0, 0, fmt.Errorf("reindex requires PostgreSQL and embedding service")
	}
	uid, err := parseUserID(userID)
	if err != nil {
		return 0, 0, err
	}
	if namespace == "" {
		namespace = "main"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, content FROM agent_memory_docs
		WHERE user_id = $1 AND namespace = $2 AND deleted_at IS NULL
	`, uid, namespace)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	totalVec := 0
	nDoc := 0
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			return nDoc, totalVec, err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nDoc, totalVec, err
		}
		if err := s.deleteChunksTx(ctx, tx, uid, namespace, path); err != nil {
			_ = tx.Rollback()
			return nDoc, totalVec, err
		}
		nv, err := s.insertChunksTx(ctx, tx, uid, namespace, path, content)
		if err != nil {
			_ = tx.Rollback()
			return nDoc, totalVec, err
		}
		if err := tx.Commit(); err != nil {
			return nDoc, totalVec, err
		}
		nDoc++
		totalVec += nv
	}
	return nDoc, totalVec, rows.Err()
}

// Stats 返回已索引的文档路径列表；PostgreSQL 时额外返回 agent_memory_chunks 行数（语义块数量）。
func (s *Service) Stats(ctx context.Context, userID, namespace string) (paths []string, vectorChunks int, err error) {
	paths, err = s.ListPaths(ctx, userID, namespace)
	if err != nil {
		return nil, 0, err
	}
	if !s.postgres {
		return paths, 0, nil
	}
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if namespace == "" {
		namespace = "main"
	}
	var n int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_memory_chunks
		WHERE user_id = $1 AND namespace = $2
	`, uid, namespace).Scan(&n)
	if err != nil {
		return paths, 0, err
	}
	return paths, int(n), nil
}

// ListPathsAfter 列出 updated_at 在 after 之后的文档 path；after 为零时间与 ListPaths 相同。
func (s *Service) ListPathsAfter(ctx context.Context, userID, namespace string, after time.Time) ([]string, error) {
	if after.IsZero() || after.Unix() <= 0 {
		return s.ListPaths(ctx, userID, namespace)
	}
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = "main"
	}
	var rows *sql.Rows
	if s.postgres {
		rows, err = s.db.QueryContext(ctx, `
			SELECT path FROM agent_memory_docs
			WHERE user_id = $1 AND namespace = $2 AND deleted_at IS NULL AND updated_at > $3
			ORDER BY path
		`, uid, namespace, after)
	} else {
		ts := after.UTC().Format("2006-01-02 15:04:05")
		rows, err = s.db.QueryContext(ctx, `
			SELECT path FROM agent_memory_docs
			WHERE user_id = ? AND namespace = ? AND deleted_at IS NULL AND updated_at > ?
			ORDER BY path
		`, uid, namespace, ts)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, strings.ReplaceAll(p, "\\", "/"))
	}
	return out, rows.Err()
}

// Enabled 用于 Handler 判断是否走库。
func (s *Service) Enabled() bool { return s != nil }

// Postgres 是否启用向量表。
func (s *Service) Postgres() bool { return s.postgres }

// ImportFromFile 将本地文件写入 DB（供 Sync 使用）。
func (s *Service) ImportFromFile(ctx context.Context, userID, namespace, relPath string, content []byte) (int, error) {
	return s.PutDoc(ctx, userID, namespace, relPath, string(content))
}
