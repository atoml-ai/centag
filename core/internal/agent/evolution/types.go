package evolution

// 提案状态机：proposed → (confirmed→) applied | canceled
// invalid 为校验失败终态；applied → rolled_back（同日志行回滚）。
const (
	StatusProposed   = "proposed"
	StatusApplied    = "applied"
	StatusCanceled   = "canceled"
	StatusRolledBack = "rolled_back"
	StatusInvalid    = "invalid"
)

// Proposal 操作面结构化提案（与 edgeag evolution 提案工具名同契约）。
type Proposal struct {
	// ID 提案唯一标识（宿主生成）。
	ID string `json:"id"`
	// Target 提案目标（pipeline_weight/retry_policy/cache_ttl/backend_switch）。
	Target string `json:"target"`
	// Params 提案参数。
	Params map[string]any `json:"params"`
	// ExpectedEffect 预期效果描述。
	ExpectedEffect string `json:"expected_effect"`
	// RollbackPoint 回滚点（propose 时对当前生效参数拍照，RFC3339）。
	RollbackPoint string `json:"rollback_point"`
	// ParamsBefore 回滚快照（apply/rollback 恢复用，propose 时齐照）。
	ParamsBefore map[string]any `json:"params_before"`
}

// LogRow agent_evolution_log 一行。
type LogRow struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Proposal   []byte `json:"proposal"`
	AppliedAt  string `json:"applied_at,omitempty"`
	Effect     string `json:"effect_measure,omitempty"`
	RollbackOf string `json:"rollback_of,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
