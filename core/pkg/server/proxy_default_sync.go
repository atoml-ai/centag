package server

import (
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// syncProxyDefaultModelFromBackend updates in-memory + on-disk proxy config when the
// edited backend is the current system default. Minimal edition persists to
// data/proxy-config.yaml; other editions still update the live config.Get() values.
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
	if dataDir := config.ResolveDataDir(); dataDir != "" {
		if err := config.SaveProxyConfigToFile(dataDir, cfg.Proxy); err != nil {
			logger.Warnf("[ProxyConfig] Failed to persist default_model after backend update: %v", err)
			return
		}
	}
	logger.Infof("[ProxyConfig] Synced default_model=%q from backend %q", nextModel, backendID)
}
