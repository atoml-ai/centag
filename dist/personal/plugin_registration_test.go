package main

import (
	"sort"
	"testing"

	"centag/core/pkg/database"
	"centag/core/pkg/plugin"
	"centag/core/pkg/storage"
)

func TestPersonalPluginRegistration(t *testing.T) {
	assertExact(t, "backends", plugin.ListBackends(), []string{"anthropic", "ollama", "openai"})
	assertExact(t, "protocols", plugin.ListProtocols(), []string{"anthropic", "openai"})
	assertExact(t, "databases", database.ListRegisteredPlugins(), []string{"postgresql", "sqlite"})
	assertExact(t, "storages", storageTypeStrings(storage.ListRegisteredTypes()), []string{
		"chroma", "elasticsearch", "file", "postgresql", "redis",
	})
	// Business plugins land incrementally; require S3 rag_retrieval (v0.3.3) at minimum.
	assertContains(t, "business", plugin.ListBusinessPlugins(), "rag_retrieval")
}

func assertContains(t *testing.T, kind string, got []string, want string) {
	t.Helper()
	for _, g := range got {
		if g == want {
			return
		}
	}
	t.Fatalf("%s: missing %q in %v", kind, want, got)
}

func storageTypeStrings(types []storage.StorageType) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

func assertExact(t *testing.T, kind string, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("%s: got %v want %v", kind, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v want %v", kind, got, want)
		}
	}
}
