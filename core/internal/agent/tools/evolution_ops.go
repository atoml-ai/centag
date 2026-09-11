package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"centag/core/internal/agent/evolution"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

// EvolutionRuntime 返回引擎级共享的 evolution 运行时（NewRuntime 首调兜底幂等建表）。
func EvolutionRuntime(db *sql.DB, driver, dataDir string) *evolution.Runtime {
	return evolution.NewRuntime(db, driver, dataDir)
}

// evolutionTool 通用包装：把 evolution.Runtime 操作暴露为 agentcore.Tool
// （操作面内置注册，不经 MCP 对外；工具名即契约）。
type evolutionTool struct {
	toolName string
	rt       *evolution.Runtime
	desc     string
	schema   map[string]any
	readonly bool
	// 权限门（TC-BILL-EVO-003）：adminRequired 工具在非 admin 会话直接拒绝。
	adminRequired bool
	isAdmin       bool
	exec          func(ctx context.Context, params map[string]any) (any, error)
}

// tokenCost 记账估算：params 序列化长度/4（循环预算以 Token 计）。
func tokenCost(params map[string]any) int {
	b, err := json.Marshal(params)
	if err != nil {
		return 256
	}
	n := len(b) / 4
	if n < 64 {
		n = 64
	}
	return n
}

func (t *evolutionTool) Name() string        { return t.toolName }
func (t *evolutionTool) Description() string { return t.desc }
func (t *evolutionTool) IsReadOnly() bool    { return t.readonly }
func (t *evolutionTool) ParamSchema() map[string]any {
	return t.schema
}

func (t *evolutionTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	if t.adminRequired && !t.isAdmin {
		return &agentcore.ToolResult{Content: "administrative access required（evolution 写操作默认 admin only）", IsError: true}, nil
	}
	if err := t.rt.Charge(1, tokenCost(params)); err != nil {
		return &agentcore.ToolResult{Content: err.Error(), IsError: true}, nil
	}
	out, err := t.exec(ctx, params)
	if err != nil {
		return &agentcore.ToolResult{Content: err.Error(), IsError: true}, nil
	}
	b, jerr := json.Marshal(out)
	if jerr != nil {
		return nil, jerr
	}
	return &agentcore.ToolResult{Content: string(b), Details: out}, nil
}

// decodeObject 归一化任意提案参数形态（nil/map/string → map[string]any）。
func decodeObject(v any) (map[string]any, error) {
	switch t := v.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return t, nil
	case string:
		m := map[string]any{}
		if err := json.Unmarshal([]byte(t), &m); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("无法解码参数形态: %T", v)
	}
}

// NewEvolutionTools 自进化操作面内置工具：
// propose_change / dryrun_request / apply_change / measure_effect / rollback_change / record_learning。
// 建表（agent_evolution_log）由调用方提前 evolution.EnsureSchema。
// isAdmin=false 时写操作（propose/apply/rollback）拒绝（TC-BILL-EVO-003：
// evolution 触发默认 admin only；dryrun/measure/learning 只读不设门槛）。
func NewEvolutionTools(rt *evolution.Runtime, sessionID string, isAdmin bool) []agentcore.Tool {
	propose := &evolutionTool{
		toolName:      "propose_change",
		adminRequired: true,
		isAdmin:       isAdmin,
		rt:            rt,
		desc:          "产出结构化优化提案（目标/参数/预期效果/回滚点），等待确认后才会应用",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target":          map[string]any{"type": "string", "enum": []string{"pipeline_weight", "retry_policy", "cache_ttl", "backend_switch"}},
				"params":          map[string]any{"type": "object"},
				"expected_effect": map[string]any{"type": "string"},
			},
			"required": []string{"target", "params"},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			target, _ := params["target"].(string)
			effect, _ := params["expected_effect"].(string)
			pm, err := decodeObject(params["params"])
			if err != nil {
				return nil, err
			}
			return rt.Propose(ctx, sessionID, target, pm, effect)
		},
	}
	dryrun := &evolutionTool{
		toolName: "dryrun_request",
		rt:       rt,
		desc:     "对提案做试算/试跑（不产生任何变更），返回试跑结论",
		readonly: true,
		schema: map[string]any{
			"type":     "object",
			"required": []string{"proposal_id"},
			"properties": map[string]any{
				"proposal_id": map[string]any{"type": "string"},
			},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["proposal_id"].(string)
			if err := rt.Dryrun(ctx, id); err != nil {
				return nil, err
			}
			return map[string]any{"proposal_id": id, "dryrun": "passed"}, nil
		},
	}
	apply := &evolutionTool{
		toolName:      "apply_change",
		adminRequired: true,
		isAdmin:       isAdmin,
		rt:            rt,
		desc:          "应用已确认提案（必经宿主人工确认闭环；须先通过 dryrun）",
		schema: map[string]any{
			"type":     "object",
			"required": []string{"proposal_id"},
			"properties": map[string]any{
				"proposal_id": map[string]any{"type": "string"},
			},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["proposal_id"].(string)
			if err := rt.Apply(ctx, id, true); err != nil {
				return nil, err
			}
			return map[string]any{"proposal_id": id, "status": "applied"}, nil
		},
	}
	rollback := &evolutionTool{
		toolName:      "rollback_change",
		adminRequired: true,
		isAdmin:       isAdmin,
		rt:            rt,
		desc:          "一键回滚已应用提案到回滚点快照（配置回到 apply 前状态）",
		schema: map[string]any{
			"type":     "object",
			"required": []string{"proposal_id"},
			"properties": map[string]any{
				"proposal_id": map[string]any{"type": "string"},
			},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["proposal_id"].(string)
			if err := rt.Rollback(ctx, id); err != nil {
				return nil, err
			}
			return map[string]any{"proposal_id": id, "status": "rolled_back"}, nil
		},
	}
	learn := &evolutionTool{
		toolName: "record_learning",
		rt:       rt,
		desc:     "将本次经验/结论归档到 var/agent/learning/（宿主归档，不经对外学习云）",
		schema: map[string]any{
			"type":     "object",
			"required": []string{"proposal_id", "content"},
			"properties": map[string]any{
				"proposal_id": map[string]any{"type": "string"},
				"content":     map[string]any{"type": "object"},
			},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["proposal_id"].(string)
			content, err := decodeObject(params["content"])
			if err != nil {
				return nil, err
			}
			path, err := rt.RecordLearning(ctx, id, content)
			if err != nil {
				return nil, err
			}
			return map[string]any{"proposal_id": id, "archive": path}, nil
		},
	}
	measure := &evolutionTool{
		toolName: "measure_effect",
		rt:       rt,
		desc:     "对提案做同指标/同窗口 effect 快照（如观测面已注入），返回快照内容",
		readonly: true,
		schema: map[string]any{
			"type":     "object",
			"required": []string{"proposal_id"},
			"properties": map[string]any{
				"proposal_id": map[string]any{"type": "string"},
			},
		},
		exec: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["proposal_id"].(string)
			snap, err := rt.MeasureEffects(ctx, id, "post")
			if err != nil {
				return nil, err
			}
			return snap, nil
		},
	}
	return []agentcore.Tool{propose, dryrun, apply, measure, rollback, learn}
}
