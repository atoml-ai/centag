package configsync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// AgentAppRow represents a single agent app catalog entry from remote.
type AgentAppRow struct {
	TypeID      string          `json:"type_id"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Vendor      string          `json:"vendor"`
	Category    string          `json:"category"`
	Enabled     bool            `json:"enabled"`
	Sort        int             `json:"sort"`
	InstallURL  string          `json:"install_url"`
	InstallHint string          `json:"install_hint"`
	MetaJSON    json.RawMessage `json:"meta_json"`
}

// AgentAppsOverlay is the in-memory overlay that merges remote data with
// the code-defined TemplateRegistry. Only display/guide/sort fields are overridden.
type AgentAppsOverlay struct {
	mu    sync.RWMutex
	apps  map[string]AgentAppRow // type_id → remote overlay data
}

// NewAgentAppsOverlay creates an empty overlay.
func NewAgentAppsOverlay() *AgentAppsOverlay {
	return &AgentAppsOverlay{apps: make(map[string]AgentAppRow)}
}

// LoadFromSnapshot applies remote agent_apps data from Snapshot.Tables["agent_apps"].
func (o *AgentAppsOverlay) LoadFromSnapshot(data json.RawMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(data) == 0 {
		return
	}

	var rows []AgentAppRow
	if err := json.Unmarshal(data, &rows); err != nil {
		logger.Warnf("agent_apps: unmarshal snapshot failed: %v", err)
		return
	}

	newApps := make(map[string]AgentAppRow, len(rows))
	for _, r := range rows {
		// Only accept rows with valid type_id
		if strings.TrimSpace(r.TypeID) == "" {
			continue
		}
		// URL validation: https only + host allowlist
		if r.InstallURL != "" && !isValidInstallURL(r.InstallURL) {
			logger.Warnf("agent_apps: rejected %s install_url %q (invalid)", r.TypeID, r.InstallURL)
			continue
		}
		r.TypeID = strings.TrimSpace(r.TypeID)
		newApps[r.TypeID] = r
	}

	o.apps = newApps
	logger.Infof("agent_apps: loaded %d remote entries", len(o.apps))
}

// Get returns the remote overlay for a type_id, or nil if not present.
func (o *AgentAppsOverlay) Get(typeID string) *AgentAppRow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	r, ok := o.apps[typeID]
	if !ok {
		return nil
	}
	return &r
}

// IsEnabled returns true if the type_id is enabled in the overlay.
// Returns true if no overlay exists (code default is enabled).
func (o *AgentAppsOverlay) IsEnabled(typeID string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	r, ok := o.apps[typeID]
	if !ok {
		return true // no overlay = code default
	}
	return r.Enabled
}

// GetSort returns the sort order for a type_id.
// Returns 0 if no overlay exists (code default).
func (o *AgentAppsOverlay) GetSort(typeID string) int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.apps[typeID].Sort
}

// All returns all overlay entries.
func (o *AgentAppsOverlay) All() []AgentAppRow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]AgentAppRow, 0, len(o.apps))
	for _, r := range o.apps {
		result = append(result, r)
	}
	return result
}

// --- DB persistence ---

// AgentAppsStore persists agent app overlay data in the database.
type AgentAppsStore struct {
	db      *sql.DB
	dialect database.Dialect
	mu      sync.RWMutex
}

// NewAgentAppsStore creates a new agent apps store.
func NewAgentAppsStore() (*AgentAppsStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &AgentAppsStore{db: mgr.GetDB(), dialect: dialect}, nil
}

func (s *AgentAppsStore) ph(n int) string {
	if s.dialect != nil {
		return s.dialect.Placeholder(n)
	}
	d := database.SQLiteDialect{}
	return d.Placeholder(n)
}

// GetAll returns all agent app rows from the database.
func (s *AgentAppsStore) GetAll() ([]AgentAppRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT type_id, display_name, description, vendor, category, enabled, sort, install_url, install_hint, meta_json FROM agent_apps`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AgentAppRow
	for rows.Next() {
		var r AgentAppRow
		var enabled int
		var metaStr string
		if err := rows.Scan(&r.TypeID, &r.DisplayName, &r.Description, &r.Vendor, &r.Category, &enabled, &r.Sort, &r.InstallURL, &r.InstallHint, &metaStr); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		r.MetaJSON = json.RawMessage(metaStr)
		result = append(result, r)
	}
	return result, rows.Err()
}

// SyncFromSnapshot applies a full-table-coverage agent_apps snapshot.
func (s *AgentAppsStore) SyncFromSnapshot(data []AgentAppRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(data) == 0 {
		logger.Warnf("agent_apps: empty snapshot, keeping existing data")
		return nil
	}

	// Validation: only known type_ids, URL checks
	var validated []AgentAppRow
	for _, r := range data {
		if strings.TrimSpace(r.TypeID) == "" {
			continue
		}
		if r.InstallURL != "" && !isValidInstallURL(r.InstallURL) {
			continue
		}
		r.TypeID = strings.TrimSpace(r.TypeID)
		validated = append(validated, r)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM agent_apps"); err != nil {
		return fmt.Errorf("clear agent_apps: %w", err)
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO agent_apps (type_id, display_name, description, vendor, category, enabled, sort, install_url, install_hint, meta_json, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12)))
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range validated {
		enabled := 0
		if r.Enabled {
			enabled = 1
		}
		metaStr := string(r.MetaJSON)
		if metaStr == "" {
			metaStr = "{}"
		}
		if _, err := stmt.Exec(r.TypeID, r.DisplayName, r.Description, r.Vendor, r.Category, enabled, r.Sort, r.InstallURL, r.InstallHint, metaStr, now, now); err != nil {
			return fmt.Errorf("insert agent_app %s: %w", r.TypeID, err)
		}
	}

	return tx.Commit()
}

// SnapshotData returns all agent app rows for Snapshot.Tables["agent_apps"].
func (s *AgentAppsStore) SnapshotData() ([]AgentAppRow, error) {
	return s.GetAll()
}

// --- URL validation ---

// allowedInstallURLHosts is the allowlist for install_url hosts.
var allowedInstallURLHosts = map[string]bool{
	"github.com":         true,
	"gitlab.com":         true,
	"codeberg.org":       true,
	"objects.githubusercontent.com": true,
	"registry.npmjs.org": true,
	"pypi.org":           true,
	"crates.io":          true,
	"marketplace.visualstudio.com": true,
	"chromewebstore.google.com":    true,
	"apps.apple.com":     true,
	// AI vendor official sites
	"anthropic.com":      true,
	"claude.ai":          true,
	"openai.com":         true,
	"google.com":         true,
	"gemini.google.com":  true,
	"tencent.com":        true,
	"codebuddy.tencent.com": true,
	"trae.ai":            true,
	"bytedance.com":      true,
}

// isValidInstallURL checks that the URL uses https and the host is in the allowlist.
func isValidInstallURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	// Must be https
	if u.Scheme != "https" {
		return false
	}
	// Check host allowlist (supports subdomains)
	host := strings.ToLower(u.Hostname())
	for allowed := range allowedInstallURLHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}
