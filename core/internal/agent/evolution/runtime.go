package evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// Runtime 自进化宿主运行时：操作面状态机 + agent_evolution_log 仓储 +
// 目标参数域 + learning 归档。操作面内置注册（不经 MCP 对外）；
// confirm 闭环由宿主裁决入口（ConfirmSurface / UI 确认通道）接入。
type Runtime struct {
	store      *Store
	state      *TargetStateStore
	learn      *LearningArchive
	dryruns    map[string]bool // 已通过 dryrun 的提案 ID（dryrun_required 守卫）
	metric MetricSource
	// requireMetric 生产默认 true：无 MetricSource 时 dryrun 失败（P0-2）。
	requireMetric bool
	thresholds    MetricThresholds
	budget        *LoopBudget
}

// NewRuntime 构造运行时（driver: "sqlite"/"postgresql"；空则从 db 推断）。
// 首次调用兜底幂等建表（wiring 亦可显式 EnsureSchema）。
func NewRuntime(db *sql.DB, driver, dataDir string) *Runtime {
	if driver != "postgresql" {
		driver = driverFromDB(driver)
	}
	rt := &Runtime{
		store:      NewStore(db, driver),
		state:      NewTargetStateStore(dataDir),
		learn:      NewLearningArchive(dataDir),
		dryruns:    make(map[string]bool),
		metric:     DefaultMetricSource(),
		thresholds: DefaultMetricThresholds(),
		// 生产默认预算（TC-EVO-008）：每会话熔断护栏；SetBudget 可覆盖。
		budget: NewLoopBudget(DefaultMaxIterations, DefaultMaxTokens, dataDir),
		// P0-2：生产路径 fail-closed，无 MetricSource 时 dryrun 拒绝。
		requireMetric: true,
	}
	ensureOnce.Do(func() {
		if err := rt.EnsureSchema(context.Background()); err != nil {
			log.Printf("[EVOLUTION] ensure schema failed: %v", err)
		}
	})
	return rt
}

// EnsureSchema 幂等建表入口（wiring 调用；TC-DB-EVO-001）。
func (r *Runtime) EnsureSchema(ctx context.Context) error { return r.store.ensure(ctx) }

// JournalRow 返回提案的审计记录（状态/效果/回滚点），供宿主诊断与端到端
// 验收读取；提案不存在时返回错误。
func (r *Runtime) JournalRow(ctx context.Context, id string) (*LogRow, error) {
	return r.store.Get(ctx, id)
}

// SetTargetState 写入目标的当前生效参数快照（宿主启动时用于把真实配置面
// 镜像进 evolution 域，使首次提案的回滚点能恢复正确的宿主值）。
func (r *Runtime) SetTargetState(target string, state TargetKeyState) error {
	return r.state.Set(target, state)
}

// Propose 产出结构化提案（TC-EVO-001）：回滚点对本轮生效参数齐照快照；
// 目标白名单 + 参数 schema 校验失败则拒绝（不落库）。
func (r *Runtime) Propose(ctx context.Context, sessionID, target string, params map[string]any, expectedEffect string) (*Proposal, error) {
	p := &Proposal{
		ID:             proposeID(),
		Target:         target,
		Params:         params,
		ExpectedEffect: expectedEffect,
	}
	cur, err := r.state.Get(target)
	if err != nil {
		return nil, err
	}
	p.RollbackPoint = now()
	before := make(map[string]any, len(cur))
	for k, v := range cur {
		before[k] = v
	}
	p.ParamsBefore = before

	if err := ValidateTarget(target, params); err != nil {
		// TC-EVO-002：参数越界 → invalid 终态（提案本身已结构化，审计全量留存）
		b, merr := marshalProposal(p)
		if merr != nil {
			return nil, merr
		}
		row := &LogRow{ID: p.ID, SessionID: sessionID, Target: target, Status: StatusInvalid, Proposal: b}
		if ierr := r.store.Insert(ctx, row); ierr != nil {
			return nil, ierr
		}
		return nil, err
	}

	b, err := marshalProposal(p)
	if err != nil {
		return nil, err
	}
	row := &LogRow{ID: p.ID, SessionID: sessionID, Target: target, Status: StatusProposed, Proposal: b}
	if err := r.store.Insert(ctx, row); err != nil {
		return nil, err
	}
	return p, nil
}

// Dryrun 试算（不落任何 mutation）：无观测面注入时**拒绝**（P0-2 fail-closed，
// 防止"无度量放行"破坏闭环）；注入 MetricSource 后为"同窗口基线快照 + 试算指标"
// 两读（TC-EVO-005：指标回退超标 → dryrun 失败，dryrun 位不被置位，apply 被禁）。
func (r *Runtime) Dryrun(ctx context.Context, id string) error {
	p, err := r.load(ctx, id, StatusProposed)
	if err != nil {
		return err
	}
	if err := ValidateTarget(p.Target, p.Params); err != nil {
		return err
	}
	// P0-2：无指标源时 dryrun 必须失败（生产路径 fail-closed；测试可用
	// RequireMetric(false) 或注入 FakeMetricSource 规避）。
	if r.metric == nil && r.requireMetric {
		return fmt.Errorf("未注入 MetricSource，dryrun 拒绝（P0-2 fail-closed）")
	}
	if r.metric != nil {
		before, err := r.metric.Snapshot(ctx)
		if err != nil {
			return fmt.Errorf("dryrun 基线快照失败: %w", err)
		}
		measure, err := r.evaluateEffect(ctx, p, before)
		if err != nil {
			// 回退也留存审计（verdict=regression），但不置 dryrun 位。
			_ = r.storeMeasure(ctx, id, measure)
			return err
		}
		if err := r.storeMeasure(ctx, id, measure); err != nil {
			return err
		}
	}
	r.dryruns[id] = true
	return nil
}

