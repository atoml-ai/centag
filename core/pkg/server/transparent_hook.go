package server

import (
	"centag/core/pkg/backend"
	"centag/core/pkg/pipeline"
)

func wireTransparentBackend(backendMgr *backend.Manager) {
	if backendMgr == nil {
		return
	}
	pipeline.ResolveBackendEndpoint = func(backendID string) (*pipeline.BackendEndpoint, error) {
		cfg, err := backendMgr.Get(backendID)
		if err != nil {
			return nil, err
		}
		return &pipeline.BackendEndpoint{
			BaseURL:     cfg.BaseURL,
			APIKey:      cfg.APIKey,
			AccountPool: cfg.AccountPool,
		}, nil
	}
	pipeline.ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		all := backendMgr.List()
		out := make([]*backend.BackendConfig, 0, len(all))
		for _, cfg := range all {
			if cfg != nil && cfg.Enabled {
				out = append(out, cfg)
			}
		}
		return out
	}
}
