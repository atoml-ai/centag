package configsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"centag/core/pkg/database"
)

// g55Provider implements the optional fetch extensions so the scheduler
// serializes all three C-tier tables (system_config / mitm_domains /
// agent_apps) into Snapshot.Tables in a single round.
type g55Provider struct {
	mockProvider
	systemConfig []KVRow
	mitmDomains  []MITMDomainRow
	agentApps    []AgentAppRow
}

func (p *g55Provider) FetchSystemConfigRows(ctx context.Context) ([]KVRow, error) {
	return p.systemConfig, nil
}

func (p *g55Provider) FetchMITMDomainsRows(ctx context.Context) ([]MITMDomainRow, error) {
	return p.mitmDomains, nil
}

func (p *g55Provider) FetchAgentAppRows(ctx context.Context) ([]AgentAppRow, error) {
	return p.agentApps, nil
}

// applyMigrationFile 读取并按方言执行单个迁移文件。
func applyMigrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../database/migrations", name))
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("apply migration %s: %v", name, err)
	}
}

// TestG55ConfigsyncFullChain 覆盖 G5-5：对 MITM / system_config / agent_apps
// 各至少一条端到端集成路径 —— provider → scheduler → Snapshot.Tables →
// 真实 store（DB）→ 可被消费读取。
func TestG55ConfigsyncFullChain(t *testing.T) {
	ctx := context.Background()

	// provider 一轮下发三张表。
	p := &g55Provider{
		systemConfig: []KVRow{
			{Key: "billing.usd_to_cny", Value: "7.35", ValueType: "number", Scope: "core", Enabled: true, Description: "fx"},
			{Key: "feature.mcp_enabled", Value: "true", ValueType: "bool", Scope: "core", Enabled: true},
		},
		mitmDomains: []MITMDomainRow{
			{Domain: "api.openai.com", Category: "llm", Enabled: true, Remark: "openai"},
			{Domain: "api.anthropic.com", Category: "llm", Enabled: true},
		},
		agentApps: []AgentAppRow{
			{TypeID: "claude_code", DisplayName: "Claude Code", Enabled: true, Sort: 1,
				InstallURL: "https://github.com/anthropics/claude-code"},
		},
	}

	stateDir := t.TempDir()
	s := NewScheduler(SchedulerConfig{Provider: p, StateDir: stateDir})
	if err := s.SyncNow(ctx); err != nil {
		t.Fatalf("SyncNow: %v", err)
	}
	snap := s.Snapshot()
	if snap == nil {
		t.Fatal("snapshot nil after sync")
	}

	db := initTestSQLiteDB(t)
	// 050 是升级迁移：它从旧表 config_key/config_value 迁数据，因此先建旧表，
	// 复现真实升级路径。
	if _, err := db.Exec(`CREATE TABLE system_config (
		config_key TEXT, config_value TEXT, description TEXT, updated_at DATETIME)`); err != nil {
		t.Fatalf("create legacy system_config: %v", err)
	}
	applyMigrationFile(t, db, "050_system_config.sqlite.sql")
	applyMigrationFile(t, db, "051_mitm_domains.sqlite.sql")
	applyMigrationFile(t, db, "052_agent_apps.sqlite.sql")

	// --- 路径 1：system_config（KV）→ SystemConfigStore ---
	t.Run("system_config_store", func(t *testing.T) {
		scStore := &SystemConfigStore{db: db, dialect: &database.SQLiteDialect{}}

		// 走生产接线：orchestrator.Apply → SetKVApplier → UpsertBatch。
		reg := NewRegistry()
		if err := reg.RegisterSystemConfig(); err != nil {
			t.Fatalf("register: %v", err)
		}
		SetKVApplier(func(_ context.Context, rows []KVRow) (ApplyResult, error) {
			if err := scStore.UpsertBatch(rows); err != nil {
				return ApplyResult{Failed: len(rows)}, err
			}
			return ApplyResult{Applied: len(rows)}, nil
		})
		t.Cleanup(func() { SetKVApplier(nil) })

		orch := NewOrchestrator(reg)
		if errs := orch.Apply(ctx, snap); len(errs) != 0 {
			t.Fatalf("orchestrator apply: %v", errs)
		}
		if got := scStore.GetString("billing.usd_to_cny", ""); got != "7.35" {
			t.Fatalf("system_config not persisted: got %q", got)
		}
		if f := scStore.GetFloat("billing.usd_to_cny", 0); f != 7.35 {
			t.Fatalf("float parse: %v", f)
		}
		if !scStore.GetBool("feature.mcp_enabled") {
			t.Fatal("bool KV not persisted")
		}
	})

	// --- 路径 2：mitm_domains → MITMDomainsStore ---
	t.Run("mitm_domains_store", func(t *testing.T) {
		rows, reason := ResolveMITMDomains(snap)
		if reason != MITMSyncSnapshot || len(rows) != 2 {
			t.Fatalf("resolve: reason=%s rows=%d", reason, len(rows))
		}
		store := &MITMDomainsStore{db: db}
		if _, err := store.SyncFromSnapshotReason(rows, reason); err != nil {
			t.Fatalf("sync: %v", err)
		}
		enabled, err := store.GetEnabledDomains()
		if err != nil {
			t.Fatalf("GetEnabledDomains: %v", err)
		}
		if len(enabled) != 2 {
			t.Fatalf("enabled domains=%v want 2", enabled)
		}
	})

	// --- 路径 3：agent_apps → AgentAppsStore + Overlay ---
	t.Run("agent_apps_store", func(t *testing.T) {
		data, ok := snap.Tables["agent_apps"]
		if !ok || len(data) == 0 {
			t.Fatal("snapshot missing agent_apps table")
		}
		var rows []AgentAppRow
		if err := json.Unmarshal(data, &rows); err != nil {
			t.Fatalf("unmarshal agent_apps: %v", err)
		}
		store := &AgentAppsStore{db: db, dialect: &database.SQLiteDialect{}}
		if err := store.SyncFromSnapshot(rows); err != nil {
			t.Fatalf("sync: %v", err)
		}
		persisted, err := store.GetAll()
		if err != nil || len(persisted) != 1 || persisted[0].TypeID != "claude_code" {
			t.Fatalf("persisted=%v err=%v", persisted, err)
		}
		// Overlay 消费同一张表。
		ov := NewAgentAppsOverlay()
		ov.LoadFromSnapshot(data)
		if !ov.IsEnabled("claude_code") || ov.GetSort("claude_code") != 1 {
			t.Fatalf("overlay not populated: %+v", ov.Get("claude_code"))
		}
	})

	// 跨链路一致性：落盘的 snapshot 必须与内存语义一致（重启可恢复）。
	// Tables 值经 MarshalIndent 会改变空白，须按 JSON 语义比较。
	t.Run("state_dir_roundtrip", func(t *testing.T) {
		loaded, err := ReadSnapshot(stateDir)
		if err != nil {
			t.Fatalf("ReadSnapshot: %v", err)
		}
		if loaded == nil {
			t.Fatal("snapshot file missing after sync")
		}
		for table, want := range snap.Tables {
			var wantV, gotV any
			if err := json.Unmarshal(want, &wantV); err != nil {
				t.Fatalf("unmarshal want %s: %v", table, err)
			}
			gotRaw, ok := loaded.Tables[table]
			if !ok {
				t.Fatalf("table %s missing after roundtrip", table)
			}
			if err := json.Unmarshal(gotRaw, &gotV); err != nil {
				t.Fatalf("unmarshal got %s: %v", table, err)
			}
			if !reflect.DeepEqual(wantV, gotV) {
				t.Fatalf("table %s diverges: want=%s got=%s", table, want, gotRaw)
			}
		}
	})
}
