// Package useraccess enforces Team-edition per-user shared resource whitelists
// and self-service flags for normal users.
package useraccess

import (
	"encoding/json"
	"strings"

	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
	"centag/core/pkg/pipeline"
)

// Applies reports whether whitelist / self-service flags apply to this user.
func Applies(ed edition.Edition, user *database.User) bool {
	return ed.IsTeam() && user != nil && user.Role == database.RoleNormal
}

func idSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func containsID(ids []string, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, x := range ids {
		if strings.TrimSpace(x) == id {
			return true
		}
	}
	return false
}

// EncodeIDs serializes an ID list for DB storage.
func EncodeIDs(ids []string) string {
	if ids == nil {
		ids = []string{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// DecodeIDs parses a JSON array of IDs from DB.
func DecodeIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// CanUseSharedBackend reports whether a system backend is allowed.
// Empty whitelist ⇒ no shared backends.
func CanUseSharedBackend(user *database.User, backendID string) bool {
	if user == nil {
		return false
	}
	return containsID(user.AllowedBackendIDs, backendID)
}

// CanUseSharedModel reports whether a shared-backend model is allowed.
// Empty whitelist ⇒ no shared models.
func CanUseSharedModel(user *database.User, modelID string) bool {
	if user == nil {
		return false
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	if containsID(user.AllowedModelIDs, modelID) {
		return true
	}
	// Also match normalized names against stored IDs.
	norm := backend.NormalizeModelName(modelID)
	for _, id := range user.AllowedModelIDs {
		if backend.NormalizeModelName(id) == norm {
			return true
		}
	}
	return false
}

// CanUseSharedPipeline reports whether a system pipeline is allowed.
func CanUseSharedPipeline(user *database.User, pipelineID string) bool {
	if user == nil {
		return false
	}
	return containsID(user.AllowedPipelineIDs, pipelineID)
}

// FilterBackends keeps tenant-owned backends and allowed+enabled system backends.
// For allowed system backends, SupportedModels are dual-filtered by allowed models.
func FilterBackends(user *database.User, list []*backend.BackendConfig) []*backend.BackendConfig {
	if user == nil {
		return nil
	}
	out := make([]*backend.BackendConfig, 0, len(list))
	for _, b := range list {
		if b == nil {
			continue
		}
		if b.TenantID != "" {
			out = append(out, b)
			continue
		}
		if !b.Enabled || !CanUseSharedBackend(user, b.ID) {
			continue
		}
		cp := *b
		cp.SupportedModels = filterModelMappings(user, b.SupportedModels)
		out = append(out, &cp)
	}
	return out
}

func filterModelMappings(user *database.User, maps []backend.ModelMapping) []backend.ModelMapping {
	if len(maps) == 0 {
		return maps
	}
	out := make([]backend.ModelMapping, 0, len(maps))
	for _, m := range maps {
		req := strings.TrimSpace(m.RequestedModel)
		act := strings.TrimSpace(m.ActualModel)
		if (req != "" && CanUseSharedModel(user, req)) || (act != "" && CanUseSharedModel(user, act)) {
			out = append(out, m)
		}
	}
	return out
}

// ModelAllowedOnBackend checks dual filter for a model on a specific backend.
// Tenant-owned backends: all models allowed. System backends: backend + model whitelist.
func ModelAllowedOnBackend(user *database.User, cfg *backend.BackendConfig, model string) bool {
	if user == nil || cfg == nil {
		return false
	}
	if cfg.TenantID != "" {
		return true
	}
	if !cfg.Enabled || !CanUseSharedBackend(user, cfg.ID) {
		return false
	}
	return CanUseSharedModel(user, model)
}

// FilterPipelines keeps tenant-owned pipelines and allowed system pipelines.
func FilterPipelines(user *database.User, list []*pipeline.AgentPatternPipeline) []*pipeline.AgentPatternPipeline {
	if user == nil {
		return nil
	}
	out := make([]*pipeline.AgentPatternPipeline, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.TenantID != "" {
			out = append(out, p)
			continue
		}
		if CanUseSharedPipeline(user, p.ID) {
			out = append(out, p)
		}
	}
	return out
}

// CanServeModel reports whether the filtered backend set can handle requestedModel.
// "auto" / empty requires at least one visible backend.
func CanServeModel(user *database.User, backends []*backend.BackendConfig, requestedModel string) bool {
	if user == nil {
		return true
	}
	filtered := FilterBackends(user, backends)
	if len(filtered) == 0 {
		return false
	}
	norm := backend.NormalizeModelName(strings.TrimSpace(requestedModel))
	if norm == "" || norm == "auto" {
		return true
	}
	for _, b := range filtered {
		if b == nil {
			continue
		}
		if b.TenantID != "" {
			if len(b.SupportedModels) == 0 {
				return true
			}
			for _, m := range b.SupportedModels {
				if backend.NormalizeModelName(m.RequestedModel) == norm || backend.NormalizeModelName(m.ActualModel) == norm {
					return true
				}
			}
			continue
		}
		if !ModelAllowedOnBackend(user, b, requestedModel) {
			continue
		}
		for _, m := range b.SupportedModels {
			if backend.NormalizeModelName(m.RequestedModel) == norm || backend.NormalizeModelName(m.ActualModel) == norm {
				return true
			}
		}
	}
	return false
}

// DefaultBoolTrue returns true when the pointer is nil (DB default / omitted).
func DefaultBoolTrue(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}
