package agentapi

import "testing"

func TestCopySystemProvidersToTenant_RequiresTenantID(t *testing.T) {
	n, err := CopySystemProvidersToTenant("")
	if err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
	if n != 0 {
		t.Fatalf("copied=%d want 0", n)
	}
}
