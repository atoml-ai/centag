package backend

import (
	"testing"
	"time"
)

func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()

	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	if collector.totalRequests.Load() != 0 {
		t.Errorf("TotalRequests = %d, want 0", collector.totalRequests.Load())
	}
}

func TestMetricsCollector_RecordSelection(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordSelection(
		"gpt-4",
		"backend1",
		StrategyHybrid,
		true,
		0.95,
		100*time.Millisecond,
	)

	total, exact, conversions, noMatches := collector.GetTotalMetrics()

	if total != 1 {
		t.Errorf("TotalRequests = %d, want 1", total)
	}

	if exact != 1 {
		t.Errorf("ExactMatches = %d, want 1", exact)
	}

	if conversions != 0 {
		t.Errorf("Conversions = %d, want 0", conversions)
	}

	if noMatches != 0 {
		t.Errorf("NoMatches = %d, want 0", noMatches)
	}

	// 检查模型指标
	modelMetrics := collector.GetModelMetrics("gpt-4")
	if modelMetrics.requestCount.Load() != 1 {
		t.Errorf("Model RequestCount = %d, want 1", modelMetrics.requestCount.Load())
	}

	// 检查后端指标
	backendMetrics := collector.GetBackendMetrics("backend1")
	if backendMetrics.selectedCount.Load() != 1 {
		t.Errorf("Backend SelectedCount = %d, want 1", backendMetrics.selectedCount.Load())
	}
}

func TestMetricsCollector_RecordExactMatch(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordExactMatch("gpt-4", "backend1")

	total, exact, conversions, _ := collector.GetTotalMetrics()

	if total != 0 {
		t.Errorf("TotalRequests = %d, want 0", total)
	}

	if exact != 1 {
		t.Errorf("ExactMatches = %d, want 1", exact)
	}

	if conversions != 0 {
		t.Errorf("Conversions = %d, want 0", conversions)
	}

	modelMetrics := collector.GetModelMetrics("gpt-4")
	if modelMetrics.exactMatchCount.Load() != 1 {
		t.Errorf("ExactMatchCount = %d, want 1", modelMetrics.exactMatchCount.Load())
	}
}

func TestMetricsCollector_RecordConversion(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordConversion("gpt-4", "qwen2.5:7b", "backend1", 0.75)

	total, _, conversions, _ := collector.GetTotalMetrics()

	if total != 0 {
		t.Errorf("TotalRequests = %d, want 0", total)
	}

	if conversions != 1 {
		t.Errorf("Conversions = %d, want 1", conversions)
	}

	modelMetrics := collector.GetModelMetrics("gpt-4")
	if modelMetrics.conversionCount.Load() != 1 {
		t.Errorf("ConversionCount = %d, want 1", modelMetrics.conversionCount.Load())
	}
}

func TestMetricsCollector_RecordNoMatch(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordNoMatch("unknown-model")

	total, _, _, noMatches := collector.GetTotalMetrics()

	if total != 0 {
		t.Errorf("TotalRequests = %d, want 0", total)
	}

	if noMatches != 1 {
		t.Errorf("NoMatches = %d, want 1", noMatches)
	}

	modelMetrics := collector.GetModelMetrics("unknown-model")
	if modelMetrics.noMatchCount.Load() != 1 {
		t.Errorf("NoMatchCount = %d, want 1", modelMetrics.noMatchCount.Load())
	}
}

func TestMetricsCollector_RecordBackendError(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordBackendError("backend1")

	backendMetrics := collector.GetBackendMetrics("backend1")
	if backendMetrics.errorCount.Load() != 1 {
		t.Errorf("ErrorCount = %d, want 1", backendMetrics.errorCount.Load())
	}
}

func TestMetricsCollector_StrategyUsage(t *testing.T) {
	collector := NewMetricsCollector()

	// 记录不同策略的选择
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)
	collector.RecordSelection("gpt-4", "backend2", StrategyHybrid, false, 0.85, 100*time.Millisecond)
	collector.RecordSelection("gpt-4", "backend3", StrategyFamily, false, 0.75, 100*time.Millisecond)

	usage := collector.GetStrategyUsage()

	if usage[StrategyExact] != 1 {
		t.Errorf("StrategyExact usage = %d, want 1", usage[StrategyExact])
	}

	if usage[StrategyHybrid] != 1 {
		t.Errorf("StrategyHybrid usage = %d, want 1", usage[StrategyHybrid])
	}

	if usage[StrategyFamily] != 1 {
		t.Errorf("StrategyFamily usage = %d, want 1", usage[StrategyFamily])
	}
}

