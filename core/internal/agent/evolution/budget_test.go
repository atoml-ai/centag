package evolution

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TC-EVO-008 循环预算熔断：超限 → Charge 报错（含预算报告文案）+ 报告一次性落盘。
func TestBudgetCircuitBreaks(t *testing.T) {
	dir := t.TempDir()
	db := testDB(t)
	rt := NewRuntime(db, "sqlite", dir)
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	rt.SetBudget(NewLoopBudget(2, 10000, dir))

	if err := rt.Charge(1, 100); err != nil {
		t.Fatalf("第 1 次记账不应熔断: %v", err)
	}
	if err := rt.Charge(1, 100); err != nil {
		t.Fatalf("第 2 次记账不应熔断: %v", err)
	}
	err := rt.Charge(1, 0)
	if err == nil {
		t.Fatal("第 3 次应熔断")
	}
	if !strings.Contains(err.Error(), "iteration=3/2") {
		t.Fatalf("预算报告文案缺失: %v", err)
	}
	// 报告已落盘（var/agent/evolution/budget-report.json）。
	b, rerr := os.ReadFile(jsonPath(dir, "budget-report.json"))
	if rerr != nil {
		t.Fatalf("预算报告未落盘: %v", rerr)
	}
	if !strings.Contains(string(b), `"used_iterations": 3`) {
		t.Fatalf("报告内容异常: %s", b)
	}
	// 二次熔断不再重复写盘（reported 旗标）。
	_ = os.Remove(jsonPath(dir, "budget-report.json"))
	if err := rt.Charge(0, 0); err == nil {
		t.Fatal("已熔断后续 Charge 应持续拒绝")
	}
	if _, err := os.Stat(jsonPath(dir, "budget-report.json")); !os.IsNotExist(err) {
		t.Fatal("报告不应重复落盘")
	}
	// 用量观测。
	iters, tokens := rt.budget.Used()
	if iters != 3 || tokens != 200 {
		t.Fatalf("usage = %d/%d", iters, tokens)
	}
}

// Token 上限单独触发熔断。
func TestBudgetTokenLimit(t *testing.T) {
	b := NewLoopBudget(0, 1000, t.TempDir())
	if err := b.Charge(1, 600); err != nil {
		t.Fatal(err)
	}
	if err := b.Charge(1, 600); err == nil || !strings.Contains(err.Error(), "tokens_used=1200/max=1000") {
		t.Fatalf("token 超限未熔断: %v", err)
	}
}

// 全 0 预算 = 双态关闭（不熔断）。
func TestBudgetUnlimited(t *testing.T) {
	b := NewLoopBudget(0, 0, t.TempDir())
	for i := 0; i < 100; i++ {
		if err := b.Charge(1, 500); err != nil {
			t.Fatalf("无上限不应熔断: %v", err)
		}
	}
	// nil runner 面也同样直通。
	var nb *LoopBudget
	if err := nb.Charge(5, 5); err != nil {
		t.Fatal(err)
	}
}
