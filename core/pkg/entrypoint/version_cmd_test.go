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
