package backend

import (
	"log"
	"math"
	"sort"
	"strings"
	"sync"
)

// ModelMatcher 模型匹配器
type ModelMatcher struct {
	config          ModelMatchingConfig
	debug           bool                // 是否启用调试日志
	normalizeCache  map[string]string   // 模型名称规范化缓存
	capacityCache   map[string]float64  // 模型参数量缓存
	cacheMu         sync.RWMutex        // 缓存读写锁
}

// NewModelMatcher 创建模型匹配器
func NewModelMatcher(config ModelMatchingConfig) *ModelMatcher {
	if config.Strategy == "" {
		config = DefaultModelMatchingConfig()
	}
	return &ModelMatcher{
		config:         config,
		debug:          false,
		normalizeCache: make(map[string]string, 100),    // 预分配容量
		capacityCache:  make(map[string]float64, 100),   // 预分配容量
	}
}

// SetDebug 设置调试模式
func (m *ModelMatcher) SetDebug(debug bool) {
	m.debug = debug
}

// logDebug 输出调试日志
func (m *ModelMatcher) logDebug(format string, args ...interface{}) {
	if m.debug {
		log.Printf(format, args...)
	}
}

// logInfo 输出信息日志（总是输出）
func (m *ModelMatcher) logInfo(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// getNormalizedName 获取规范化的模型名（带缓存）
func (m *ModelMatcher) getNormalizedName(modelName string) string {
	// 先尝试从缓存读取
	m.cacheMu.RLock()
	if normalized, ok := m.normalizeCache[modelName]; ok {
		m.cacheMu.RUnlock()
		return normalized
	}
	m.cacheMu.RUnlock()

	// 缓存未命中，执行规范化
	normalized := NormalizeModelName(modelName)

	// 写入缓存
	m.cacheMu.Lock()
	if len(m.normalizeCache) < 1000 { // 限制缓存大小
		m.normalizeCache[modelName] = normalized
	}
	m.cacheMu.Unlock()

	return normalized
}

// getModelCapacity 获取模型参数量（带缓存）
func (m *ModelMatcher) getModelCapacity(modelName string) float64 {
	// 先尝试从缓存读取
	m.cacheMu.RLock()
	if capacity, ok := m.capacityCache[modelName]; ok {
		m.cacheMu.RUnlock()
		return capacity
	}
	m.cacheMu.RUnlock()

	// 缓存未命中，查询参数量
	capacity := GetModelCapacity(modelName)

	// 写入缓存
	m.cacheMu.Lock()
	if len(m.capacityCache) < 1000 { // 限制缓存大小
		m.capacityCache[modelName] = capacity
	}
	m.cacheMu.Unlock()

	return capacity
}

// Match 在给定后端配置中查找匹配的模型
func (m *ModelMatcher) Match(
	requestedModel string,
	backends []*BackendConfig,
) *ModelMatchResult {
	m.logDebug("[ModelMatcher] Start matching for requested model: %s, strategy: %s",
		requestedModel, m.config.Strategy)

	// 遍历所有后端，查找匹配的模型
	var candidates []*ModelMatchResult
	checkedCount := 0

	for _, backend := range backends {
		if !backend.Enabled {
			m.logDebug("[ModelMatcher] Backend %s (%s) is disabled, skipping",
				backend.ID, backend.Name)
			continue
		}

		m.logDebug("[ModelMatcher] Checking backend: %s (%s), models: %d",
			backend.ID, backend.Name, len(backend.SupportedModels))

		for _, mapping := range backend.SupportedModels {
			checkedCount++
			m.logDebug("[ModelMatcher] Checking mapping: requested=%s, actual=%s",
				mapping.RequestedModel, mapping.ActualModel)

			if result := m.matchSingleModel(requestedModel, backend, mapping); result != nil {
				m.logDebug("[ModelMatcher] Found candidate: backend=%s, actual=%s, score=%.3f, exact=%v",
					backend.ID, result.ActualModel, result.CompatibilityScore, result.IsExact)
				candidates = append(candidates, result)
			}
		}
	}

	m.logDebug("[ModelMatcher] Checked %d model mappings across %d backends, found %d candidates",
		checkedCount, len(backends), len(candidates))

	// 如果没有候选，返回 nil
	if len(candidates) == 0 {
		m.logDebug("[ModelMatcher] No matching candidates found for model: %s", requestedModel)
		return nil
	}

	// 选择最佳匹配
	bestMatch := m.selectBestMatch(candidates)
	m.logInfo("[ModelMatcher] Selected best match: backend=%s, model=%s, score=%.3f, exact=%v",
		bestMatch.BackendID, bestMatch.ActualModel, bestMatch.CompatibilityScore, bestMatch.IsExact)

	return bestMatch
}

// matchSingleModel 匹配单个模型映射
func (m *ModelMatcher) matchSingleModel(
	requestedModel string,
	backend *BackendConfig,
	mapping ModelMapping,
) *ModelMatchResult {
	// 使用缓存的规范化方法
	normalizedRequested := m.getNormalizedName(requestedModel)
	normalizedActual := m.getNormalizedName(mapping.RequestedModel)

	m.logDebug("[ModelMatcher] Matching models: requested=%s (normalized=%s) vs actual=%s (normalized=%s)",
		requestedModel, normalizedRequested, mapping.RequestedModel, normalizedActual)

	// 计算匹配度
	var score float64
	var isExact bool
	var details MatchDetails

	switch m.config.Strategy {
	case StrategyExact:
		score, isExact, details = m.exactMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] ExactMatch strategy: score=%.3f, exact=%v", score, isExact)
	case StrategyFamily:
		score, isExact, details = m.familyMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] FamilyMatch strategy: score=%.3f, exact=%v, nameSim=%.3f, capacity=%.3f, family=%.3f",
			score, isExact, details.NameSimilarity, details.CapacityMatch, details.FamilyMatch)
	case StrategyCapacity:
		score, isExact, details = m.capacityMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] CapacityMatch strategy: score=%.3f, exact=%v, nameSim=%.3f, capacity=%.3f, family=%.3f",
			score, isExact, details.NameSimilarity, details.CapacityMatch, details.FamilyMatch)
	case StrategyHybrid:
		score, isExact, details = m.hybridMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] HybridMatch strategy: score=%.3f, exact=%v, nameSim=%.3f, capacity=%.3f, family=%.3f",
			score, isExact, details.NameSimilarity, details.CapacityMatch, details.FamilyMatch)
	case StrategyCustom:
		// 使用预配置的兼容性评分
		score = mapping.CompatibilityScore
		isExact = mapping.IsExact
		_, _, details = m.hybridMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] Custom strategy: score=%.3f (pre-configured), exact=%v", score, isExact)
	default:
		score, isExact, details = m.hybridMatchScore(normalizedRequested, normalizedActual)
		m.logDebug("[ModelMatcher] Default (Hybrid) strategy: score=%.3f, exact=%v", score, isExact)
	}

	// 检查是否满足最小兼容性阈值
	minCompat := m.config.GetMinCompatibility()
	m.logDebug("[ModelMatcher] Checking compatibility threshold: score=%.3f >= threshold=%.3f", score, minCompat)
	if score < minCompat {
		m.logDebug("[ModelMatcher] Score %.3f below threshold %.3f, rejecting", score, minCompat)
		return nil
	}

	// 检查是否允许转换
	allowConversion := m.config.AllowConversion()
	m.logDebug("[ModelMatcher] Checking model conversion: isExact=%v, allowConversion=%v", isExact, allowConversion)
	if !isExact && !allowConversion {
		m.logDebug("[ModelMatcher] Non-exact match but conversion disabled, rejecting")
		return nil
	}

	m.logDebug("[ModelMatcher] Match accepted: score=%.3f, isExact=%v", score, isExact)
	return &ModelMatchResult{
		BackendID:          backend.ID,
		BackendName:        backend.Name,
		RequestedModel:     requestedModel,
		ActualModel:        mapping.ActualModel,
		IsExact:            isExact,
		CompatibilityScore: score,
		Strategy:           m.config.Strategy,
		Details:            details,
	}
}

