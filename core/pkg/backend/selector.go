package backend

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// BackendSelector 后端选择器
type BackendSelector struct {
	config  ModelMatchingConfig
	matcher *ModelMatcher // 缓存的匹配器，避免重复创建
	debug   bool          // 是否启用调试日志
}

// NewBackendSelector 创建后端选择器
func NewBackendSelector(config ModelMatchingConfig) *BackendSelector {
	if config.Strategy == "" {
		config = DefaultModelMatchingConfig()
	}
	return &BackendSelector{
		config:  config,
		matcher: NewModelMatcher(config), // 初始化时创建一次
		debug:   false,                    // 默认关闭调试日志
	}
}

// SetDebug 设置调试模式
func (s *BackendSelector) SetDebug(debug bool) {
	s.debug = debug
	if s.matcher != nil {
		s.matcher.SetDebug(debug)
	}
}

// logDebug 输出调试日志
func (s *BackendSelector) logDebug(format string, args ...interface{}) {
	if s.debug {
		log.Printf(format, args...)
	}
}

// logInfo 输出信息日志（总是输出）
func (s *BackendSelector) logInfo(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// SelectBackendByModel 根据请求的模型选择最佳后端
func (s *BackendSelector) SelectBackendByModel(
	requestedModel string,
	backends []*BackendConfig,
) (*BackendConfig, string, error) {
	s.logDebug("[BackendSelector] Starting backend selection for model: %s", requestedModel)
	s.logDebug("[BackendSelector] Total backends available: %d", len(backends))

	// 特殊处理：auto 模型 - 选择第一个启用的健康后端
	normalizedRequested := NormalizeModelName(requestedModel)
	if normalizedRequested == "auto" {
		s.logInfo("[BackendSelector] Auto model detected, selecting first healthy backend")
		enabledBackends := s.filterEnabled(backends)
		if len(enabledBackends) == 0 {
			return nil, "", NewNoUsableBackendError(fmt.Errorf("no enabled backends available"))
		}
		// 选择第一个健康状态良好的后端
		for _, backend := range enabledBackends {
			if backend.HealthStatus.Status == "healthy" || backend.HealthStatus.Status == "" {
				// 返回该后端的第一个模型
				if len(backend.SupportedModels) > 0 {
					firstModel := backend.SupportedModels[0].ActualModel
					s.logInfo("[BackendSelector] Selected backend: %s (%s), model: %s",
						backend.ID, backend.Name, firstModel)
					return backend, firstModel, nil
				}
			}
		}
		// 如果没有健康的，返回第一个启用的后端
		if len(enabledBackends) > 0 && len(enabledBackends[0].SupportedModels) > 0 {
			firstModel := enabledBackends[0].SupportedModels[0].ActualModel
			s.logInfo("[BackendSelector] Selected first backend: %s (%s), model: %s",
				enabledBackends[0].ID, enabledBackends[0].Name, firstModel)
			return enabledBackends[0], firstModel, nil
		}
		return nil, "", fmt.Errorf("no models available in enabled backends")
	}

	// 获取所有启用的后端
	enabledBackends := s.filterEnabled(backends)
	s.logDebug("[BackendSelector] Enabled backends: %d", len(enabledBackends))
	if len(enabledBackends) == 0 {
		return nil, "", NewNoUsableBackendError(fmt.Errorf("no enabled backends available"))
	}

	// 查找精确/松匹配，并优先「字面精确」与「同免费档」的后端
	exactMatches := s.findExactMatches(requestedModel, enabledBackends)
	exactMatches = preferNameAlignedBackends(requestedModel, exactMatches)
	s.logDebug("[BackendSelector] Exact matches found: %d", len(exactMatches))
	if len(exactMatches) > 0 {
		// 从精确匹配中选择优先级最高的
		selected := s.selectBestCandidate(exactMatches, requestedModel)
		if selected != nil {
			s.logInfo("[BackendSelector] Selected exact match backend: %s (%s), priority=%d, weight=%d",
				selected.ID, selected.Name, selected.Priority, selected.Weight)
			if mapping := FindLooseModelMapping(requestedModel, selected); mapping != nil {
				actual := strings.TrimSpace(mapping.ActualModel)
				if actual == "" {
					actual = strings.TrimSpace(mapping.RequestedModel)
				}
				if actual == "" {
					actual = requestedModel
				}
				s.logDebug("[BackendSelector] Exact/loose match actual model: %s", actual)
				return selected, actual, nil
			}
			s.logDebug("[BackendSelector] Using requested model as actual model: %s", requestedModel)
			return selected, requestedModel, nil
		}
	}

	// 检查是否允许模型转换
	allowConversion := s.config.AllowConversion()
	s.logDebug("[BackendSelector] Model conversion allowed: %v (weight=%d)", allowConversion, s.config.ConversionWeight)
	if !allowConversion {
		return nil, "", fmt.Errorf("no exact match found and model conversion is disabled")
	}

	// 查找兼容模型（使用缓存的matcher）
	compatibleModels := s.findAllCompatibleModels(requestedModel, enabledBackends)
	s.logDebug("[BackendSelector] Compatible models found: %d", len(compatibleModels))
	if len(compatibleModels) == 0 {
		return nil, "", fmt.Errorf("no compatible model found for %s", requestedModel)
	}

	// 根据严格度过滤
	s.logDebug("[BackendSelector] Filtering by strictness: %d", s.config.DefaultStrictness)
	compatibleModels = s.filterByStrictness(compatibleModels, s.config.DefaultStrictness)
	s.logDebug("[BackendSelector] After strictness filter: %d candidates", len(compatibleModels))
	if len(compatibleModels) == 0 {
		return nil, "", fmt.Errorf("no compatible model found for %s with strictness %d",
			requestedModel, s.config.DefaultStrictness)
	}

	// 选择最佳候选
	selected := s.selectBestCandidate(compatibleModels, requestedModel)
	if selected == nil {
		return nil, "", fmt.Errorf("failed to select backend for model %s", requestedModel)
	}

	// 查找实际使用的模型
	actualModel := s.findActualModel(requestedModel, selected)
	s.logDebug("[BackendSelector] Actual model for conversion: %s -> %s", requestedModel, actualModel)
	if actualModel == "" {
		return nil, "", fmt.Errorf("no actual model mapping found for %s", requestedModel)
	}

	s.logInfo("[BackendSelector] Final selection: backend=%s (%s), actualModel=%s",
		selected.ID, selected.Name, actualModel)
	return selected, actualModel, nil
}

// filterEnabled 过滤启用的后端
func (s *BackendSelector) filterEnabled(backends []*BackendConfig) []*BackendConfig {
	enabled := make([]*BackendConfig, 0)
	for _, backend := range backends {
		if backend.Enabled {
			enabled = append(enabled, backend)
		}
	}
	return enabled
}

// findExactMatches 查找精确/松匹配（RequestedModel 或 ActualModel）。
func (s *BackendSelector) findExactMatches(
	requestedModel string,
	backends []*BackendConfig,
) []*BackendConfig {
	var matches []*BackendConfig

	for _, backend := range backends {
		for _, mapping := range backend.SupportedModels {
			if ModelNamesLooselyEqual(requestedModel, mapping.RequestedModel) ||
				ModelNamesLooselyEqual(requestedModel, mapping.ActualModel) {
				matches = append(matches, backend)
				break // 找到一个精确匹配就退出
			}
		}
	}

	return matches
}

// findAllCompatibleModels 查找所有兼容模型
func (s *BackendSelector) findAllCompatibleModels(
	requestedModel string,
	backends []*BackendConfig,
) []*BackendConfig {
	var compatible []*BackendConfig

	// 使用缓存的matcher，避免重复创建
	for _, backend := range backends {
		for range backend.SupportedModels {
			// 使用匹配器检查兼容性
			result := s.matcher.Match(requestedModel, []*BackendConfig{backend})
			if result != nil && result.CompatibilityScore >= s.config.GetMinCompatibility() {
				// 避免重复添加
				found := false
				for _, b := range compatible {
					if b.ID == backend.ID {
						found = true
						break
					}
				}
				if !found {
					compatible = append(compatible, backend)
				}
				break // 找到一个兼容匹配就退出
			}
		}
	}

	return compatible
}

// filterByStrictness 根据严格度过滤
func (s *BackendSelector) filterByStrictness(
	backends []*BackendConfig,
	strictness int,
) []*BackendConfig {
	if len(backends) == 0 {
		return backends
	}

	s.logDebug("[BackendSelector] FilterByStrictness: input=%d backends, strictness=%d, preferExact=%v",
		len(backends), strictness, s.config.PreferExact())

	// 使用缓存的matcher
	var filtered []*BackendConfig

	for _, backend := range backends {
		// 检查后端是否至少有一个支持的模型
		if len(backend.SupportedModels) == 0 {
			s.logDebug("[BackendSelector] Backend %s has no supported models, skipping", backend.ID)
			continue
		}

		// 检查是否有精确匹配映射
		hasExactMatch := false
		for _, mapping := range backend.SupportedModels {
			if mapping.IsExact {
				hasExactMatch = true
				break
			}
		}

		if s.config.PreferExact() {
			// 严格模式（strictness <= 30）：只保留有精确匹配的后端
			if hasExactMatch {
				s.logDebug("[BackendSelector] Backend %s passed strictness check (has exact match)", backend.ID)
				filtered = append(filtered, backend)
			} else {
				s.logDebug("[BackendSelector] Backend %s failed strictness check (no exact match)", backend.ID)
			}
		} else {
			// 宽松模式：保留所有有支持模型的后端
			s.logDebug("[BackendSelector] Backend %s passed strictness check (relaxed mode)", backend.ID)
			filtered = append(filtered, backend)
		}
	}

	s.logDebug("[BackendSelector] FilterByStrictness: output=%d backends", len(filtered))
	return filtered
}

// selectBestCandidate 选择最佳候选后端
func (s *BackendSelector) selectBestCandidate(
	candidates []*BackendConfig,
	_ string, // requestedModel - 保留参数以保持接口一致性，但暂未使用
) *BackendConfig {
	if len(candidates) == 0 {
		return nil
	}

	s.logDebug("[BackendSelector] Selecting best candidate from %d options", len(candidates))

	// 优先使用权重
	sort.Slice(candidates, func(i, j int) bool {
		// 优先级高的排在前面
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		// 权重大的排在前面
		return candidates[i].Weight > candidates[j].Weight
	})

	// 输出排序后的候选信息
	for i, c := range candidates {
		s.logDebug("[BackendSelector] Candidate %d: ID=%s, Name=%s, Priority=%d, Weight=%d",
			i+1, c.ID, c.Name, c.Priority, c.Weight)
	}

	best := candidates[0]
	s.logDebug("[BackendSelector] Selected: ID=%s, Name=%s, Priority=%d, Weight=%d",
		best.ID, best.Name, best.Priority, best.Weight)

	return best
}

// findActualModel 查找实际使用的模型
func (s *BackendSelector) findActualModel(
	requestedModel string,
	backend *BackendConfig,
) string {
	if mapping := FindLooseModelMapping(requestedModel, backend); mapping != nil {
		if am := strings.TrimSpace(mapping.ActualModel); am != "" {
			return am
		}
		return strings.TrimSpace(mapping.RequestedModel)
	}

	// 如果没有精确/松匹配，返回第一个兼容的
	if len(backend.SupportedModels) > 0 {
		return backend.SupportedModels[0].ActualModel
	}

	return ""
}

// preferNameAlignedBackends 在候选后端中优先保留字面精确或同免费档命中的后端。
func preferNameAlignedBackends(requestedModel string, backends []*BackendConfig) []*BackendConfig {
	if len(backends) <= 1 {
		return backends
	}
	req := strings.TrimSpace(requestedModel)
	reqLower := strings.ToLower(req)
	reqFree := ModelHasFreeTier(req)

	var literal, sameTier []*BackendConfig
	for _, b := range backends {
		if b == nil {
			continue
		}
		literalHit := false
		sameTierHit := false
		for _, m := range b.SupportedModels {
			requested := strings.TrimSpace(m.RequestedModel)
			actual := strings.TrimSpace(m.ActualModel)
			if !ModelNamesLooselyEqual(req, requested) && !ModelNamesLooselyEqual(req, actual) {
				continue
			}
			if strings.EqualFold(reqLower, strings.ToLower(actual)) ||
				strings.EqualFold(reqLower, strings.ToLower(requested)) {
				literalHit = true
			}
			if ModelHasFreeTier(actual) == reqFree || ModelHasFreeTier(requested) == reqFree {
				sameTierHit = true
			}
		}
		if literalHit {
			literal = append(literal, b)
		} else if sameTierHit {
			sameTier = append(sameTier, b)
		}
	}
	if len(literal) > 0 {
		return literal
	}
	if len(sameTier) > 0 {
		return sameTier
	}
	return backends
}

// FindLooseModelMapping 在单个后端内按松等规则查找模型映射。
// 优先精确匹配；其次同免费档松匹配；最后才是跨档松匹配（如仅有 mino2.5 时匹配 mino2.5 free）。
func FindLooseModelMapping(requestedModel string, backend *BackendConfig) *ModelMapping {
	if backend == nil {
		return nil
	}
	req := strings.TrimSpace(requestedModel)
	if req == "" {
		return nil
	}
	reqLower := strings.ToLower(req)
	reqFree := ModelHasFreeTier(req)

	var best *ModelMapping
	bestScore := -1
	for i := range backend.SupportedModels {
		mapping := &backend.SupportedModels[i]
		requested := strings.TrimSpace(mapping.RequestedModel)
		actual := strings.TrimSpace(mapping.ActualModel)
		if !ModelNamesLooselyEqual(req, requested) && !ModelNamesLooselyEqual(req, actual) {
			continue
		}
		score := 1 // loose
		if strings.EqualFold(reqLower, strings.ToLower(actual)) ||
			strings.EqualFold(reqLower, strings.ToLower(requested)) {
			score = 3 // exact
		} else if ModelHasFreeTier(actual) == reqFree || ModelHasFreeTier(requested) == reqFree {
			score = 2 // same free-tier
		}
		if score > bestScore {
			bestScore = score
			best = mapping
			if score == 3 {
				break
			}
		}
	}
	return best
}

// SelectBackendForRequest 选择后端处理请求
// 这是更高级的接口，集成所有选择逻辑
func (s *BackendSelector) SelectBackendForRequest(
	requestedModel string,
	backends []*BackendConfig,
) (*BackendSelection, error) {
	s.logDebug("[BackendSelector] SelectBackendForRequest: model=%s", requestedModel)

	backend, actualModel, err := s.SelectBackendByModel(requestedModel, backends)
	if err != nil {
		return nil, err
	}

	// 创建选择结果
	isExact := NormalizeModelName(requestedModel) == NormalizeModelName(actualModel)
	score := s.calculateCompatibilityScore(requestedModel, actualModel)

	s.logDebug("[BackendSelector] Creating selection: backend=%s, actualModel=%s, isExact=%v, score=%.3f",
		backend.ID, actualModel, isExact, score)

	selection := &BackendSelection{
		BackendID:          backend.ID,
		BackendName:        backend.Name,
		BackendType:        backend.Type,
		RequestedModel:     requestedModel,
		ActualModel:        actualModel,
		IsExactMatch:       isExact,
		CompatibilityScore: score,
		Strategy:           s.config.Strategy,
		BackendConfig:      backend,
	}

	confidence := selection.GetConfidenceLevel()
	s.logDebug("[BackendSelector] Selection confidence level: %s", confidence)

	return selection, nil
}

// calculateCompatibilityScore 计算兼容性评分
func (s *BackendSelector) calculateCompatibilityScore(requestedModel, _ string) float64 { // actualModel - 保留参数以保持接口一致性，但暂未使用
	// 使用缓存的matcher
	result := s.matcher.Match(requestedModel, []*BackendConfig{})
	if result != nil {
		return result.CompatibilityScore
	}
	return 0.0
}

// GetCompatibleBackends 获取所有兼容的后端
func (s *BackendSelector) GetCompatibleBackends(
	requestedModel string,
	backends []*BackendConfig,
) []*BackendConfig {
	return s.findAllCompatibleModels(requestedModel, backends)
}

// HasExactMatch 检查是否有精确匹配
func (s *BackendSelector) HasExactMatch(
	requestedModel string,
	backends []*BackendConfig,
) bool {
	matches := s.findExactMatches(requestedModel, backends)
	return len(matches) > 0
}

// CountCompatibleBackends 计算兼容后端数量
func (s *BackendSelector) CountCompatibleBackends(
	requestedModel string,
	backends []*BackendConfig,
) int {
	return len(s.findAllCompatibleModels(requestedModel, backends))
}
