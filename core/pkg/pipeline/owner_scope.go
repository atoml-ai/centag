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
