package backend

import (
	"fmt"
)

// StrategyExecutor 策略执行器接口
type StrategyExecutor interface {
	// Execute 执行匹配策略，返回匹配结果
	Execute(requestedModel string, backends []*BackendConfig) []*ModelMatchResult
	// Name 返回策略名称
	Name() ModelMatchStrategy
}

// ExactMatchStrategy 精确匹配策略
type ExactMatchStrategy struct {
	minCompatibility float64
}

// NewExactMatchStrategy 创建精确匹配策略
func NewExactMatchStrategy(minCompatibility float64) *ExactMatchStrategy {
	return &ExactMatchStrategy{
		minCompatibility: minCompatibility,
	}
}

// Execute 执行精确匹配
func (s *ExactMatchStrategy) Execute(requestedModel string, backends []*BackendConfig) []*ModelMatchResult {
	var results []*ModelMatchResult
	normalizedRequested := NormalizeModelName(requestedModel)
	
	for _, backend := range backends {
		if !backend.Enabled {
			continue
		}
		
		for _, mapping := range backend.SupportedModels {
			normalizedMapping := NormalizeModelName(mapping.RequestedModel)
			
			// 精确匹配：模型名完全相同
			if normalizedRequested == normalizedMapping {
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            true,
					CompatibilityScore: 1.0,
					Strategy:           StrategyExact,
					Details: MatchDetails{
						NameSimilarity:  1.0,
						CapacityMatch:   1.0,
						FamilyMatch:     1.0,
					},
				}
				
				if result.CompatibilityScore >= s.minCompatibility {
					results = append(results, result)
				}
			}
		}
	}
	
	return results
}

// Name 返回策略名称
func (s *ExactMatchStrategy) Name() ModelMatchStrategy {
	return StrategyExact
}

// FamilyMatchStrategy 家族匹配策略
type FamilyMatchStrategy struct {
	minCompatibility float64
}

// NewFamilyMatchStrategy 创建家族匹配策略
func NewFamilyMatchStrategy(minCompatibility float64) *FamilyMatchStrategy {
	return &FamilyMatchStrategy{
		minCompatibility: minCompatibility,
	}
}

// Execute 执行家族匹配
func (s *FamilyMatchStrategy) Execute(requestedModel string, backends []*BackendConfig) []*ModelMatchResult {
	var results []*ModelMatchResult
	normalizedRequested := NormalizeModelName(requestedModel)
	
	for _, backend := range backends {
		if !backend.Enabled {
			continue
		}
		
		for _, mapping := range backend.SupportedModels {
			normalizedMapping := NormalizeModelName(mapping.RequestedModel)
			
			// 先检查精确匹配
			if normalizedRequested == normalizedMapping {
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            true,
					CompatibilityScore: 1.0,
					Strategy:           StrategyFamily,
					Details: MatchDetails{
						NameSimilarity:  1.0,
						CapacityMatch:   1.0,
						FamilyMatch:     1.0,
					},
				}
				results = append(results, result)
				continue
			}
			
			// 检查家族匹配
			if IsSameFamily(normalizedRequested, normalizedMapping) {
				// 计算评分
				nameSim := calculateLevenshteinSimilarity(normalizedRequested, normalizedMapping)
				capacityMatch := calculateCapacityRatio(normalizedRequested, normalizedMapping)
				
				score := 0.7 + nameSim*0.2 + capacityMatch*0.1
				
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            false,
					CompatibilityScore: score,
					Strategy:           StrategyFamily,
					Details: MatchDetails{
						NameSimilarity: nameSim,
						CapacityMatch:  capacityMatch,
						FamilyMatch:   1.0,
					},
				}
				
				if result.CompatibilityScore >= s.minCompatibility {
					results = append(results, result)
				}
			}
		}
	}
	
	return results
}

// Name 返回策略名称
func (s *FamilyMatchStrategy) Name() ModelMatchStrategy {
	return StrategyFamily
}

// CapacityMatchStrategy 参数量匹配策略
type CapacityMatchStrategy struct {
	minCompatibility float64
	tolerance        float64
}

// NewCapacityMatchStrategy 创建参数量匹配策略
func NewCapacityMatchStrategy(minCompatibility, tolerance float64) *CapacityMatchStrategy {
	return &CapacityMatchStrategy{
		minCompatibility: minCompatibility,
		tolerance:        tolerance,
	}
}

// Execute 执行参数量匹配
func (s *CapacityMatchStrategy) Execute(requestedModel string, backends []*BackendConfig) []*ModelMatchResult {
	var results []*ModelMatchResult
	normalizedRequested := NormalizeModelName(requestedModel)
	
	for _, backend := range backends {
		if !backend.Enabled {
			continue
		}
		
		for _, mapping := range backend.SupportedModels {
			normalizedMapping := NormalizeModelName(mapping.RequestedModel)
			
			// 先检查精确匹配
			if normalizedRequested == normalizedMapping {
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            true,
					CompatibilityScore: 1.0,
					Strategy:           StrategyCapacity,
					Details: MatchDetails{
						NameSimilarity:  1.0,
						CapacityMatch:   1.0,
						FamilyMatch:     1.0,
					},
				}
				results = append(results, result)
				continue
			}
			
			// 计算参数量匹配度
			capacityRatio := calculateCapacityRatio(normalizedRequested, normalizedMapping)
			
			// 检查是否在容忍度范围内
			if capacityRatio >= (1.0 - s.tolerance) && capacityRatio <= (1.0 + s.tolerance) {
				nameSim := calculateLevenshteinSimilarity(normalizedRequested, normalizedMapping)
				isFamily := 0.0
				if IsSameFamily(normalizedRequested, normalizedMapping) {
					isFamily = 1.0
				}
				
				// 计算综合评分
				score := capacityRatio*0.6 + nameSim*0.3 + isFamily*0.1
				
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            false,
					CompatibilityScore: score,
					Strategy:           StrategyCapacity,
					Details: MatchDetails{
						NameSimilarity: nameSim,
						CapacityMatch:  capacityRatio,
						FamilyMatch:   isFamily,
					},
				}
				
				if result.CompatibilityScore >= s.minCompatibility {
					results = append(results, result)
				}
			}
		}
	}
	
	return results
}

