package evolution

import (
	"context"
	"fmt"
)

// Surface 宿主呼叫面：诊断工具（观测面）注入 + 操作面由 Runtime 内置。
// 可与 edgeag evolution.Controller 组合驱动自进化闭环；本包 RunOnce 提供
// 最小可用闭环（不依赖 LLM 环节）。
//
// 预留 API：当前生产链路走 tools/evolution_ops.go 内置工具面（不经 Surface），
// Surface 仅供范式对齐（edgeag Controller 组合）与单测闭环使用。
type Surface struct {
	diag map[string]DiagnoseFn
	rt   *Runtime
	// confirmFn 宿主 confirm 闭环裁决（approved=人为确认）。
	confirmFn func(proposal *Proposal) (bool, error)
	// dryrunRequired manifest guardrail；默认 true（全量生效必经试跑）。
	dryrunRequired bool
	sessionID      string
}

// DiagnoseFn 诊断工具适配（观测面同名工具 read_state/read_metrics/read_log/read_database）。
type DiagnoseFn func(ctx context.Context, params map[string]any) (any, error)

// NewSurface 构造宿主工具面。
func NewSurface(rt *Runtime, sessionID string, diagnose map[string]DiagnoseFn) *Surface {
	return &Surface{rt: rt, sessionID: sessionID, diag: diagnose}
}

// SetConfirm 注入宿主 confirm 闭环裁决。
func (s *Surface) SetConfirm(fn func(proposal *Proposal) (bool, error)) { s.confirmFn = fn }

// Confirm 边界确认动作（宿主既有的确认通道统一入口）。
func (s *Surface) Confirm(ctx context.Context, p *Proposal) (bool, error) {
	if s.confirmFn == nil {
		return false, fmt.Errorf("evolution surface: confirm 闭环未注入（宿主确认通道缺失）")
	}
	return s.confirmFn(p)
}

// RunOnce 最小可用闭环：propose → dryrun → confirm（弃议 cancelled / 确认 applied）。
func (s *Surface) RunOnce(ctx context.Context, target string, params map[string]any, expectedEffect string) (*Proposal, error) {
	p, err := s.rt.Propose(ctx, s.sessionID, target, params, expectedEffect)
	if err != nil {
		return nil, err
	}
	if err := s.rt.Dryrun(ctx, p.ID); err != nil {
		return nil, fmt.Errorf("dryrun 未通过: %w", err)
	}
	approved, err := s.Confirm(ctx, p)
	if err != nil {
		return nil, err
	}
	if !approved {
		// Cancel 路径：配置不变，状态 canceled（TC-EVO-003）。
		if err := s.rt.Cancel(ctx, p.ID); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err := s.rt.Apply(ctx, p.ID, true); err != nil {
		return nil, err
	}
	return p, nil
}
