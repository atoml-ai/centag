package configsync

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CheckResult is the outcome of a configuration consistency check (§3.2 Step 4).
type CheckResult struct {
	OK     bool
	Counts map[string]int
	Errors []string
}

// Err returns a non-nil summarised error when the check failed.
func (r CheckResult) Err() error {
	if r.OK {
		return nil
	}
	return fmt.Errorf("consistency check failed: %s", strings.Join(r.Errors, "; "))
}

func (r *CheckResult) fail(format string, args ...any) {
	r.OK = false
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

// ConsistencyChecker validates persisted snapshots before they are reported as
// successfully applied. It covers schema version, per-table decode/count and
// critical key value checks.
type ConsistencyChecker struct {
	// RequiredKeys are validated only when present in the system_config table.
	RequiredKeys []string
}

// NewConsistencyChecker builds a checker seeded with the built-in critical keys.
func NewConsistencyChecker() *ConsistencyChecker {
	return &ConsistencyChecker{
		RequiredKeys: []string{
			"billing.usd_to_cny",
			"scheduler.price_weight",
			"scheduler.performance_weight",
			"scheduler.quality_weight",
			"scheduler.latency_weight",
			"scheduler.privacy_weight",
			"scheduler.match_weight",
		},
	}
}

// VerifySnapshot checks a freshly fetched snapshot. Known table types are
// decoded and counted; critical keys are range-checked. Unknown tables are
// ignored (forward compatible).
func (c *ConsistencyChecker) VerifySnapshot(snap *Snapshot) CheckResult {
	res := CheckResult{OK: true, Counts: map[string]int{}}
	if snap == nil {
		res.fail("snapshot is nil")
		return res
	}
	if snap.Schema != snapshotSchema {
		res.fail("schema version %d (expected %d)", snap.Schema, snapshotSchema)
	}
	if snap.Tables == nil {
		return res
	}

	if raw, ok := snap.Tables[systemConfigTable]; ok && len(raw) > 0 {
		var rows []KVRow
		if err := json.Unmarshal(raw, &rows); err != nil {
			res.fail("system_config decode: %v", err)
		} else {
			res.Counts[systemConfigTable] = len(rows)
			c.checkKVRows(rows, &res)
		}
	}
	if raw, ok := snap.Tables["mitm_domains"]; ok && len(raw) > 0 {
		var rows []MITMDomainRow
		if err := json.Unmarshal(raw, &rows); err != nil {
			res.fail("mitm_domains decode: %v", err)
		} else {
			res.Counts["mitm_domains"] = len(rows)
		}
	}
	if raw, ok := snap.Tables["agent_apps"]; ok && len(raw) > 0 {
		var rows []AgentAppRow
		if err := json.Unmarshal(raw, &rows); err != nil {
			res.fail("agent_apps decode: %v", err)
		} else {
			res.Counts["agent_apps"] = len(rows)
		}
	}
	return res
}

func (c *ConsistencyChecker) checkKVRows(rows []KVRow, res *CheckResult) {
	seen := make(map[string]bool, len(rows))
	values := make(map[string]string, len(rows))
	for _, r := range rows {
		if strings.TrimSpace(r.Key) == "" {
			res.fail("system_config row with empty key")
			continue
		}
		if seen[r.Key] {
			res.fail("duplicate system_config key %q", r.Key)
		}
		seen[r.Key] = true
		values[r.Key] = r.Value
	}
	for _, k := range c.RequiredKeys {
		v, ok := values[k]
		if !ok {
			continue
		}
		if err := validateCriticalKey(k, v); err != nil {
			res.fail("key %q: %v", k, err)
		}
	}
}

// validateCriticalKey range-checks a known critical system_config value.
func validateCriticalKey(key, value string) error {
	switch key {
	case "billing.usd_to_cny":
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || f <= 0 {
			return fmt.Errorf("invalid exchange rate %q", value)
		}
	case "scheduler.price_weight", "scheduler.performance_weight", "scheduler.quality_weight",
		"scheduler.latency_weight", "scheduler.privacy_weight", "scheduler.match_weight":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 || n > 100 {
			return fmt.Errorf("weight out of range [0,100]: %q", value)
		}
	}
	return nil
}

// CheckLastGood performs a periodic health check on the last-good snapshot.
// A missing snapshot is healthy (nothing persisted yet).
func (c *ConsistencyChecker) CheckLastGood(stateDir string) CheckResult {
	snap, err := ReadSnapshot(stateDir)
	if err != nil {
		res := CheckResult{OK: false, Counts: map[string]int{}}
		res.fail("read last-good snapshot: %v", err)
		return res
	}
	if snap == nil {
		return CheckResult{OK: true, Counts: map[string]int{}}
	}
	return c.VerifySnapshot(snap)
}