// Confirm confirm 闭环裁决入口：approved → Apply；否则 Cancel（状态回滚、配置不变，
// TC-EVO-003）。
func (r *Runtime) Confirm(ctx context.Context, id string, approved bool, dryrunRequired bool) error {
	if !approved {
		return r.Cancel(ctx, id)
	}
	return r.Apply(ctx, id, dryrunRequired)
}

// Apply 应用提案：状态 proposed → applied（先要求 dryrun 通过，TC-EVO-005）。
func (r *Runtime) Apply(ctx context.Context, id string, dryrunRequired bool) error {
	p, err := r.load(ctx, id, StatusProposed)
	if err != nil {
		return err
	}
	if dryrunRequired && !r.dryruns[id] {
		return fmt.Errorf("提案 %s 必先通过 dryrun", id)
	}
	if err := ValidateTarget(p.Target, p.Params); err != nil {
		return fmt.Errorf("提案 %s 参数失效: %w", id, err)
	}
	if err := r.state.Set(p.Target, TargetKeyState(p.Params)); err != nil {
		return err
	}
	// P0-1：通过适配器写入真实宿主配置面；无适配器或应用失败均阻止状态迁移。
	if ap := HostAppliers.Get(p.Target); ap != nil {
		if err := ap.Apply(ctx, TargetKeyState(p.Params)); err != nil {
			_ = r.state.Set(p.Target, TargetKeyState(p.ParamsBefore))
			return fmt.Errorf("宿主应用失败（target=%s）: %w", p.Target, err)
		}
	} else {
		_ = r.state.Set(p.Target, TargetKeyState(p.ParamsBefore))
		return noApplierError(p.Target)
	}
	return r.store.UpdateStatus(ctx, id, StatusApplied, now(), "")
}

// Cancel 撤销提案（配置不变，TC-EVO-003）。
func (r *Runtime) Cancel(ctx context.Context, id string) error {
	p, err := r.load(ctx, id, StatusProposed)
	if err != nil {
		return err
	}
	_ = p
	return r.store.UpdateStatus(ctx, id, StatusCanceled, "", "")
}

// Rollback 一键回滚（TC-EVO-007）：恢复回滚点快照参数；applied → rolled_back。
func (r *Runtime) Rollback(ctx context.Context, appliedID string) error {
	row, err := r.store.Get(ctx, appliedID)
	if err != nil {
		return err
	}
	if row.Status != StatusApplied {
		return fmt.Errorf("提案 %s 未处于 applied（现 %q），不可回滚", appliedID, row.Status)
	}
	var p Proposal
	if err := unmarshalProposal(row.Proposal, &p); err != nil {
		return err
	}
	if err := r.state.Set(p.Target, TargetKeyState(p.ParamsBefore)); err != nil {
		return err
	}
	// P0-1：回滚同样走适配器，真实恢复宿主配置面。
	if ap := HostAppliers.Get(p.Target); ap != nil {
		if err := ap.Rollback(ctx, TargetKeyState(p.ParamsBefore)); err != nil {
			return fmt.Errorf("宿主回滚失败（target=%s）: %w", p.Target, err)
		}
	}
	return r.store.UpdateStatus(ctx, appliedID, StatusRolledBack, "", appliedID)
}

// RollbackOf 取回同提案的前置记录（线性恢复）。
func (r *Runtime) RollbackOf(ctx context.Context, row *LogRow) (*LogRow, error) {
	if row.RollbackOf == "" {
		return nil, fmt.Errorf("记录 %s 无关联回滚点", row.ID)
	}
	return r.store.Get(ctx, row.RollbackOf)
}

// RecordLearning 归档学习（learning 不受产品 confirm 链路约束，纯本地归档）。
func (r *Runtime) RecordLearning(ctx context.Context, proposalID string, content any) (string, error) {
	var measure any
	if row, err := r.store.Get(ctx, proposalID); err == nil && row.Effect != "" {
		_ = json.Unmarshal([]byte(row.Effect), &measure)
	}
	return r.learn.Record(proposalID, measure, content)
}

// LearningDir 相对 dataDir 的 learning 归档目录。
func (r *Runtime) LearningDir() string { return r.learn.Dir() }

// External helpers
func (r *Runtime) load(ctx context.Context, id, wantStatus string) (*Proposal, error) {
	row, err := r.store.Get(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("提案 %s 不存在", id)
	}
	if err != nil {
		return nil, err
	}
	if row.Status != wantStatus {
		return nil, fmt.Errorf("提案 %s 状态 %q ≠ %q", id, row.Status, wantStatus)
	}
	var p Proposal
	if err := unmarshalProposal(row.Proposal, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
