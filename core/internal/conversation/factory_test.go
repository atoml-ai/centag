package conversation

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"centag/core/internal/edition"
)

func TestNewStore_PersonalRequiresDB(t *testing.T) {
	_, err := NewStore(Options{Edition: edition.Personal})
	if err == nil {
		t.Fatal("expected error when personal store has no db")
	}
}

func TestNewStore_PersonalUsesSQLiteDialect(t *testing.T) {
	// personal always selects SQLStore with sqlite dialect (gateway profile).
	store, err := NewStore(Options{Edition: edition.Personal, DB: &sql.DB{}, Driver: "postgresql"})
	if err != nil {
		t.Fatal(err)
	}
	sqlStore, ok := store.(*SQLStore)
	if !ok {
		t.Fatalf("want *SQLStore, got %T", store)
	}
	if sqlStore.dialect != dialectSQLite {
		t.Fatalf("personal store dialect=%v want sqlite (ignore driver hint)", sqlStore.dialect)
	}
}

func TestNewStore_TeamRequiresDB(t *testing.T) {
	_, err := NewStore(Options{Edition: edition.Team})
	if err == nil {
		t.Fatal("expected error when team store has no db")
	}
}

func TestPrepareEphemeralFileRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime", "conversations")
	if err := os.MkdirAll(filepath.Join(root, "old-session"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := PrepareEphemeralFileRoot(root); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("want empty root after prepare, got %d entries", len(entries))
	}
}

func TestSQLStore_RebindPostgres(t *testing.T) {
	s := NewSQLStore(nil, dialectPostgres)
	got := s.rebind("SELECT id FROM t WHERE a = ? AND b = ?")
	if got != "SELECT id FROM t WHERE a = $1 AND b = $2" {
		t.Fatalf("rebind=%q", got)
	}
	s2 := NewSQLStore(nil, dialectSQLite)
	if s2.rebind("SELECT ?") != "SELECT ?" {
		t.Fatal("sqlite should keep ?")
	}
}

func TestIsPostgresDriver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"postgresql", true},
		{"postgres", true},
		{"pgx", true},
		{"sqlite", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isPostgresDriver(tc.in); got != tc.want {
			t.Fatalf("isPostgresDriver(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}
