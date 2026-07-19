package server

import (
	"context"
	"testing"

	"centag/core/pkg/config"
)

func TestEnsureSystemProxyEgressAPIKey_AlreadyInConfig(t *testing.T) {
	cfg := &config.Config{
		SystemProxy: config.SystemProxyConfig{EgressAPIKey: "llmproxy_existing"},
	}
	changed, err := EnsureSystemProxyEgressAPIKey(context.Background(), cfg, 0)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no change when key already set")
	}
	if cfg.SystemProxy.EgressAPIKey != "llmproxy_existing" {
		t.Fatalf("key mutated: %q", cfg.SystemProxy.EgressAPIKey)
	}
}

func TestEnsureSystemProxyEgressAPIKey_PersistFromEnv(t *testing.T) {
	t.Setenv("LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY", "llmproxy_from_env")
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")
	cfg := &config.Config{SystemProxy: config.SystemProxyConfig{}}
	changed, err := EnsureSystemProxyEgressAPIKey(context.Background(), cfg, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected persist env key into config")
	}
	if cfg.SystemProxy.EgressAPIKey != "llmproxy_from_env" {
		t.Fatalf("got %q", cfg.SystemProxy.EgressAPIKey)
	}
}

func TestEnsureSystemProxyEgressAPIKey_NilConfig(t *testing.T) {
	_, err := EnsureSystemProxyEgressAPIKey(context.Background(), nil, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
