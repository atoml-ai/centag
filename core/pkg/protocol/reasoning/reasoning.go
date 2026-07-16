// Package reasoning 提供 Reasoning 归一化的纯函数。
//
// 各协议插件把原生 thinking 字段解析后，写入统一结构；
// backend 再映射为厂商方言。
package reasoning

import (
	"strings"
)

// effortLevels 定义支持的 effort 级别，从低到高。
var effortLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh"}

// effortAliases 定义 effort 别名映射。
var effortAliases = map[string]string{
	"off":    "none",
	"med":    "medium",
	"medium": "medium",
	"high":   "high",
	"low":    "low",
}

// effortBudgetMap 定义 effort 到 budget tokens 的默认映射。
var effortBudgetMap = map[string]int{
	"none":    0,
	"minimal": 1024,
	"low":     2048,
	"medium":  4096,
	"high":    8192,
	"xhigh":   16384,
}

// NormalizeEffort 标准化 effort 字符串。
// 支持别名（off→none, med→medium）和大小写不敏感。
// 返回标准化后的 effort 字符串，无效输入返回 "none"。
func NormalizeEffort(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "none"
	}

	// 检查别名
	if alias, ok := effortAliases[s]; ok {
		return alias
	}

	// 检查是否为有效级别
	for _, level := range effortLevels {
		if s == level {
			return s
		}
	}

	return "none"
}

// EffortToBudget 将 effort 字符串转换为 budget tokens。
// 无效 effort 返回 0。
func EffortToBudget(effort string) int {
	effort = NormalizeEffort(effort)
	if budget, ok := effortBudgetMap[effort]; ok {
		return budget
	}
	return 0
}

// BudgetToEffort 将 budget tokens 转换为 effort 字符串。
// 找不到匹配的级别时返回 "none"。
func BudgetToEffort(budget int) string {
	if budget <= 0 {
		return "none"
	}

	// 找到最接近且不超过的级别
	for i := len(effortLevels) - 1; i >= 0; i-- {
		level := effortLevels[i]
		if budget >= effortBudgetMap[level] {
			return level
		}
	}

	return "none"
}

// IsValidEffort 检查 effort 字符串是否有效。
func IsValidEffort(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}

	// 检查别名
	if _, ok := effortAliases[s]; ok {
		return true
	}

	// 检查是否为有效级别
	for _, level := range effortLevels {
		if s == level {
			return true
		}
	}
	return false
}

// EffortLevels 返回所有支持的 effort 级别。
func EffortLevels() []string {
	result := make([]string, len(effortLevels))
	copy(result, effortLevels)
	return result
}
