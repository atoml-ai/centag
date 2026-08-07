package useraccess

import (
	"testing"

	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
	"centag/core/pkg/groupmodel"
	"centag/core/pkg/pipeline"
)

func TestApplies(t *testing.T) {
	normal := &database.User{Role: database.RoleNormal}
	admin := &database.User{Role: database.RoleAdmin}
	if !Applies(edition.Team, normal) {
		t.Fatal("team normal should apply")
	}
	if Applies(edition.Team, admin) {
		t.Fatal("admin should not apply")
	}
	if Applies(edition.Personal, normal) {
		t.Fatal("personal should not apply")
	}
}

func TestFilterBackends_EmptyMeansNoShared(t *testing.T) {
	user := &database.User{
		Role:              database.RoleNormal,
		AllowedBackendIDs: []string{},
		AllowedModelIDs:   []string{},
	}
	list := []*backend.BackendConfig{
		{ID: "sys-a", Enabled: true, TenantID: "", SupportedModels: []backend.ModelMapping{{RequestedModel: "m1", ActualModel: "m1"}}},
		{ID: "other-1", Enabled: true, TenantID: "user:999"},
	}
	got := FilterBackends(user, list)
	// With the new scope-based filtering, user sees only their own backends.
	// Since user has no TenantID and user.ID=0, scope is "user:0", so "user:999" is not included.
	if len(got) != 0 {
		t.Fatalf("expected no backends (user scope mismatch), got %#v", got)
	}
}

func TestFilterBackends_OwnScopeVisible(t *testing.T) {
	user := &database.User{
		ID:                 42,
		Role:               database.RoleNormal,
		AllowedBackendIDs:  []string{},
		AllowedModelIDs:    []string{},
	}
	list := []*backend.BackendConfig{
		{ID: "sys-a", Enabled: true, TenantID: ""},
		{ID: "own-1", Enabled: true, TenantID: "user:42"},
		{ID: "other-1", Enabled: true, TenantID: "user:999"},
	}
	got := FilterBackends(user, list)
	if len(got) != 1 || got[0].ID != "own-1" {
		t.Fatalf("expected only own-1 backend (scope user:42), got %#v", got)
	}
}

func TestFilterBackends_DualModelFilter(t *testing.T) {
	user := &database.User{
		Role:              database.RoleNormal,
		AllowedBackendIDs: []string{"sys-a"},
		AllowedModelIDs:   []string{"m1"},
	}
	list := []*backend.BackendConfig{
		{
			ID: "sys-a", Enabled: true, TenantID: "",
			SupportedModels: []backend.ModelMapping{
				{RequestedModel: "m1", ActualModel: "m1"},
				{RequestedModel: "m2", ActualModel: "m2"},
			},
		},
		{ID: "sys-b", Enabled: true, TenantID: ""},
	}
	got := FilterBackends(user, list)
	if len(got) != 1 || got[0].ID != "sys-a" {
		t.Fatalf("expected sys-a only, got %#v", got)
	}
	if len(got[0].SupportedModels) != 1 || got[0].SupportedModels[0].RequestedModel != "m1" {
		t.Fatalf("expected only m1, got %#v", got[0].SupportedModels)
	}
}

func TestFilterBackends_DisabledSharedSkipped(t *testing.T) {
	user := &database.User{
		AllowedBackendIDs: []string{"sys-a"},
		AllowedModelIDs:   []string{"m1"},
	}
	list := []*backend.BackendConfig{
		{ID: "sys-a", Enabled: false, TenantID: "", SupportedModels: []backend.ModelMapping{{RequestedModel: "m1"}}},
	}
	got := FilterBackends(user, list)
	if len(got) != 0 {
		t.Fatalf("disabled shared backend must be filtered out")
	}
}

func TestFilterPipelines_EmptyMeansNoShared(t *testing.T) {
	user := &database.User{AllowedPipelineIDs: []string{}}
	list := []*pipeline.AgentPatternPipeline{
		{ID: "sys-p", TenantID: ""},
		{ID: "other-p", TenantID: "user:999"},
	}
	got := FilterPipelines(user, list)
	// With the new scope-based filtering, user sees only their own pipelines.
	// Since user has no TenantID and user.ID=0, scope is "user:0", so "user:999" is not included.
	if len(got) != 0 {
		t.Fatalf("expected no pipelines (user scope mismatch), got %#v", got)
	}
}

func TestFilterPipelines_OwnScopeVisible(t *testing.T) {
	user := &database.User{
		ID:                  42,
		AllowedPipelineIDs:  []string{},
	}
	list := []*pipeline.AgentPatternPipeline{
		{ID: "sys-p", TenantID: ""},
		{ID: "own-p", TenantID: "user:42"},
		{ID: "other-p", TenantID: "user:999"},
	}
	got := FilterPipelines(user, list)
	if len(got) != 1 || got[0].ID != "own-p" {
		t.Fatalf("expected only own-p pipeline (scope user:42), got %#v", got)
	}
}

