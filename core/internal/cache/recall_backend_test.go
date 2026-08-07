package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestUnconfiguredRecallBackend_MissNoFakeHit(t *testing.T) {
	tests := []struct {
		name     string
		backend  *UnconfiguredRecallBackend
		wantName string
	}{
		{name: "with plugin id", backend: &UnconfiguredRecallBackend{PluginID: "acme-kb"}, wantName: "acme-kb"},
		{name: "nil-safe default", backend: &UnconfiguredRecallBackend{}, wantName: "unconfigured-external"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.backend.Name() != tt.wantName {
				t.Fatalf("Name=%q want %q", tt.backend.Name(), tt.wantName)
			}
			if tt.backend.Kind() != "external" {
				t.Fatalf("Kind=%q", tt.backend.Kind())
			}
			hit, err := tt.backend.Lookup(context.Background(), RecallQuery{Key: "k", Text: "q"})
			if err == nil || hit != nil {
				t.Fatalf("Lookup must miss with error, hit=%v err=%v", hit, err)
			}
			if !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("err=%v", err)
			}
			if err := tt.backend.Store(context.Background(), RecallEntry{
				Key: "k", Request: "q", Response: "a", TTL: time.Minute,
			}); err == nil || !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("Store err=%v", err)
			}
		})
	}
}
