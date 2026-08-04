package scheduler

import (
	"sync"
	"time"

	"centag/core/pkg/logger"
)

// CircuitState 熔断器状态
type CircuitState string

const (
	StateClosed   CircuitState = "closed"   // 正常状态
	StateOpen     CircuitState = "open"     // 熔断打开（拒绝请求）
	StateHalfOpen CircuitState = "half-open" // 半开状态（试探性恢复）
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold    int           `json:"failure_threshold"`     // 失败阈值
	SuccessThreshold    int           `json:"success_threshold"`     // 成功阈值（半开状态）
	Timeout             time.Duration `json:"timeout"`               // 熔断超时
	WindowDuration      time.Duration `json:"window_duration"`       // 统计窗口
	ErrorRateThreshold  float64       `json:"error_rate_threshold"`  // [+] 错误率阈值（0=禁用，如 65 表示 65%）
	MinRequestsInWindow int           `json:"min_requests_in_window"` // [+] 窗口内最小请求数（防止低流量误熔断）
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		WindowDuration:   1 * time.Minute,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu               sync.RWMutex
	backendID        string
	state            CircuitState
	failures         []time.Time
	successes        []time.Time
	requests         []time.Time  // [+] 窗口内全部请求（含成功）
	lastFailureTime  time.Time
	lastStateChange  time.Time
	config           CircuitBreakerConfig
	stateChangeCallbacks []func(CircuitState)
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(backendID string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		backendID: backendID,
		state:     StateClosed,
		failures:  make([]time.Time, 0),
		successes: make([]time.Time, 0),
		config:    config,
		lastStateChange: time.Now(),
	}
}

// Allow 检查是否允许请求
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	// 清理过期记录
	cb.cleanupOldRecords(now)

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否超时
		if now.Sub(cb.lastStateChange) > cb.config.Timeout {
			cb.changeState(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		// 半开状态：只放行有限数量的探测请求（SuccessThreshold 个），
		// 避免半开后端被全量流量冲击，形成"全量失败→重开→再全量"的惊群。
		if cb.config.SuccessThreshold > 0 && len(cb.successes)+len(cb.failures) >= cb.config.SuccessThreshold {
			return false
		}
		return true
	}

	return false
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.successes = append(cb.successes, now)
	cb.requests = append(cb.requests, now) // [+] 记录到全部请求

	if cb.state == StateHalfOpen {
		// 半开状态：达到成功阈值则关闭
		if len(cb.successes) >= cb.config.SuccessThreshold {
			cb.changeState(StateClosed)
			cb.failures = make([]time.Time, 0)
			cb.successes = make([]time.Time, 0)
			cb.requests = make([]time.Time, 0)
		}
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.failures = append(cb.failures, now)
	cb.requests = append(cb.requests, now) // [+] 记录到全部请求
	cb.lastFailureTime = now

	if cb.state == StateHalfOpen {
		// 半开状态：任何失败都重新打开
		cb.changeState(StateOpen)
		cb.successes = make([]time.Time, 0)
	} else if cb.state == StateClosed {
		// [+] 支持 error_rate 与 failure_threshold OR 关系
		shouldOpen := false

		// 条件 1：失败次数达到阈值
		if cb.config.FailureThreshold > 0 && len(cb.failures) >= cb.config.FailureThreshold {
			shouldOpen = true
		}

		// 条件 2：错误率达到阈值（需满足最小请求数）
		if cb.config.ErrorRateThreshold > 0 && len(cb.requests) >= cb.config.MinRequestsInWindow {
			errorRate := float64(len(cb.failures)) / float64(len(cb.requests)) * 100
			if errorRate >= cb.config.ErrorRateThreshold {
				shouldOpen = true
			}
		}

		if shouldOpen {
			cb.changeState(StateOpen)
		}
	}
}

// GetState 获取状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount 获取失败次数
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.failures)
}

// GetSuccessCount 获取成功次数
func (cb *CircuitBreaker) GetSuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.successes)
}

// GetRequestCount 获取窗口内请求总数
func (cb *CircuitBreaker) GetRequestCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.requests)
}

// GetErrorRate 获取错误率（百分比）
func (cb *CircuitBreaker) GetErrorRate() float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if len(cb.requests) == 0 {
		return 0
	}
	return float64(len(cb.failures)) / float64(len(cb.requests)) * 100
}

// IsHealthy 检查是否健康
func (cb *CircuitBreaker) IsHealthy() bool {
	state := cb.GetState()
	return state == StateClosed || state == StateHalfOpen
}

// IsOpen reports whether the breaker is fully open (rejecting requests).
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// changeState 改变状态
func (cb *CircuitBreaker) changeState(newState CircuitState) {
	cb.state = newState
	cb.lastStateChange = time.Now()

	// 触发回调
	for _, callback := range cb.stateChangeCallbacks {
		callback(newState)
	}

	if logger.Sugar != nil {
		logger.Warnf("[CircuitBreaker] %s state -> %s", cb.backendID, newState)
	}
}

// cleanupOldRecords 清理过期记录
func (cb *CircuitBreaker) cleanupOldRecords(now time.Time) {
	cutoff := now.Add(-cb.config.WindowDuration)

	// 清理过期失败记录
	newFailures := make([]time.Time, 0)
	for _, t := range cb.failures {
		if t.After(cutoff) {
			newFailures = append(newFailures, t)
		}
	}
	cb.failures = newFailures

	// 清理过期成功记录
	newSuccesses := make([]time.Time, 0)
	for _, t := range cb.successes {
		if t.After(cutoff) {
			newSuccesses = append(newSuccesses, t)
		}
	}
	cb.successes = newSuccesses

	// [+] 清理过期请求记录
	newRequests := make([]time.Time, 0)
	for _, t := range cb.requests {
		if t.After(cutoff) {
			newRequests = append(newRequests, t)
		}
	}
	cb.requests = newRequests
}

