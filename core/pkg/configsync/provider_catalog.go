package configsync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
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

// --- DB-backed ProviderCatalogStore (R14) ---

// DBProviderCatalogStore persists provider catalog entries in the database.
type DBProviderCatalogStore struct {
	db      *sql.DB
	dialect database.Dialect
	mu      sync.RWMutex
}

// NewDBProviderCatalogStore creates a database-backed provider catalog store.
func NewDBProviderCatalogStore() (*DBProviderCatalogStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &DBProviderCatalogStore{db: mgr.GetDB(), dialect: dialect}, nil
}

func (s *DBProviderCatalogStore) ph(n int) string {
	if s.dialect != nil {
		return s.dialect.Placeholder(n)
	}
	d := database.SQLiteDialect{}
	return d.Placeholder(n)
}

// GetAll returns all provider catalog entries from the database.
func (s *DBProviderCatalogStore) GetAll() []ProviderCatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, name, provider_type, base_url, env_key, icon, description, default_models, enabled FROM provider_catalog`)
	if err != nil {
		logger.Warnf("provider catalog: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var result []ProviderCatalogEntry
	for rows.Next() {
		var e ProviderCatalogEntry
		var modelsStr string
		var enabled int
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.BaseURL, &e.EnvKey, &e.Icon, &e.Description, &modelsStr, &enabled); err != nil {
			logger.Warnf("provider catalog: scan failed: %v", err)
			continue
		}
		e.Enabled = enabled != 0
		_ = json.Unmarshal([]byte(modelsStr), &e.DefaultModels)
		result = append(result, e)
	}
	return result
}

// GetByID returns a single provider catalog entry by ID.
func (s *DBProviderCatalogStore) GetByID(id string) (*ProviderCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var e ProviderCatalogEntry
	var modelsStr string
	var enabled int
	query := fmt.Sprintf(`SELECT id, name, provider_type, base_url, env_key, icon, description, default_models, enabled
		FROM provider_catalog WHERE id = %s`, s.ph(1))
	err := s.db.QueryRow(query, id).Scan(&e.ID, &e.Name, &e.Type, &e.BaseURL, &e.EnvKey, &e.Icon, &e.Description, &modelsStr, &enabled)
	if err != nil {
		return nil, err
	}
	e.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(modelsStr), &e.DefaultModels)
	return &e, nil
}

// Upsert inserts or updates a provider catalog entry.
func (s *DBProviderCatalogStore) Upsert(entry ProviderCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modelsJSON, _ := json.Marshal(entry.DefaultModels)
	query := fmt.Sprintf(`INSERT INTO provider_catalog (id, name, provider_type, base_url, env_key, icon, description, default_models, enabled, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT(id) DO UPDATE SET
			name = EXCLUDED.name, provider_type = EXCLUDED.provider_type, base_url = EXCLUDED.base_url,
			env_key = EXCLUDED.env_key, icon = EXCLUDED.icon, description = EXCLUDED.description,
			default_models = EXCLUDED.default_models, enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11))

	now := time.Now()
	enabled := 0
	if entry.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(query, entry.ID, entry.Name, entry.Type, entry.BaseURL, entry.EnvKey, entry.Icon, entry.Description, string(modelsJSON), enabled, now, now)
	return err
}

// UpsertBatch inserts or updates multiple provider catalog entries.
func (s *DBProviderCatalogStore) UpsertBatch(entries []ProviderCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO provider_catalog (id, name, provider_type, base_url, env_key, icon, description, default_models, enabled, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT(id) DO UPDATE SET
			name = EXCLUDED.name, provider_type = EXCLUDED.provider_type, base_url = EXCLUDED.base_url,
			env_key = EXCLUDED.env_key, icon = EXCLUDED.icon, description = EXCLUDED.description,
			default_models = EXCLUDED.default_models, enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11)))
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, e := range entries {
		modelsJSON, _ := json.Marshal(e.DefaultModels)
		enabled := 0
		if e.Enabled {
			enabled = 1
		}
		if _, err := stmt.Exec(e.ID, e.Name, e.Type, e.BaseURL, e.EnvKey, e.Icon, e.Description, string(modelsJSON), enabled, now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete removes a provider catalog entry by ID.
func (s *DBProviderCatalogStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM provider_catalog WHERE id = %s", s.ph(1)), id)
	return err
}

// Clear removes all provider catalog entries.
func (s *DBProviderCatalogStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM provider_catalog")
	return err
}

// GetSyncTime is not used for DB store (sync time is implicit from data).
func (s *DBProviderCatalogStore) GetSyncTime() time.Time { return time.Time{} }

// SetSyncTime is not used for DB store.
func (s *DBProviderCatalogStore) SetSyncTime(t time.Time) error { return nil }

// --- Default provider catalog entries (R14) ---

// DefaultProviderCatalogEntries returns hardcoded provider types as the final
// fallback when no remote data or DB data is available. This ensures the
// provider catalog is never empty during startup.
func DefaultProviderCatalogEntries() []ProviderCatalogEntry {
	return []ProviderCatalogEntry{
		{ID: "openai", Name: "OpenAI", Type: "openai", BaseURL: "https://api.openai.com/v1", EnvKey: "OPENAI_API_KEY", Enabled: true,
			DefaultModels: []ProviderModel{{Name: "gpt-4o", SupportsTools: true, SupportsImages: true, MaxContextTokens: 128000}}},
		{ID: "anthropic", Name: "Anthropic", Type: "anthropic", BaseURL: "https://api.anthropic.com", EnvKey: "ANTHROPIC_API_KEY", Enabled: true,
			DefaultModels: []ProviderModel{{Name: "claude-sonnet-4-20250514", SupportsTools: true, SupportsImages: true, MaxContextTokens: 200000}}},
		{ID: "deepseek", Name: "DeepSeek", Type: "openai", BaseURL: "https://api.deepseek.com/v1", EnvKey: "DEEPSEEK_API_KEY", Enabled: true,
			DefaultModels: []ProviderModel{{Name: "deepseek-chat", SupportsTools: true, MaxContextTokens: 64000}}},
		{ID: "kimi", Name: "Kimi (Moonshot)", Type: "openai", BaseURL: "https://api.moonshot.cn/v1", EnvKey: "KIMI_API_KEY", Enabled: true,
			DefaultModels: []ProviderModel{{Name: "moonshot-v1-128k", SupportsImages: true, MaxContextTokens: 128000}}},
		{ID: "siliconflow", Name: "SiliconFlow", Type: "openai", BaseURL: "https://api.siliconflow.cn/v1", EnvKey: "SILICONFLOW_API_KEY", Enabled: true,
			DefaultModels: []ProviderModel{{Name: "Qwen/Qwen2.5-72B-Instruct", SupportsTools: true, MaxContextTokens: 32000}}},
	}
}
