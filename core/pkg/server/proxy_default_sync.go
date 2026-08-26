package server

import (
	"context"
	"fmt"
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

// applyPinnedProxyDefaults 将 Agent 接入向导所选的后端/模型写入系统默认配置
// （proxy.default_backend_id / proxy.default_model，即 system.default_backend / system.default_model 变量），
// 使透明流水线的默认出站被替换为用户所选组合。模型为空时回落该后端的推荐默认模型。
func applyPinnedProxyDefaults(backendID, model string) error {
	backendID = strings.TrimSpace(backendID)
	if backendID == "" {
		return fmt.Errorf("backend id is empty")
	}
	mgr := backend.GetManager()
	if mgr == nil {
		return fmt.Errorf("backend manager unavailable")
	}
	b, err := mgr.Get(backendID)
	if err != nil || b == nil {
		return fmt.Errorf("backend %q not found", backendID)
	}
	nextModel := strings.TrimSpace(model)
	if nextModel == "" {
		nextModel = strings.TrimSpace(backend.PreferredDefaultModel(b))
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("config unavailable")
	}
	changed := false
	if strings.TrimSpace(cfg.Proxy.DefaultBackendID) != backendID {
		cfg.Proxy.DefaultBackendID = backendID
		changed = true
	}
	if nextModel != "" && strings.TrimSpace(cfg.Proxy.DefaultModel) != nextModel {
		cfg.Proxy.DefaultModel = nextModel
		changed = true
	}
	if !changed {
		return nil
	}
	if err := config.PersistProxyConfig(context.Background(), cfg.Proxy); err != nil {
		return err
	}
	logger.Infof("[ProxyConfig] Agent setup pinned default backend=%q model=%q", backendID, nextModel)
	return nil
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
