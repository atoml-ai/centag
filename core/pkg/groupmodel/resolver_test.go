package groupmodel

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const resolverSchema = `
CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	tenant_id TEXT,
	group_id TEXT,
	policy_mode TEXT NOT NULL DEFAULT 'group'
);
CREATE TABLE user_plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT 'USD',
	available_backend_ids TEXT NOT NULL DEFAULT '[]',
	available_models TEXT NOT NULL DEFAULT '[]',
	available_pipeline_ids TEXT NOT NULL DEFAULT '[]',
	price_type TEXT NOT NULL DEFAULT 'cost',
	budget_amount REAL,
	budget_period TEXT NOT NULL DEFAULT 'monthly',
	budget_start_at TEXT,
	budget_end_at TEXT,
	token_quota_input INTEGER,
	token_quota_output INTEGER,
	token_quota_total INTEGER,
	token_quota_period TEXT NOT NULL DEFAULT 'monthly',
	token_quota_start_at TEXT,
	token_quota_end_at TEXT,
	rate_limit_rpm INTEGER NOT NULL DEFAULT 0,
	rate_limit_tpm INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE user_pricing_overrides (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	backend_id TEXT NOT NULL,
	model TEXT NOT NULL,
	price_type TEXT NOT NULL DEFAULT 'cost',
	input_price_per_m REAL NOT NULL DEFAULT 0,
	output_price_per_m REAL NOT NULL DEFAULT 0,
	currency TEXT NOT NULL DEFAULT 'USD',
	effective_at TEXT,
	expires_at TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE group_plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT 'USD',
	available_backend_ids TEXT NOT NULL DEFAULT '[]',
	available_models TEXT NOT NULL DEFAULT '[]',
	available_pipeline_ids TEXT NOT NULL DEFAULT '[]',
	price_type TEXT NOT NULL DEFAULT 'cost',
	budget_amount REAL,
	budget_period TEXT NOT NULL DEFAULT 'monthly',
	budget_start_at TEXT,
	budget_end_at TEXT,
	token_quota_input INTEGER,
	token_quota_output INTEGER,
	token_quota_total INTEGER,
	token_quota_period TEXT NOT NULL DEFAULT 'monthly',
	token_quota_start_at TEXT,
	token_quota_end_at TEXT,
	rate_limit_rpm INTEGER NOT NULL DEFAULT 0,
	rate_limit_tpm INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (group_id)
);
CREATE TABLE group_pricing_overrides (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id TEXT NOT NULL,
	backend_id TEXT NOT NULL,
	model TEXT NOT NULL,
	price_type TEXT NOT NULL DEFAULT 'cost',
	input_price_per_m REAL NOT NULL DEFAULT 0,
	output_price_per_m REAL NOT NULL DEFAULT 0,
	currency TEXT NOT NULL DEFAULT 'USD',
	effective_at TEXT,
	expires_at TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE group_quotas (
	group_id TEXT PRIMARY KEY,
	daily_limit INTEGER DEFAULT 0,
	monthly_limit INTEGER DEFAULT 0,
	used_today INTEGER DEFAULT 0,
	used_this_month INTEGER DEFAULT 0,
	daily_request_limit INTEGER DEFAULT 0,
	monthly_request_limit INTEGER DEFAULT 0,
	max_backends INTEGER DEFAULT 0,
	max_api_keys INTEGER DEFAULT 0
);
`

func newResolverDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(resolverSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func TestResolver_GroupMode(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (1, 'alice', 'g_1', 'group')`)
	mustExec(t, db, `INSERT INTO group_plans (group_id, available_backend_ids, available_models, available_pipeline_ids, rate_limit_rpm) VALUES ('g_1', 'b1,b2', 'm1,m2', 'p1', 60)`)
	mustExec(t, db, `INSERT INTO group_quotas (group_id, daily_limit, monthly_limit, daily_request_limit, max_backends, max_api_keys) VALUES ('g_1', 1000, 20000, 500, 3, 5)`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ep.IsGroup() || ep.GroupID != "g_1" {
		t.Fatalf("expected group policy, got mode=%q group=%q", ep.Mode, ep.GroupID)
	}
	if !ep.HasPlan {
		t.Fatal("expected HasPlan")
	}
	if !ep.IsAllowedBackend("b1") || ep.IsAllowedBackend("b3") {
		t.Fatalf("backend allowlist wrong: %v", ep.AllowBackends)
	}
	if !ep.IsAllowedModel("m2") || ep.IsAllowedModel("nope") {
		t.Fatalf("model allowlist wrong: %v", ep.AllowModels)
	}
	if !ep.IsAllowedPipeline("p1") || ep.IsAllowedPipeline("nope") {
		t.Fatalf("pipeline allowlist wrong: %v", ep.AllowPipelines)
	}
	if ep.RateLimitRPM != 60 {
		t.Fatalf("rpm: got %d", ep.RateLimitRPM)
	}
	if ep.GroupDailyTokenLimit != 1000 || ep.GroupDailyRequestLimit != 500 ||
		ep.GroupMaxBackends != 3 || ep.GroupMaxAPIKeys != 5 {
		t.Fatalf("group quota wrong: %+v", ep)
	}
	if ep.TokenQuotaTotal != nil {
		t.Fatalf("user token total leaked into group mode: %+v", ep.TokenQuotaTotal)
	}
}

func TestResolver_GroupMode_NoPlan_Defaults(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (1, 'alice', 'g_1', 'group')`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ep.HasPlan {
		t.Fatal("no plan but HasPlan")
	}
	if !ep.IsAllowedModel("anything") || !ep.IsAllowedBackend("anything") {
		t.Fatal("empty allowlist must allow all")
	}
	if ep.GroupDailyTokenLimit != 0 || ep.IsTokenQuotaEnabled() {
		t.Fatal("default policy must be unlimited")
	}
}

func TestResolver_CustomMode(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (1, 'alice', 'g_1', 'custom')`)
	mustExec(t, db, `INSERT INTO user_plans (user_id, available_models, budget_amount, token_quota_input, token_quota_total) VALUES (1, 'm1', 10.5, 100000, 250)`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ep.IsGroup() {
		t.Fatal("expected custom policy")
	}
	if !ep.IsBudgetEnabled() || *ep.BudgetAmount != 10.5 {
		t.Fatalf("budget wrong: %+v", ep.BudgetAmount)
	}
	if !ep.IsTokenQuotaEnabled() || *ep.TokenQuotaInput != 100000 {
		t.Fatalf("token quota wrong: %+v", ep.TokenQuotaInput)
	}
	if ep.TokenQuotaTotal == nil || *ep.TokenQuotaTotal != 250 {
		t.Fatalf("token total wrong: %+v", ep.TokenQuotaTotal)
	}
	// group quota must not leak into custom mode.
	if ep.GroupDailyTokenLimit != 0 {
		t.Fatal("group quota leaked into custom mode")
	}
}

func TestResolver_CustomMode_NoPlan(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, policy_mode) VALUES (1, 'bob', 'custom')`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ep.HasPlan {
		t.Fatal("no user plan but HasPlan")
	}
	if ep.IsTokenQuotaEnabled() {
		t.Fatal("no plan must have no token quota")
	}
}

func TestResolveForEdition_SyntheticFullAccess(t *testing.T) {
	pol, err := ResolveForEdition(nil, context.Background(), 1, false)
	if err != nil {
		t.Fatalf("ResolveForEdition: %v", err)
	}
	if !pol.HasPlan || !pol.IsAllowedModel("anything") || pol.IsTokenQuotaEnabled() {
		t.Fatalf("synthetic full access wrong: %+v", pol)
	}
}

func TestResolver_TemplateAssignment_PerMember(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `
CREATE TABLE plan_templates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	currency TEXT NOT NULL DEFAULT 'USD',
	available_backend_ids TEXT NOT NULL DEFAULT '[]',
	available_models TEXT NOT NULL DEFAULT '[]',
	available_pipeline_ids TEXT NOT NULL DEFAULT '[]',
	price_type TEXT NOT NULL DEFAULT 'revenue',
	budget_amount REAL,
	budget_period TEXT NOT NULL DEFAULT 'monthly',
	budget_start_at TEXT,
	budget_end_at TEXT,
	token_quota_input INTEGER,
	token_quota_output INTEGER,
	token_quota_total INTEGER,
	token_quota_period TEXT NOT NULL DEFAULT 'monthly',
	token_quota_start_at TEXT,
	token_quota_end_at TEXT,
	rate_limit_rpm INTEGER NOT NULL DEFAULT 0,
	rate_limit_tpm INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE user_plan_assignments (
	user_id INTEGER PRIMARY KEY,
	template_id INTEGER NOT NULL,
	assigned_at TEXT,
	assigned_by TEXT
);
CREATE TABLE group_plan_assignments (
	group_id TEXT PRIMARY KEY,
	template_id INTEGER NOT NULL,
	metering_mode TEXT NOT NULL DEFAULT 'per_member',
	assigned_at TEXT
);`)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (1, 'a', 'g1', 'group')`)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (2, 'b', 'g1', 'group')`)
	mustExec(t, db, `INSERT INTO plan_templates (id, name, available_models, token_quota_total, token_quota_period)
		VALUES (10, '中度', 'm1', 100000, 'daily')`)
	mustExec(t, db, `INSERT INTO group_plan_assignments (group_id, template_id, metering_mode) VALUES ('g1', 10, 'per_member')`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ep.HasPlan || ep.TemplateID != 10 || ep.TemplateName != "中度" {
		t.Fatalf("template resolve wrong: %+v", ep)
	}
	if ep.MeteringMode != MeteringPerMember {
		t.Fatalf("metering: %q", ep.MeteringMode)
	}
	if ep.UsesSharedPool() {
		t.Fatal("per_member must not use shared pool")
	}
	if ep.TokenQuotaTotal == nil || *ep.TokenQuotaTotal != 100000 {
		t.Fatalf("quota wrong: %+v", ep.TokenQuotaTotal)
	}
}

