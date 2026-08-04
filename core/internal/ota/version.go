// Package ota implements online system-update checks against public GitHub Releases.
package ota

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizeVersion strips a leading "v"/"V" and surrounding space.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return strings.TrimSpace(v[1:])
	}
	return v
}

// CompareVersions compares two semver-ish versions (major.minor.patch[+prerelease]).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Non-numeric / empty segments are treated as 0; a bare "dev" is always older than a release.
func CompareVersions(a, b string) int {
	na := NormalizeVersion(a)
	nb := NormalizeVersion(b)
	if na == nb {
		return 0
	}
	if na == "" || strings.EqualFold(na, "dev") {
		if nb == "" || strings.EqualFold(nb, "dev") {
			return 0
		}
		return -1
	}
	if nb == "" || strings.EqualFold(nb, "dev") {
		return 1
	}

	ap, aPre := splitPrerelease(na)
	bp, bPre := splitPrerelease(nb)
	aParts := parseNumericParts(ap)
	bParts := parseNumericParts(bp)
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	// Release without prerelease is newer than same numbers with prerelease.
	if aPre == "" && bPre != "" {
		return 1
	}
	if aPre != "" && bPre == "" {
		return -1
	}
	return strings.Compare(aPre, bPre)
}

func splitPrerelease(v string) (core, pre string) {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func parseNumericParts(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

// IsUpgrade reports whether remote is strictly newer than current.
func IsUpgrade(current, remote string) bool {
	return CompareVersions(current, remote) < 0
}

// FormatTag returns a GitHub-style tag (vX.Y.Z).
func FormatTag(version string) string {
	v := NormalizeVersion(version)
	if v == "" {
		return ""
	}
	return "v" + v
}

// AssetName builds the canonical OTA package filename.
// Example: update-package-centag-team-0.2.8-linux-amd64.tar.gz
func AssetName(edition, version, goos, goarch string) string {
	ed := strings.TrimSpace(strings.ToLower(edition))
	if ed == "" {
		ed = "team"
	}
	ver := NormalizeVersion(version)
	return fmt.Sprintf("update-package-centag-%s-%s-%s-%s.tar.gz", ed, ver, goos, goarch)
}
