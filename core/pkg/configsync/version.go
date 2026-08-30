package configsync

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareVersions compares two semver strings (MAJOR.MINOR.PATCH).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Non-semver strings are compared lexicographically.
// "dev" and "" are treated as less than any numeric version.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
		// Treat "dev" and empty as less than any numeric version.
	}
	aIsDev := a == "" || a == "dev"
	bIsDev := b == "" || b == "dev"
	if aIsDev && bIsDev {
		return 0
	}
	if aIsDev {
		return -1
	}
	if bIsDev {
		return 1
	}
	aParts := parseVersion(a)
	bParts := parseVersion(b)
	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

// parseVersion splits a semver string into [MAJOR, MINOR, PATCH].
// Invalid parts default to 0.
func parseVersion(v string) [3]int {
	var result [3]int
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			result[i] = n
		}
	}
	return result
}

// ValidateSemver checks that a version string is a valid semver (MAJOR.MINOR.PATCH).
func ValidateSemver(v string) error {
	if v == "" || v == "dev" {
		return nil
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return fmt.Errorf("expected MAJOR.MINOR.PATCH, got %q", v)
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return fmt.Errorf("non-numeric version part %q in %q", p, v)
		}
	}
	return nil
}

// MatchVersion checks if a row matches the given client query.
// It checks edition, channel (for release.* rows), and version range.
func MatchVersion(row *Row, q Query) bool {
	if !row.Enabled {
		return false
	}
	// Edition check.
	if row.Edition != "all" && row.Edition != q.Edition {
		return false
	}
	// Channel check: only for release.* rows.
	if strings.HasPrefix(row.Key, "release.") && row.Channel != "" && row.Channel != q.Channel {
		return false
	}
	// Version range check.
	if q.Version != "" && q.Version != "dev" {
		if row.MinVersion != "" && CompareVersions(q.Version, row.MinVersion) < 0 {
			return false
		}
		if row.MaxVersion != "" && CompareVersions(q.Version, row.MaxVersion) > 0 {
			return false
		}
	}
	return true
}

// SelectBestRow selects the best matching row for a given config_key from
// a set of matching rows. Priority wins; ties broken by UpdatedAt (newest).
func SelectBestRow(rows []Row) *Row {
	if len(rows) == 0 {
		return nil
	}
	best := &rows[0]
	for i := 1; i < len(rows); i++ {
		r := &rows[i]
		if r.Priority > best.Priority {
			best = r
		} else if r.Priority == best.Priority && r.UpdatedAt.After(best.UpdatedAt) {
			best = r
		}
	}
	return best
}
