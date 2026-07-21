package server

import (
	"context"
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// syncProxyDefaultModelFromBackend updates in-memory + persisted proxy config when the
// edited backend is the current system default.
// Minimal: data/proxy-config.yaml；其它发行版：system_config.proxy_config。
func syncProxyDefaultModelFromBackend(backendID string) {
	backendID = strings.TrimSpace(backendID)
	if backendID == "" {
		return
	}

	cfg := config.Get()
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Proxy.DefaultBackendID) != backendID {
		return
	}

	mgr := backend.GetManager()
	if mgr == nil {
		return
	}
	b, err := mgr.Get(backendID)
	if err != nil || b == nil {
		return
	}

	nextModel := strings.TrimSpace(backend.PreferredDefaultModel(b))
	if nextModel == "" {
		return
	}
	if nextModel == strings.TrimSpace(cfg.Proxy.DefaultModel) {
		return
	}

	cfg.Proxy.DefaultModel = nextModel
	if err := config.PersistProxyConfig(context.Background(), cfg.Proxy); err != nil {
		logger.Warnf("[ProxyConfig] Failed to persist default_model after backend update: %v", err)
		return
	}
	logger.Infof("[ProxyConfig] Synced default_model=%q from backend %q", nextModel, backendID)
}
