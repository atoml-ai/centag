package backend

import (
	"centag/core/pkg/logger"
)

// fileBackendsConfig is the YAML structure for initial-backends.yaml.
type fileBackendsConfig struct {
	Version     string                 `yaml:"version" json:"version"`
	Description string                 `yaml:"description" json:"description"`
	Backends    []fileBackendEntry     `yaml:"backends" json:"backends"`
}

// fileBackendEntry is a single backend entry in the YAML file.
type fileBackendEntry struct {
	ID              string                `yaml:"id" json:"id"`
	Name            string                `yaml:"name" json:"name"`
	Type            string                `yaml:"type" json:"type"`
	BaseURL         string                `yaml:"base_url" json:"base_url"`
	APIKey          string                `yaml:"api_key" json:"api_key"`
	Enabled         bool                  `yaml:"enabled" json:"enabled"`
	Timeout         int                   `yaml:"timeout" json:"timeout"`
	MaxRetries      int                   `yaml:"max_retries" json:"max_retries"`
	AutoFetchModels bool                  `yaml:"auto_fetch_models" json:"auto_fetch_models"`
	Description     string                `yaml:"description" json:"description"`
	ProbeModel      string                `yaml:"probe_model,omitempty" json:"probe_model,omitempty"`
	SupportedModels []fileModelMapping    `yaml:"supported_models" json:"supported_models"`
	Capabilities    fileModelCapabilities `yaml:"capabilities" json:"capabilities"`
	Weight          int                   `yaml:"weight" json:"weight"`
	Priority        int                   `yaml:"priority" json:"priority"`
}

// fileModelMapping maps requested model to actual model.
type fileModelMapping struct {
	RequestedModel     string  `yaml:"requested_model" json:"requested_model"`
	ActualModel        string  `yaml:"actual_model" json:"actual_model"`
	IsExact            bool    `yaml:"is_exact" json:"is_exact"`
	CompatibilityScore float64 `yaml:"compatibility_score" json:"compatibility_score"`
}

// fileModelCapabilities describes model capabilities.
type fileModelCapabilities struct {
	MaxContextTokens int      `yaml:"max_context_tokens" json:"max_context_tokens"`
	Features         []string `yaml:"features" json:"features"`
	SupportsTools    bool     `yaml:"supports_tools" json:"supports_tools"`
}

// fileEntryToBackendConfig converts a fileBackendEntry to a BackendConfig,
// applying defaults for Timeout, MaxRetries, Weight, and normalising the API key.
func fileEntryToBackendConfig(fe *fileBackendEntry) *BackendConfig {
	supportedModels := make([]ModelMapping, 0, len(fe.SupportedModels))
	for _, sm := range fe.SupportedModels {
		supportedModels = append(supportedModels, ModelMapping{
			RequestedModel:     sm.RequestedModel,
			ActualModel:        sm.ActualModel,
			CompatibilityScore: sm.CompatibilityScore,
			IsExact:            sm.IsExact,
		})
	}

	capabilities := ModelCapabilities{
		MaxContextTokens: fe.Capabilities.MaxContextTokens,
		Features:         fe.Capabilities.Features,
		SupportsTools:    fe.Capabilities.SupportsTools,
	}

	timeout := fe.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	maxRetries := fe.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	weight := fe.Weight
	if weight <= 0 {
		weight = 1
	}

	return &BackendConfig{
		ID:              fe.ID,
		Name:            fe.Name,
		Type:            fe.Type,
		BaseURL:         fe.BaseURL,
		APIKey:          NormalizeOpenAICompatibleAPIKey(fe.APIKey),
		Enabled:         fe.Enabled,
		Timeout:         timeout,
		MaxRetries:      maxRetries,
		AutoFetchModels: fe.AutoFetchModels,
		Description:     fe.Description,
		ProbeModel:      fe.ProbeModel,
		SupportedModels: supportedModels,
		Capabilities:    capabilities,
		Weight:          weight,
		Priority:        fe.Priority,
	}
}

// backendConfigToFileEntry converts a BackendConfig to a fileBackendEntry
// for YAML serialization.
func backendConfigToFileEntry(b *BackendConfig) fileBackendEntry {
	supportedModels := make([]fileModelMapping, 0, len(b.SupportedModels))
	for _, sm := range b.SupportedModels {
		supportedModels = append(supportedModels, fileModelMapping{
			RequestedModel:     sm.RequestedModel,
			ActualModel:        sm.ActualModel,
			IsExact:            sm.IsExact,
			CompatibilityScore: sm.CompatibilityScore,
		})
	}

	return fileBackendEntry{
		ID:              b.ID,
		Name:            b.Name,
		Type:            b.Type,
		BaseURL:         b.BaseURL,
		APIKey:          b.APIKey,
		Enabled:         b.Enabled,
		Timeout:         b.Timeout,
		MaxRetries:      b.MaxRetries,
		AutoFetchModels: b.AutoFetchModels,
		Description:     b.Description,
		ProbeModel:      b.ProbeModel,
		SupportedModels: supportedModels,
		Capabilities: fileModelCapabilities{
			MaxContextTokens: b.Capabilities.MaxContextTokens,
			Features:         b.Capabilities.Features,
			SupportsTools:    b.Capabilities.SupportsTools,
		},
		Weight:   b.Weight,
		Priority: b.Priority,
	}
}

// LoadFromFile reads backend configs from a YAML file and loads them into the manager.
// This is used by the minimal edition which has no database.
func (m *Manager) LoadFromFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store := NewFileBackendStore(filePath)
	backends, err := store.Load()
	if err != nil {
		return err
	}

	m.backends = make(map[string]*BackendConfig, len(backends))
	for _, b := range backends {
		m.backends[b.ID] = b
	}

	logger.Infof("Loaded %d backends from file: %s", len(m.backends), filePath)
	return nil
}

// SaveToFile writes backend configs to a YAML file.
// This is used by the minimal edition's config-generator.
func (m *Manager) SaveToFile(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backends := make([]*BackendConfig, 0, len(m.backends))
	for _, b := range m.backends {
		backends = append(backends, b)
	}

	store := NewFileBackendStore(filePath)
	return store.Save(backends)
}

// ReloadFromFile re-reads backend configs from a YAML file, replacing the current in-memory state.
// Used by the config-generator's reload endpoint.
func (m *Manager) ReloadFromFile(filePath string) error {
	return m.LoadFromFile(filePath)
}


