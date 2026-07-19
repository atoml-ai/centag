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

func TestParseServerFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"long", []string{"--server", "http://10.0.0.1:20060"}, "http://10.0.0.1:20060", false},
		{"equals", []string{"--server=http://x:1"}, "http://x:1", false},
		{"short", []string{"-s", "http://y:2"}, "http://y:2", false},
		{"missing value", []string{"--server"}, "", true},
		{"unknown", []string{"--nope"}, "", true},
		{"empty", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := parseServerFlag(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && s != tt.want {
				t.Fatalf("got %q want %q", s, tt.want)
			}
		})
	}
}