func TestMetricsCollector_ResetMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 添加一些指标
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)
	collector.RecordConversion("gpt-4", "qwen2.5:7b", "backend1", 0.75)
	collector.RecordNoMatch("unknown-model")

	// 重置指标
	collector.ResetMetrics()

	total, exact, conversions, noMatches := collector.GetTotalMetrics()

	if total != 0 || exact != 0 || conversions != 0 || noMatches != 0 {
		t.Error("All metrics should be reset to 0")
	}

	// 检查策略使用也被重置
	usage := collector.GetStrategyUsage()
	for _, count := range usage {
		if count != 0 {
			t.Error("Strategy usage should be reset")
		}
	}
}

func TestModelMetrics_GetAverageScore(t *testing.T) {
	metrics := &ModelMetrics{}

	metrics.RecordSelection(true, 0.9, 100*time.Millisecond)
	metrics.RecordSelection(true, 0.8, 100*time.Millisecond)
	metrics.RecordSelection(true, 0.7, 100*time.Millisecond)

	avgScore := metrics.GetAverageScore()
	expected := (0.9 + 0.8 + 0.7) / 3.0

	// 允许小的浮点数误差
	if avgScore < expected-0.0001 || avgScore > expected+0.0001 {
		t.Errorf("AverageScore = %.6f, want %.6f", avgScore, expected)
	}
}

func TestModelMetrics_GetAverageLatency(t *testing.T) {
	metrics := &ModelMetrics{}

	metrics.RecordSelection(true, 1.0, 100*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 200*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 300*time.Millisecond)

	avgLatency := metrics.GetAverageLatency()
	expected := 200 * time.Millisecond

	if avgLatency != expected {
		t.Errorf("AverageLatency = %v, want %v", avgLatency, expected)
	}
}

func TestModelMetrics_GetMatchRate(t *testing.T) {
	metrics := &ModelMetrics{}

	// 记录 5 次请求，其中 1 次无匹配
	metrics.RecordSelection(true, 1.0, 100*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 100*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 100*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 100*time.Millisecond)
	metrics.noMatchCount.Store(1)
	metrics.requestCount.Store(5)

	matchRate := metrics.GetMatchRate()
	expected := 0.8

	if matchRate != expected {
		t.Errorf("MatchRate = %f, want %f", matchRate, expected)
	}
}

func TestModelMetrics_ConfidenceLevels(t *testing.T) {
	collector := NewMetricsCollector()

	// 高置信度
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)
	collector.RecordSelection("gpt-4", "backend2", StrategyHybrid, false, 0.85, 100*time.Millisecond)

	// 中置信度
	collector.RecordSelection("gpt-4", "backend3", StrategyHybrid, false, 0.6, 100*time.Millisecond)

	// 低置信度
	collector.RecordSelection("gpt-4", "backend4", StrategyHybrid, false, 0.4, 100*time.Millisecond)

	modelMetrics := collector.GetModelMetrics("gpt-4")

	if modelMetrics.highConfidence.Load() != 2 {
		t.Errorf("HighConfidence = %d, want 2", modelMetrics.highConfidence.Load())
	}

	if modelMetrics.mediumConfidence.Load() != 1 {
		t.Errorf("MediumConfidence = %d, want 1", modelMetrics.mediumConfidence.Load())
	}

	if modelMetrics.lowConfidence.Load() != 1 {
		t.Errorf("LowConfidence = %d, want 1", modelMetrics.lowConfidence.Load())
	}
}

func TestBackendMetrics_RecordSelection(t *testing.T) {
	metrics := &BackendMetrics{}

	metrics.RecordSelection(100 * time.Millisecond)
	metrics.RecordSelection(200 * time.Millisecond)
	metrics.RecordSelection(300 * time.Millisecond)

	if metrics.selectedCount.Load() != 3 {
		t.Errorf("SelectedCount = %d, want 3", metrics.selectedCount.Load())
	}
}

