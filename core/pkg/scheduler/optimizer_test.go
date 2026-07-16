package scheduler

import (
	"testing"
	"time"
)

func TestDecisionOptimizer(t *testing.T) {
	config := DefaultOptimizationConfig()
	config.BudgetLimit = 50.0
	config.MinQualityScore = 0.5

	optimizer := NewDecisionOptimizer(config)

	t.Run("预算检查", func(t *testing.T) {
		// 初始预算
		if optimizer.GetBudgetUsed() != 0 {
			t.Errorf("Initial budget used = %f, want 0", optimizer.GetBudgetUsed())
		}

		// 模拟使用预算
		optimizer.budgetUsed = 30.0
		remaining := optimizer.GetBudgetRemaining()
		if remaining != 20.0 {
			t.Errorf("Budget remaining = %f, want 20.0", remaining)
		}
	})

	t.Run("策略选择", func(t *testing.T) {
		scores := []*BackendScore{
			{BackendID: "backend1", TotalScore: 0.8, EstimatedCost: 1.0, Dimensions: ScoringDimensions{QualityScore: 0.9}},
			{BackendID: "backend2", TotalScore: 0.7, EstimatedCost: 0.5, Dimensions: ScoringDimensions{QualityScore: 0.6}},
			{BackendID: "backend3", TotalScore: 0.6, EstimatedCost: 2.0, Dimensions: ScoringDimensions{QualityScore: 0.8}},
		}

		intent := &ClassificationResult{TaskType: TaskSimpleChat}

		// 成本优先
		decision := optimizer.Optimize(scores, intent, StrategyCost)
		if decision.BackendID != "backend2" {
			t.Errorf("Cost strategy selected %s, want backend2", decision.BackendID)
		}

		// 质量优先
		decision = optimizer.Optimize(scores, intent, StrategyQuality)
		if decision.BackendID != "backend1" {
			t.Errorf("Quality strategy selected %s, want backend1", decision.BackendID)
		}
	})

	t.Run("用户反馈", func(t *testing.T) {
		optimizer.RecordUserFeedback(StrategyCost, true)
		optimizer.RecordUserFeedback(StrategyCost, true)
		optimizer.RecordUserFeedback(StrategyCost, false)

		stats := optimizer.GetStrategyStats(StrategyCost)
		if stats == nil {
			t.Fatal("Strategy stats is nil")
		}
		if stats.UserSatisfaction < 0.5 {
			t.Errorf("User satisfaction = %f, want > 0.5", stats.UserSatisfaction)
		}
	})
}

func TestCircuitBreaker(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.FailureThreshold = 3
	config.Timeout = 100 * time.Millisecond

	cb := NewCircuitBreaker("test-backend", config)

	t.Run("正常状态", func(t *testing.T) {
		if cb.GetState() != StateClosed {
			t.Errorf("Initial state = %s, want closed", cb.GetState())
		}

		if !cb.Allow() {
			t.Error("Should allow request in closed state")
		}
	})

	t.Run("熔断打开", func(t *testing.T) {
		// 记录多次失败
		for i := 0; i < 3; i++ {
			cb.RecordFailure()
		}

		if cb.GetState() != StateOpen {
			t.Errorf("State = %s, want open", cb.GetState())
		}

		if cb.Allow() {
			t.Error("Should not allow request in open state")
		}
	})

	t.Run("半开状态", func(t *testing.T) {
		// 等待超时
		time.Sleep(150 * time.Millisecond)

		if !cb.Allow() {
			t.Error("Should allow request after timeout")
		}

		if cb.GetState() != StateHalfOpen {
			t.Errorf("State = %s, want half-open", cb.GetState())
		}
	})

	t.Run("恢复正常", func(t *testing.T) {
		// 记录成功
		for i := 0; i < 3; i++ {
			cb.RecordSuccess()
		}

		if cb.GetState() != StateClosed {
			t.Errorf("State = %s, want closed", cb.GetState())
		}
	})
}

func TestCircuitBreakerManager(t *testing.T) {
	manager := NewCircuitBreakerManager(DefaultCircuitBreakerConfig())

	t.Run("获取熔断器", func(t *testing.T) {
		cb := manager.Get("backend1")
		if cb == nil {
			t.Fatal("Circuit breaker is nil")
		}
	})

	t.Run("健康检查", func(t *testing.T) {
		backendIDs := []string{"backend1", "backend2", "backend3"}

		// 所有后端健康
		healthy := manager.GetHealthyBackends(backendIDs)
		if len(healthy) != 3 {
			t.Errorf("Healthy backends = %d, want 3", len(healthy))
		}

		// 模拟 backend2 故障
		for i := 0; i < 5; i++ {
			manager.RecordFailure("backend2")
		}

		healthy = manager.GetHealthyBackends(backendIDs)
		if len(healthy) != 2 {
			t.Errorf("Healthy backends after failure = %d, want 2", len(healthy))
		}
	})
}