// exactMatchScore 精确匹配评分
func (m *ModelMatcher) exactMatchScore(requested, actual string) (float64, bool, MatchDetails) {
	if requested == actual {
		return 1.0, true, MatchDetails{
			NameSimilarity:  1.0,
			CapacityMatch:   1.0,
			FamilyMatch:     1.0,
		}
	}
	return 0.0, false, MatchDetails{}
}

// familyMatchScore 家族匹配评分
func (m *ModelMatcher) familyMatchScore(requested, actual string) (float64, bool, MatchDetails) {
	// 先检查精确匹配
	if requested == actual {
		return 1.0, true, MatchDetails{
			NameSimilarity:  1.0,
			CapacityMatch:   1.0,
			FamilyMatch:     1.0,
		}
	}
	
	// 检查家族匹配
	isSameFamily := IsSameFamily(requested, actual)
	familyScore := 0.0
	if isSameFamily {
		familyScore = 1.0
	}
	
	// 计算名称相似度
	nameSim := m.calculateNameSimilarity(requested, actual)
	
	// 计算综合评分
	score := familyScore*0.7 + nameSim*0.3
	
	return score, false, MatchDetails{
		NameSimilarity: nameSim,
		CapacityMatch:  m.calculateCapacityMatch(requested, actual),
		FamilyMatch:   familyScore,
	}
}

