package conversation

import (
	"context"
	"testing"
)

func TestFileStore_AppendInvalidMessage(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.AppendMessage(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil message")
	}
	if err := store.AppendMessage(context.Background(), &Message{Role: "user", Content: "x"}); err == nil {
		t.Fatal("expected error for empty session id")
	}
}

func TestFileStore_EnsureSessionNil(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if _, err := store.EnsureSession(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestFileStore_ListEmptyRoot(t *testing.T) {
	store := NewFileStore(t.TempDir() + "/missing-subdir-will-be-created-by-factory-not-here")
	// NewFileStore does not create root; ListSessions should tolerate missing dir
	list, err := store.ListSessions(context.Background(), ListSessionsQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
}

func TestDefaultStore_SetGet(t *testing.T) {
	s := NewFileStore(t.TempDir())
	SetDefault(s)
	defer SetDefault(nil)
	if Default() != s {
		t.Fatal("Default mismatch")
	}
}
