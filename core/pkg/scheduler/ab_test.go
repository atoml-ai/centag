package scheduler

import (
	"math/rand"
	"sync"
	"time"
)

// ABTestConfig A/B 测试配置
type ABTestConfig struct {
	Name          string              `json:"name"`           // 测试名称
	Enabled       bool                `json:"enabled"`        // 是否启用
	TrafficSplit  map[string]float64  `json:"traffic_split"`  // 流量分配（策略->比例）
	StartTime     time.Time           `json:"start_time"`     // 开始时间
	EndTime       time.Time           `json:"end_time"`       // 结束时间
	MinSamples    int64               `json:"min_samples"`    // 最小样本数
	ConfidenceLevel float64           `json:"confidence_level"` // 置信水平
}

// ABTestResult A/B 测试结果
type ABTestResult struct {
	Strategy      string  `json:"strategy"`
	Impressions   int64   `json:"impressions"`
	Successes     int64   `json:"successes"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	AvgCost       float64 `json:"avg_cost"`
	UserSatisfaction float64 `json:"user_satisfaction"`
	WinProbability float64 `json:"win_probability"` // 获胜概率
}

// ABTestManager A/B 测试管理器
type ABTestManager struct {
	mu       sync.RWMutex
	tests    map[string]*ABTest
	results  map[string]map[string]*ABTestResult // test_id -> strategy -> result
	rng      *rand.Rand
}

// NewABTestManager 创建 A/B 测试管理器
func NewABTestManager() *ABTestManager {
	return &ABTestManager{
		tests:   make(map[string]*ABTest),
		results: make(map[string]map[string]*ABTestResult),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ABTest A/B 测试实例
type ABTest struct {
	Config        ABTestConfig
	CurrentWinner string // 当前获胜策略
	IsComplete    bool   // 是否完成
}

// CreateTest 创建测试
func (m *ABTestManager) CreateTest(config ABTestConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.TrafficSplit == nil {
		config.TrafficSplit = make(map[string]float64)
	}

	test := &ABTest{
		Config:     config,
		IsComplete: false,
	}

	m.tests[config.Name] = test
	m.results[config.Name] = make(map[string]*ABTestResult)

	// 初始化结果
	for strategy := range config.TrafficSplit {
		m.results[config.Name][strategy] = &ABTestResult{
			Strategy: strategy,
		}
	}

	return nil
}

// SelectStrategy 选择测试策略
func (m *ABTestManager) SelectStrategy(testName string) string {
	m.mu.RLock()
	test, ok := m.tests[testName]
	m.mu.RUnlock()

	if !ok || !test.Config.Enabled {
		return ""
	}

	// 检查时间范围
	now := time.Now()
	if now.Before(test.Config.StartTime) || now.After(test.Config.EndTime) {
		return ""
	}

	// 按流量分配选择
	r := m.rng.Float64()
	cumulative := 0.0

	for strategy, split := range test.Config.TrafficSplit {
		cumulative += split
		if r <= cumulative {
			return strategy
		}
	}

	// 返回最后一个
	for strategy := range test.Config.TrafficSplit {
		return strategy
	}

	return ""
}

// RecordResult 记录结果
func (m *ABTestManager) RecordResult(testName, strategy string, success bool, latencyMs int64, cost float64, satisfaction float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	results, ok := m.results[testName]
	if !ok {
		return
	}

	result, ok := results[strategy]
	if !ok {
		result = &ABTestResult{
			Strategy: strategy,
		}
		results[strategy] = result
	}

	// 更新统计
	result.Impressions++
	if success {
		result.Successes++
	}

	// 指数移动平均
	n := float64(result.Impressions)
	result.ConversionRate = float64(result.Successes) / float64(result.Impressions)
	result.AvgLatencyMs = int64((float64(result.AvgLatencyMs)*(n-1) + float64(latencyMs)) / n)
	result.AvgCost = (result.AvgCost*(n-1) + cost) / n
	result.UserSatisfaction = (result.UserSatisfaction*(n-1) + satisfaction) / n

	// 检查是否达到最小样本数并更新获胜者
	test := m.tests[testName]
	if result.Impressions >= test.Config.MinSamples && !test.IsComplete {
		m.updateWinner(testName)
	}
}

// updateWinner 更新获胜者
func (m *ABTestManager) updateWinner(testName string) {
	results := m.results[testName]
	test := m.tests[testName]

	var bestStrategy string
	var bestScore float64 = -1

	for strategy, result := range results {
		// 综合评分：转化率 50% + 满意度 30% + 成本 20%
		score := result.ConversionRate*0.5 + result.UserSatisfaction*0.3 + (1-result.AvgCost/100)*0.2

		if score > bestScore {
			bestScore = score
			bestStrategy = strategy
		}
	}

	if bestStrategy != "" {
		test.CurrentWinner = bestStrategy
		// 计算获胜概率（简化版）
		for strategy, result := range results {
			if strategy == bestStrategy {
				result.WinProbability = 0.5 + bestScore*0.5 // 0.5-1.0
			} else {
				result.WinProbability = 0.5 - bestScore*0.5 // 0.0-0.5
			}
		}
	}
}

// GetResults 获取测试结果
func (m *ABTestManager) GetResults(testName string) map[string]*ABTestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results, ok := m.results[testName]
	if !ok {
		return nil
	}

	// 返回副本
	resultCopy := make(map[string]*ABTestResult)
	for k, v := range results {
		vCopy := *v
		resultCopy[k] = &vCopy
	}
	return resultCopy
}

// GetWinner 获取获胜策略
func (m *ABTestManager) GetWinner(testName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	test, ok := m.tests[testName]
	if !ok {
		return ""
	}

	return test.CurrentWinner
}

// CompleteTest 完成测试
func (m *ABTestManager) CompleteTest(testName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testName]
	if !ok {
		return
	}

	test.IsComplete = true
	m.updateWinner(testName)
}

// DeleteTest 删除测试
func (m *ABTestManager) DeleteTest(testName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tests, testName)
	delete(m.results, testName)
}

// GetAllTests 获取所有测试
func (m *ABTestManager) GetAllTests() map[string]*ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tests := make(map[string]*ABTest)
	for k, v := range m.tests {
		vCopy := *v
		tests[k] = &vCopy
	}
	return tests
}

// GetBestStrategy 获取最佳策略（综合所有测试结果）
func (m *ABTestManager) GetBestStrategy() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bestStrategy string
	var bestWinProb float64 = -1

	for _, results := range m.results {
		for strategy, result := range results {
			if result.WinProbability > bestWinProb {
				bestWinProb = result.WinProbability
				bestStrategy = strategy
			}
		}
	}

	return bestStrategy
}
