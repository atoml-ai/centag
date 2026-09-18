package configsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestSyncStatusGranularity covers P2-2: a round that fetches successfully but
// fails to persist must NOT be reported as a clean sync, and the per-stage
// flags must reflect what actually happened.
func TestSyncStatusGranularity(t *testing.T) {
	t.Run("persist_failure_is_not_clean_success", func(t *testing.T) {
		dir := t.TempDir()
		// Occupy the stateDir path with a regular file so MkdirAll fails.
		blocked := filepath.Join(dir, "state")
		if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		var applied int
		s := NewScheduler(SchedulerConfig{
			Provider: &mockProvider{rows: []Row{validRow()}},
			StateDir: blocked,
			OnUpdate: func(*Snapshot) { applied++ },
		})
		if err := s.SyncNow(context.Background()); err != nil {
			t.Fatalf("SyncNow returned error (fetch should succeed): %v", err)
		}

		st := s.Status()
		if st.LastFetchOK != true {
			t.Fatalf("LastFetchOK = false, want true")
		}
		if st.LastPersistOK != false {
			t.Fatalf("LastPersistOK = true, want false (write must fail)")
		}
		if st.LastApplyOK != true {
			t.Fatalf("LastApplyOK = false, want true (onUpdate ran)")
		}
		if st.LastSyncOK != false {
			t.Fatalf("LastSyncOK = true, want false when persist failed")
		}
		if st.LastError == "" {
			t.Fatalf("LastError empty, want snapshot write error")
		}
		if applied != 1 {
			t.Fatalf("onUpdate called %d times, want 1 (fail-open in-memory)", applied)
		}
	})

	t.Run("successful_round_sets_all_stages", func(t *testing.T) {
		var applied int
		s := NewScheduler(SchedulerConfig{
			Provider: &mockProvider{rows: []Row{validRow()}},
			StateDir: t.TempDir(),
			OnUpdate: func(*Snapshot) { applied++ },
		})
		if err := s.SyncNow(context.Background()); err != nil {
			t.Fatalf("SyncNow: %v", err)
		}
		st := s.Status()
		if !st.LastFetchOK || !st.LastPersistOK || !st.LastApplyOK || !st.LastSyncOK {
			t.Fatalf("expected all stage flags true, got %+v", st)
		}
		if st.LastError != "" {
			t.Fatalf("LastError = %q, want empty", st.LastError)
		}
		if applied != 1 {
			t.Fatalf("onUpdate called %d times, want 1", applied)
		}
	})

	t.Run("no_state_dir_reports_persist_ok", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{
			Provider: &mockProvider{rows: []Row{validRow()}},
		})
		if err := s.SyncNow(context.Background()); err != nil {
			t.Fatalf("SyncNow: %v", err)
		}
		st := s.Status()
		if !st.LastPersistOK {
			t.Fatalf("LastPersistOK = false without StateDir, want true")
		}
		if !st.LastSyncOK {
			t.Fatalf("LastSyncOK = false without StateDir, want true")
		}
	})

	t.Run("fetch_failure_clears_fetch_ok", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{
			Provider: &mockProvider{rows: []Row{validRow()}},
			StateDir: t.TempDir(),
		})
		if err := s.SyncNow(context.Background()); err != nil {
			t.Fatalf("SyncNow: %v", err)
		}
		s.provider = &mockProvider{cfgErr: context.DeadlineExceeded}
		if err := s.SyncNow(context.Background()); err == nil {
			t.Fatal("SyncNow succeeded, want fetch failure")
		}
		st := s.Status()
		if st.LastFetchOK {
			t.Fatalf("LastFetchOK = true after fetch failure, want false")
		}
		if st.LastSyncOK {
			t.Fatalf("LastSyncOK = true after fetch failure, want false")
		}
	})
}
