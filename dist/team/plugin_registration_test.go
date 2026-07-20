package main

import (
	"sort"
	"testing"

	"centag/core/pkg/database"
	"centag/core/pkg/plugin"
	"centag/core/pkg/storage"
)

// team 与 personal 二进制插件集合对齐。
func TestTeamPluginRegistration(t *testing.T) {
	assertExact(t, "backends", plugin.ListBackends(), []string{"anthropic", "ollama", "openai"})
	assertExact(t, "protocols", plugin.ListProtocols(), []string{"anthropic", "openai"})
	assertExact(t, "databases", database.ListRegisteredPlugins(), []string{"postgresql", "sqlite"})
	assertExact(t, "storages", storageTypeStrings(storage.ListRegisteredTypes()), []string{
		"chroma", "elasticsearch", "file", "postgresql", "redis",
	})
	assertExact(t, "business", plugin.ListBusinessPlugins(), []string{
		"answer_synthesizer",
		"geo_router",
		"mem0",
		"optimizer",
		"pi_agent",
		"pii_redactor",
		"question_splitter",
		"rag_retrieval",
		"reviewer",
		"router",
		"summarizer",
		"tasktype_detector",
		"translator",
	})
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
