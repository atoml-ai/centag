package entrypoint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCleanupCommand(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"centag"}, false},
		{[]string{"centag", "run"}, false},
		{[]string{"centag", "cleanup"}, true},
		{[]string{"centag", "--cleanup"}, true},
		{[]string{"centag", "-cleanup"}, true},
		{[]string{"centag", "Cleanup"}, false},
	}
	for _, tc := range cases {
		if got := IsCleanupCommand(tc.args); got != tc.want {
			t.Errorf("IsCleanupCommand(%v)=%v want %v", tc.args, got, tc.want)
		}
	}
}

func TestHandleCleanupCommand_NotCleanup(t *testing.T) {
	if HandleCleanupCommand([]string{"centag", "version"}) {
		t.Fatal("expected false for non-cleanup args")
	}
}

func TestRunCleanupCommand_SQLiteExit0(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	_ = os.Unsetenv("SQLITE_PATH")
	// default / missing conf → sqlite defaults → remove db file (idempotent), exit 0
	if code := runCleanupCommand(); code != 0 {
		t.Fatalf("expected exit 0 for sqlite cleanup, got %d", code)
	}
}

func TestRunCleanupCommand_InvalidConfigExit1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "centag.conf"), []byte(`{bad`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if code := runCleanupCommand(); code != 1 {
		t.Fatalf("expected exit 1 for invalid conf, got %d", code)
	}
}

func TestRunCleanupCommand_PGUnreachableExit1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	conf := `{"db_driver":"postgresql","pg_host":"127.0.0.1","pg_port":"1","pg_user":"postgres","pg_db":"centag","clean_data_on_uninstall":true}`
	if err := os.WriteFile(filepath.Join(dir, "centag.conf"), []byte(conf), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if code := runCleanupCommand(); code != 1 {
		t.Fatalf("expected exit 1 for unreachable PG, got %d", code)
	}
}
