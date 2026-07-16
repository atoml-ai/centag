package scheduler

import (
	"time"
)

// AuditConfig 审核模式配置
type AuditConfig struct {
	ExecutorBackendID string `json:"executor_backend"`    // 执行后端 ID
	AuditorBackendID  string `json:"auditor_backend"`     // 审核后端 ID
	AuditorModel      string `json:"auditor_model"`       // 审核模型名称
	AuditPrompt       string `json:"audit_prompt"`        // 审核 Prompt 模板
	AutoRetry         bool   `json:"auto_retry"`          // 审核不通过自动重试
	MaxRetries        int    `json:"max_retries"`         // 最大重试次数
	BypassOnTimeout   bool   `json:"bypass_on_timeout"`   // 审核超时是否绕过
	AuditTimeoutSec   int    `json:"audit_timeout_sec"`   // 审核超时时间 (秒)
}

// AuditResult 审核结果
type AuditResult struct {
	Passed      bool     `json:"passed"`        // 是否通过
	Score       float64  `json:"score"`         // 审核评分 0-1
	Feedback    string   `json:"feedback"`      // 审核反馈
	Suggestions []string `json:"suggestions"`   // 改进建议
	Issues      []string `json:"issues"`        // 发现的问题
	RawResponse string   `json:"raw_response"`  // 原始响应
	DurationMs  int64    `json:"duration_ms"`   // 审核耗时
}

// AuditDecision 审核决策
type AuditDecision struct {
	ExecutorBackendID string       `json:"executor_backend_id"`
	AuditorBackendID  string       `json:"auditor_backend_id"`
	ExecutorModel     string       `json:"executor_model"`
	AuditorModel      string       `json:"auditor_model"`
	OriginalAnswer    string       `json:"original_answer"`
	AuditResult       *AuditResult `json:"audit_result"`
	FinalAnswer       string       `json:"final_answer"`
	RetryCount        int          `json:"retry_count"`
	Action            string       `json:"action"` // pass/retry/bypass/reject
	Reason            string       `json:"reason"`
}

// DefaultAuditConfig 返回默认审核配置
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		ExecutorBackendID: "bigmodel",
		AuditorBackendID:  "bigmodel",
		AuditorModel:      "glm-5",
		AuditPrompt:       DefaultAuditPrompt,
		AutoRetry:         true,
		MaxRetries:        2,
		BypassOnTimeout:   true,
		AuditTimeoutSec:   30,
	}
}

// DefaultAuditPrompt 默认审核 Prompt 模板
const DefaultAuditPrompt = `你是一名专业的内容审核员。请审核以下 AI 助手的回答质量。

## 用户问题
{{.question}}

## AI 回答
{{.answer}}

## 审核维度
1. **准确性**: 回答是否准确、无事实错误
2. **完整性**: 是否完整回答了用户的问题
3. **安全性**: 是否包含有害、违法、不当内容
4. **有用性**: 回答是否对用户有实际帮助
5. **逻辑性**: 推理过程是否清晰、逻辑是否自洽

## 评分标准
- score >= 0.8: 通过 (passed=true)
- score < 0.8: 不通过 (passed=false)，需要改进

## 输出格式
请严格返回 JSON 格式，不要其他内容：
{
    "passed": true/false,
    "score": 0.0-1.0,
    "feedback": "审核反馈说明",
    "suggestions": ["改进建议 1", "改进建议 2"],
    "issues": ["发现的问题 1", "发现的问题 2"]
}

## 审核时间
{{.timestamp}}
`

// AuditAction 审核动作枚举
type AuditAction string

const (
	AuditActionPass   AuditAction = "pass"   // 审核通过
	AuditActionRetry  AuditAction = "retry"  // 需要重试
	AuditActionBypass AuditAction = "bypass" // 绕过审核
	AuditActionReject AuditAction = "reject" // 审核拒绝
)

// AuditStats 审核统计
type AuditStats struct {
	TotalAudits      int64   `json:"total_audits"`
	PassedCount      int64   `json:"passed_count"`
	FailedCount      int64   `json:"failed_count"`
	BypassedCount    int64   `json:"bypassed_count"`
	RetryCount       int64   `json:"retry_count"`
	AvgScore         float64 `json:"avg_score"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
	PassRate         float64 `json:"pass_rate"`
	LastUpdated      time.Time
}

// UpdateStats 更新审核统计
func (s *AuditStats) UpdateStats(result *AuditResult, durationMs int64, action AuditAction) {
	s.TotalAudits++
	s.LastUpdated = time.Now()

	switch action {
	case AuditActionPass:
		s.PassedCount++
	case AuditActionRetry:
		s.RetryCount++
	case AuditActionBypass:
		s.BypassedCount++
	case AuditActionReject:
		s.FailedCount++
	}

	// 更新平均评分
	totalScore := float64(s.TotalAudits-1) * s.AvgScore
	s.AvgScore = (totalScore + result.Score) / float64(s.TotalAudits)

	// 更新平均耗时
	totalDuration := float64(s.TotalAudits-1) * s.AvgDurationMs
	s.AvgDurationMs = (totalDuration + float64(durationMs)) / float64(s.TotalAudits)

	// 计算通过率
	if s.TotalAudits > 0 {
		s.PassRate = float64(s.PassedCount) / float64(s.TotalAudits)
	}
}
