package useraccess

import (
	"testing"

	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
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
		{ID: "own-1", Enabled: true, TenantID: "t1"},
	}
	got := FilterBackends(user, list)
	if len(got) != 1 || got[0].ID != "own-1" {
		t.Fatalf("expected only own backend, got %#v", got)
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
		{ID: "own-p", TenantID: "t1"},
	}
	got := FilterPipelines(user, list)
	if len(got) != 1 || got[0].ID != "own-p" {
		t.Fatalf("expected only own pipeline, got %#v", got)
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
