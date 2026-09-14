package evolution

import "testing"

// proposeID must stay unique even when the system clock is too coarse to
// advance between rapid calls (e.g. Windows), otherwise the second proposal
// INSERT hits the primary key and is silently dropped.
func TestProposeIDUnique(t *testing.T) {
	const n = 10000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := proposeID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate propose id %q at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}
