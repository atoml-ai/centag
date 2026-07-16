package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteNodePlugin_NewRemoteNodePlugin(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://localhost:8080")
	if plugin == nil {
		t.Fatal("NewRemoteNodePlugin returned nil")
	}

	descriptor := plugin.Descriptor()
	if descriptor.Name != "http://localhost:8080" {
		t.Errorf("expected name 'http://localhost:8080', got '%s'", descriptor.Name)
	}
	if descriptor.Kind != "remote.node" {
		t.Errorf("expected kind 'remote.node', got '%s'", descriptor.Kind)
	}
}

func TestRemoteNodePlugin_ConcurrentFailureCount(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid-host-for-testing").(*RemoteNodePlugin)

	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.recordFailure()
		}()
	}

	wg.Wait()

	count := plugin.GetFailureCount()
	if count != concurrency {
		t.Errorf("expected failure count %d, got %d", concurrency, count)
	}

	if !plugin.IsCircuitOpen() {
		t.Error("expected circuit breaker to be open after consecutive failures")
	}
}

func TestRemoteNodePlugin_ConcurrentFailureAndReset(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid-host-for-testing").(*RemoteNodePlugin)

	var wg sync.WaitGroup
	workers := 50
	iterations := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				plugin.recordFailure()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	count := plugin.GetFailureCount()
	if count != workers*iterations {
		t.Errorf("expected failure count %d, got %d", workers*iterations, count)
	}

	plugin.resetFailure()

	count = plugin.GetFailureCount()
	if count != 0 {
		t.Errorf("expected failure count 0 after reset, got %d", count)
	}

	if plugin.IsCircuitOpen() {
		t.Error("expected circuit breaker to be closed after reset")
	}
}

func TestRemoteNodePlugin_CircuitBreakerOpen(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid-host-for-testing").(*RemoteNodePlugin)

	for i := 0; i < 5; i++ {
		plugin.recordFailure()
	}

	if !plugin.IsCircuitOpen() {
		t.Error("expected circuit breaker to be open after 5 failures")
	}

	status, _ := plugin.GetHealthStatus()
	if status != "unhealthy" {
		t.Errorf("expected health status 'unhealthy', got '%s'", status)
	}
}

func TestRemoteNodePlugin_CircuitBreakerClose(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid-host-for-testing").(*RemoteNodePlugin)

	for i := 0; i < 5; i++ {
		plugin.recordFailure()
	}

	if !plugin.IsCircuitOpen() {
		t.Fatal("expected circuit breaker to be open")
	}

	plugin.resetFailure()

	if plugin.IsCircuitOpen() {
		t.Error("expected circuit breaker to be closed after reset")
	}

	status, _ := plugin.GetHealthStatus()
	if status != "healthy" {
		t.Errorf("expected health status 'healthy' after reset, got '%s'", status)
	}
}

func TestRemoteNodePlugin_HealthCheckLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL).(*RemoteNodePlugin)

	if plugin.IsHealthCheckRunning() {
		t.Error("expected health check to not be running initially")
	}

	plugin.StartHealthCheck()

	time.Sleep(100 * time.Millisecond)

	if !plugin.IsHealthCheckRunning() {
		t.Error("expected health check to be running after StartHealthCheck")
	}

	plugin.StopHealthCheck()

	time.Sleep(100 * time.Millisecond)

	if plugin.IsHealthCheckRunning() {
		t.Error("expected health check to not be running after StopHealthCheck")
	}

	status, lastCheck := plugin.GetHealthStatus()
	if status != "healthy" {
		t.Errorf("expected healthy status after health check, got '%s'", status)
	}
	if lastCheck.IsZero() {
		t.Error("expected last health check time to be set")
	}
}

func TestRemoteNodePlugin_HealthCheckDoubleStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL).(*RemoteNodePlugin)

	plugin.StartHealthCheck()
	plugin.StartHealthCheck()

	time.Sleep(100 * time.Millisecond)

	plugin.StopHealthCheck()

	time.Sleep(100 * time.Millisecond)

	if plugin.IsHealthCheckRunning() {
		t.Error("expected health check to not be running after double start and stop")
	}
}

func TestRemoteNodePlugin_ConcurrentHealthCheckStartStop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL).(*RemoteNodePlugin)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.StartHealthCheck()
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.StopHealthCheck()
		}()
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	running := plugin.IsHealthCheckRunning()
	if running {
		t.Log("Note: health check still running after concurrent start/stop - this is acceptable due to race conditions")
	}

	plugin.StopHealthCheck()
	time.Sleep(100 * time.Millisecond)
	if plugin.IsHealthCheckRunning() {
		t.Error("health check should not be running after explicit stop")
	}
}

func TestRemoteNodePlugin_RaceConditionOnExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"output":{"content":"test"}}`))
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL).(*RemoteNodePlugin)

	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			req := &NodeExecutionRequest{
				SchemaVersion:  "v1",
				PipelineID:    "test-pipeline",
				NodeID:        "test-node",
				Implementation: "test",
				Input:         &NodeInput{Content: "test"},
			}

			_, err := plugin.Execute(ctx, req)
			if err != nil {
				atomic.AddInt32(&failureCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Success: %d, Failures: %d", successCount, failureCount)
}

func TestRemoteNodePlugin_GetFailureCountAtomic(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid").(*RemoteNodePlugin)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.recordFailure()
		}()
	}

	wg.Wait()

	count := plugin.GetFailureCount()
	if count != 100 {
		t.Errorf("expected exactly 100 failures, got %d", count)
	}
}

func TestRemoteNodePlugin_IsCircuitOpenAtomic(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid").(*RemoteNodePlugin)

	if plugin.IsCircuitOpen() {
		t.Error("expected circuit to be closed initially")
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.recordFailure()
		}()
	}

	wg.Wait()

	if !plugin.IsCircuitOpen() {
		t.Error("expected circuit to be open after failures")
	}

	plugin.resetFailure()

	if plugin.IsCircuitOpen() {
		t.Error("expected circuit to be closed after reset")
	}
}

func TestRemoteNodePlugin_HealthStatusProtection(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid").(*RemoteNodePlugin)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.GetHealthStatus()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.updateHealthStatus("healthy")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.updateHealthStatus("unhealthy")
		}()
	}

	wg.Wait()
}

func TestRemoteNodePlugin_ConcurrentReset(t *testing.T) {
	plugin := NewRemoteNodePlugin("http://invalid").(*RemoteNodePlugin)

	for i := 0; i < 10; i++ {
		plugin.recordFailure()
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plugin.resetFailure()
		}()
	}

	wg.Wait()

	if plugin.GetFailureCount() != 0 {
		t.Error("expected failure count to be 0 after concurrent reset")
	}
}