// capacityMatchScore 参数量匹配评分
func (m *ModelMatcher) capacityMatchScore(requested, actual string) (float64, bool, MatchDetails) {
	// 先检查精确匹配
	if requested == actual {
		return 1.0, true, MatchDetails{
			NameSimilarity:  1.0,
			CapacityMatch:   1.0,
			FamilyMatch:     1.0,
		}
	}
	
	// 计算参数量匹配度
	capacityScore := m.calculateCapacityMatch(requested, actual)
	
	// 计算名称相似度
	nameSim := m.calculateNameSimilarity(requested, actual)
	
	// 计算家族匹配度
	familyScore := 0.0
	if IsSameFamily(requested, actual) {
		familyScore = 1.0
	}
	
	// 计算综合评分
	score := capacityScore*0.6 + nameSim*0.3 + familyScore*0.1
	
	return score, false, MatchDetails{
		NameSimilarity: nameSim,
		CapacityMatch:  capacityScore,
		FamilyMatch:   familyScore,
	}
}

// hybridMatchScore 混合匹配评分
func (m *ModelMatcher) hybridMatchScore(requested, actual string) (float64, bool, MatchDetails) {
	// 先检查精确匹配
	if requested == actual {
		return 1.0, true, MatchDetails{
			NameSimilarity:  1.0,
			CapacityMatch:   1.0,
			FamilyMatch:     1.0,
		}
	}
	
	// 计算各维度评分
	nameSim := m.calculateNameSimilarity(requested, actual)
	capacityMatch := m.calculateCapacityMatch(requested, actual)
	familyScore := 0.0
	if IsSameFamily(requested, actual) {
		familyScore = 1.0
	}
	
	// 使用配置的权重计算综合评分（三维度之和应为1.0，不足1.0时等比归一化）
	weights := m.config.HybridWeights
	total := weights.NameSimilarity + weights.CapacityMatch + weights.FamilyMatch
	if total <= 0 {
		total = 1.0
	}
	score := (nameSim*weights.NameSimilarity +
		capacityMatch*weights.CapacityMatch +
		familyScore*weights.FamilyMatch) / total
	
	return score, false, MatchDetails{
		NameSimilarity: nameSim,
		CapacityMatch:  capacityMatch,
		FamilyMatch:   familyScore,
	}
}

// calculateNameSimilarity 计算名称相似度（基于编辑距离）
func (m *ModelMatcher) calculateNameSimilarity(requested, actual string) float64 {
	// 相同返回1
	if requested == actual {
		return 1.0
	}

	// 计算编辑距离(使用优化的版本)
	distance := levenshteinDistanceOptimized(requested, actual)
	maxLen := max(len(requested), len(actual))

	if maxLen == 0 {
		return 0.0
	}

	// 转换为相似度（0-1）
	similarity := 1.0 - float64(distance)/float64(maxLen)

	m.logDebug("[ModelMatcher] NameSimilarity: requested=%s, actual=%s, distance=%d, maxLen=%d, baseSimilarity=%.3f",
		requested, actual, distance, maxLen, similarity)

	// 如果包含相同的族或前缀，给予额外加分
	if len(actual) >= 3 && strings.HasPrefix(requested, actual[:3]) ||
		len(requested) >= 3 && strings.HasPrefix(actual, requested[:3]) {
		similarity += 0.2
		m.logDebug("[ModelMatcher] Common prefix detected, added bonus: newSimilarity=%.3f", similarity)
	}

	final := math.Min(similarity, 1.0)
	m.logDebug("[ModelMatcher] Final nameSimilarity=%.3f", final)
	return final
}