// Name 返回策略名称
func (s *CapacityMatchStrategy) Name() ModelMatchStrategy {
	return StrategyCapacity
}

// HybridMatchStrategy 混合匹配策略
type HybridMatchStrategy struct {
	minCompatibility float64
	weights          HybridWeights
}

// NewHybridMatchStrategy 创建混合匹配策略
func NewHybridMatchStrategy(minCompatibility float64, weights HybridWeights) *HybridMatchStrategy {
	return &HybridMatchStrategy{
		minCompatibility: minCompatibility,
		weights:          weights,
	}
}

// Execute 执行混合匹配
func (s *HybridMatchStrategy) Execute(requestedModel string, backends []*BackendConfig) []*ModelMatchResult {
	var results []*ModelMatchResult
	normalizedRequested := NormalizeModelName(requestedModel)
	
	for _, backend := range backends {
		if !backend.Enabled {
			continue
		}
		
		for _, mapping := range backend.SupportedModels {
			normalizedMapping := NormalizeModelName(mapping.RequestedModel)
			
			// 先检查精确匹配
			if normalizedRequested == normalizedMapping {
				result := &ModelMatchResult{
					BackendID:          backend.ID,
					BackendName:        backend.Name,
					RequestedModel:     requestedModel,
					ActualModel:        mapping.ActualModel,
					IsExact:            true,
					CompatibilityScore: 1.0,
					Strategy:           StrategyHybrid,
					Details: MatchDetails{
						NameSimilarity:  1.0,
						CapacityMatch:   1.0,
						FamilyMatch:     1.0,
					},
				}
				results = append(results, result)
				continue
			}
			
			// 计算各维度评分
			nameSim := calculateLevenshteinSimilarity(normalizedRequested, normalizedMapping)
			capacityMatch := calculateCapacityRatio(normalizedRequested, normalizedMapping)
			familyMatch := 0.0
			if IsSameFamily(normalizedRequested, normalizedMapping) {
				familyMatch = 1.0
			}
			
			// 使用配置权重计算综合评分
			score := nameSim*s.weights.NameSimilarity +
				capacityMatch*s.weights.CapacityMatch +
				familyMatch*0.2
			
			result := &ModelMatchResult{
				BackendID:          backend.ID,
				BackendName:        backend.Name,
				RequestedModel:     requestedModel,
				ActualModel:        mapping.ActualModel,
				IsExact:            false,
				CompatibilityScore: score,
				Strategy:           StrategyHybrid,
				Details: MatchDetails{
					NameSimilarity: nameSim,
					CapacityMatch:  capacityMatch,
					FamilyMatch:   familyMatch,
				},
			}
			
			if result.CompatibilityScore >= s.minCompatibility {
				results = append(results, result)
			}
		}
	}
	
	return results
}

// Name 返回策略名称
func (s *HybridMatchStrategy) Name() ModelMatchStrategy {
	return StrategyHybrid
}

// GetStrategyExecutor 根据策略类型获取策略执行器
func GetStrategyExecutor(strategy ModelMatchStrategy, config ModelMatchingConfig) StrategyExecutor {
	minComp := config.GetMinCompatibility()
	
	switch strategy {
	case StrategyExact:
		return NewExactMatchStrategy(minComp)
	case StrategyFamily:
		return NewFamilyMatchStrategy(minComp)
	case StrategyCapacity:
		return NewCapacityMatchStrategy(minComp, config.CapacityTolerance)
	case StrategyHybrid:
		return NewHybridMatchStrategy(minComp, config.HybridWeights)
	case StrategyCustom:
		// 自定义策略使用混合策略，但允许自定义评分
		return NewHybridMatchStrategy(minComp, config.HybridWeights)
	default:
		return NewHybridMatchStrategy(minComp, config.HybridWeights)
	}
}

// calculateLevenshteinSimilarity 计算基于编辑距离的相似度
func calculateLevenshteinSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	distance := levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))
	
	if maxLen == 0 {
		return 0.0
	}
	
	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity
}

// calculateCapacityRatio 计算参数量比率（0-1）
func calculateCapacityRatio(model1, model2 string) float64 {
	capacity1 := GetModelCapacity(model1)
	capacity2 := GetModelCapacity(model2)

	if capacity1 == 0 || capacity2 == 0 {
		return 0.5
	}

	minCapacity := min(capacity1, capacity2)
	maxCapacity := max(capacity1, capacity2)

	return minCapacity / maxCapacity
}

// String 返回策略的字符串表示
func (s *ExactMatchStrategy) String() string {
	return fmt.Sprintf("ExactMatchStrategy(minCompatibility=%.2f)", s.minCompatibility)
}

func (s *FamilyMatchStrategy) String() string {
	return fmt.Sprintf("FamilyMatchStrategy(minCompatibility=%.2f)", s.minCompatibility)
}

func (s *CapacityMatchStrategy) String() string {
	return fmt.Sprintf("CapacityMatchStrategy(minCompatibility=%.2f, tolerance=%.2f)", s.minCompatibility, s.tolerance)
}

func (s *HybridMatchStrategy) String() string {
	return fmt.Sprintf("HybridMatchStrategy(minCompatibility=%.2f, weights=%.2f,%.2f)",
		s.minCompatibility, s.weights.NameSimilarity, s.weights.CapacityMatch)
}
