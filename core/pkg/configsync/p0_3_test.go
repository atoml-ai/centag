package configsync

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

const p03SnapshotBody = `{"schema":1,"generated_at":"2026-09-18T00:00:00Z",
 "config":[],
 "tables":{
   "system_config":[{"key":"billing.usd_to_cny","value":"7.3","value_type":"number","scope":"core","enabled":true,"description":"fx"}],
   "mitm_domains":[{"domain":"api.openai.com","category":"llm","enabled":true,"remark":"openai"}],
   "agent_apps":[{"type_id":"claude_code","display_name":"Claude Code","enabled":true,"sort":1}]
 }}`

// P0-3: snapshot 通道必须真正实现 system_config / mitm_domains / agent_apps 拉取。
func TestSnapshotProviderTables_P0_3(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(snapshotJSON(p03SnapshotBody))
	defer ts.Close()
	p := NewSnapshotProvider([]string{ts.URL})

	kv, err := p.FetchSystemConfigRows(ctx)
	if err != nil || len(kv) != 1 || kv[0].Key != "billing.usd_to_cny" || kv[0].Value != "7.3" {
		t.Fatalf("system_config rows=%v err=%v", kv, err)
	}
	domains, err := p.FetchMITMDomainsRows(ctx)
	if err != nil || len(domains) != 1 || domains[0].Domain != "api.openai.com" {
		t.Fatalf("mitm_domains rows=%v err=%v", domains, err)
	}
	apps, err := p.FetchAgentAppRows(ctx)
	if err != nil || len(apps) != 1 || apps[0].TypeID != "claude_code" {
		t.Fatalf("agent_apps rows=%v err=%v", apps, err)
	}
}

// 缺表语义：返回 (nil, nil)，调用方据此保留 last-good，而非当成清空。
func TestSnapshotProviderTables_MissingIsNil_P0_3(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(snapshotJSON(`{"schema":1,"config":[]}`))
	defer ts.Close()
	p := NewSnapshotProvider([]string{ts.URL})

	if rows, err := p.FetchSystemConfigRows(ctx); err != nil || rows != nil {
		t.Fatalf("expected nil,nil got rows=%v err=%v", rows, err)
	}
	if rows, err := p.FetchMITMDomainsRows(ctx); err != nil || rows != nil {
		t.Fatalf("expected nil,nil got rows=%v err=%v", rows, err)
	}
}

// P0-3: kvTableAdapter.apply 必须真正把行写入已接线的 applier。
func TestKVTableAdapter_ApplyWrites_P0_3(t *testing.T) {
	reg := NewRegistry()
	if err := reg.RegisterSystemConfig(); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get(systemConfigTable); !ok {
		t.Fatalf("system_config table not registered under canonical name")
	}

	var got []KVRow
	SetKVApplier(func(_ context.Context, rows []KVRow) (ApplyResult, error) {
		got = append(got, rows...)
		return ApplyResult{Applied: len(rows)}, nil
	})
	t.Cleanup(func() { SetKVApplier(nil) })

	snap := &Snapshot{
		Schema: snapshotSchema,
		Tables: map[string]json.RawMessage{
			systemConfigTable: json.RawMessage(p03SnapshotBodyTables),
		},
	}
	orch := NewOrchestrator(reg)
	if errs := orch.Apply(context.Background(), snap); len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}
	if len(got) != 1 || got[0].Key != "billing.usd_to_cny" || got[0].Value != "7.3" || !got[0].Enabled {
		t.Fatalf("applier got %+v", got)
	}
}

// 序列化格式必须与 SystemConfigStore.SyncFromSnapshot / Snapshot.Tables 对齐。
func TestKVTableAdapter_SerializeRoundTrip_P0_3(t *testing.T) {
	a := &kvTableAdapter{spec: KeyValueSpec{Key: systemConfigTable, Type: KVJSON, Scope: KVScopeCore}}
	raw, err := a.serialize([]any{KVRow{Key: "k", Value: "v", Enabled: true}})
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	var rows []KVRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("serialize not []KVRow compatible: %v", err)
	}
	if len(rows) != 1 || rows[0].Key != "k" {
		t.Fatalf("rows=%+v", rows)
	}
	items, err := a.deserialize(raw)
	if err != nil || len(items) != 1 {
		t.Fatalf("deserialize items=%v err=%v", items, err)
	}
	if _, ok := items[0].(KVRow); !ok {
		t.Fatalf("deserialize type = %T, want KVRow", items[0])
	}
}

const p03SnapshotBodyTables = `[{"key":"billing.usd_to_cny","value":"7.3","value_type":"number","scope":"core","enabled":true}]`

// P0-3 §3.2 Step 4: 一致性校验器。
func TestConsistencyChecker(t *testing.T) {
	checker := NewConsistencyChecker()

	valid := &Snapshot{
		Schema: snapshotSchema,
		Tables: map[string]json.RawMessage{
			systemConfigTable: []byte(p03SnapshotBodyTables),
			"mitm_domains":    []byte(`[{"domain":"api.openai.com","enabled":true}]`),
		},
	}
	if res := checker.VerifySnapshot(valid); !res.OK {
		t.Fatalf("valid snapshot rejected: %v", res.Err())
	} else if res.Counts[systemConfigTable] != 1 || res.Counts["mitm_domains"] != 1 {
		t.Fatalf("counts=%v", res.Counts)
	}

	badSchema := &Snapshot{Schema: 999}
	if res := checker.VerifySnapshot(badSchema); res.OK {
		t.Fatalf("schema mismatch must fail")
	}

	badRate := &Snapshot{
		Schema: snapshotSchema,
		Tables: map[string]json.RawMessage{
			systemConfigTable: []byte(`[{"key":"billing.usd_to_cny","value":"-1"}]`),
		},
	}
	if res := checker.VerifySnapshot(badRate); res.OK {
		t.Fatalf("invalid exchange rate must fail")
	}

	dup := &Snapshot{
		Schema: snapshotSchema,
		Tables: map[string]json.RawMessage{
			systemConfigTable: []byte(`[{"key":"a","value":"1"},{"key":"a","value":"2"}]`),
		},
	}
	if res := checker.VerifySnapshot(dup); res.OK {
		t.Fatalf("duplicate keys must fail")
	}

	if res := checker.CheckLastGood(t.TempDir()); !res.OK {
		t.Fatalf("missing last-good snapshot should be healthy: %v", res.Err())
	}
}
