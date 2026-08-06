package cache

import "testing"

func TestEnrichAndAttachMetadata(t *testing.T) {
	m := EnrichCacheWriteMetadata(nil, "exact")
	if m["cache_type"] != "exact" {
		t.Fatalf("cache_type=%v", m["cache_type"])
	}
	if m["created_at"] == nil || m["created_at"] == "" {
		t.Fatal("created_at missing")
	}
	m = AttachRequestContextMetadata(m, "sess-1", "req-1", "backend-a")
	if m["session_id"] != "sess-1" || m["request_id"] != "req-1" || m["backend_name"] != "backend-a" {
		t.Fatalf("%v", m)
	}
	// do not overwrite
	m2 := AttachRequestContextMetadata(m, "sess-2", "req-2", "backend-b")
	if m2["session_id"] != "sess-1" {
		t.Fatal("should keep existing session_id")
	}
}
