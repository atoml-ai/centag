package tools

// IntersectAllowedTools 返回 toolNames 与 allowed 的交集（保持 toolNames 顺序）。
// 纯函数：空/部分/完全交集均可处理（任务8 / R02）。
func IntersectAllowedTools(toolNames, allowed []string) []string {
	if len(toolNames) == 0 || len(allowed) == 0 {
		return nil
	}
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	var result []string
	for _, t := range toolNames {
		if set[t] {
			result = append(result, t)
		}
	}
	return result
}
