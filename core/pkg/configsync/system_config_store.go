package configsync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// SystemConfigStore provides type-safe access to the system_config KV table.
type SystemConfigStore struct {
	db      *sql.DB
	dialect database.Dialect
	mu      sync.RWMutex
}

// NewSystemConfigStore creates a new KV store backed by the database.
func NewSystemConfigStore() (*SystemConfigStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &SystemConfigStore{db: mgr.GetDB(), dialect: dialect}, nil
}

func (s *SystemConfigStore) ph(n int) string {
	if s.dialect != nil {
		return s.dialect.Placeholder(n)
	}
	d := database.SQLiteDialect{}
	return d.Placeholder(n)
}

// KVRow is a single key-value entry from the system_config table.
type KVRow struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type"`
	Scope       string `json:"scope"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// Get returns the raw string value for a key.
func (s *SystemConfigStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value string
	query := fmt.Sprintf("SELECT value FROM system_config WHERE key = %s AND enabled = 1", s.ph(1))
	err := s.db.QueryRow(query, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetFloat returns the value as a float64, or the fallback on error.
func (s *SystemConfigStore) GetFloat(key string, fallback float64) float64 {
	val, err := s.Get(key)
	if err != nil {
		return fallback
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		logger.Warnf("system_config: parse float %s=%q failed: %v, using default %v", key, val, err, fallback)
		return fallback
	}
	return f
}

// GetInt returns the value as an int, or the fallback on error.
func (s *SystemConfigStore) GetInt(key string, fallback int) int {
	val, err := s.Get(key)
	if err != nil {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		logger.Warnf("system_config: parse int %s=%q failed: %v, using default %v", key, val, err, fallback)
		return fallback
	}
	return i
}

// GetBool returns the value as a bool.
func (s *SystemConfigStore) GetBool(key string) bool {
	val, err := s.Get(key)
	if err != nil {
		return false
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return b
}

// GetString returns the string value, or the fallback on error.
func (s *SystemConfigStore) GetString(key, fallback string) string {
	val, err := s.Get(key)
	if err != nil {
		return fallback
	}
	return val
}

// GetAll returns all enabled KV rows.
func (s *SystemConfigStore) GetAll() ([]KVRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT key, value, value_type, scope, enabled, description FROM system_config WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []KVRow
	for rows.Next() {
		var r KVRow
		var enabled int
		if err := rows.Scan(&r.Key, &r.Value, &r.ValueType, &r.Scope, &enabled, &r.Description); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetAllAsMap returns all enabled KV rows as a map[string]string.
func (s *SystemConfigStore) GetAllAsMap() (map[string]string, error) {
	rows, err := s.GetAll()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	return m, nil
}

// Upsert inserts or updates a KV row.
func (s *SystemConfigStore) Upsert(key, value, valueType, scope, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf(`INSERT INTO system_config (key, value, value_type, scope, enabled, description, created_at, updated_at)
		VALUES (%s, %s, %s, %s, 1, %s, %s, %s)
		ON CONFLICT(key) DO UPDATE SET
			value = EXCLUDED.value,
			value_type = EXCLUDED.value_type,
			scope = EXCLUDED.scope,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7))

	now := time.Now()
	_, err := s.db.Exec(query, key, value, valueType, scope, description, now, now)
	return err
}

// UpsertBatch inserts or updates multiple KV rows in a transaction.
func (s *SystemConfigStore) UpsertBatch(rows []KVRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO system_config (key, value, value_type, scope, enabled, description, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT(key) DO UPDATE SET
			value = EXCLUDED.value,
			value_type = EXCLUDED.value_type,
			scope = EXCLUDED.scope,
			enabled = EXCLUDED.enabled,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8)))
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range rows {
		enabled := 0
		if r.Enabled {
			enabled = 1
		}
		if _, err := stmt.Exec(r.Key, r.Value, r.ValueType, r.Scope, enabled, r.Description, now, now); err != nil {
			return fmt.Errorf("upsert kv %s: %w", r.Key, err)
		}
	}
	return tx.Commit()
}

