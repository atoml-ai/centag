package auth

// BudgetChecker checks whether an API key has sufficient budget remaining.
type BudgetChecker struct{}

// CheckBudgetResult holds the result of a budget check.
type CheckBudgetResult struct {
	OK        bool
	Remaining float64
}

// Check verifies that used_usd has not exceeded budget_usd.
// budgetUSD == 0 means unlimited (no cap).
func (BudgetChecker) Check(usedUSD, budgetUSD float64) CheckBudgetResult {
	if budgetUSD == 0 {
		return CheckBudgetResult{OK: true, Remaining: 0}
	}
	if usedUSD >= budgetUSD {
		return CheckBudgetResult{OK: false, Remaining: 0}
	}
	return CheckBudgetResult{OK: true, Remaining: budgetUSD - usedUSD}
}
