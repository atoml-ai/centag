package evolution

import (
	"encoding/json"
	"fmt"
)

// DefaultLoopBudget 生产默认预算（TC-EVO-008）：NewRuntime 自动注入，
// 每会话 50 次迭代 / 100k Token（粗粒度护栏，精确用量以 evol-tag 分桶为准）；
// 0 值维度 = 不限，可经 SetBudget 覆盖或关闭。
const (
	DefaultMaxIterations = 50
	DefaultMaxTokens     = 100000
)

// LoopBudget 单 session 循环预算（TC-EVO-008：循环次数 + Token 上限，超限熔断）。
// 权限语义：null budget = 不限；Charge 超限→ error 并一次性落盘预算报告。
type LoopBudget struct {
	MaxIterations int
	MaxTokens     int

	usedIterations int
	usedTokens     int
	reported       bool
	dataDir        string
}

// NewLoopBudget 构造预算（dataDir 用于超限报告落盘 var/agent/evolution/）。
func NewLoopBudget(maxIterations, maxTokens int, dataDir string) *LoopBudget {
	return &LoopBudget{MaxIterations: maxIterations, MaxTokens: maxTokens, dataDir: dataDir}
}

// Charge 记账：超限时返回熔断错误（错误文案自带预算报告），报告一次性落盘。
func (b *LoopBudget) Charge(iterations, tokens int) error {
	if b == nil {
		return nil
	}
	b.usedIterations += iterations
	b.usedTokens += tokens
	if (b.MaxIterations > 0 && b.usedIterations > b.MaxIterations) ||
		(b.MaxTokens > 0 && b.usedTokens > b.MaxTokens) {
		if !b.reported && b.dataDir != "" {
			_ = b.persistReport()
			b.reported = true
		}
		return fmt.Errorf("evolution 循环预算熔断：%s", b.Report())
	}
	return nil
}

// Report 预算报告文本（超限熔断输出）。
func (b *LoopBudget) Report() string {
	return fmt.Sprintf("iteration=%d/%d tokens_used=%d/max=%d；建议人工复核本次会话的优化提案（var/agent/evolution/ 下留痕），后续以人工批准为准",
		b.usedIterations, b.MaxIterations, b.usedTokens, b.MaxTokens)
}

// Used 当前用量（观测用）。
func (b *LoopBudget) Used() (iterations, tokens int) { return b.usedIterations, b.usedTokens }

// persistReport 报告落盘（审计可回溯；失败只忽略——主链路错误已返回）。
func (b *LoopBudget) persistReport() error {
	doc := map[string]any{
		"max_iterations":  b.MaxIterations,
		"max_tokens":      b.MaxTokens,
		"used_iterations": b.usedIterations,
		"used_tokens":     b.usedTokens,
		"reason":          "budget exceeded",
		"created_at":      now(),
	}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(jsonPath(b.dataDir, "budget-report.json"), buf); err != nil {
		return err
	}
	return nil
}