// SyncFromSnapshot applies a system_config snapshot (from Snapshot.Tables["system_config"])
// to the database. This is the "apply" step that persists remote KV data.
func (s *SystemConfigStore) SyncFromSnapshot(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}

	var entries []KVRow
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("unmarshal system_config: %w", err)
	}

	// Only upsert entries that are present in the snapshot.
	// Unknown keys from remote are rejected (not persisted).
	return s.UpsertBatch(entries)
}

// SnapshotData returns all enabled KV rows as a JSON-serializable structure
// suitable for Snapshot.Tables["system_config"].
func (s *SystemConfigStore) SnapshotData() (json.RawMessage, error) {
	rows, err := s.GetAll()
	if err != nil {
		return nil, err
	}
	return json.Marshal(rows)
}

// --- Built-in KV key registration ---

// BuiltInKVKeys returns the default system_config keys with their types and defaults.
// This is the single source of truth for which keys exist and their default values.
func BuiltInKVKeys() []KeyValueSpec {
	return []KeyValueSpec{
		{Key: "billing.usd_to_cny", Type: KVNumber, Scope: KVScopeCore, Default: func() any { return 7.2 }},
		{Key: "scheduler.price_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 20 }},
		{Key: "scheduler.performance_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 20 }},
		{Key: "scheduler.quality_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 25 }},
		{Key: "scheduler.latency_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 15 }},
		{Key: "scheduler.privacy_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 10 }},
		{Key: "scheduler.match_weight", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 10 }},
		{Key: "pipeline.default_timeout_ms", Type: KVInt, Scope: KVScopeCore, Default: func() any { return 30000 }},
		{Key: "retry.codes", Type: KVString, Scope: KVScopeCore, Default: func() any { return "" }},
	}
}

// ApplySystemConfig applies system_config KV values to runtime components.
// This is called from the OnUpdate callback after syncing remote data.
func ApplySystemConfig(store *SystemConfigStore) {
	if store == nil {
		return
	}

	// Apply exchange rate
	rate := store.GetFloat("billing.usd_to_cny", 7.2)
	applyExchangeRate(rate)

	// Apply scheduler weights
	weights := map[string]int{
		"price":       store.GetInt("scheduler.price_weight", 20),
		"performance": store.GetInt("scheduler.performance_weight", 20),
		"quality":     store.GetInt("scheduler.quality_weight", 25),
		"latency":     store.GetInt("scheduler.latency_weight", 15),
		"privacy":     store.GetInt("scheduler.privacy_weight", 10),
		"match":       store.GetInt("scheduler.match_weight", 10),
	}
	applySchedulerWeights(weights)

	logger.Infof("system_config: applied — exchange_rate=%.2f, weights=%v", rate, weights)
}

// applyExchangeRate is a function variable to allow testing without importing billing.
var applyExchangeRate = func(rate float64) {
	// Default no-op; replaced at init time by the entrypoint.
}

// SetExchangeRateApplier sets the function used to apply the exchange rate.
// Called by entrypoint to wire up billing.SetUSDToCNY.
func SetExchangeRateApplier(fn func(float64)) {
	applyExchangeRate = fn
}

// applySchedulerWeights is a function variable to allow testing without importing config.
var applySchedulerWeights = func(weights map[string]int) {
	// Default no-op; replaced at init time by the entrypoint.
}

// SetSchedulerWeightsApplier sets the function used to apply scheduler weights.
// Called by entrypoint to wire up config.SetSchedulerWeights.
func SetSchedulerWeightsApplier(fn func(map[string]int)) {
	applySchedulerWeights = fn
}

// ParseRetryCodes parses a comma-separated string of HTTP status codes into a map.
func ParseRetryCodes(codesStr string) map[int]bool {
	codes := make(map[int]bool)
	if codesStr == "" {
		return codes
	}
	for _, s := range strings.Split(codesStr, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if code, err := strconv.Atoi(s); err == nil && code >= 100 && code < 600 {
			codes[code] = true
		}
	}
	return codes
}