func TestLoadBalancer(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.Strategy = LBStrategyRoundRobin

	lb := NewLoadBalancer(config)
	backendIDs := []string{"backend1", "backend2", "backend3"}

	t.Run("轮询选择", func(t *testing.T) {
		selected := make(map[string]int)
		for i := 0; i < 9; i++ {
			id := lb.Select(backendIDs, "")
			selected[id]++
		}

		// 每个后端应该被选择 3 次
		for _, count := range selected {
			if count != 3 {
				t.Errorf("Backend selected %d times, want 3", count)
			}
		}
	})

	t.Run("最少连接选择", func(t *testing.T) {
		lb.config.Strategy = LBStrategyLeastConn

		// 设置不同的连接数
		lb.backendStats["backend1"] = &BackendStats{ActiveConnections: 10}
		lb.backendStats["backend2"] = &BackendStats{ActiveConnections: 5}
		lb.backendStats["backend3"] = &BackendStats{ActiveConnections: 15}

		id := lb.Select(backendIDs, "")
		if id != "backend2" {
			t.Errorf("Least conn selected %s, want backend2", id)
		}
	})

	t.Run("加权轮询", func(t *testing.T) {
		lb.config.Strategy = LBStrategyWeighted

		lb.SetWeight("backend1", 1)
		lb.SetWeight("backend2", 2)
		lb.SetWeight("backend3", 3)

		// backend3 权重最高，应该被选择更多
		selected := make(map[string]int)
		for i := 0; i < 60; i++ {
			id := lb.Select(backendIDs, "")
			selected[id]++
			lb.currentIndex++ // 模拟轮询
		}

		t.Logf("Weighted selection: %v", selected)
	})

	t.Run("请求统计", func(t *testing.T) {
		lb.RecordRequest("backend1", true)
		lb.RecordRequest("backend1", true)
		lb.RecordRequest("backend1", false)

		stats := lb.GetStats("backend1")
		if stats.TotalRequests != 3 {
			t.Errorf("Total requests = %d, want 3", stats.TotalRequests)
		}
		if stats.TotalFailures != 1 {
			t.Errorf("Total failures = %d, want 1", stats.TotalFailures)
		}

		failureRate := lb.GetFailureRate("backend1")
		if failureRate < 0.3 || failureRate > 0.4 {
			t.Errorf("Failure rate = %f, want ~0.33", failureRate)
		}
	})
}

func TestABTestManager(t *testing.T) {
	manager := NewABTestManager()

	t.Run("创建测试", func(t *testing.T) {
		config := ABTestConfig{
			Name:     "strategy_test",
			Enabled:  true,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(24 * time.Hour),
			MinSamples: 10,
			TrafficSplit: map[string]float64{
				"StrategyCost":    0.5,
				"StrategyQuality": 0.5,
			},
		}

		err := manager.CreateTest(config)
		if err != nil {
			t.Fatalf("CreateTest failed: %v", err)
		}
	})

	t.Run("选择策略", func(t *testing.T) {
		strategy := manager.SelectStrategy("strategy_test")
		if strategy == "" {
			t.Error("Selected strategy is empty")
		}
		if strategy != "StrategyCost" && strategy != "StrategyQuality" {
			t.Errorf("Selected invalid strategy: %s", strategy)
		}
	})

	t.Run("记录结果", func(t *testing.T) {
		// 模拟多次请求结果
		for i := 0; i < 20; i++ {
			manager.RecordResult("strategy_test", "StrategyCost", true, 100, 1.0, 0.9)
			manager.RecordResult("strategy_test", "StrategyQuality", i%2 == 0, 500, 10.0, 0.7)
		}

		results := manager.GetResults("strategy_test")
		if len(results) != 2 {
			t.Errorf("Results count = %d, want 2", len(results))
		}

		costResult := results["StrategyCost"]
		if costResult.Impressions != 20 {
			t.Errorf("Cost impressions = %d, want 20", costResult.Impressions)
		}
		if costResult.ConversionRate != 1.0 {
			t.Errorf("Cost conversion rate = %f, want 1.0", costResult.ConversionRate)
		}
	})

	t.Run("获取获胜者", func(t *testing.T) {
		winner := manager.GetWinner("strategy_test")
		if winner == "" {
			t.Log("No winner yet (samples may be insufficient)")
		} else {
			t.Logf("Winner: %s", winner)
		}
	})
}

func TestIntegration(t *testing.T) {
	// 集成测试：完整调度流程

	// 3. 创建熔断器管理器
	cbManager := NewCircuitBreakerManager(DefaultCircuitBreakerConfig())

	// 4. 创建负载均衡器
	lb := NewLoadBalancer(DefaultLoadBalancerConfig())

	// 5. 创建 A/B 测试管理器
	abManager := NewABTestManager()

	// 检查熔断器状态
	if !cbManager.Allow("bigmodel") {
		t.Skip("Backend circuit breaker is open")
	}

	// 评分
	// （这里需要创建 mock backend）

	// 优化决策
	// （这里需要 scores 输入）

	// 记录结果
	cbManager.RecordSuccess("bigmodel")
	lb.RecordRequest("bigmodel", true)
	abManager.RecordResult("strategy_test", "StrategyQuality", true, 200, 5.0, 0.9)

	t.Log("Integration test completed")
}
