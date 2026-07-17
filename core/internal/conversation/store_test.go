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
