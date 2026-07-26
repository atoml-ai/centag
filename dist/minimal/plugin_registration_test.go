package main

import (
	"sort"
	"testing"

	"centag/core/pkg/database"
	"centag/core/pkg/plugin"
	"centag/core/pkg/storage"
)

func TestMinimalPluginRegistration(t *testing.T) {
	assertExact(t, "backends", plugin.ListBackends(), []string{"anthropic", "ollama", "openai"})
	assertExact(t, "protocols", plugin.ListProtocols(), []string{"anthropic", "openai", "responses"})
	assertExact(t, "databases", database.ListRegisteredPlugins(), nil)
	assertExact(t, "storages", storageTypeStrings(storage.ListRegisteredTypes()), nil)
	assertExact(t, "business", plugin.ListBusinessPlugins(), []string{"router"})
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
