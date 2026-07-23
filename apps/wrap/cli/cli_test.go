package cli

import "testing"

func TestRun_RejectsUnknownCommand(t *testing.T) {
	if err := Run([]string{"rm", "-rf", "/"}); err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRun_RejectsUnknownFlag(t *testing.T) {
	if err := Run([]string{"enable", "--evil"}); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRun_Help(t *testing.T) {
	if err := Run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_MissingCommand(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCommonFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantS     string
		wantToken string
		wantErr   bool
	}{
		{"long", []string{"--server", "http://10.0.0.1:20060"}, "http://10.0.0.1:20060", "", false},
		{"equals", []string{"--server=http://x:1"}, "http://x:1", "", false},
		{"short", []string{"-s", "http://y:2"}, "http://y:2", "", false},
		{"token long", []string{"--token", "llmproxy_abc"}, "", "llmproxy_abc", false},
		{"token short", []string{"-t", "llmproxy_x"}, "", "llmproxy_x", false},
		{"token equals", []string{"--token=llmproxy_y"}, "", "llmproxy_y", false},
		{"both", []string{"--server", "http://h:1", "--token", "k"}, "http://h:1", "k", false},
		{"missing server value", []string{"--server"}, "", "", true},
		{"missing token value", []string{"--token"}, "", "", true},
		{"unknown", []string{"--nope"}, "", "", true},
		{"empty", nil, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := parseCommonFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if f.Server != tt.wantS || f.Token != tt.wantToken {
				t.Fatalf("got %+v want server=%q token=%q", f, tt.wantS, tt.wantToken)
			}
		})
	}
}

func TestParseRunArgs(t *testing.T) {
	f, argv, err := parseRunArgs([]string{"--server", "http://h:20060", "--token", "llmproxy_k", "--", "opencode", "-v"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Server != "http://h:20060" || f.Token != "llmproxy_k" {
		t.Fatalf("flags=%+v", f)
	}
	if len(argv) != 2 || argv[0] != "opencode" || argv[1] != "-v" {
		t.Fatalf("argv=%v", argv)
	}
	f, argv, err = parseRunArgs([]string{"opencode"})
	if err != nil || len(argv) != 1 || argv[0] != "opencode" || f.Token != "" {
		t.Fatalf("argv=%v flags=%+v err=%v", argv, f, err)
	}
}
