package conversation

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"centag/core/internal/edition"

	_ "modernc.org/sqlite"
)

func TestFileStore_RoundTrip(t *testing.T) {
	root := t.TempDir()
	store := NewFileStore(root)
	ctx := context.Background()

	sess, err := store.EnsureSession(ctx, &Session{UserID: 1, Category: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" {
		t.Fatal("expected session id")
	}
	again, err := store.EnsureSession(ctx, &Session{ID: sess.ID, UserID: 1})
	if err != nil || again.ID != sess.ID {
		t.Fatalf("ensure existing: %v %+v", err, again)
	}

	if err := store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "user", Content: "hello world"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "assistant", Content: "hi"}); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetSession(ctx, sess.ID)
	if err != nil || got.MessageCount != 2 {
		t.Fatalf("session count=%d err=%v", got.MessageCount, err)
	}
	msgs, err := store.ListMessages(ctx, sess.ID, PageQuery{Limit: 10})
	if err != nil || len(msgs) != 2 {
		t.Fatalf("msgs=%d err=%v", len(msgs), err)
	}
	list, err := store.ListSessions(ctx, ListSessionsQuery{UserID: 1, Category: "code"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	cats, err := store.ListCategories(ctx, 1, "")
	if err != nil || len(cats) != 1 || cats[0] != "code" {
		t.Fatalf("cats=%v err=%v", cats, err)
	}
}

func TestSQLStore_SQLiteRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite", "file:conv_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mig, err := os.ReadFile(filepath.Join("..", "..", "pkg", "database", "migrations", "029_conversations.sqlite.sql"))
	if err != nil {
		// fallback relative to module root when tests run from package dir
		mig, err = os.ReadFile(filepath.Join("pkg", "database", "migrations", "029_conversations.sqlite.sql"))
	}
	if err != nil {
		// embed via inline schema for robust tests
		mig = []byte(`
CREATE TABLE conversation_sessions (
    id TEXT PRIMARY KEY, user_id INTEGER NOT NULL DEFAULT 0, tenant_id TEXT DEFAULT '',
    title TEXT DEFAULT '', category TEXT DEFAULT 'general', pipeline_id TEXT DEFAULT '',
    proxy_mode TEXT DEFAULT '', message_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now'))
);
CREATE TABLE conversation_messages (
    id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT DEFAULT '',
    request_id TEXT DEFAULT '', model TEXT DEFAULT '', backend TEXT DEFAULT '', pipeline_id TEXT DEFAULT '',
    input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0, latency_ms INTEGER DEFAULT 0,
    status_code INTEGER DEFAULT 200, created_at TEXT DEFAULT (datetime('now'))
);`)
	}
	if _, err := db.Exec(string(mig)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewSQLStore(db, dialectSQLite)
	ctx := context.Background()
	sess, err := store.EnsureSession(ctx, &Session{UserID: 2, Category: "chat", TenantID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "user", Content: "q1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "assistant", Content: "a1", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetSession(ctx, sess.ID)
	if err != nil || got == nil || got.MessageCount != 2 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	msgs, err := store.ListMessages(ctx, sess.ID, PageQuery{})
	if err != nil || len(msgs) != 2 {
		t.Fatalf("msgs=%d err=%v", len(msgs), err)
	}
	list, err := store.ListSessions(ctx, ListSessionsQuery{UserID: 2, TenantID: "t", Category: "chat", Limit: 10})
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	cats, err := store.ListCategories(ctx, 2, "t")
	if err != nil || len(cats) != 1 || cats[0] != "chat" {
		t.Fatalf("cats=%v err=%v", cats, err)
	}
	missing, err := store.GetSession(ctx, "no-such-session")
	if err != nil || missing != nil {
		t.Fatalf("missing session want nil,nil got %+v err=%v", missing, err)
	}
}

func TestNewStore_ByEdition(t *testing.T) {
	fs, err := NewStore(Options{Edition: edition.Minimal, FileRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fs.(*FileStore); !ok {
		t.Fatalf("minimal want FileStore, got %T", fs)
	}

	db, err := sql.Open("sqlite", "file:conv_factory?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE conversation_sessions (id TEXT PRIMARY KEY, user_id INTEGER, tenant_id TEXT,
		title TEXT, category TEXT, pipeline_id TEXT, proxy_mode TEXT, message_count INTEGER, created_at TEXT, updated_at TEXT)`)
	_, _ = db.Exec(`CREATE TABLE conversation_messages (id TEXT PRIMARY KEY, session_id TEXT, role TEXT, content TEXT,
		request_id TEXT, model TEXT, backend TEXT, pipeline_id TEXT, input_tokens INTEGER, output_tokens INTEGER,
		latency_ms INTEGER, status_code INTEGER, created_at TEXT)`)

	ps, err := NewStore(Options{Edition: edition.Personal, DB: db, Driver: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ps.(*SQLStore); !ok {
		t.Fatalf("personal want SQLStore, got %T", ps)
	}
}

// seedDeleteStore populates a store with 3 sessions (2 messages each) for delete tests.
func seedDeleteStore(t *testing.T, store Store, ctx context.Context) {
	t.Helper()
	for _, cat := range []string{"chat", "code", "chat"} {
		sess, err := store.EnsureSession(ctx, &Session{UserID: 1, Category: cat})
		if err != nil {
			t.Fatal(err)
		}
		_ = store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "user", Content: "q"})
		_ = store.AppendMessage(ctx, &Message{SessionID: sess.ID, Role: "assistant", Content: "a"})
	}
}

func TestStore_DeleteSessions_ByID(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		s := NewFileStore(t.TempDir())
		testDeleteSessionsByID(t, s)
	})
	t.Run("sqlite", func(t *testing.T) {
		s := newSQLStore(t)
		testDeleteSessionsByID(t, s)
	})
}

func testDeleteSessionsByID(t *testing.T, store Store) {
	ctx := context.Background()
	seedDeleteStore(t, store, ctx)
	all, _ := store.ListSessions(ctx, ListSessionsQuery{Limit: 100})
	if len(all) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(all))
	}
	target := all[0]
	n, err := store.DeleteSessions(ctx, DeleteSessionsQuery{IDs: []string{target.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
	got, _ := store.GetSession(ctx, target.ID)
	if got != nil {
		t.Fatalf("session should be gone, got %+v", got)
	}
	msgs, _ := store.ListMessages(ctx, target.ID, PageQuery{})
	if len(msgs) != 0 {
		t.Fatalf("messages should be gone, got %d", len(msgs))
	}
	left, _ := store.ListSessions(ctx, ListSessionsQuery{Limit: 100})
	if len(left) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(left))
	}
}

func TestStore_DeleteSessions_ByFilter(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		s := NewFileStore(t.TempDir())
		testDeleteSessionsByFilter(t, s)
	})
	t.Run("sqlite", func(t *testing.T) {
		s := newSQLStore(t)
		testDeleteSessionsByFilter(t, s)
	})
}

func testDeleteSessionsByFilter(t *testing.T, store Store) {
	ctx := context.Background()
	seedDeleteStore(t, store, ctx)
	n, err := store.DeleteSessions(ctx, DeleteSessionsQuery{Category: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 code session deleted, got %d", n)
	}
	left, _ := store.ListSessions(ctx, ListSessionsQuery{Limit: 100})
	if len(left) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(left))
	}
}

func TestStore_DeleteMessages(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		s := NewFileStore(t.TempDir())
		testDeleteMessages(t, s)
	})
	t.Run("sqlite", func(t *testing.T) {
		s := newSQLStore(t)
		testDeleteMessages(t, s)
	})
}

func testDeleteMessages(t *testing.T, store Store) {
	ctx := context.Background()
	seedDeleteStore(t, store, ctx)
	all, _ := store.ListSessions(ctx, ListSessionsQuery{Limit: 100})
	if len(all) == 0 {
		t.Fatal("no sessions")
	}
	sid := all[0].ID
	msgs, _ := store.ListMessages(ctx, sid, PageQuery{})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	// 单选删除一条 user 消息
	n, err := store.DeleteMessages(ctx, DeleteMessagesQuery{SessionID: sid, IDs: []string{msgs[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 message deleted, got %d", n)
	}
	sess, _ := store.GetSession(ctx, sid)
	if sess == nil || sess.MessageCount != 1 {
		t.Fatalf("message_count should be 1, got %+v", sess)
	}
	// role 条件删除剩余 assistant
	n, err = store.DeleteMessages(ctx, DeleteMessagesQuery{SessionID: sid, Role: "assistant"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 assistant deleted, got %d", n)
	}
	left, _ := store.ListMessages(ctx, sid, PageQuery{})
	if len(left) != 0 {
		t.Fatalf("expected 0 messages left, got %d", len(left))
	}
}

func newSQLStore(t *testing.T) Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:conv_del?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS conversation_sessions (
		id TEXT PRIMARY KEY, user_id INTEGER NOT NULL DEFAULT 0, tenant_id TEXT DEFAULT '',
		title TEXT DEFAULT '', category TEXT DEFAULT 'general', pipeline_id TEXT DEFAULT '',
		proxy_mode TEXT DEFAULT '', message_count INTEGER DEFAULT 0,
		created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS conversation_messages (
		id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT DEFAULT '',
		request_id TEXT DEFAULT '', model TEXT DEFAULT '', backend TEXT DEFAULT '', pipeline_id TEXT DEFAULT '',
		input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0, latency_ms INTEGER DEFAULT 0,
		status_code INTEGER DEFAULT 200, created_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return NewSQLStore(db, dialectSQLite)
}
