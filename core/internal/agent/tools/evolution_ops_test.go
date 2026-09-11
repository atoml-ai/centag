package tools

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"centag/core/internal/agent/evolution"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
	_ "modernc.org/sqlite"
)

func evolutionOpsCtx() context.Context { return context.Background() }

// evolutionToolsByTest 面向用例构造：返回 name → tool 映射。
func evolutionToolsByTest(t testing.TB, rt *evolution.Runtime, session string, admin bool) map[string]agentcore.Tool {
	t.Helper()
	out := map[string]agentcore.Tool{}
	for _, tt := range NewEvolutionTools(rt, session, admin) {
		out[tt.Name()] = tt
	}
	return out
}

// TC-BILL-EVO-003 evolution 写操作默认 admin only：普通用户拒绝、admin 放行；
// dryrun/measure/learning 只读工具不设门槛。
func TestEvolutionOpsAdminOnly(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rt := evolution.NewRuntime(db, "sqlite", t.TempDir())
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	user := evolutionToolsByTest(t, rt, "sess-user", false)
	admin := evolutionToolsByTest(t, rt, "sess-admin", true)
	ctx := evolutionOpsCtx()

	// 普通用户：propose/apply/rollback 拒绝（403 语义）。
	denied := []struct {
		tool   agentcore.Tool
		params map[string]any
	}{
		{user["propose_change"], map[string]any{"target": "retry_policy", "params": map[string]any{"max_retries": 2.0}}},
		{user["apply_change"], map[string]any{"proposal_id": "ghost"}},
		{user["rollback_change"], map[string]any{"proposal_id": "ghost"}},
	}
	for _, d := range denied {
		res, err := d.tool.Execute(ctx, d.params)
		if err != nil || res == nil || !res.IsError || !strings.Contains(res.Content, "administrative access required") {
			t.Fatalf("普通用户 %s 应被 403 语义拒绝: %v / %+v", d.tool.Name(), err, res)
		}
	}
	// admin 放行：propose 成功（落库 proposed）。
	res, err := admin["propose_change"].Execute(ctx, map[string]any{"target": "retry_policy", "params": map[string]any{"max_retries": 2.0}})
	if err != nil || res.IsError {
		t.Fatalf("admin propose 应放行: %v / %+v", err, res)
	}
	// 只读工具普通用户可用：dryrun 拒绝的是 ghost（而非权限门）。
	res, err = user["dryrun_request"].Execute(ctx, map[string]any{"proposal_id": "ghost"})
	if res == nil || strings.Contains(res.Content, "administrative access required") {
		t.Fatalf("dryrun 不应触发权限门: %v / %+v", err, res)
	}
}
