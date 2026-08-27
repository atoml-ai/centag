package pipeline

import "context"

type ownerScopeKey struct{}

// WithOwnerScope attaches the caller's resource ownership scope (legacy tenant_id
// or synthetic "user:{id}") so Execute/HasPipeline can resolve private pipelines.
func WithOwnerScope(ctx context.Context, scope string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ownerScopeKey{}, scope)
}

// OwnerScopeFromContext returns the ownership scope when present.
func OwnerScopeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ownerScopeKey{}).(string)
	return v
}

type userIDKey struct{}

// WithUserID attaches the authenticated user ID to the context so downstream
// hooks (e.g. FilterAllowedBackend) can resolve the user's policy.
func WithUserID(ctx context.Context, userID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserIDFromContext returns the user ID when present.
func UserIDFromContext(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	v, _ := ctx.Value(userIDKey{}).(int64)
	return v
}
