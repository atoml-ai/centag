package scheduler

import (
	"testing"
	"time"
)

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
