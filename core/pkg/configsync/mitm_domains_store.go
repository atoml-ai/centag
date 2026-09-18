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

// MITMDomainRow represents a single MITM domain entry.
type MITMDomainRow struct {
	Domain   string `json:"domain"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
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

// MITMSyncReason conveys why a MITM domain sync was triggered (§3.2 Step 3).
// It decides the empty-input policy so a missing/failed table never wipes the
// authoritative last-good configuration.
type MITMSyncReason string

const (
	// MITMSyncSnapshot: a valid table was present in the snapshot.
	MITMSyncSnapshot MITMSyncReason = "snapshot"
	// MITMSyncAbsent: the snapshot did not contain the table → keep last-good.
	MITMSyncAbsent MITMSyncReason = "absent"
	// MITMSyncFetchError: the provider fetch failed → keep last-good.
	MITMSyncFetchError MITMSyncReason = "fetch_error"
)

// MITMSyncResult reports the outcome of a MITM domain sync.
type MITMSyncResult struct {
	Reason       MITMSyncReason
	Applied      int
	Deleted      int
	Existing     int
	DeletionRate float64
	Rejected     bool
}

// SyncFromSnapshot applies a full-table-coverage MITM domains snapshot,
// treating an empty input as an explicit "delete all" (reason snapshot).
// Kept for callers that only have the snapshot path.
func (s *MITMDomainsStore) SyncFromSnapshot(data []MITMDomainRow) error {
	_, err := s.SyncFromSnapshotReason(data, MITMSyncSnapshot)
	return err
}

// SyncFromSnapshotReason applies a full-table MITM snapshot and explicitly
// distinguishes the three trigger reasons (§3.2 Step 3):
//
//	snapshot    — valid table present; an empty table means "delete all" (warn)
//	absent      — table missing from snapshot → keep last-good, never wipe
//	fetch_error — provider fetch failed → keep last-good, never wipe
//
// Protection on the snapshot path: format validation, dedup, count limits and
// a deletion circuit breaker (>50% and >10 enabled dropped) for non-empty
// updates. An explicit empty table intentionally bypasses the breaker (that is
// exactly the remote "no domains" signal) but is logged at warn level.
func (s *MITMDomainsStore) SyncFromSnapshotReason(data []MITMDomainRow, reason MITMSyncReason) (MITMSyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := MITMSyncResult{Reason: reason}

	if reason == MITMSyncAbsent || reason == MITMSyncFetchError {
		res.Rejected = true
		logger.Warnf("mitm_domains: sync reason=%s, keeping last-good table", reason)
		return res, nil
	}

	// Count existing enabled domains up front for rate + reporting.
	var existingEnabled int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM mitm_domains WHERE enabled = 1").Scan(&existingEnabled)
	var existingTotal int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM mitm_domains").Scan(&existingTotal)

	explicitEmpty := len(data) == 0

	if len(data) > 500 {
		logger.Warnf("mitm_domains: snapshot too large (%d domains), capping at 500", len(data))
		data = data[:500]
	}

	validated := validateMITMDomains(data)
	// A non-empty table that validates to nothing is corruption, not an
	// intentional wipe → reject and keep last-good.
	if !explicitEmpty && len(validated) == 0 {
		res.Rejected = true
		logger.Warnf("mitm_domains: snapshot had %d rows but none valid, keeping last-good", len(data))
		return res, fmt.Errorf("mitm_domains: all %d rows invalid", len(data))
	}

	newEnabledCount := 0
	for _, r := range validated {
		if r.Enabled {
			newEnabledCount++
		}
	}
	if existingEnabled > 0 {
		dropped := existingEnabled - newEnabledCount
		if dropped < 0 {
			dropped = 0
		}
		res.DeletionRate = float64(dropped) / float64(existingEnabled)
		if !explicitEmpty && dropped > 10 && res.DeletionRate > 0.5 {
			res.Rejected = true
			logger.Warnf("mitm_domains: deletion circuit breaker (existing=%d, new_enabled=%d, dropped=%d, rate=%.2f), rejecting snapshot",
				existingEnabled, newEnabledCount, dropped, res.DeletionRate)
			return res, fmt.Errorf("mitm_domains: deletion circuit breaker: dropped %d of %d (>50%% and >10)", dropped, existingEnabled)
		}
	}

	if explicitEmpty {
		logger.Warnf("mitm_domains: remote explicitly empty → deleting all %d domain rows", existingTotal)
	} else if res.DeletionRate > 0.5 {
		logger.Warnf("mitm_domains: large deletion %.0f%% (%d of %d enabled) accepted", res.DeletionRate*100, existingEnabled-newEnabledCount, existingEnabled)
	}

	// Full table coverage: DELETE all, then INSERT new set.
	tx, err := s.db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM mitm_domains"); err != nil {
		return res, fmt.Errorf("clear mitm_domains: %w", err)
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO mitm_domains (domain, category, enabled, remark, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6)))
	if err != nil {
		return res, err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range validated {
		enabled := 0
		if r.Enabled {
			enabled = 1
		}
		if _, err := stmt.Exec(r.Domain, r.Category, enabled, r.Remark, now, now); err != nil {
			return res, fmt.Errorf("insert domain %s: %w", r.Domain, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}

	res.Applied = len(validated)
	res.Existing = existingTotal
	res.Deleted = existingTotal - len(validated)
	if res.Deleted < 0 {
		res.Deleted = 0
	}
	logger.Infof("mitm_domains: synced reason=%s applied=%d deleted=%d existing=%d deletion_rate=%.2f",
		reason, res.Applied, res.Deleted, res.Existing, res.DeletionRate)
	return res, nil
}

// validateMITMDomains normalizes, dedups and format-checks rows.
func validateMITMDomains(data []MITMDomainRow) []MITMDomainRow {
	seen := make(map[string]bool, len(data))
	var validated []MITMDomainRow
	for _, r := range data {
		domain := strings.TrimSpace(strings.ToLower(r.Domain))
		if domain == "" {
			continue
		}
		if !strings.Contains(domain, ".") {
			logger.Warnf("mitm_domains: invalid domain %q (no dot), skipped", domain)
			continue
		}
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
	return validated
}

// EnsureSeeded inserts seed domains when the table is empty. When seed is
// omitted, the built-in defaults are used. This restores the fresh-install
// seed that the old fallback provided, without re-introducing the "missing
// table wipes existing config" bug.
func (s *MITMDomainsStore) EnsureSeeded(seed ...MITMDomainRow) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM mitm_domains").Scan(&count); err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, nil
	}

	if len(seed) == 0 {
		seed = DefaultMITMDomains()
	}
	rows := validateMITMDomains(seed)
	if len(rows) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO mitm_domains (domain, category, enabled, remark, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6)))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range rows {
		enabled := 0
		if r.Enabled {
			enabled = 1
		}
		if _, err := stmt.Exec(r.Domain, r.Category, enabled, r.Remark, now, now); err != nil {
			return 0, fmt.Errorf("seed domain %s: %w", r.Domain, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	logger.Infof("mitm_domains: seeded %d built-in domains (fresh install)", len(rows))
	return len(rows), nil
}

// ResolveMITMDomains extracts the MITM table from a snapshot and classifies the
// trigger reason (§3.2 Step 3): key present → snapshot (an empty result is an
// explicit "delete all"); key absent or undecodable → absent (keep last-good).
func ResolveMITMDomains(snap *Snapshot) ([]MITMDomainRow, MITMSyncReason) {
	if snap == nil || snap.Tables == nil {
		return nil, MITMSyncAbsent
	}
	data, ok := snap.Tables["mitm_domains"]
	if !ok {
		return nil, MITMSyncAbsent
	}
	rows := []MITMDomainRow{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, MITMSyncAbsent
		}
	}
	return rows, MITMSyncSnapshot
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
