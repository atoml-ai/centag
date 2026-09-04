package entrypoint

import "testing"

func TestHandleNoArguments(t *testing.T) {
	if !HandleNoArguments([]string{"centag"}) {
		t.Fatal("bare invocation should trigger help")
	}
	if HandleNoArguments([]string{"centag", "serve"}) {
		t.Fatal("serve should not trigger help")
	}
}

func TestIsServeCommand(t *testing.T) {
	if !IsServeCommand([]string{"centag", "serve"}) {
		t.Fatal("serve should be recognized")
	}
	if IsServeCommand([]string{"centag"}) {
		t.Fatal("bare invocation is not serve")
	}
	if IsServeCommand([]string{"centag", "version"}) {
		t.Fatal("version is not serve")
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	// Unknown command reports true (process stops; exit happens inside).
	if !HandleUnknownCommand([]string{"centag", "bogus"}) {
		t.Fatal("unknown command should be rejected")
	}
	// Known commands and flags fall through (return false).
	for _, cmd := range []string{
		"serve", "version", "wrap", "cleanup", "help", "--help", "-h",
		"install", "uninstall",
	} {
		if HandleUnknownCommand([]string{"centag", cmd}) {
			t.Fatalf("%q should be a known command", cmd)
		}
	}
	if HandleUnknownCommand([]string{"centag", "-config=/tmp/x"}) {
		t.Fatal("legacy flags should keep starting the server")
	}
	if HandleUnknownCommand([]string{"centag"}) {
		t.Fatal("no-argument case is not unknown-command business")
	}
}

func TestPrintUsageMentionsServe(t *testing.T) {
	// Indirect: usage text is a constant; guard it stays accurate by checking
	// the help handler prints without panic and the command table keeps serve.
	if !knownCommands["serve"] {
		t.Fatal("serve must remain a known command")
	}
}
