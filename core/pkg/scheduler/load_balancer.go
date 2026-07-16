package scheduler

import (
	"hash/fnv"
	"sync"
	"time"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	LBStrategyRoundRobin  LoadBalanceStrategy = "round_robin"  // 轮询
	LBStrategyLeastConn   LoadBalanceStrategy = "least_conn"   // 最少连接
	LBStrategyWeighted    LoadBalanceStrategy = "weighted"     // 加权轮询
	LBStrategyConsistentHash LoadBalanceStrategy = "consistent_hash" // 一致性哈希
)

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy      LoadBalanceStrategy `json:"strategy"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	EnableStickySession bool          `json:"enable_sticky_session"`
	StickySessionTTL    time.Duration `json:"sticky_session_ttl"`
}

// DefaultLoadBalancerConfig 默认负载均衡配置
func DefaultLoadBalancerConfig() LoadBalancerConfig {
	return LoadBalancerConfig{
		Strategy:      LBStrategyRoundRobin,
		HealthCheckInterval: 10 * time.Second,
		EnableStickySession: false,
		StickySessionTTL:    5 * time.Minute,
	}
}

// BackendStats 后端统计
type BackendStats struct {
	BackendID       string
	ActiveConnections int
	TotalRequests   int64
	TotalFailures   int64
	LastRequestAt   time.Time
	Weight          int
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	mu              sync.RWMutex
	backendStats    map[string]*BackendStats
	strategy        LoadBalanceStrategy
	currentIndex    int
	stickySessions  map[string]string // session_id -> backend_id
	sessionExpiry   map[string]time.Time
	config          LoadBalancerConfig
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(config LoadBalancerConfig) *LoadBalancer {
	if config.Strategy == "" {
		config.Strategy = LBStrategyRoundRobin
	}
	return &LoadBalancer{
		backendStats:   make(map[string]*BackendStats),
		strategy:       config.Strategy,
		stickySessions: make(map[string]string),
		sessionExpiry:  make(map[string]time.Time),
		config:         config,
	}
}

// Select 选择后端
func (lb *LoadBalancer) Select(backendIDs []string, sessionID string) string {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(backendIDs) == 0 {
		return ""
	}

	// 检查粘性会话
	if lb.config.EnableStickySession && sessionID != "" {
		if backendID, ok := lb.stickySessions[sessionID]; ok {
			if expiry, ok := lb.sessionExpiry[sessionID]; ok && time.Now().Before(expiry) {
				// 检查后端是否还在列表中
				for _, id := range backendIDs {
					if id == backendID {
						return backendID
					}
				}
			}
		}
	}

	var selected string

	switch lb.strategy {
	case LBStrategyRoundRobin:
		selected = lb.selectRoundRobin(backendIDs)
	case LBStrategyLeastConn:
		selected = lb.selectLeastConn(backendIDs)
	case LBStrategyWeighted:
		selected = lb.selectWeighted(backendIDs)
	case LBStrategyConsistentHash:
		selected = lb.selectConsistentHash(backendIDs, sessionID)
	default:
		selected = backendIDs[0]
	}

	// 记录粘性会话
	if lb.config.EnableStickySession && sessionID != "" {
		lb.stickySessions[sessionID] = selected
		lb.sessionExpiry[sessionID] = time.Now().Add(lb.config.StickySessionTTL)
	}

	return selected
}

// selectRoundRobin 轮询选择
func (lb *LoadBalancer) selectRoundRobin(backendIDs []string) string {
	if len(backendIDs) == 0 {
		return ""
	}

	lb.currentIndex = (lb.currentIndex + 1) % len(backendIDs)
	return backendIDs[lb.currentIndex]
}

// selectLeastConn 最少连接选择
func (lb *LoadBalancer) selectLeastConn(backendIDs []string) string {
	if len(backendIDs) == 0 {
		return ""
	}

	var selected string
	minConn := -1

	for _, id := range backendIDs {
		stats := lb.getOrCreateStats(id)
		if minConn < 0 || stats.ActiveConnections < minConn {
			minConn = stats.ActiveConnections
			selected = id
		}
	}

	return selected
}

// selectWeighted 加权轮询选择
func (lb *LoadBalancer) selectWeighted(backendIDs []string) string {
	if len(backendIDs) == 0 {
		return ""
	}

	// 计算总权重
	totalWeight := 0
	for _, id := range backendIDs {
		stats := lb.getOrCreateStats(id)
		totalWeight += stats.Weight
	}

	if totalWeight == 0 {
		return backendIDs[0]
	}

	// 按权重选择
	currentWeight := 0
	for _, id := range backendIDs {
		stats := lb.getOrCreateStats(id)
		currentWeight += stats.Weight
		if lb.currentIndex%totalWeight < currentWeight {
			return id
		}
	}

	return backendIDs[len(backendIDs)-1]
}

// selectConsistentHash 一致性哈希选择
func (lb *LoadBalancer) selectConsistentHash(backendIDs []string, key string) string {
	if len(backendIDs) == 0 {
		return ""
	}

	if key == "" {
		key = time.Now().String()
	}

	// 使用 FNV 哈希
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	index := int(hash) % len(backendIDs)
	return backendIDs[index]
}

// getOrCreateStats 获取或创建统计
func (lb *LoadBalancer) getOrCreateStats(backendID string) *BackendStats {
	stats, ok := lb.backendStats[backendID]
	if !ok {
		stats = &BackendStats{
			BackendID: backendID,
			Weight:    1,
		}
		lb.backendStats[backendID] = stats
	}
	return stats
}

// RecordRequest 记录请求
func (lb *LoadBalancer) RecordRequest(backendID string, success bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.getOrCreateStats(backendID)
	stats.TotalRequests++
	stats.LastRequestAt = time.Now()

	if !success {
		stats.TotalFailures++
	}
}

// IncrementConnections 增加连接数
func (lb *LoadBalancer) IncrementConnections(backendID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.getOrCreateStats(backendID)
	stats.ActiveConnections++
}

// DecrementConnections 减少连接数
func (lb *LoadBalancer) DecrementConnections(backendID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.getOrCreateStats(backendID)
	if stats.ActiveConnections > 0 {
		stats.ActiveConnections--
	}
}

// SetWeight 设置权重
func (lb *LoadBalancer) SetWeight(backendID string, weight int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.getOrCreateStats(backendID)
	stats.Weight = weight
}

// GetStats 获取统计
func (lb *LoadBalancer) GetStats(backendID string) *BackendStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats, ok := lb.backendStats[backendID]
	if !ok {
		return nil
	}

	// 返回副本
	statsCopy := *stats
	return &statsCopy
}

// GetAllStats 获取所有统计
func (lb *LoadBalancer) GetAllStats() map[string]*BackendStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make(map[string]*BackendStats)
	for k, v := range lb.backendStats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

// GetFailureRate 获取失败率
func (lb *LoadBalancer) GetFailureRate(backendID string) float64 {
	stats := lb.GetStats(backendID)
	if stats == nil || stats.TotalRequests == 0 {
		return 0
	}
	return float64(stats.TotalFailures) / float64(stats.TotalRequests)
}

// Reset 重置负载均衡器
func (lb *LoadBalancer) Reset() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.backendStats = make(map[string]*BackendStats)
	lb.currentIndex = 0
	lb.stickySessions = make(map[string]string)
	lb.sessionExpiry = make(map[string]time.Time)
}

// CleanupSessions 清理过期会话
func (lb *LoadBalancer) CleanupSessions() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := time.Now()
	for sessionID, expiry := range lb.sessionExpiry {
		if now.After(expiry) {
			delete(lb.stickySessions, sessionID)
			delete(lb.sessionExpiry, sessionID)
		}
	}
}
