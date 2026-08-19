package pipeline

// ScheduleRequest is passed to the optional scheduler hook.
type ScheduleRequest struct {
	Question       string
	RequestedModel string
	Strategy       string
	// 分类器配置（从流水线节点 CustomConfig 读取）
	ClassifyBackend string // 分类后端 ID
	ClassifyModel   string // 分类模型
	ClassifyPrompt  string // 自定义分类提示词
}

// ScheduleResult carries the scheduler decision back to the pipeline node.
type ScheduleResult struct {
	BackendID          string
	Model              string
	Reason             string
	TaskType           string
	EstimatedCost      float64
	EstimatedLatencyMs int64
}

// ScheduleBackend optionally selects a backend for builtin.scheduler nodes.
// Wired from server startup to avoid an import cycle with the scheduler package.
var ScheduleBackend func(req ScheduleRequest) (*ScheduleResult, error)