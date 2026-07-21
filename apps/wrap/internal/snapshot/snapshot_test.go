package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)

	s := &Snapshot{
		ClientMode: "remote",
		Proxy:      ProxyState{Mode: "pac", PACURL: "http://old/pac"},
		CA:         CAState{FingerprintSHA256: "abc", InstalledByUs: true},
		Centag:     CentagRef{APIBase: "http://team:20060", MITMProxy: "team:8081"},
		CreatedAt:  time.Unix(0, 0).UTC(),
	}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	if !Exists() {
		t.Fatal("Exists should be true")
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientMode != "remote" || got.Proxy.PACURL != "http://old/pac" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	path, _ := Path()
	if filepath.Dir(path) == "" {
		t.Fatal("empty path")
	}
	if err := Remove(); err != nil {
		t.Fatal(err)
	}
	if Exists() {
		t.Fatal("Exists should be false after Remove")
	}
	if err := Remove(); err != nil {
		t.Fatal("Remove idempotent:", err)
	}
}

func TestSave_Nil(t *testing.T) {
	if err := Save(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Missing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
	_ = os.MkdirAll(filepath.Join(dir, ".centag"), 0o755)
}