// OnStateChange 注册状态变化回调
func (cb *CircuitBreaker) OnStateChange(callback func(CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.stateChangeCallbacks = append(cb.stateChangeCallbacks, callback)
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = make([]time.Time, 0)
	cb.successes = make([]time.Time, 0)
	cb.requests = make([]time.Time, 0) // [+] 重置请求记录
	cb.lastStateChange = time.Now()
}

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	mu           sync.RWMutex
	breakers     map[string]*CircuitBreaker
	config       CircuitBreakerConfig
	rateLimitWeight int // 429 加重系数（默认 2）
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	if config.FailureThreshold == 0 {
		config = DefaultCircuitBreakerConfig()
	}
	// [+] 设置默认最小请求数
	if config.MinRequestsInWindow == 0 {
		config.MinRequestsInWindow = 10
	}
	return &CircuitBreakerManager{
		breakers:         make(map[string]*CircuitBreaker),
		config:           config,
		rateLimitWeight:  2,
	}
}

// UpdateConfig 热更新熔断器配置。已创建的熔断器会在下次 Allow/Record 时使用新配置。
func (m *CircuitBreakerManager) UpdateConfig(config CircuitBreakerConfig, rateLimitWeight int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if config.FailureThreshold > 0 {
		m.config = config
	}
	if rateLimitWeight > 0 {
		m.rateLimitWeight = rateLimitWeight
	}
	// 同步更新已有熔断器的配置
	for _, cb := range m.breakers {
		cb.mu.Lock()
		cb.config = config
		cb.mu.Unlock()
	}
}

// GetConfig 返回当前配置（热读取）。
func (m *CircuitBreakerManager) GetConfig() CircuitBreakerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetRateLimitWeight 返回 429 加重系数。
func (m *CircuitBreakerManager) GetRateLimitWeight() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w := m.rateLimitWeight
	if w <= 0 {
		w = 2
	}
	return w
}

// Get 获取后端熔断器
func (m *CircuitBreakerManager) Get(backendID string) *CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.breakers[backendID]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		cb = NewCircuitBreaker(backendID, m.config)
		m.breakers[backendID] = cb
		m.mu.Unlock()
	}

	return cb
}

// IsOpen reports whether the backend circuit breaker is open.
func (m *CircuitBreakerManager) IsOpen(backendID string) bool {
	if backendID == "" {
		return false
	}
	return m.Get(backendID).IsOpen()
}

// Allow 检查是否允许请求
func (m *CircuitBreakerManager) Allow(backendID string) bool {
	cb := m.Get(backendID)
	return cb.Allow()
}

// RecordSuccess 记录成功
func (m *CircuitBreakerManager) RecordSuccess(backendID string) {
	cb := m.Get(backendID)
	cb.RecordSuccess()
}

// RecordFailure 记录失败
func (m *CircuitBreakerManager) RecordFailure(backendID string) {
	cb := m.Get(backendID)
	cb.RecordFailure()
}

// RecordFailureWithWeight 记录带权重的失败（用于429等限流错误）。
// weight 次数会被追加到 failures 列表中，实现加重计数效果。
func (m *CircuitBreakerManager) RecordFailureWithWeight(backendID string, weight int) {
	if weight <= 1 {
		m.RecordFailure(backendID)
		return
	}
	cb := m.Get(backendID)
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	for i := 0; i < weight; i++ {
		cb.failures = append(cb.failures, now)
		cb.requests = append(cb.requests, now) // [+] 记录到全部请求
	}
	cb.lastFailureTime = now

	if cb.state == StateHalfOpen {
		cb.changeState(StateOpen)
		cb.successes = make([]time.Time, 0)
	} else if cb.state == StateClosed {
		// [+] 支持 error_rate 与 failure_threshold OR 关系
		shouldOpen := false

		// 条件 1：失败次数达到阈值
		if cb.config.FailureThreshold > 0 && len(cb.failures) >= cb.config.FailureThreshold {
			shouldOpen = true
		}

		// 条件 2：错误率达到阈值（需满足最小请求数）
		if cb.config.ErrorRateThreshold > 0 && len(cb.requests) >= cb.config.MinRequestsInWindow {
			errorRate := float64(len(cb.failures)) / float64(len(cb.requests)) * 100
			if errorRate >= cb.config.ErrorRateThreshold {
				shouldOpen = true
			}
		}

		if shouldOpen {
			cb.changeState(StateOpen)
		}
	}
}

// RecordRateLimitFailure 记录429限流失败（自动应用加重系数）。
func (m *CircuitBreakerManager) RecordRateLimitFailure(backendID string) {
	weight := m.GetRateLimitWeight()
	m.RecordFailureWithWeight(backendID, weight)
}

// GetHealthyBackends 获取健康后端列表
func (m *CircuitBreakerManager) GetHealthyBackends(backendIDs []string) []string {
	healthy := make([]string, 0)
	for _, id := range backendIDs {
		cb := m.Get(id)
		if cb.IsHealthy() {
			healthy = append(healthy, id)
		}
	}
	return healthy
}

// GetAllStates 获取所有熔断器状态
func (m *CircuitBreakerManager) GetAllStates() map[string]CircuitState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]CircuitState)
	for id, cb := range m.breakers {
		states[id] = cb.GetState()
	}
	return states
}
