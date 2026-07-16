package server

import (
	"path/filepath"
	"testing"
)

func TestNormalizeSyncMode_DefaultToFull(t *testing.T) {
	if got := normalizeSyncMode(""); got != "full" {
		t.Fatalf("expected full, got %s", got)
	}
	if got := normalizeSyncMode("unknown"); got != "full" {
		t.Fatalf("expected full for unknown mode, got %s", got)
	}
}

func TestNormalizeSyncMode_Incremental(t *testing.T) {
	if got := normalizeSyncMode("incremental"); got != "incremental" {
		t.Fatalf("expected incremental, got %s", got)
	}
	if got := normalizeSyncMode(" InCreMental "); got != "incremental" {
		t.Fatalf("expected incremental with spaces/case, got %s", got)
	}
}

func TestCalcSyncAction_NewFile(t *testing.T) {
	got := calcSyncAction("incremental", false, nil, []byte("new"))
	if got != "new" {
		t.Fatalf("expected new, got %s", got)
	}
}

func TestCalcSyncAction_IncrementalSkipped(t *testing.T) {
	got := calcSyncAction("incremental", true, []byte("same"), []byte("same"))
	if got != "skipped" {
		t.Fatalf("expected skipped, got %s", got)
	}
}

func TestCalcSyncAction_FullUpdatesEvenSameContent(t *testing.T) {
	got := calcSyncAction("full", true, []byte("same"), []byte("same"))
	if got != "updated" {
		t.Fatalf("expected updated in full mode, got %s", got)
	}
}

func TestIsPathWithinBase_AllowsNestedFile(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "memory", "a.md")
	if !isPathWithinBase(base, target) {
		t.Fatalf("expected nested path to be allowed")
	}
}

func TestIsPathWithinBase_RejectsTraversalOutside(t *testing.T) {
	base := t.TempDir()
	outsideRoot := filepath.Dir(base)
	target := filepath.Join(outsideRoot, "outside.md")
	if isPathWithinBase(base, target) {
		t.Fatalf("expected outside path to be rejected")
	}
}

func TestIsSafeMemoryRelPath(t *testing.T) {
	cases := []struct {
		path string
		ok   bool
	}{
		{path: "MEMORY.md", ok: true},
		{path: "memory/2026-01-01.md", ok: true},
		{path: "../evil.md", ok: false},
		{path: "/abs.md", ok: false},
		{path: "", ok: false},
	}
	for _, tc := range cases {
		if got := isSafeMemoryRelPath(tc.path); got != tc.ok {
			t.Fatalf("path %q expected %v got %v", tc.path, tc.ok, got)
		}
	}
}

func TestGetIndexQueueMetrics_Default(t *testing.T) {
	h := &MemoryHandler{}
	m := h.getIndexQueueMetrics()
	if m.Enabled {
		t.Fatalf("expected disabled queue metrics by default")
	}
	if m.Length != 0 || m.Processed != 0 || m.Failed != 0 || m.Dropped != 0 || m.LastError != "" {
		t.Fatalf("unexpected default metrics: %+v", m)
	}
}

func TestGetIndexQueueMetrics_WithQueueState(t *testing.T) {
	h := &MemoryHandler{
		indexQueue: make(chan memoryIndexTask, 4),
	}
	h.indexQueue <- memoryIndexTask{userID: "1", agentID: "main", path: "MEMORY.md"}
	h.indexQueue <- memoryIndexTask{userID: "1", agentID: "main", path: "memory/a.md"}
	h.indexProcessed.Store(3)
	h.indexFailed.Store(1)
	h.indexDropped.Store(2)
	h.setIndexLastError("boom")

	m := h.getIndexQueueMetrics()
	if !m.Enabled {
		t.Fatalf("expected enabled queue metrics")
	}
	if m.Length != 2 {
		t.Fatalf("expected queue length 2, got %d", m.Length)
	}
	if m.Processed != 3 || m.Failed != 1 || m.Dropped != 2 || m.LastError != "boom" {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}