func TestResolver_UnknownUser(t *testing.T) {
	db := newResolverDB(t)
	r := NewResolver(db, "sqlite", WithTTL(0))
	ep, err := r.Resolve(context.Background(), 999)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ep.HasPlan || ep.IsGroup() {
		t.Fatalf("unknown user must resolve to default, got %+v", ep)
	}
}

func TestResolver_Overrides(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, group_id, policy_mode) VALUES (1, 'alice', 'g_1', 'group')`)
	mustExec(t, db, `INSERT INTO users (id, username, policy_mode) VALUES (2, 'bob', 'custom')`)
	mustExec(t, db, `INSERT INTO group_pricing_overrides (group_id, backend_id, model, price_type, input_price_per_m, output_price_per_m) VALUES ('g_1', 'b1', 'm1', 'cost', 1.5, 2.5)`)
	mustExec(t, db, `INSERT INTO user_pricing_overrides (user_id, backend_id, model, price_type, input_price_per_m, output_price_per_m) VALUES (2, 'b1', 'm1', 'cost', 3.5, 4.5)`)

	r := NewResolver(db, "sqlite", WithTTL(0))

	// group-mode user sees group override.
	ov, err := r.ResolveOverride(context.Background(), 1, "b1", "m1", "cost")
	if err != nil {
		t.Fatalf("ResolveOverride: %v", err)
	}
	if ov == nil || ov.InputPricePerM != 1.5 {
		t.Fatalf("group override wrong: %+v", ov)
	}

	// custom-mode user sees user override.
	ov2, err := r.ResolveOverride(context.Background(), 2, "b1", "m1", "cost")
	if err != nil {
		t.Fatalf("ResolveOverride: %v", err)
	}
	if ov2 == nil || ov2.InputPricePerM != 3.5 {
		t.Fatalf("user override wrong: %+v", ov2)
	}

	// no match → nil.
	ov3, err := r.ResolveOverride(context.Background(), 1, "b9", "m9", "cost")
	if err != nil {
		t.Fatalf("ResolveOverride: %v", err)
	}
	if ov3 != nil {
		t.Fatalf("expected nil override, got %+v", ov3)
	}
}

func TestResolver_ExpiredOverrideIgnored(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, policy_mode) VALUES (1, 'bob', 'custom')`)
	mustExec(t, db, `INSERT INTO user_pricing_overrides (user_id, backend_id, model, price_type, input_price_per_m, output_price_per_m, expires_at) VALUES (1, 'b1', 'm1', 'cost', 3.5, 4.5, '2020-01-01 00:00:00')`)

	r := NewResolver(db, "sqlite", WithTTL(0))
	ov, err := r.ResolveOverride(context.Background(), 1, "b1", "m1", "cost")
	if err != nil {
		t.Fatalf("ResolveOverride: %v", err)
	}
	if ov != nil {
		t.Fatalf("expired override must be ignored, got %+v", ov)
	}
}

