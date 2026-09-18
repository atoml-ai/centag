package configsync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// skillProvider is a mockProvider that additionally implements the optional
// FetchSkillRows provider extension, letting a test control the remote skill
// table independently (empty / populated / unsupported / error).
type skillProvider struct {
	mockProvider
	skillRows  []RemoteSkillRow
	skillErr   error
	skillFetch int
	mu         sync.Mutex
}

func (s *skillProvider) FetchSkillRows(ctx context.Context) ([]RemoteSkillRow, error) {
	s.mu.Lock()
	s.skillFetch++
	s.mu.Unlock()
	return s.skillRows, s.skillErr
}

func (s *skillProvider) skillFetches() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.skillFetch
}

func skillRow(name string) RemoteSkillRow {
	return RemoteSkillRow{Name: name, Enabled: true}
}

// Explicit empty remote table must be surfaced (SkillsFetched=true) so the
// consumer can clear local skills — §3.3 Step 2.
func TestSkillSync_ExplicitEmptyIsSurfaced(t *testing.T) {
	p := &skillProvider{mockProvider: mockProvider{rows: []Row{validRow()}}, skillRows: nil}
	var got *Snapshot
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) { got = snap },
	})
	if err := s.SyncNow(context.Background()); err != nil {
		t.Fatalf("SyncNow: %v", err)
	}
	if got == nil {
		t.Fatal("onUpdate not called")
	}
	if !got.SkillsFetched {
		t.Fatal("SkillsFetched=false; explicit empty table must be signalled")
	}
	if len(got.Skills) != 0 {
		t.Fatalf("expected empty skills, got %d", len(got.Skills))
	}
}

// A populated remote table is delivered with SkillsFetched=true.
func TestSkillSync_Populated(t *testing.T) {
	p := &skillProvider{
		mockProvider: mockProvider{rows: []Row{validRow()}},
		skillRows:    []RemoteSkillRow{skillRow("s1"), skillRow("s2")},
	}
	var got *Snapshot
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) { got = snap },
	})
	if err := s.SyncNow(context.Background()); err != nil {
		t.Fatalf("SyncNow: %v", err)
	}
	if got == nil || !got.SkillsFetched || len(got.Skills) != 2 {
		t.Fatalf("expected 2 fetched skills, got %+v", got)
	}
}

// A provider without a skill channel must leave SkillsFetched=false so the
// consumer keeps last-good.
func TestSkillSync_AbsentKeepsLastGood(t *testing.T) {
	p := &mockProvider{rows: []Row{validRow()}}
	var got *Snapshot
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) { got = snap },
	})
	if err := s.SyncNow(context.Background()); err != nil {
		t.Fatalf("SyncNow: %v", err)
	}
	if got == nil {
		t.Fatal("onUpdate not called")
	}
	if got.SkillsFetched {
		t.Fatal("SkillsFetched=true for provider without a skill channel")
	}
}

// A fetch error must keep last-good (no onUpdate) and be recorded as failure.
func TestSkillSync_FetchErrorKeepsLastGood(t *testing.T) {
	p := &skillProvider{
		mockProvider: mockProvider{rows: []Row{validRow()}},
		skillErr:     errors.New("feishu 500"),
	}
	var called bool
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) { called = true },
	})
	if err := s.SyncSkillsNow(context.Background()); err == nil {
		t.Fatal("expected error from SyncSkillsNow")
	}
	if called {
		t.Fatal("onUpdate must not run when the skill fetch fails")
	}
}

// SyncSkillsNow delivers a minimal, skill-only snapshot (no config/prices), so
// the periodic skill poll cannot clobber the last-good config snapshot.
func TestSkillSync_SkillsOnlySnapshot(t *testing.T) {
	p := &skillProvider{
		mockProvider: mockProvider{rows: []Row{validRow()}},
		skillRows:    []RemoteSkillRow{skillRow("s1")},
	}
	var got *Snapshot
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) { got = snap },
	})
	if err := s.SyncSkillsNow(context.Background()); err != nil {
		t.Fatalf("SyncSkillsNow: %v", err)
	}
	if got == nil {
		t.Fatal("onUpdate not called")
	}
	if len(got.Config) != 0 || len(got.Prices) != 0 {
		t.Fatalf("skill poll must not carry config/prices: %+v", got)
	}
	if !got.SkillsFetched || len(got.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %+v", got)
	}
}

// StartSkillPoll must run once immediately and then stop on Stop().
func TestSkillSync_StartSkillPoll(t *testing.T) {
	p := &skillProvider{
		mockProvider: mockProvider{rows: []Row{validRow()}},
		skillRows:    []RemoteSkillRow{skillRow("s1")},
	}
	s := NewScheduler(SchedulerConfig{
		Provider: p,
		OnUpdate: func(snap *Snapshot) {},
	})
	s.StartSkillPoll(context.Background(), 20*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for p.skillFetches() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if p.skillFetches() == 0 {
		t.Fatal("StartSkillPoll did not run the initial fetch")
	}
	s.Stop()
	after := p.skillFetches()
	time.Sleep(60 * time.Millisecond)
	if p.skillFetches() != after {
		t.Fatalf("skill poll continued after Stop(): %d -> %d", after, p.skillFetches())
	}
}
