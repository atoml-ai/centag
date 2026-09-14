package engine

import (
	"testing"

	"centag/apps/wrap/internal/snapshot"
)

func TestIsCentagProxy(t *testing.T) {
	cases := []struct {
		name string
		p    snapshot.ProxyState
		want bool
	}{
		{"centag pac", snapshot.ProxyState{Mode: "pac", PACURL: "http://127.0.0.1:20060/api/v1/proxy/pac"}, true},
		{"external pac", snapshot.ProxyState{Mode: "pac", PACURL: "http://proxy.corp/pac.js"}, false},
		{"external manual", snapshot.ProxyState{Mode: "manual", HTTP: "127.0.0.1:12000"}, false},
		{"centag mitm manual", snapshot.ProxyState{Mode: "manual", HTTP: "127.0.0.1:8081"}, true},
		{"centag api manual", snapshot.ProxyState{Mode: "manual", HTTP: "127.0.0.1:20060"}, true},
		{"remote host manual", snapshot.ProxyState{Mode: "manual", HTTP: "10.0.0.5:8081"}, false},
		{"off", snapshot.ProxyState{Mode: "off"}, false},
	}
	for _, tc := range cases {
		if got := isCentagProxy(tc.p); got != tc.want {
			t.Errorf("%s: isCentagProxy=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestFirstNonSelfManual(t *testing.T) {
	if got := firstNonSelfManual(snapshot.ProxyState{HTTP: "127.0.0.1:12000", HTTPS: "127.0.0.1:12000"}); got != "127.0.0.1:12000" {
		t.Errorf("got %q, want 127.0.0.1:12000", got)
	}
	if got := firstNonSelfManual(snapshot.ProxyState{HTTP: "127.0.0.1:8081"}); got != "" {
		t.Errorf("self manual should be skipped, got %q", got)
	}
}