func TestCanServeModel(t *testing.T) {
	user := &database.User{
		AllowedBackendIDs: []string{"sys-a"},
		AllowedModelIDs:   []string{"m1"},
	}
	backends := []*backend.BackendConfig{
		{
			ID: "sys-a", Enabled: true, TenantID: "",
			SupportedModels: []backend.ModelMapping{{RequestedModel: "m1", ActualModel: "m1"}},
		},
	}
	if !CanServeModel(user, backends, "m1") {
		t.Fatal("m1 should be allowed")
	}
	if CanServeModel(user, backends, "m2") {
		t.Fatal("m2 should be denied")
	}
	if !CanServeModel(user, backends, "auto") {
		t.Fatal("auto should be allowed when backends visible")
	}
}

func TestEncodeDecodeIDs(t *testing.T) {
	raw := EncodeIDs([]string{"a", "b"})
	got := DecodeIDs(raw)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("roundtrip failed: %v", got)
	}
	if len(DecodeIDs("")) != 0 {
		t.Fatal("empty should decode to empty slice")
	}
}

func polWithPlan(allowBackends, allowModels, allowPipelines []string) *groupmodel.EffectivePolicy {
	return &groupmodel.EffectivePolicy{
		Mode:           groupmodel.PolicyModeGroup,
		GroupID:        "g1",
		AllowBackends:  allowBackends,
		AllowModels:    allowModels,
		AllowPipelines: allowPipelines,
		HasPlan:        true,
	}
}

func TestFilterBackendsByPolicy_OwnTenantPlusAllowedSystem(t *testing.T) {
	user := &database.User{Role: database.RoleNormal}
	tid := "t1"
	user.TenantID = &tid
	pol := polWithPlan([]string{"sys-a"}, []string{"m1"}, nil)
	list := []*backend.BackendConfig{
		{ID: "sys-a", Enabled: true, TenantID: "", SupportedModels: []backend.ModelMapping{{RequestedModel: "m1", ActualModel: "m1"}, {RequestedModel: "m2", ActualModel: "m2"}}},
		{ID: "sys-b", Enabled: true, TenantID: ""},
		{ID: "own-1", Enabled: true, TenantID: "t1"},
		{ID: "other-1", Enabled: true, TenantID: "t2"},
		{ID: "sys-c", Enabled: false, TenantID: ""},
	}
	got := FilterBackendsByPolicy(user, list, pol)
	if len(got) != 2 {
		t.Fatalf("expected own-1 + sys-a, got %#v", got)
	}
	if got[0].ID != "sys-a" && got[1].ID != "sys-a" {
		t.Fatal("sys-a should be allowed")
	}
	for _, b := range got {
		if b.ID == "sys-a" {
			if len(b.SupportedModels) != 1 || b.SupportedModels[0].RequestedModel != "m1" {
				t.Fatalf("expected only m1 models, got %#v", b.SupportedModels)
			}
		}
		if b.ID == "other-1" {
			t.Fatal("another tenant's backend must be hidden")
		}
	}
}

func TestFilterBackendsFor_NoPlanOwnOnly(t *testing.T) {
	user := &database.User{
		ID:   7,
		Role: database.RoleNormal,
	}
	list := []*backend.BackendConfig{
		{ID: "sys-a", Enabled: true, TenantID: ""},
		{ID: "sys-b", Enabled: true, TenantID: ""},
		{ID: "own-1", Enabled: true, TenantID: "user:7"},
	}
	got := FilterBackendsFor(user, list, nil)
	if len(got) != 1 || got[0].ID != "own-1" {
		t.Fatalf("expected own-only without plan, got %#v", got)
	}
}

func TestFilterPipelinesByPolicy_OwnTenantPlusAllowedSystem(t *testing.T) {
	user := &database.User{Role: database.RoleNormal}
	tid := "t1"
	user.TenantID = &tid
	pol := polWithPlan(nil, nil, []string{"sys-p"})
	list := []*pipeline.AgentPatternPipeline{
		{ID: "sys-p", TenantID: ""},
		{ID: "sys-q", TenantID: ""},
		{ID: "own-p", TenantID: "t1"},
		{ID: "other-p", TenantID: "t2"},
	}
	got := FilterPipelinesByPolicy(user, list, pol)
	if len(got) != 2 {
		t.Fatalf("expected own-p + sys-p, got %#v", got)
	}
	for _, p := range got {
		if p.ID == "other-p" {
			t.Fatal("another tenant's pipeline must be hidden")
		}
	}
}

func TestFilterPipelinesByPolicy_EmptyAllowlistMeansAllSystemAllowed(t *testing.T) {
	user := &database.User{Role: database.RoleNormal}
	pol := polWithPlan(nil, nil, nil)
	list := []*pipeline.AgentPatternPipeline{
		{ID: "sys-p", TenantID: ""},
		{ID: "sys-q", TenantID: ""},
	}
	got := FilterPipelinesByPolicy(user, list, pol)
	if len(got) != 2 {
		t.Fatalf("empty allowlist should allow all system pipelines, got %#v", got)
	}
}
