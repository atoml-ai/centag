package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSidecarBinaryExplicit(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "centag-personal")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveSidecarBinary(Config{Edition: EditionPersonal, BinPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(bin)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	if err := ensureDirs(dir); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"storage", "logs", "data"} {
		if st, err := os.Stat(filepath.Join(dir, sub)); err != nil || !st.IsDir() {
			t.Fatalf("missing %s: %v", sub, err)
		}
	}
}
