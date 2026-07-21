package entrypoint

import "testing"

func TestIsVersionCommand(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"centag"}, false},
		{[]string{"centag", "version"}, true},
		{[]string{"centag", "--version"}, true},
		{[]string{"centag", "-version"}, true},
		{[]string{"centag", "-V"}, true},
		{[]string{"centag", "-v"}, false},
		{[]string{"centag", "serve"}, false},
	}
	for _, tc := range cases {
		if got := IsVersionCommand(tc.args); got != tc.want {
			t.Fatalf("IsVersionCommand(%v)=%v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestNormalizeVersionLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"dev", "dev"},
		{"v0.2.7", "v0.2.7"},
		{"0.2.7", "v0.2.7"},
		{"v20260721-211854", "v20260721-211854"},
	}
	for _, tc := range cases {
		if got := normalizeVersionLabel(tc.in); got != tc.want {
			t.Fatalf("normalizeVersionLabel(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
