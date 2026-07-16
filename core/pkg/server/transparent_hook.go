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
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
		}, nil
	}
}