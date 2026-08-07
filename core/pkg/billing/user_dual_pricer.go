package billing

import (
	"context"
	"sync/atomic"
)

// UserDualPricer applies per-user (or per-group via policy) price overrides
// onto a dual cost/revenue estimate. Team edition registers an implementation
// that consults group_pricing_overrides / user_pricing_overrides.
type UserDualPricer interface {
	// ApplyOverrides mutates cost/revenue breakdowns when an override applies.
	// It must leave fields untouched when no override matches.
	ApplyOverrides(
		ctx context.Context,
		userID int64,
		backendID, model string,
		inputTokens, outputTokens int,
		cost, revenue *CostBreakdown,
	)
}

// atomic.Value cannot Store(nil); wrap the interface so clear is possible.
type userDualPricerBox struct {
	p UserDualPricer
}

var userDualPricer atomic.Value // stores userDualPricerBox

// SetUserDualPricer injects the Team user/group override applier (or nil to clear).
func SetUserDualPricer(p UserDualPricer) {
	userDualPricer.Store(userDualPricerBox{p: p})
}

// GetUserDualPricer returns the registered applier, or nil.
func GetUserDualPricer() UserDualPricer {
	v := userDualPricer.Load()
	if v == nil {
		return nil
	}
	box, ok := v.(userDualPricerBox)
	if !ok {
		return nil
	}
	return box.p
}
