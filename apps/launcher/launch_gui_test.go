package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestWrapEnableArgs pins the `wrap enable` argv, including the --ca-only path
// used so GUI(Chromium) launches trust the CA without writing the OS proxy.
func TestWrapEnableArgs(t *testing.T) {
	cases := []struct {
		name  string
		force bool
		extra []string
		want  []string
	}{
		{"plain", false, nil, []string{"wrap", "enable"}},
		{"force", true, nil, []string{"wrap", "enable", "--force"}},
		{"ca-only", false, []string{"--ca-only"}, []string{"wrap", "enable", "--ca-only"}},
		{"force+extra", true, []string{"--ca-only"}, []string{"wrap", "enable", "--force", "--ca-only"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrapEnableArgs(tc.force, tc.extra...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("wrapEnableArgs=%v want %v", got, tc.want)
			}
		})
	}
}

// TestLaunchSystemProxyApp_NonChromiumFailsWithoutSystemProxy pins the red line:
// an app without a per-process proxy handle must NOT be launched by touching the
// OS system proxy — it fails with guidance instead.
func TestLaunchSystemProxyApp_NonChromiumFailsWithoutSystemProxy(t *testing.T) {
	err := launchSystemProxyApp(&launcherApp{}, catalogApp{ID: "foo", DisplayName: "Foo"})
	if !errors.Is(err, errNoPerProcessProxy) {
		t.Fatalf("got %v want errNoPerProcessProxy", err)
	}
	if !strings.Contains(err.Error(), "配置文件") {
		t.Fatalf("missing app-config guidance: %v", err)
	}
	if !strings.Contains(err.Error(), "wrap run") {
		t.Fatalf("missing wrap-run guidance: %v", err)
	}
}
