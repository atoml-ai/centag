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
	if out["cache_type"] != "exact" {
		t.Fatalf("cache_type=%v", out["cache_type"])
	}
}

func TestParseRFC3339Loose(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{name: "rfc3339", in: "2026-08-01T12:00:00Z", ok: true},
		{name: "date only", in: "2026-08-01", ok: true},
		{name: "datetime no zone", in: "2026-08-01T15:04:05", ok: true},
		{name: "empty", in: "", ok: false},
		{name: "garbage", in: "not-a-date", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseRFC3339Loose(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
		})
	}
}

func TestEntryTimestamp_StringAndCreatedAt(t *testing.T) {
	ts, ok := entryTimestamp(map[string]interface{}{"timestamp": "2026-08-01T12:00:00Z"})
	if !ok || ts.Year() != 2026 {
		t.Fatalf("string timestamp: ok=%v ts=%v", ok, ts)
	}
	ts, ok = entryTimestamp(map[string]interface{}{
		"metadata": map[string]interface{}{"created_at": "2026-07-15"},
	})
	if !ok || ts.Month() != time.July {
		t.Fatalf("created_at: ok=%v ts=%v", ok, ts)
	}
	if _, ok := entryTimestamp(nil); ok {
		t.Fatal("nil entry")
	}
}

func TestMatchCacheListFilters_ModelEmptyRejects(t *testing.T) {
	entry := map[string]interface{}{"key": "k", "request": "x"}
	if matchCacheListFilters(entry, cacheListFilters{Model: "qwen"}) {
		t.Fatal("missing model must not match model filter")
	}
	if !matchCacheListFilters(entry, cacheListFilters{}) {
		t.Fatal("empty filters match all")
	}
}
