package server

import (
	"context"
	"fmt"

	"centag/core/pkg/database"
	"centag/core/pkg/groupmodel"
)

// policyForUser resolves the effective group-model policy for a Team normal
// user. Returns nil when no database / resolver is available or resolution
// fails, so callers fall back to legacy whitelist filtering. Under the group
// model (migration 036) the allowlists live in the user's effective policy
// (group_plans / user_plans), not the legacy users.allowed_* columns.
func policyForUser(ctx context.Context, user *database.User) *groupmodel.EffectivePolicy {
	if user == nil {
		return nil
	}
	dbm := database.Get()
	if dbm == nil || dbm.GetDB() == nil {
		return nil
	}
	pol, err := groupmodel.NewResolver(dbm.GetDB(), dbm.DriverName()).Resolve(ctx, user.ID)
	if err != nil {
		return nil
	}
	return pol
}

// ownTenantID returns the user's own resource scope for ownership checks.
// Under the group model (036), when the user has no legacy TenantID (typical
// for team-admin-created users), a synthetic scope "user:{id}" is generated
// so the user can create/edit/delete their own backends and pipelines.
// The scope is kept consistent across Create/Update/Delete and List filtering.
func ownTenantID(user *database.User) string {
	if user == nil {
		return ""
	}
	if user.TenantID != nil && *user.TenantID != "" {
		return *user.TenantID
	}
	// Generate a synthetic user-scoped identifier for team normal users.
	return fmt.Sprintf("user:%d", user.ID)
}
