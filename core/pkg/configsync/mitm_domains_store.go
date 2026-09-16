package configsync

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// MITMDomainRow represents a single MITM domain entry.
type MITMDomainRow struct {
	Domain    string `json:"domain"`
	Category  string `json:"category"`
	Enabled   bool   `json:"enabled"`
	Remark    string `json:"remark"`
}

// MITMDomainsStore manages the MITM domain whitelist in the database.
type MITMDomainsStore struct {
	db      *sql.DB
	dialect database.Dialect
	mu      sync.RWMutex
}

// NewMITMDomainsStore creates a new MITM domains store.
func NewMITMDomainsStore() (*MITMDomainsStore, error) {
	mgr := database.Get()
	if mgr == nil || mgr.GetDB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var dialect database.Dialect = &database.SQLiteDialect{}
	if mgr.DriverName() == "postgresql" {
		dialect = &database.PostgreSQLDialect{}
	}
	return &MITMDomainsStore{db: mgr.GetDB(), dialect: dialect}, nil
}

func (s *MITMDomainsStore) ph(n int) string {
	if s.dialect != nil {
		return s.dialect.Placeholder(n)
	}
	d := database.SQLiteDialect{}
	return d.Placeholder(n)
}

// GetAll returns all MITM domain rows.
func (s *MITMDomainsStore) GetAll() ([]MITMDomainRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT domain, category, enabled, remark FROM mitm_domains")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MITMDomainRow
	for rows.Next() {
		var r MITMDomainRow
		var enabled int
		if err := rows.Scan(&r.Domain, &r.Category, &enabled, &r.Remark); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetEnabledDomains returns only enabled domains as a flat list.
func (s *MITMDomainsStore) GetEnabledDomains() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT domain FROM mitm_domains WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

// SyncFromSnapshot applies a full-table-coverage MITM domains snapshot.
// This replaces all existing domains (not union/merge).
// Protection: format validation, dedup, count limits, deletion circuit breaker.
func (s *MITMDomainsStore) SyncFromSnapshot(data []MITMDomainRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Protection: count limits
	if len(data) == 0 {
		logger.Warnf("mitm_domains: empty snapshot rejected, keeping existing data")
		return nil
	}
	if len(data) > 500 {
		logger.Warnf("mitm_domains: snapshot too large (%d domains), capping at 500", len(data))
		data = data[:500]
	}

	// Protection: dedup and format validation
	seen := make(map[string]bool, len(data))
	var validated []MITMDomainRow
	for _, r := range data {
		domain := strings.TrimSpace(strings.ToLower(r.Domain))
		if domain == "" {
			continue
		}
		// Format: must contain at least one dot
		if !strings.Contains(domain, ".") {
			logger.Warnf("mitm_domains: invalid domain %q (no dot), skipped", domain)
			continue
		}
		// Dedup
		if seen[domain] {
			continue
		}
		seen[domain] = true
		validated = append(validated, MITMDomainRow{
			Domain:   domain,
			Category: r.Category,
			Enabled:  r.Enabled,
			Remark:   r.Remark,
		})
	}

	// Protection: deletion circuit breaker
	// Count existing enabled domains
	var existingCount int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM mitm_domains WHERE enabled = 1").Scan(&existingCount)
	newEnabledCount := 0
	for _, r := range validated {
		if r.Enabled {
			newEnabledCount++
		}
	}
	// If deletion rate > 50% AND absolute drop > 10, reject
	if existingCount > 0 {
		dropped := existingCount - newEnabledCount
		if dropped > 10 && float64(dropped)/float64(existingCount) > 0.5 {
			logger.Warnf("mitm_domains: deletion circuit breaker triggered (existing=%d, new_enabled=%d, dropped=%d), rejecting snapshot", existingCount, newEnabledCount, dropped)
			return fmt.Errorf("mitm_domains: deletion circuit breaker: dropped %d of %d domains (>50%% and >10)", dropped, existingCount)
		}
	}

	// Full table coverage: DELETE all, then INSERT new set
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM mitm_domains"); err != nil {
		return fmt.Errorf("clear mitm_domains: %w", err)
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO mitm_domains (domain, category, enabled, remark, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6)))
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
		if _, err := stmt.Exec(r.Domain, r.Category, enabled, r.Remark, now, now); err != nil {
			return fmt.Errorf("insert domain %s: %w", r.Domain, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Infof("mitm_domains: synced %d domains (full table coverage, %d existing dropped)", len(validated), existingCount)
	return nil
}

// SnapshotData returns all MITM domains as a flat list for Snapshot.Tables["mitm_domains"].
func (s *MITMDomainsStore) SnapshotData() ([]MITMDomainRow, error) {
	return s.GetAll()
}

// DefaultMITMDomains returns the built-in MITM domain list as the final fallback.
// This is the same list as DefaultMITMDomains() in mitm_default_domains.go,
// but structured as MITMDomainRow for the sync framework.
func DefaultMITMDomains() []MITMDomainRow {
	// Delegate to the existing default list
	domains := defaultMITMDomainList()
	rows := make([]MITMDomainRow, 0, len(domains))
	for _, d := range domains {
		rows = append(rows, MITMDomainRow{
			Domain:   d,
			Category: "default",
			Enabled:  true,
		})
	}
	return rows
}

// defaultMITMDomainList returns the raw domain strings from the existing defaults.
// This is a separate function to avoid import cycles with the mitm package.
func defaultMITMDomainList() []string {
	return []string{
		// China majors
		"api.deepseek.com", "api.kimi.com", "api.siliconflow.cn",
		"dashscope.aliyuncs.com", "hunyuan.tencentcloudapi.com",
		"qianfan.baidubce.com",
		// Global
		"api.openai.com", "api.anthropic.com", "api.mistral.ai",
		"api.x.ai", "generativelanguage.googleapis.com",
		// Cloud
		"openai.azure.com", "services.ai.azure.com",
		// Inference
		"api.together.xyz", "api.fireworks.ai", "api.deepinfra.com",
		// Aggregators
		"openrouter.ai", "api.portkey.ai",
	}
}
