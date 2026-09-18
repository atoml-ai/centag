package configsync

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"centag/core/pkg/logger"
)

type mitmEmptyProvider struct {
	mockProvider
}

func (m *mitmEmptyProvider) FetchMITMDomainsRows(ctx context.Context) ([]MITMDomainRow, error) {
	return []MITMDomainRow{}, nil
}

// 提供方明确返回空表时，调度器必须落盘该空表，入口才能走「删除全部」。
func TestScheduler_PersistsExplicitEmptyMITMTable(t *testing.T) {
	p := &mitmEmptyProvider{}
	p.rows = []Row{validRow()}
	s := NewScheduler(SchedulerConfig{Provider: p})
	if err := s.doSync(context.Background()); err != nil {
		t.Fatalf("doSync: %v", err)
	}
	rows, reason := ResolveMITMDomains(s.Snapshot())
	if reason != MITMSyncSnapshot || len(rows) != 0 {
		t.Fatalf("explicit empty mitm table must persist: reason=%s rows=%v", reason, rows)
	}
}

// 提供方不支持该表时保持「缺表」语义。
func TestScheduler_AbsentMITMTable(t *testing.T) {
	p := &mockProvider{rows: []Row{validRow()}}
	s := NewScheduler(SchedulerConfig{Provider: p})
	if err := s.doSync(context.Background()); err != nil {
		t.Fatalf("doSync: %v", err)
	}
	if _, reason := ResolveMITMDomains(s.Snapshot()); reason != MITMSyncAbsent {
		t.Fatalf("absent mitm table must stay absent, got reason=%s", reason)
	}
}

func TestMain(m *testing.M) {
	_ = logger.Init(logger.Config{Level: "error", Format: "console", Output: "stdout"})
	os.Exit(m.Run())
}

func newTestMITMStore(t *testing.T) *MITMDomainsStore {
	t.Helper()
	db := initTestSQLiteDB(t)
	migrationSQL, err := os.ReadFile("../database/migrations/051_mitm_domains.sqlite.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(migrationSQL)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return &MITMDomainsStore{db: db}
}

func seedMITM(t *testing.T, store *MITMDomainsStore, rows []MITMDomainRow) {
	t.Helper()
	if _, err := store.SyncFromSnapshotReason(rows, MITMSyncSnapshot); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func countAll(t *testing.T, store *MITMDomainsStore) int {
	t.Helper()
	rows, err := store.GetAll()
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	return len(rows)
}

// 显式空表 → 删除全部（P0-4 §3.2 Step 3）。
func TestMITMSync_ExplicitEmptyDeletesAll(t *testing.T) {
	store := newTestMITMStore(t)
	seedMITM(t, store, []MITMDomainRow{
		{Domain: "a.example.com", Enabled: true},
		{Domain: "b.example.com", Enabled: true},
	})
	if got := countAll(t, store); got != 2 {
		t.Fatalf("seed count=%d", got)
	}
	res, err := store.SyncFromSnapshotReason([]MITMDomainRow{}, MITMSyncSnapshot)
	if err != nil {
		t.Fatalf("explicit empty should not error: %v", err)
	}
	if res.Rejected || res.Applied != 0 || countAll(t, store) != 0 {
		t.Fatalf("explicit empty should wipe: res=%+v count=%d", res, countAll(t, store))
	}
}

// 缺表/拉取失败 → 保持 last-good，绝不覆盖（P0-4）。
func TestMITMSync_AbsentAndFetchErrorKeepLastGood(t *testing.T) {
	for _, reason := range []MITMSyncReason{MITMSyncAbsent, MITMSyncFetchError} {
		store := newTestMITMStore(t)
		seedMITM(t, store, []MITMDomainRow{{Domain: "keep.example.com", Enabled: true}})
		res, err := store.SyncFromSnapshotReason(nil, reason)
		if err != nil {
			t.Fatalf("reason=%s err=%v", reason, err)
		}
		if !res.Rejected || res.Reason != reason {
			t.Fatalf("reason=%s res=%+v", reason, res)
		}
		if countAll(t, store) != 1 {
			t.Fatalf("reason=%s must keep last-good, count=%d", reason, countAll(t, store))
		}
	}
}

// 非空快照触发删除熔断（>50% 且 >10）→ 拒绝并保留 last-good。
func TestMITMSync_DeletionBreaker(t *testing.T) {
	store := newTestMITMStore(t)
	var seed []MITMDomainRow
	for _, d := range []string{
		"a.com", "b.com", "c.com", "d.com", "e.com", "f.com", "g.com", "h.com",
		"i.com", "j.com", "k.com", "l.com", "m.com", "n.com", "o.com", "p.com",
		"q.com", "r.com", "s.com", "t.com",
	} {
		seed = append(seed, MITMDomainRow{Domain: d, Enabled: true})
	}
	seedMITM(t, store, seed)

	res, err := store.SyncFromSnapshotReason([]MITMDomainRow{{Domain: "a.com", Enabled: true}}, MITMSyncSnapshot)
	if err == nil || !res.Rejected {
		t.Fatalf("breaker should reject: err=%v res=%+v", err, res)
	}
	if countAll(t, store) != 20 {
		t.Fatalf("last-good must be preserved, count=%d", countAll(t, store))
	}
}

// 全无效行的非空表视为损坏，拒绝。
func TestMITMSync_InvalidRowsRejected(t *testing.T) {
	store := newTestMITMStore(t)
	seedMITM(t, store, []MITMDomainRow{{Domain: "keep.com", Enabled: true}})
	res, err := store.SyncFromSnapshotReason([]MITMDomainRow{{Domain: "nodot", Enabled: true}}, MITMSyncSnapshot)
	if err == nil || !res.Rejected || countAll(t, store) != 1 {
		t.Fatalf("invalid-only snapshot must be rejected: err=%v res=%+v count=%d", err, res, countAll(t, store))
	}
}

// scheduler 对显式空表也要能区分「空表」与「缺表」。
func TestResolveMITMDomains_Reason(t *testing.T) {
	rows, reason := ResolveMITMDomains(&Snapshot{})
	if reason != MITMSyncAbsent || rows != nil {
		t.Fatalf("absent: rows=%v reason=%s", rows, reason)
	}
	rows, reason = ResolveMITMDomains(&Snapshot{Tables: map[string]json.RawMessage{"mitm_domains": []byte(`[]`)}})
	if reason != MITMSyncSnapshot || len(rows) != 0 {
		t.Fatalf("explicit empty: rows=%v reason=%s", rows, reason)
	}
	rows, reason = ResolveMITMDomains(&Snapshot{Tables: map[string]json.RawMessage{
		"mitm_domains": []byte(`[{"domain":"a.com","enabled":true}]`),
	}})
	if reason != MITMSyncSnapshot || len(rows) != 1 || rows[0].Domain != "a.com" {
		t.Fatalf("present: rows=%v reason=%s", rows, reason)
	}
}

// 新装 seed 仅在空表时执行一次。
func TestMITMSync_EnsureSeeded(t *testing.T) {
	store := newTestMITMStore(t)
	n, err := store.EnsureSeeded()
	if err != nil || n == 0 {
		t.Fatalf("seed: n=%d err=%v", n, err)
	}
	if countAll(t, store) != n {
		t.Fatalf("count=%d want %d", countAll(t, store), n)
	}
	again, err := store.EnsureSeeded()
	if err != nil || again != 0 {
		t.Fatalf("second seed should be no-op: n=%d err=%v", again, err)
	}
}
