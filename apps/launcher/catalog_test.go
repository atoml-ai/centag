package main

import (
	"strings"
	"testing"
	"time"
)

const sampleCatalogJSON = `{"apps":[
 {"id":"opencode","display_name":"OpenCode","vendor":"OpenCode","launch_mode":"wrap_run","argv":["opencode"],"installed":true,"path":"/usr/local/bin/opencode"},
 {"id":"codex","display_name":"Codex CLI","vendor":"OpenAI","launch_mode":"wrap_run","argv":["codex"],"installed":false},
 {"id":"claude-code","display_name":"Claude Code","vendor":"Anthropic","launch_mode":"wrap_run","argv":["claude"],"installed":true,"path":"/usr/local/bin/claude"}
]}`

func TestParseCatalog(t *testing.T) {
	apps, err := parseCatalog([]byte(sampleCatalogJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 3 {
		t.Fatalf("got %d apps", len(apps))
	}
	if apps[0].ID != "opencode" || !apps[0].Installed || apps[0].Argv[0] != "opencode" {
		t.Fatalf("bad first entry %#v", apps[0])
	}
}

func TestParseCatalog_Malformed(t *testing.T) {
	if _, err := parseCatalog([]byte("{not json")); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestInstalledApps(t *testing.T) {
	apps, _ := parseCatalog([]byte(sampleCatalogJSON))
	inst := installedApps(apps)
	if len(inst) != 2 {
		t.Fatalf("got %d installed", len(inst))
	}
	// Sorted by Vendor: Anthropic (claude-code) before OpenCode.
	if inst[0].ID != "claude-code" || inst[1].ID != "opencode" {
		t.Fatalf("unexpected order: %s, %s", inst[0].ID, inst[1].ID)
	}
}

func TestListCatalogApps_EmptyBinary(t *testing.T) {
	if _, err := listCatalogApps("  "); err == nil {
		t.Fatal("expected error for empty binary")
	}
}

// TestCatalogCache covers TC-DET-006: 30s TTL means one fetch per window.
func TestCatalogCache(t *testing.T) {
	prevFetch, prevNow := catalogFetch, catalogNow
	t.Cleanup(func() {
		catalogFetch, catalogNow = prevFetch, prevNow
		appCatalogCache = catalogCacheState{}
	})

	fetches := 0
	base := time.Unix(1000, 0)
	catalogNow = func() time.Time { return base }
	catalogFetch = func(string) ([]catalogApp, error) {
		fetches++
		return []catalogApp{{ID: "x"}}, nil
	}

	t.Run("TC-DET-006", func(t *testing.T) {
		if _, err := listCatalogAppsCached("/bin/x"); err != nil {
			t.Fatal(err)
		}
		if _, err := listCatalogAppsCached("/bin/x"); err != nil {
			t.Fatal(err)
		}
		if fetches != 1 {
			t.Fatalf("expected 1 fetch within TTL, got %d", fetches)
		}
		base = base.Add(31 * time.Second)
		if _, err := listCatalogAppsCached("/bin/x"); err != nil {
			t.Fatal(err)
		}
		if fetches != 2 {
			t.Fatalf("expected refetch after TTL, got %d", fetches)
		}
	})
}

func TestBuildWrapRunArgvCommand(t *testing.T) {
	a := &launcherApp{
		cfg: Config{Port: 20060},
		hub: &sidecarHub{binary: "/opt/centag/centag-personal"},
	}
	cmd := buildWrapRunArgvCommand(a, []string{"opencode"})
	for _, want := range []string{"wrap run", "--server", "--", "opencode", "20060"} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("missing %q in %q", want, cmd)
		}
	}
}
