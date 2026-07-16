package registry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version 语义化版本
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string // 预发布版本，如 "alpha", "beta", "rc"
	Build string // 构建元数据
}

// ParseVersion 解析版本字符串
func ParseVersion(v string) (*Version, error) {
	// 移除前缀 'v' 或 'V'
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	
	// 正则匹配：主版本.次版本.修订版本-预发布+构建
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z-\.]+))?(?:\+([0-9A-Za-z-\.]+))?$`)
	matches := re.FindStringSubmatch(v)
	
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}
	
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	if matches[2] == "" {
		minor = 0
	}
	patch, _ := strconv.Atoi(matches[3])
	if matches[3] == "" {
		patch = 0
	}
	
	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Pre:   matches[4],
		Build: matches[5],
	}, nil
}

// String 返回版本字符串
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare 比较版本
// 返回值: -1 表示 v < other, 0 表示相等, 1 表示 v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	
	// 比较预发布版本
	if v.Pre != "" && other.Pre == "" {
		return -1 // 有预发布版本 < 无预发布版本
	}
	if v.Pre == "" && other.Pre != "" {
		return 1
	}
	if v.Pre != "" && other.Pre != "" {
		return comparePreRelease(v.Pre, other.Pre)
	}
	
	return 0
}

// comparePreRelease 比较预发布版本
func comparePreRelease(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aNum, aErr := strconv.Atoi(aParts[i])
		bNum, bErr := strconv.Atoi(bParts[i])
		
		// 都是数字，按数字比较
		if aErr == nil && bErr == nil {
			if aNum < bNum {
				return -1
			}
			if aNum > bNum {
				return 1
			}
			continue
		}
		
		// 按字符串比较
		cmp := strings.Compare(aParts[i], bParts[i])
		if cmp != 0 {
			return cmp
		}
	}
	
	// 较短的预发布版本较小
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	
	return 0
}

// Satisfies 检查版本是否满足约束
func (v *Version) Satisfies(constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	
	// 精确版本
	if !strings.ContainsAny(constraint, "^~><=") {
		other, err := ParseVersion(constraint)
		if err != nil {
			return false, err
		}
		return v.Compare(other) == 0, nil
	}
	
	// 解析约束
	switch {
	case strings.HasPrefix(constraint, "^"):
		// ^1.2.3 表示 >=1.2.3 <2.0.0
		base, err := ParseVersion(constraint[1:])
		if err != nil {
			return false, err
		}
		if v.Compare(base) < 0 {
			return false, nil
		}
		upper := &Version{Major: base.Major + 1, Minor: 0, Patch: 0}
		return v.Compare(upper) < 0, nil
		
	case strings.HasPrefix(constraint, "~"):
		// ~1.2.3 表示 >=1.2.3 <1.3.0
		base, err := ParseVersion(constraint[1:])
		if err != nil {
			return false, err
		}
		if v.Compare(base) < 0 {
			return false, nil
		}
		upper := &Version{Major: base.Major, Minor: base.Minor + 1, Patch: 0}
		return v.Compare(upper) < 0, nil
		
	case strings.HasPrefix(constraint, ">="):
		base, err := ParseVersion(constraint[2:])
		if err != nil {
			return false, err
		}
		return v.Compare(base) >= 0, nil
		
	case strings.HasPrefix(constraint, ">"):
		base, err := ParseVersion(constraint[1:])
		if err != nil {
			return false, err
		}
		return v.Compare(base) > 0, nil
		
	case strings.HasPrefix(constraint, "<="):
		base, err := ParseVersion(constraint[2:])
		if err != nil {
			return false, err
		}
		return v.Compare(base) <= 0, nil
		
	case strings.HasPrefix(constraint, "<"):
		base, err := ParseVersion(constraint[1:])
		if err != nil {
			return false, err
		}
		return v.Compare(base) < 0, nil
	}
	
	return false, fmt.Errorf("unsupported constraint: %s", constraint)
}

// LatestVersion 从版本列表中找出最新版本
func LatestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}
	
	var latest *Version
	var latestStr string
	
	for _, v := range versions {
		parsed, err := ParseVersion(v)
		if err != nil {
			continue // 跳过无效版本
		}
		
		if latest == nil || parsed.Compare(latest) > 0 {
			latest = parsed
			latestStr = v
		}
	}
	
	if latest == nil {
		return "", fmt.Errorf("no valid versions found")
	}
	
	return latestStr, nil
}

// CompatibleVersions 找出满足约束的版本
func CompatibleVersions(versions []string, constraint string) ([]string, error) {
	var compatible []string
	
	for _, v := range versions {
		parsed, err := ParseVersion(v)
		if err != nil {
			continue
		}
		
		satisfies, err := parsed.Satisfies(constraint)
		if err != nil {
			continue
		}
		
		if satisfies {
			compatible = append(compatible, v)
		}
	}
	
	return compatible, nil
}
