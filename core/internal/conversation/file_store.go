package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileStore persists sessions as JSON metadata + JSONL messages under root.
type FileStore struct {
	root string
	mu   sync.Mutex
	locks sync.Map // sessionID -> *sync.Mutex
}

// NewFileStore creates a file-backed conversation store.
func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) sessionDir(id string) string {
	return filepath.Join(s.root, id)
}

func (s *FileStore) sessionLock(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (s *FileStore) EnsureSession(ctx context.Context, sess *Session) (*Session, error) {
	if sess == nil {
		return nil, fmt.Errorf("session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if sess.ID == "" {
		sess.ID = newID("s_")
	}
	if sess.Category == "" {
		sess.Category = "general"
	}
	dir := s.sessionDir(sess.ID)
	metaPath := filepath.Join(dir, "session.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		var existing Session
		if json.Unmarshal(data, &existing) == nil {
			return &existing, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	if err := writeJSON(metaPath, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *FileStore) AppendMessage(ctx context.Context, m *Message) error {
	if m == nil || m.SessionID == "" {
		return fmt.Errorf("invalid message")
	}
	lk := s.sessionLock(m.SessionID)
	lk.Lock()
	defer lk.Unlock()

	dir := s.sessionDir(m.SessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if m.ID == "" {
		m.ID = newID("m_")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	f, err := os.OpenFile(filepath.Join(dir, "messages.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(m); err != nil {
		return err
	}

	metaPath := filepath.Join(dir, "session.json")
	sess, _ := s.readSessionMeta(metaPath)
	if sess == nil {
		sess = &Session{ID: m.SessionID, Category: "general", CreatedAt: m.CreatedAt}
	}
	sess.MessageCount++
	sess.UpdatedAt = m.CreatedAt
	if sess.Title == "" && m.Role == "user" {
		sess.Title = truncate(m.Content, 80)
	}
	return writeJSON(metaPath, sess)
}

func (s *FileStore) ListSessions(ctx context.Context, q ListSessionsQuery) ([]*Session, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sess, err := s.readSessionMeta(filepath.Join(s.root, e.Name(), "session.json"))
		if err != nil || sess == nil {
			continue
		}
		if q.UserID != 0 && sess.UserID != q.UserID {
			continue
		}
		if q.TenantID != "" && sess.TenantID != q.TenantID {
			continue
		}
		if q.Category != "" && sess.Category != q.Category {
			continue
		}
		if !q.Since.IsZero() && sess.UpdatedAt.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && sess.UpdatedAt.After(q.Until) {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if q.Offset > len(out) {
		return nil, nil
	}
	end := q.Offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[q.Offset:end], nil
}

func (s *FileStore) GetSession(ctx context.Context, id string) (*Session, error) {
	return s.readSessionMeta(filepath.Join(s.sessionDir(id), "session.json"))
}

func (s *FileStore) ListMessages(ctx context.Context, sessionID string, q PageQuery) ([]*Message, error) {
	path := filepath.Join(s.sessionDir(sessionID), "messages.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []*Message
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m Message
		if json.Unmarshal([]byte(line), &m) == nil {
			all = append(all, &m)
		}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if q.Offset > len(all) {
		return nil, nil
	}
	end := q.Offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[q.Offset:end], nil
}

func (s *FileStore) ListCategories(ctx context.Context, userID int64, tenantID string) ([]string, error) {
	sessions, err := s.ListSessions(ctx, ListSessionsQuery{UserID: userID, TenantID: tenantID, Limit: 10000})
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var cats []string
	for _, sess := range sessions {
		c := sess.Category
		if c == "" {
			c = "general"
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats, nil
}

// sessionMatchesQuery reports whether a session matches the delete selection.
func sessionMatchesQuery(sess *Session, q DeleteSessionsQuery) bool {
	if sess == nil {
		return false
	}
	if len(q.IDs) > 0 {
		for _, id := range q.IDs {
			if id == sess.ID {
				return true
			}
		}
		return false
	}
	if q.UserID != 0 && sess.UserID != q.UserID {
		return false
	}
	if q.TenantID != "" && sess.TenantID != q.TenantID {
		return false
	}
	if q.Category != "" && sess.Category != q.Category {
		return false
	}
	if !q.Since.IsZero() && sess.UpdatedAt.Before(q.Since) {
		return false
	}
	if !q.Until.IsZero() && sess.UpdatedAt.After(q.Until) {
		return false
	}
	return true
}

func (s *FileStore) DeleteSessions(ctx context.Context, q DeleteSessionsQuery) (int64, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var deleted int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sess, err := s.readSessionMeta(filepath.Join(s.root, e.Name(), "session.json"))
		if err != nil || sess == nil {
			continue
		}
		if !sessionMatchesQuery(sess, q) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.root, e.Name())); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *FileStore) DeleteMessages(ctx context.Context, q DeleteMessagesQuery) (int64, error) {
	if q.SessionID == "" {
		return 0, nil
	}
	lk := s.sessionLock(q.SessionID)
	lk.Lock()
	defer lk.Unlock()

	dir := s.sessionDir(q.SessionID)
	path := filepath.Join(dir, "messages.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var kept []*Message
	var deleted int64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m Message
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		if messageMatchesQuery(&m, q) {
			deleted++
			continue
		}
		kept = append(kept, &m)
	}
	if deleted == 0 {
		return 0, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	enc := json.NewEncoder(f)
	for _, m := range kept {
		if err := enc.Encode(m); err != nil {
			f.Close()
			return 0, err
		}
	}
	if err := f.Close(); err != nil {
		return 0, err
	}

	metaPath := filepath.Join(dir, "session.json")
	if sess, err := s.readSessionMeta(metaPath); err == nil && sess != nil {
		sess.MessageCount = len(kept)
		_ = writeJSON(metaPath, sess)
	}
	return deleted, nil
}

func messageMatchesQuery(m *Message, q DeleteMessagesQuery) bool {
	if m == nil {
		return false
	}
	if len(q.IDs) > 0 {
		for _, id := range q.IDs {
			if id == m.ID {
				return true
			}
		}
		return false
	}
	return q.Role != "" && m.Role == q.Role
}

func (s *FileStore) readSessionMeta(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