// calculateCapacityMatch 计算参数量匹配度
func (m *ModelMatcher) calculateCapacityMatch(requested, actual string) float64 {
	// 使用缓存的方法获取参数量
	reqCapacity := m.getModelCapacity(requested)
	actCapacity := m.getModelCapacity(actual)

	m.logDebug("[ModelMatcher] CapacityMatch: requested=%s (%.2fB), actual=%s (%.2fB)",
		requested, reqCapacity, actual, actCapacity)

	// 如果无法获取参数量，返回中等评分
	if reqCapacity == 0 || actCapacity == 0 {
		m.logDebug("[ModelMatcher] Unknown capacity, returning default score 0.5")
		return 0.5
	}

	// 计算差异比例
	diff := math.Abs(reqCapacity - actCapacity)
	maxCapacity := math.Max(reqCapacity, actCapacity)
	ratio := diff / maxCapacity

	// 根据容忍度评分
	tolerance := m.config.CapacityTolerance
	m.logDebug("[ModelMatcher] Capacity diff=%.2fB, ratio=%.3f, tolerance=%.3f",
		diff, ratio, tolerance)

	if ratio <= tolerance {
		// 在容忍度范围内，评分随差异减小而增加
		score := 1.0 - (ratio / tolerance) * 0.2
		m.logDebug("[ModelMatcher] Within tolerance, score=%.3f", score)
		return score
	}

	// 超出容忍度，评分随差异增加而降低
	if ratio > 0.5 {
		m.logDebug("[ModelMatcher] Exceeds 0.5 ratio, score=0.0")
		return 0.0
	}
	score := 0.3 * (1.0 - (ratio - tolerance)/(0.5 - tolerance))
	m.logDebug("[ModelMatcher] Outside tolerance, score=%.3f", score)
	return score
}

// selectBestMatch 从候选结果中选择最佳匹配
func (m *ModelMatcher) selectBestMatch(candidates []*ModelMatchResult) *ModelMatchResult {
	if len(candidates) == 0 {
		return nil
	}

	m.logDebug("[ModelMatcher] Selecting best match from %d candidates", len(candidates))

	// 如果配置要求优先精确匹配，优先返回精确匹配
	if m.config.PreferExact() {
		m.logDebug("[ModelMatcher] PreferExact mode enabled, looking for exact match first")
		for _, candidate := range candidates {
			if candidate.IsExact {
				m.logDebug("[ModelMatcher] Found exact match candidate: backend=%s, score=%.3f",
					candidate.BackendID, candidate.CompatibilityScore)
				return candidate
			}
		}
		m.logDebug("[ModelMatcher] No exact match found, falling back to highest score")
	}

	// 按兼容性评分排序
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CompatibilityScore != candidates[j].CompatibilityScore {
			return candidates[i].CompatibilityScore > candidates[j].CompatibilityScore
		}
		// 评分相同时，优先选择精确匹配
		return candidates[i].IsExact
	})

	best := candidates[0]
	m.logDebug("[ModelMatcher] Best candidate: backend=%s, model=%s, score=%.3f, exact=%v",
		best.BackendID, best.ActualModel, best.CompatibilityScore, best.IsExact)

	// 输出所有候选信息
	for i, c := range candidates {
		m.logDebug("[ModelMatcher] Candidate %d: backend=%s, model=%s, score=%.3f, exact=%v",
			i+1, c.BackendID, c.ActualModel, c.CompatibilityScore, c.IsExact)
	}

	return best
}

// levenshteinDistance 计算编辑距离
func levenshteinDistance(s1, s2 string) int {
	m, n := len(s1), len(s2)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}
	
	// 创建矩阵
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}
	
	// 填充矩阵
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			dp[i][j] = min(
				dp[i-1][j]+1,      // 删除
				dp[i][j-1]+1,      // 插入
				dp[i-1][j-1]+cost, // 替换
			)
		}
	}
	
	return dp[m][n]
}

// levenshteinDistanceOptimized 计算编辑距离（优化版本，使用滚动数组）
// 空间复杂度从 O(m*n) 降低到 O(n)
func levenshteinDistanceOptimized(s1, s2 string) int {
	m, n := len(s1), len(s2)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	// 确保s1是较短的字符串，减少空间使用
	if m > n {
		s1, s2 = s2, s1
		m, n = n, m
	}

	// 只使用两行滚动数组
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	// 初始化第一行
	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	// 填充矩阵
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,      // 删除
				curr[j-1]+1,    // 插入
				prev[j-1]+cost, // 替换
			)
		}
		// 交换行
		prev, curr = curr, prev
	}

	return prev[n]
}