func TestResolver_CacheInvalidation(t *testing.T) {
	db := newResolverDB(t)
	mustExec(t, db, `INSERT INTO users (id, username, policy_mode) VALUES (1, 'bob', 'custom')`)

	r := NewResolver(db, "sqlite") // default TTL
	ep, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ep.HasPlan {
		t.Fatal("no plan initially")
	}

	// New plan created; without invalidation the cache is stale...
	mustExec(t, db, `INSERT INTO user_plans (user_id, available_models) VALUES (1, 'm1')`)
	// ...so explicit invalidation is required for prompt propagation.
	r.Invalidate(1)
	ep2, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ep2.HasPlan || !ep2.IsAllowedModel("m1") {
		t.Fatalf("expected fresh plan after invalidate, got %+v", ep2)
	}

	// Moving a user to a group is a per-user change → Invalidate(userID).
	mustExec(t, db, `UPDATE users SET group_id = 'g_9', policy_mode = 'group' WHERE id = 1`)
	mustExec(t, db, `INSERT INTO group_plans (group_id, available_models) VALUES ('g_9', 'gm1')`)
	r.Invalidate(1)
	ep3, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ep3.IsGroup() || !ep3.IsAllowedModel("gm1") {
		t.Fatalf("expected fresh group policy after Invalidate, got %+v", ep3)
	}

	// Group plan change → InvalidateGroup covers every member.
	mustExec(t, db, `UPDATE group_plans SET available_models = 'gm2' WHERE group_id = 'g_9'`)
	r.InvalidateGroup("g_9")
	ep4, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ep4.IsAllowedModel("gm2") || ep4.IsAllowedModel("gm1") {
		t.Fatalf("expected fresh group plan after InvalidateGroup, got %+v", ep4)
	}
}

func TestParseAllowList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"[]", nil},
		{"{}", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{"{a,b,c}", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{`{"a","b"}`, []string{"a", "b"}},
	}
	for _, tc := range cases {
		got := parseAllowList(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("parseAllowList(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("parseAllowList(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestFlexTime(t *testing.T) {
	var tm time.Time
	ft := &flexTime{&tm}
	if err := ft.Scan(nil); err != nil {
		t.Fatalf("scan nil: %v", err)
	}
	if !tm.IsZero() {
		t.Fatal("nil should leave zero time")
	}
	if err := ft.Scan("2026-01-02 15:04:05"); err != nil {
		t.Fatalf("scan text: %v", err)
	}
	if tm.Year() != 2026 || tm.Month() != time.January || tm.Day() != 2 {
		t.Fatalf("parsed %v", tm)
	}
}
