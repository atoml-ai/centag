// Package useraccess enforces Team-edition per-user shared resource whitelists
// and self-service flags for normal users.
package useraccess

import (
	"encoding/json"
	"fmt"
	"strings"

	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
	"centag/core/pkg/groupmodel"
	"centag/core/pkg/pipeline"
)

// userOwnerScope returns the user's own resource scope for ownership checks.
// Under the group model (036), when the user has no legacy TenantID (typical
// for team-admin-created users), a synthetic scope "user:{id}" is generated
// so the user can create/edit/delete their own backends and pipelines.
func userOwnerScope(user *database.User) string {
	if user == nil {
		return ""
	}
	if user.TenantID != nil && *user.TenantID != "" {
		return *user.TenantID
	}
	return fmt.Sprintf("user:%d", user.ID)
}

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
	// Wildcard: "*" in the allowlist grants access to all backends.
	for _, id := range user.AllowedBackendIDs {
		if strings.TrimSpace(id) == "*" {
			return true
		}
	}
	return containsID(user.AllowedBackendIDs, backendID)
}

// CanUseSharedModel reports whether a shared-backend model is allowed.
// Empty whitelist ⇒ no shared models.  A wildcard "*" in the list allows all models.
func CanUseSharedModel(user *database.User, modelID string) bool {
	if user == nil {
		return false
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	// Wildcard: "*" in the allowlist grants access to all models.
	for _, id := range user.AllowedModelIDs {
		if strings.TrimSpace(id) == "*" {
			return true
		}
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
// A wildcard "*" in the list allows all pipelines.
func CanUseSharedPipeline(user *database.User, pipelineID string) bool {
	if user == nil {
		return false
	}
	// Wildcard: "*" in the allowlist grants access to all pipelines.
	for _, id := range user.AllowedPipelineIDs {
		if strings.TrimSpace(id) == "*" {
			return true
		}
	}
	return containsID(user.AllowedPipelineIDs, pipelineID)
}

// FilterBackends keeps the user's own scope-scoped backends and allowed+enabled
// system backends. For allowed system backends, SupportedModels are dual-filtered
// by allowed models.
func FilterBackends(user *database.User, list []*backend.BackendConfig) []*backend.BackendConfig {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*backend.BackendConfig, 0, len(list))
	for _, b := range list {
		if b == nil {
			continue
		}
		if b.TenantID != "" {
			// Include only the user's own scope-scoped backends (not other users').
			if b.TenantID == own {
				out = append(out, b)
			}
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

// FilterPipelines keeps the user's own scope-scoped pipelines and allowed
// system pipelines.
func FilterPipelines(user *database.User, list []*pipeline.AgentPatternPipeline) []*pipeline.AgentPatternPipeline {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*pipeline.AgentPatternPipeline, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.TenantID != "" {
			// Include only the user's own scope-scoped pipelines (not other users').
			if p.TenantID == own {
				out = append(out, p)
			}
			continue
		}
		if CanUseSharedPipeline(user, p.ID) {
			out = append(out, p)
		}
	}
	return out
}

// FilterBackendsFor and FilterPipelinesFor resolve resource visibility for a
// Team normal user under EffectivePlan. Without an active plan, only the user's
// own scope-scoped resources are visible (no shared system resources).
func FilterBackendsFor(user *database.User, list []*backend.BackendConfig, pol *groupmodel.EffectivePolicy) []*backend.BackendConfig {
	if pol != nil && pol.HasPlan {
		return FilterBackendsByPolicy(user, list, pol)
	}
	return filterOwnBackendsOnly(user, list)
}

func FilterPipelinesFor(user *database.User, list []*pipeline.AgentPatternPipeline, pol *groupmodel.EffectivePolicy) []*pipeline.AgentPatternPipeline {
	if pol != nil && pol.HasPlan {
		return FilterPipelinesByPolicy(user, list, pol)
	}
	return filterOwnPipelinesOnly(user, list)
}

func filterOwnBackendsOnly(user *database.User, list []*backend.BackendConfig) []*backend.BackendConfig {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*backend.BackendConfig, 0, len(list))
	for _, b := range list {
		if b != nil && b.TenantID != "" && b.TenantID == own {
			out = append(out, b)
		}
	}
	return out
}

func filterOwnPipelinesOnly(user *database.User, list []*pipeline.AgentPatternPipeline) []*pipeline.AgentPatternPipeline {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*pipeline.AgentPatternPipeline, 0, len(list))
	for _, p := range list {
		if p != nil && p.TenantID != "" && p.TenantID == own {
			out = append(out, p)
		}
	}
	return out
}

// FilterBackendsByPolicy keeps the user's own scope-scoped backends plus
// policy-allowed, enabled system backends. Empty policy allowlist = all system
// backends allowed; model mappings are dual-filtered.
func FilterBackendsByPolicy(user *database.User, list []*backend.BackendConfig, pol *groupmodel.EffectivePolicy) []*backend.BackendConfig {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*backend.BackendConfig, 0, len(list))
	for _, b := range list {
		if b == nil {
			continue
		}
		if b.TenantID != "" {
			if b.TenantID == own {
				out = append(out, b)
			}
			continue
		}
		if !b.Enabled || !pol.IsAllowedBackend(b.ID) {
			continue
		}
		cp := *b
		cp.SupportedModels = filterModelMappingsByPolicy(pol, b.SupportedModels)
		out = append(out, &cp)
	}
	return out
}

func filterModelMappingsByPolicy(pol *groupmodel.EffectivePolicy, maps []backend.ModelMapping) []backend.ModelMapping {
	if len(maps) == 0 || pol == nil || len(pol.AllowModels) == 0 {
		return maps
	}
	out := make([]backend.ModelMapping, 0, len(maps))
	for _, m := range maps {
		req := strings.TrimSpace(m.RequestedModel)
		act := strings.TrimSpace(m.ActualModel)
		if (req != "" && pol.IsAllowedModel(req)) || (act != "" && pol.IsAllowedModel(act)) {
			out = append(out, m)
		}
	}
	return out
}

// FilterPipelinesByPolicy keeps the user's own scope-scoped pipelines plus
// policy-allowed system pipelines. Empty policy allowlist = all system
// pipelines allowed.
func FilterPipelinesByPolicy(user *database.User, list []*pipeline.AgentPatternPipeline, pol *groupmodel.EffectivePolicy) []*pipeline.AgentPatternPipeline {
	if user == nil {
		return nil
	}
	own := userOwnerScope(user)
	out := make([]*pipeline.AgentPatternPipeline, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.TenantID != "" {
			if p.TenantID == own {
				out = append(out, p)
			}
			continue
		}
		if pol.IsAllowedPipeline(p.ID) {
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
