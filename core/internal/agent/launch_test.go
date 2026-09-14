package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

type fakeTemplate struct {
	at   AgentType
	meta AgentSetupMeta
}

func (f fakeTemplate) AgentType() AgentType                           { return f.at }
func (f fakeTemplate) DisplayName() string                            { return string(f.at) }
func (f fakeTemplate) Description() string                            { return "" }
func (f fakeTemplate) Meta() AgentSetupMeta                           { return f.meta }
func (f fakeTemplate) ConfigFiles(*BackendInfo) ([]ConfigFile, error) { return nil, nil }
func (f fakeTemplate) SetupCommand(*BackendInfo) string               { return "" }
func (f fakeTemplate) PlatformCommands(*BackendInfo) PlatformCommands {
	return PlatformCommands{}
}
func (f fakeTemplate) VerifyCommand(*BackendInfo) string { return "" }
func (f fakeTemplate) Steps(*BackendInfo) []ConfigStep   { return nil }
func (f fakeTemplate) WriteConfig(*BackendInfo) error    { return nil }

func emptyRegistry() *TemplateRegistry {
	return &TemplateRegistry{templates: map[AgentType]AgentTemplate{}}
}

// TestProxyApps covers TC-CAT-001/003/004/006: derivation, dedup, skip, sort.
func TestProxyApps(t *testing.T) {
	t.Run("TC-CAT-001", func(t *testing.T) {
		apps := ProxyApps(NewTemplateRegistry())
		var oc *ProxyApp
		for i := range apps {
			if apps[i].ID == "opencode" {
				oc = &apps[i]
			}
		}
		if oc == nil {
			t.Fatal("opencode not in catalog")
		}
		if oc.LaunchMode != LaunchWrapRun {
			t.Fatalf("launch_mode=%q want wrap_run", oc.LaunchMode)
		}
		if len(oc.Argv) == 0 || oc.Argv[0] != "opencode" {
			t.Fatalf("argv=%v want [opencode]", oc.Argv)
		}
		if len(oc.Install.CLIBinaries) != 1 || oc.Install.CLIBinaries[0] != "opencode" {
			t.Fatalf("install.cli_binaries=%v", oc.Install.CLIBinaries)
		}
	})

	t.Run("TC-CAT-003", func(t *testing.T) {
		seen := map[string]bool{}
		for _, a := range ProxyApps(NewTemplateRegistry()) {
			if seen[a.ID] {
				t.Fatalf("duplicate catalog id %q", a.ID)
			}
			seen[a.ID] = true
		}
	})

	t.Run("TC-CAT-004", func(t *testing.T) {
		// Declares wrap_cli but has no CompanionCLI binary → must be skipped.
		r := emptyRegistry()
		r.Register(fakeTemplate{at: "broken", meta: AgentSetupMeta{
			Category:      AgentCategoryCLI,
			AccessMethods: []AccessMethod{AccessWrapCLI},
		}})
		if got := ProxyApps(r); len(got) != 0 {
			t.Fatalf("expected empty catalog, got %#v", got)
		}
	})

	t.Run("TC-CAT-002 desktop edition", func(t *testing.T) {
		apps := ProxyApps(NewTemplateRegistry())
		var od *ProxyApp
		for i := range apps {
			if apps[i].ID == "opencode-desktop" {
				od = &apps[i]
			}
		}
		if od == nil {
			t.Fatal("opencode-desktop not in catalog")
		}
		if od.LaunchMode != LaunchSystemProxy {
			t.Fatalf("launch_mode=%q want system_proxy", od.LaunchMode)
		}
		if len(od.Install.Aliases) == 0 {
			t.Fatal("desktop entry must carry detection aliases")
		}
		// CLI and desktop editions are distinct catalog entries.
		if od.ID == "opencode" {
			t.Fatal("desktop must be a separate entry from the CLI")
		}
	})

	t.Run("TC-CAT-005 codex MSIX edition", func(t *testing.T) {
		apps := ProxyApps(NewTemplateRegistry())
		var cd *ProxyApp
		for i := range apps {
			if apps[i].ID == "codex-desktop" {
				cd = &apps[i]
			}
		}
		if cd == nil {
			t.Fatal("codex-desktop not in catalog")
		}
		if cd.LaunchMode != LaunchSystemProxy {
			t.Fatalf("launch_mode=%q want system_proxy", cd.LaunchMode)
		}
		if len(cd.Install.WinMSIX) == 0 || !strings.HasPrefix(cd.Install.WinMSIX[0], "OpenAI.Codex_") {
			t.Fatalf("install.win_msix=%v want OpenAI.Codex_ prefix", cd.Install.WinMSIX)
		}
		if cd.Vendor != VendorOpenAI {
			t.Fatalf("vendor=%q want openai", cd.Vendor)
		}
	})

	t.Run("TC-CAT-006", func(t *testing.T) {
		apps := ProxyApps(NewTemplateRegistry())
		for i := 1; i < len(apps); i++ {
			a, b := apps[i-1], apps[i]
			if a.Vendor > b.Vendor || (a.Vendor == b.Vendor && a.ID > b.ID) {
				t.Fatalf("not sorted at %d: %s/%s > %s/%s", i, a.Vendor, a.ID, b.Vendor, b.ID)
			}
		}
	})
}

// TestAgentMetaCompat covers TC-REG-003: legacy AgentSetupMeta JSON keys remain
// present so old frontends/desktop shells keep working.
func TestAgentMetaCompat(t *testing.T) {
	t.Run("TC-REG-003", func(t *testing.T) {
		tmpl, ok := NewTemplateRegistry().Get(AgentOpenCode)
		if !ok {
			t.Fatal("opencode template missing")
		}
		b, err := json.Marshal(tmpl.Meta().Normalize())
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"access_methods", "companion_cli", "write_mode", "config_paths", "install_url"} {
			if !strings.Contains(string(b), `"`+key+`"`) {
				t.Fatalf("meta JSON missing legacy key %q: %s", key, b)
			}
		}
	})
}

// TestProxyAppJSON covers TC-CAT-005: empty install/model_config are omitted.
func TestProxyAppJSON(t *testing.T) {
	t.Run("TC-CAT-005", func(t *testing.T) {
		app := ProxyApp{ID: "x", DisplayName: "X", LaunchMode: LaunchWrapRun}
		b, err := json.Marshal(app)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, key := range []string{"install", "model_config"} {
			if strings.Contains(s, `"`+key+`"`) {
				t.Fatalf("empty %s should be omitted: %s", key, s)
			}
		}
	})
}
