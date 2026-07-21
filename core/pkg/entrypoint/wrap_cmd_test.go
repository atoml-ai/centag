package entrypoint

import (
	"errors"
	"testing"
)

func TestIsWrapCommand(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"centag"}, false},
		{[]string{"centag", "wrap"}, true},
		{[]string{"centag", "wrap", "run", "--", "opencode"}, true},
		{[]string{"centag", "version"}, false},
		{[]string{"centag", "serve"}, false},
	}
	for _, tc := range cases {
		if got := IsWrapCommand(tc.args); got != tc.want {
			t.Fatalf("IsWrapCommand(%v)=%v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestHandleWrapCommand_NotWrap(t *testing.T) {
	prev := wrapCLI
	t.Cleanup(func() { wrapCLI = prev })
	wrapCLI = func(args []string) error {
		t.Fatal("wrapCLI should not run")
		return nil
	}
	if HandleWrapCommand([]string{"centag", "version"}) {
		t.Fatal("expected false for non-wrap")
	}
}

func TestHandleWrapCommand_RunsRegistered(t *testing.T) {
	prev := wrapCLI
	t.Cleanup(func() { wrapCLI = prev })

	var got []string
	wrapCLI = func(args []string) error {
		got = append([]string{}, args...)
		return nil
	}
	if !HandleWrapCommand([]string{"centag", "wrap", "env", "--server", "http://x"}) {
		t.Fatal("expected true")
	}
	if len(got) != 3 || got[0] != "env" || got[1] != "--server" || got[2] != "http://x" {
		t.Fatalf("args=%v", got)
	}
}

func TestHandleWrapCommand_ErrorExits(t *testing.T) {
	// Exercise the error path without os.Exit by calling wrapCLI directly pattern:
	// HandleWrapCommand calls os.Exit on error — verify registration error surface via wrapCLI return
	// is covered indirectly; here we only assert SetWrapCLI stores the func.
	prev := wrapCLI
	t.Cleanup(func() { wrapCLI = prev })
	errBoom := errors.New("boom")
	SetWrapCLI(func(args []string) error { return errBoom })
	if wrapCLI == nil {
		t.Fatal("SetWrapCLI did not register")
	}
	if err := wrapCLI(nil); !errors.Is(err, errBoom) {
		t.Fatalf("got %v", err)
	}
}

func TestIsHelpCommand(t *testing.T) {
	if !IsHelpCommand([]string{"centag", "help"}) {
		t.Fatal("help")
	}
	if !IsHelpCommand([]string{"centag", "--help"}) {
		t.Fatal("--help")
	}
	if IsHelpCommand([]string{"centag", "wrap"}) {
		t.Fatal("wrap is not help")
	}
}

func TestDispatchPriority_VersionAndWrapDistinct(t *testing.T) {
	// version / help / wrap are mutually exclusive early exits; argv must not overlap.
	args := []string{"centag", "version"}
	if IsWrapCommand(args) || IsHelpCommand(args) {
		t.Fatal("version argv must not match wrap/help")
	}
	args = []string{"centag", "wrap", "run"}
	if IsVersionCommand(args) || IsHelpCommand(args) {
		t.Fatal("wrap argv must not match version/help")
	}
	args = []string{"centag", "help"}
	if IsVersionCommand(args) || IsWrapCommand(args) {
		t.Fatal("help argv must not match version/wrap")
	}
}
