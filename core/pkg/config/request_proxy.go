package config

import "context"

type proxyDefaultsKey struct{}

// ProxyDefaults is a request-scoped override for {{system.default_*}} resolution.
type ProxyDefaults struct {
	DefaultBackendID string
	DefaultModel     string
}

// WithProxyDefaults attaches per-request proxy defaults to ctx (Team user overrides).
func WithProxyDefaults(ctx context.Context, d ProxyDefaults) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, proxyDefaultsKey{}, d)
}

// ProxyDefaultsFromContext returns request-scoped proxy defaults when present.
func ProxyDefaultsFromContext(ctx context.Context) (ProxyDefaults, bool) {
	if ctx == nil {
		return ProxyDefaults{}, false
	}
	v, ok := ctx.Value(proxyDefaultsKey{}).(ProxyDefaults)
	return v, ok
}
