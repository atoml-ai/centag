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

// isCurrentDefaultBackend reports whether backendID is the current system default backend.
func isCurrentDefaultBackend(backendID string) bool {
	cfg := config.Get()
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.Proxy.DefaultBackendID) == strings.TrimSpace(backendID) &&
		strings.TrimSpace(backendID) != ""
}

// resyncDefaultAfterBackendDelete re-selects the default backend/model after the
// previous default was deleted. It picks the next available backend (highest weight
// among remaining), syncs proxy default_backend_id + default_model, and persists.
// If no backend remains, the default backend/model are cleared.
func resyncDefaultAfterBackendDelete() {
	cfg := config.Get()
	if cfg == nil {
		return
	}
	mgr := backend.GetManager()
	if mgr == nil {
		return
	}

	remaining := mgr.List()
	if len(remaining) == 0 {
		// 无剩余后端：默认后端与默认模型置空。
		if strings.TrimSpace(cfg.Proxy.DefaultBackendID) == "" && strings.TrimSpace(cfg.Proxy.DefaultModel) == "" {
			return
		}
		cfg.Proxy.DefaultBackendID = ""
		cfg.Proxy.DefaultModel = ""
		if err := config.PersistProxyConfig(context.Background(), cfg.Proxy); err != nil {
			logger.Warnf("[ProxyConfig] Failed to clear default after deleting last backend: %v", err)
			return
		}
		logger.Info("[ProxyConfig] Cleared default backend/model (no backends remain)")
		return
	}

	// 选取剩余后端中权重最高的启用后端；无启用后端则退化为第一个。
	var next *backend.BackendConfig
	enabled := mgr.GetEnabled()
	if len(enabled) > 0 {
		next = enabled[0]
		for _, b := range enabled[1:] {
			if b.Weight > next.Weight {
				next = b
			}
		}
	} else {
		next = remaining[0]
	}
	if next == nil {
		return
	}

	cfg.Proxy.DefaultBackendID = next.ID
	cfg.Proxy.DefaultModel = strings.TrimSpace(backend.PreferredDefaultModel(next))
	if err := config.PersistProxyConfig(context.Background(), cfg.Proxy); err != nil {
		logger.Warnf("[ProxyConfig] Failed to persist default after backend delete: %v", err)
		return
	}
	logger.Infof("[ProxyConfig] Default backend switched to %q after delete (model=%q)", next.ID, cfg.Proxy.DefaultModel)
}
