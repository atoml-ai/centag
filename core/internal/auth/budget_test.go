package auth

import (
	"testing"
)

func TestBudgetChecker_Unlimited(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(100, 0)
	if !r.OK {
		t.Error("budget_usd == 0 should be unlimited")
	}
	if r.Remaining != 0 {
		t.Errorf("remaining should be 0 for unlimited, got %f", r.Remaining)
	}
}

func TestBudgetChecker_WithinBudget(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(50, 100)
	if !r.OK {
		t.Error("used_usd < budget_usd should pass")
	}
	if r.Remaining != 50 {
		t.Errorf("remaining should be 50, got %f", r.Remaining)
	}
}

func TestBudgetChecker_ExactBudget(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(100, 100)
	if r.OK {
		t.Error("used_usd == budget_usd should fail")
	}
	if r.Remaining != 0 {
		t.Errorf("remaining should be 0, got %f", r.Remaining)
	}
}

func TestBudgetChecker_OverBudget(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(150, 100)
	if r.OK {
		t.Error("used_usd > budget_usd should fail")
	}
	if r.Remaining != 0 {
		t.Errorf("remaining should be 0, got %f", r.Remaining)
	}
}

func TestBudgetChecker_ZeroUsed(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(0, 50)
	if !r.OK {
		t.Error("used_usd == 0 and budget_usd > 0 should pass")
	}
	if r.Remaining != 50 {
		t.Errorf("remaining should be 50, got %f", r.Remaining)
	}
}

func TestBudgetChecker_Fractional(t *testing.T) {
	var bc BudgetChecker
	r := bc.Check(0.0015, 1.0)
	if !r.OK {
		t.Error("small fractional usage within budget should pass")
	}
	if r.Remaining < 0.9984 || r.Remaining > 0.9986 {
		t.Errorf("remaining should be ~0.9985, got %f", r.Remaining)
	}
}
