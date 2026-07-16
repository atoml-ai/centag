package pipeline

// ScheduleRequest is passed to the optional scheduler hook.
type ScheduleRequest struct {
	Question       string
	RequestedModel string
	Strategy       string
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