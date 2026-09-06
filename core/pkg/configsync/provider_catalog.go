package configsync

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// ProviderCatalogEntry represents a provider type available for users to select
// when adding a new backend. This is a catalog of available provider types,
// NOT actual backend configurations.
type ProviderCatalogEntry struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Type          string              `json:"type"` // openai, anthropic, ollama, gemini, etc.
	BaseURL       string              `json:"base_url"`
	EnvKey        string              `json:"env_key,omitempty"` // Environment variable for API key
	Icon          string              `json:"icon,omitempty"`
	Description   string              `json:"description,omitempty"`
	DefaultModels []ProviderModel     `json:"default_models,omitempty"`
	Enabled       bool                `json:"enabled"`
	UpdatedAt     time.Time           `json:"updated_at"`
	Remark        string              `json:"remark,omitempty"`
}

// ProviderModel represents a default model for a provider type.
type ProviderModel struct {
	Name             string `json:"name"`
	SupportsTools    bool   `json:"supports_tools"`
	SupportsImages   bool   `json:"supports_images"`
	SupportsThinking bool   `json:"supports_thinking"`
	MaxContextTokens int    `json:"max_context_tokens"`
}

// ProviderCatalog is the collection of available provider types.
type ProviderCatalog struct {
	Entries  []ProviderCatalogEntry `json:"entries"`
	SyncTime time.Time              `json:"sync_time"`
	Source   string                 `json:"source,omitempty"` // "feishu", "manual", etc.
}

// ProviderCatalogStore is the interface for provider catalog persistence.
type ProviderCatalogStore interface {
	// GetAll returns all provider catalog entries.
	GetAll() []ProviderCatalogEntry

	// GetByID returns a single provider catalog entry by ID.
	GetByID(id string) (*ProviderCatalogEntry, error)

	// Upsert inserts or updates a provider catalog entry.
	Upsert(entry ProviderCatalogEntry) error

	// UpsertBatch inserts or updates multiple provider catalog entries.
	UpsertBatch(entries []ProviderCatalogEntry) error

	// Delete removes a provider catalog entry by ID.
	Delete(id string) error

	// Clear removes all provider catalog entries.
	Clear() error

	// GetSyncTime returns the last sync time.
	GetSyncTime() time.Time

	// SetSyncTime updates the last sync time.
	SetSyncTime(t time.Time) error
}

// InMemoryProviderCatalogStore is an in-memory implementation of ProviderCatalogStore.
type InMemoryProviderCatalogStore struct {
	mu        sync.RWMutex
	entries   map[string]ProviderCatalogEntry
	syncTime  time.Time
}

// NewInMemoryProviderCatalogStore creates a new in-memory provider catalog store.
func NewInMemoryProviderCatalogStore() *InMemoryProviderCatalogStore {
	return &InMemoryProviderCatalogStore{
		entries: make(map[string]ProviderCatalogEntry),
	}
}

// GetAll returns all provider catalog entries.
func (s *InMemoryProviderCatalogStore) GetAll() []ProviderCatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProviderCatalogEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

// GetByID returns a single provider catalog entry by ID.
func (s *InMemoryProviderCatalogStore) GetByID(id string) (*ProviderCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	if !ok {
		return nil, nil
	}
	return &e, nil
}

// Upsert inserts or updates a provider catalog entry.
func (s *InMemoryProviderCatalogStore) Upsert(entry ProviderCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.UpdatedAt = time.Now()
	s.entries[entry.ID] = entry
	return nil
}

// UpsertBatch inserts or updates multiple provider catalog entries.
func (s *InMemoryProviderCatalogStore) UpsertBatch(entries []ProviderCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, e := range entries {
		e.UpdatedAt = now
		s.entries[e.ID] = e
	}
	return nil
}

// Delete removes a provider catalog entry by ID.
func (s *InMemoryProviderCatalogStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	return nil
}

// Clear removes all provider catalog entries.
func (s *InMemoryProviderCatalogStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]ProviderCatalogEntry)
	return nil
}

// GetSyncTime returns the last sync time.
func (s *InMemoryProviderCatalogStore) GetSyncTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncTime
}

// SetSyncTime updates the last sync time.
func (s *InMemoryProviderCatalogStore) SetSyncTime(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncTime = t
	return nil
}

// ProviderCatalogApplier syncs provider catalog entries from remote configsync.
// Unlike BackendApplier which adds backends to the user's configured list,
// this applier updates the catalog of available provider types that users
// can choose from when adding a new backend.
type ProviderCatalogApplier struct {
	store ProviderCatalogStore
	mu    sync.RWMutex
}

// NewProviderCatalogApplier creates a ProviderCatalogApplier.
func NewProviderCatalogApplier(store ProviderCatalogStore) *ProviderCatalogApplier {
	return &ProviderCatalogApplier{store: store}
}

// Apply processes backend.* rows and updates the provider catalog.
// This does NOT add backends to the user's configured list.
func (a *ProviderCatalogApplier) Apply(rows []Row) {
	if a.store == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	var entries []ProviderCatalogEntry
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "backend.") || !r.Enabled {
			continue
		}

		// Parse the backend config to extract provider catalog info
		var raw map[string]interface{}
		if err := json.Unmarshal(r.Value, &raw); err != nil {
			continue
		}

		// Extract required fields
		id, _ := raw["id"].(string)
		if id == "" {
			continue
		}

		name, _ := raw["name"].(string)
		providerType, _ := raw["type"].(string)
		baseURL, _ := raw["base_url"].(string)
		description, _ := raw["description"].(string)

		// Skip entries without required fields
		if providerType == "" || baseURL == "" {
			continue
		}

		// Parse default models if present
		var defaultModels []ProviderModel
		if modelsRaw, ok := raw["supported_models"]; ok {
			if modelsJSON, err := json.Marshal(modelsRaw); err == nil {
				json.Unmarshal(modelsJSON, &defaultModels)
			}
		}

		entry := ProviderCatalogEntry{
			ID:            id,
			Name:          name,
			Type:          providerType,
			BaseURL:       baseURL,
			Description:   description,
			DefaultModels: defaultModels,
			Enabled:       true,
			Remark:        r.Remark,
		}

		entries = append(entries, entry)
	}

	if len(entries) > 0 {
		if err := a.store.UpsertBatch(entries); err == nil {
			a.store.SetSyncTime(time.Now())
		}
	}
}

// GetCatalog returns the current provider catalog.
func (a *ProviderCatalogApplier) GetCatalog() []ProviderCatalogEntry {
	if a.store == nil {
		return nil
	}
	return a.store.GetAll()
}
