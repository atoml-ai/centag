package server

import (
	"testing"
	"time"
)

func TestMatchCacheListFilters_SessionModelQuery(t *testing.T) {
	entry := map[string]interface{}{
		"key":       "k1",
		"request":   "hello world",
		"response":  "hi there",
		"timestamp": time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		"metadata": map[string]interface{}{
			"session_id": "sess-A",
			"model":      "qwen/qwen3",
			"request_id": "rid-1",
		},
	}
	if !matchCacheListFilters(entry, cacheListFilters{SessionID: "sess-A"}) {
		t.Fatal("session match")
	}
	if matchCacheListFilters(entry, cacheListFilters{SessionID: "sess-B"}) {
		t.Fatal("session mismatch")
	}
	if !matchCacheListFilters(entry, cacheListFilters{Model: "qwen"}) {
		t.Fatal("model substring")
	}
	if !matchCacheListFilters(entry, cacheListFilters{Query: "rid-1"}) {
		t.Fatal("q request_id")
	}
	if !matchCacheListFilters(entry, cacheListFilters{Query: "hello"}) {
		t.Fatal("q request text")
	}
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !matchCacheListFilters(entry, cacheListFilters{HasFrom: true, From: from, HasTo: true, To: to}) {
		t.Fatal("date range")
	}
	early := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if matchCacheListFilters(entry, cacheListFilters{HasTo: true, To: early}) {
		t.Fatal("outside to")
	}
}

func TestFlattenCacheEntryForAPI(t *testing.T) {
	entry := map[string]interface{}{
		"metadata": map[string]interface{}{
			"session_id": "s1",
			"model":      "m1",
			"cache_type": "exact",
		},
	}
	out := flattenCacheEntryForAPI(entry)
	if out["session_id"] != "s1" || out["model"] != "m1" {
		t.Fatalf("%v", out)
	}
}
