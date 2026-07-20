package backend

import (
	"strings"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// DBBackendStore implements BackendStore using the system config database.
// It is used by the personal / team editions.
type DBBackendStore struct{}

// NewDBBackendStore creates a new DBBackendStore.
func NewDBBackendStore() *DBBackendStore {
	return &DBBackendStore{}
}

// Load reads backend configs from the database, applies API key merging from
// the initial JSON seed file, and returns them as BackendConfig instances.
func (s *DBBackendStore) Load() ([]*BackendConfig, error) {
	cfg := config.Get()
	if cfg == nil || len(cfg.Backends) == 0 {
		logger.Info("No backend configs found in database, starting with empty list")
		return nil, nil
	}

	enriched, ok := MergeAPIKeysFromInitialFile(cfg.Backends)
	if ok {
		cfg.Backends = enriched
		if err := config.SaveConfig(cfg); err != nil {
			logger.Warnf("写入补全后的 API Key 到数据库失败: %v（内存仍使用补全后的值）", err)
		}
	}

	backends := make([]*BackendConfig, 0, len(cfg.Backends))
	for i := range cfg.Backends {
		backends = append(backends, configBackendToBackend(&cfg.Backends[i]))
	}

	logger.Info("Loaded backend configs from database", logger.GetField("count", len(backends)))
	return backends, nil
}

// Save persists all given backend configs to the database.
func (s *DBBackendStore) Save(backends []*BackendConfig) error {
	backendConfigs := make([]config.BackendConfig, 0, len(backends))
	for _, b := range backends {
		backendConfigs = append(backendConfigs, backendToConfigBackend(b))
	}

	if err := config.SaveBackendsToDB(backendConfigs); err != nil {
		return err
	}

	logger.Info("Saved backend configs to database", logger.GetField("count", len(backends)))
	return nil
}

// configBackendToBackend converts a config.BackendConfig to a BackendConfig.
func configBackendToBackend(c *config.BackendConfig) *BackendConfig {
	supportedModels := make([]ModelMapping, 0, len(c.SupportedModels))
	for _, sm := range c.SupportedModels {
		supportedModels = append(supportedModels, ModelMapping{
			RequestedModel:     sm.RequestedModel,
			ActualModel:        sm.ActualModel,
			CompatibilityScore: sm.CompatibilityScore,
			IsExact:            sm.IsExact,
		})
	}

	capabilities := ModelCapabilities(c.Capabilities)

	var healthStatus *BackendHealthStatus
	if c.HealthStatus != nil {
		healthStatus = &BackendHealthStatus{
			Status:       c.HealthStatus.Status,
			LastCheckAt:  c.HealthStatus.LastCheckAt,
			LastError:    c.HealthStatus.LastError,
			ResponseTime: c.HealthStatus.ResponseTime,
			ModelsCount:  c.HealthStatus.ModelsCount,
		}
	}

	return &BackendConfig{
		ID:              c.ID,
		Name:            c.Name,
		Type:            c.Type,
		BaseURL:         c.BaseURL,
		APIKey:          NormalizeOpenAICompatibleAPIKey(c.APIKey),
		Enabled:         c.Enabled,
		Timeout:         c.Timeout,
		MaxRetries:      c.MaxRetries,
		Description:     c.Description,
		Metadata:        c.Metadata,
		SupportedModels: supportedModels,
		Capabilities:    capabilities,
		AutoFetchModels: c.AutoFetchModels,
		ProbeModel:      strings.TrimSpace(c.ProbeModel),
		HealthStatus:    healthStatus,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		Weight:          c.Weight,
		Priority:        c.Priority,
		TenantID:        c.TenantID,
	}
}

// backendToConfigBackend converts a BackendConfig to a config.BackendConfig.
func backendToConfigBackend(b *BackendConfig) config.BackendConfig {
	supportedModels := make([]config.ModelMapping, 0, len(b.SupportedModels))
	for _, sm := range b.SupportedModels {
		supportedModels = append(supportedModels, config.ModelMapping{
			RequestedModel:     sm.RequestedModel,
			ActualModel:        sm.ActualModel,
			CompatibilityScore: sm.CompatibilityScore,
			IsExact:            sm.IsExact,
		})
	}

	capabilities := config.ModelCapabilities(b.Capabilities)

	var healthStatus *config.BackendHealthStatus
	if b.HealthStatus != nil {
		healthStatus = &config.BackendHealthStatus{
			Status:       b.HealthStatus.Status,
			LastCheckAt:  b.HealthStatus.LastCheckAt,
			LastError:    b.HealthStatus.LastError,
			ResponseTime: b.HealthStatus.ResponseTime,
			ModelsCount:  b.HealthStatus.ModelsCount,
		}
	}

	return config.BackendConfig{
		ID:              b.ID,
		Name:            b.Name,
		Type:            b.Type,
		BaseURL:         b.BaseURL,
		APIKey:          b.APIKey,
		Enabled:         b.Enabled,
		Timeout:         b.Timeout,
		MaxRetries:      b.MaxRetries,
		Metadata:        b.Metadata,
		Description:     b.Description,
		SupportedModels: supportedModels,
		Capabilities:    capabilities,
		AutoFetchModels: b.AutoFetchModels,
		ProbeModel:      strings.TrimSpace(b.ProbeModel),
		HealthStatus:    healthStatus,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
		Weight:          b.Weight,
		Priority:        b.Priority,
		TenantID:        b.TenantID,
	}
}
