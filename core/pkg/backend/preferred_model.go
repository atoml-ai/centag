package backend

import "strings"

// PreferredDefaultModel returns the backend's preferred chat/default model.
// Order: ProbeModel → first SupportedModels ActualModel → RequestedModel.
func PreferredDefaultModel(cfg *BackendConfig) string {
	if cfg == nil {
		return ""
	}
	if m := strings.TrimSpace(cfg.ProbeModel); m != "" {
		return m
	}
	if len(cfg.SupportedModels) == 0 {
		return ""
	}
	if m := strings.TrimSpace(cfg.SupportedModels[0].ActualModel); m != "" {
		return m
	}
	return strings.TrimSpace(cfg.SupportedModels[0].RequestedModel)
}