func TestBackendMetrics_GetAverageLatency(t *testing.T) {
	metrics := &BackendMetrics{}

	metrics.RecordSelection(100 * time.Millisecond)
	metrics.RecordSelection(200 * time.Millisecond)
	metrics.RecordSelection(300 * time.Millisecond)

	avgLatency := metrics.GetAverageLatency()
	expected := 200 * time.Millisecond

	if avgLatency != expected {
		t.Errorf("AverageLatency = %v, want %v", avgLatency, expected)
	}
}

func TestBackendMetrics_GetSelectionRate(t *testing.T) {
	metrics := &BackendMetrics{}

	metrics.totalRequests.Store(10)
	metrics.selectedCount.Store(8)

	selectionRate := metrics.GetSelectionRate()
	expected := 0.8

	if selectionRate != expected {
		t.Errorf("SelectionRate = %f, want %f", selectionRate, expected)
	}
}

func TestMetricsCollector_MultipleModels(t *testing.T) {
	collector := NewMetricsCollector()

	// 为不同模型记录指标
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)
	collector.RecordSelection("gpt-3.5", "backend2", StrategyHybrid, false, 0.85, 100*time.Millisecond)
	collector.RecordSelection("claude-3", "backend3", StrategyHybrid, false, 0.75, 100*time.Millisecond)

	// 检查每个模型的指标
	gpt4Metrics := collector.GetModelMetrics("gpt-4")
	if gpt4Metrics.requestCount.Load() != 1 {
		t.Errorf("gpt-4 RequestCount = %d, want 1", gpt4Metrics.requestCount.Load())
	}

	gpt35Metrics := collector.GetModelMetrics("gpt-3.5")
	if gpt35Metrics.requestCount.Load() != 1 {
		t.Errorf("gpt-3.5 RequestCount = %d, want 1", gpt35Metrics.requestCount.Load())
	}

	claudeMetrics := collector.GetModelMetrics("claude-3")
	if claudeMetrics.requestCount.Load() != 1 {
		t.Errorf("claude-3 RequestCount = %d, want 1", claudeMetrics.requestCount.Load())
	}
}

func TestMetricsCollector_MultipleBackends(t *testing.T) {
	collector := NewMetricsCollector()

	// 为不同后端记录指标
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)
	collector.RecordSelection("gpt-4", "backend2", StrategyHybrid, false, 0.85, 100*time.Millisecond)
	collector.RecordSelection("gpt-4", "backend1", StrategyExact, true, 1.0, 100*time.Millisecond)

	// 检查每个后端的指标
	backend1Metrics := collector.GetBackendMetrics("backend1")
	if backend1Metrics.selectedCount.Load() != 2 {
		t.Errorf("backend1 SelectedCount = %d, want 2", backend1Metrics.selectedCount.Load())
	}

	backend2Metrics := collector.GetBackendMetrics("backend2")
	if backend2Metrics.selectedCount.Load() != 1 {
		t.Errorf("backend2 SelectedCount = %d, want 1", backend2Metrics.selectedCount.Load())
	}
}

func TestModelMetrics_ScoreRanges(t *testing.T) {
	metrics := &ModelMetrics{}

	// 记录不同评分
	metrics.RecordSelection(true, 0.95, 100*time.Millisecond)
	metrics.RecordSelection(true, 0.5, 100*time.Millisecond)
	metrics.RecordSelection(true, 0.1, 100*time.Millisecond)

	// 因为字段是私有的,我们检查平均值来验证功能
	avgScore := metrics.GetAverageScore()
	expected := (0.95 + 0.5 + 0.1) / 3.0
	if avgScore < expected-0.01 || avgScore > expected+0.01 {
		t.Errorf("AverageScore = %f, want ~%f", avgScore, expected)
	}
}

func TestModelMetrics_LatencyRanges(t *testing.T) {
	metrics := &ModelMetrics{}

	// 记录不同延迟
	metrics.RecordSelection(true, 1.0, 50*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 200*time.Millisecond)
	metrics.RecordSelection(true, 1.0, 500*time.Millisecond)

	// 因为字段是私有的,我们检查平均值来验证功能
	avgLatency := metrics.GetAverageLatency()
	expected := (50 + 200 + 500) / 3 * time.Millisecond
	if avgLatency < expected-time.Millisecond || avgLatency > expected+time.Millisecond {
		t.Errorf("AverageLatency = %v, want ~%v", avgLatency, expected)
	}
}
