package scheduler

import (
	"testing"
	"time"
)

func TestCircuitBreaker_ErrorRateThreshold(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    100, // 设置高阈值，只依赖错误率
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  65, // 65% 错误率触发熔断
		MinRequestsInWindow: 10, // 最少 10 个请求
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 发送 10 个请求，6 个失败（60%）- 不触发熔断
	for i := 0; i < 6; i++ {
		cb.RecordFailure()
	}
	for i := 0; i < 4; i++ {
		cb.RecordSuccess()
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed, got %s", cb.GetState())
	}

	// 再发送 1 个失败请求（总共 11 个请求，7 个失败 = 63.6%）- 不触发熔断
	cb.RecordFailure()

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed, got %s", cb.GetState())
	}

	// 再发送 1 个失败请求（总共 12 个请求，8 个失败 = 66.7%）- 触发熔断
	cb.RecordFailure()

	if cb.GetState() != StateOpen {
		t.Errorf("expected state open, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_ErrorRateDisabled(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  0, // 禁用错误率熔断
		MinRequestsInWindow: 10,
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 发送 20 个请求，15 个失败（75%）- 不触发熔断（因为禁用了错误率）
	for i := 0; i < 15; i++ {
		cb.RecordFailure()
	}
	for i := 0; i < 5; i++ {
		cb.RecordSuccess()
	}

	// 只有失败次数达到 5 时才触发熔断
	if cb.GetState() != StateOpen {
		t.Errorf("expected state open, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_MinRequestsNotMet(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    100, // 设置高阈值
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  50, // 50% 错误率
		MinRequestsInWindow: 10, // 最少 10 个请求
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 发送 5 个请求，4 个失败（80%）- 不触发熔断（因为请求总数 < 10）
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	for i := 0; i < 1; i++ {
		cb.RecordSuccess()
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed, got %s", cb.GetState())
	}

	// 发送 5 个成功请求（总共 10 个请求，4 个失败 = 40%）- 不触发熔断
	for i := 0; i < 5; i++ {
		cb.RecordSuccess()
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_FailureThresholdOR(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5, // 失败次数阈值
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  65, // 错误率阈值
		MinRequestsInWindow: 10,
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 发送 5 个失败请求 - 由 failure_threshold 触发熔断
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("expected state open, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_GetErrorRate(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    100,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  0,
		MinRequestsInWindow: 10,
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 无请求
	if rate := cb.GetErrorRate(); rate != 0 {
		t.Errorf("expected error rate 0, got %f", rate)
	}

	// 发送 10 个请求，5 个失败
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	for i := 0; i < 5; i++ {
		cb.RecordSuccess()
	}

	if rate := cb.GetErrorRate(); rate != 50 {
		t.Errorf("expected error rate 50, got %f", rate)
	}
}

func TestCircuitBreaker_GetRequestCount(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    100,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  0,
		MinRequestsInWindow: 10,
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 发送请求
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	for i := 0; i < 7; i++ {
		cb.RecordSuccess()
	}

	if count := cb.GetRequestCount(); count != 10 {
		t.Errorf("expected request count 10, got %d", count)
	}
}

func TestCircuitBreakerManager_ErrorRateConfig(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  65,
		MinRequestsInWindow: 10,
	}

	manager := NewCircuitBreakerManager(config)

	// 获取配置
	gotConfig := manager.GetConfig()
	if gotConfig.ErrorRateThreshold != 65 {
		t.Errorf("expected error rate threshold 65, got %f", gotConfig.ErrorRateThreshold)
	}
	if gotConfig.MinRequestsInWindow != 10 {
		t.Errorf("expected min requests 10, got %d", gotConfig.MinRequestsInWindow)
	}

	// 热更新配置
	newConfig := CircuitBreakerConfig{
		FailureThreshold:    10,
		SuccessThreshold:    5,
		Timeout:             60 * time.Second,
		WindowDuration:      2 * time.Minute,
		ErrorRateThreshold:  70,
		MinRequestsInWindow: 15,
	}
	manager.UpdateConfig(newConfig, 2)

	gotConfig = manager.GetConfig()
	if gotConfig.ErrorRateThreshold != 70 {
		t.Errorf("expected error rate threshold 70, got %f", gotConfig.ErrorRateThreshold)
	}
	if gotConfig.MinRequestsInWindow != 15 {
		t.Errorf("expected min requests 15, got %d", gotConfig.MinRequestsInWindow)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		WindowDuration:      1 * time.Minute,
		ErrorRateThreshold:  65,
		MinRequestsInWindow: 10,
	}

	cb := NewCircuitBreaker("test-backend", config)

	// 触发熔断
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("expected state open, got %s", cb.GetState())
	}

	// 重置
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed after reset, got %s", cb.GetState())
	}

	if cb.GetRequestCount() != 0 {
		t.Errorf("expected request count 0 after reset, got %d", cb.GetRequestCount())
	}
}
