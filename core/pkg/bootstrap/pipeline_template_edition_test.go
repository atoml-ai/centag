package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInitialPipelineTemplatesByEdition(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "config", "initdata", "pipeline-templates", "common")
	teamDir := filepath.Join(root, "config", "initdata", "pipeline-templates", "team")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(commonDir, "a.yaml", "id: common-pipe\nname: Common\nnodes: []\n")
	write(teamDir, "b.yaml", "id: team-pipe\nname: Team\nnodes: []\n")

	t.Setenv("PROJECT_ROOT", root)
	t.Setenv("INITDATA_PATH", "")

	ids := func(edition string) map[string]bool {
		out := map[string]bool{}
		for _, tmpl := range LoadInitialPipelineTemplatesWithEdition(edition) {
			out[tmpl.ID] = true
		}
		return out
	}

	personal := ids("personal")
	if !personal["common-pipe"] || personal["team-pipe"] {
		t.Fatalf("personal: want only common-pipe, got %v", personal)
	}
	minimal := ids("minimal")
	if !minimal["common-pipe"] || minimal["team-pipe"] {
		t.Fatalf("minimal: want only common-pipe, got %v", minimal)
	}
	team := ids("team")
	if !team["common-pipe"] || !team["team-pipe"] {
		t.Fatalf("team: want common-pipe+team-pipe, got %v", team)
	}
}
