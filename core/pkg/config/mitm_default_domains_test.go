package config

import "testing"

func TestDefaultMITMDomains_IncludesOpenCodeAndCatalog(t *testing.T) {
	domains := DefaultMITMDomains()
	if len(domains) < 80 {
		t.Fatalf("expected a broad default list, got %d", len(domains))
	}
	need := []string{
		"opencode.ai", "api.openai.com", "api.anthropic.com", "openrouter.ai",
		"generativelanguage.googleapis.com", "openai.azure.com", "aiplatform.googleapis.com",
		"api.groq.com", "api.together.xyz", "api.fireworks.ai", "codestral.mistral.ai",
	}
	set := map[string]bool{}
	for _, d := range domains {
		set[d] = true
	}
	for _, d := range need {
		if !set[d] {
			t.Fatalf("missing required default MITM domain %q", d)
		}
	}
}

func TestHostMatchesMITMDomain(t *testing.T) {
	if !HostMatchesMITMDomain("api.openai.com", "api.openai.com") {
		t.Fatal("exact match")
	}
	if !HostMatchesMITMDomain("foo.openai.azure.com", "openai.azure.com") {
		t.Fatal("subdomain match")
	}
	if HostMatchesMITMDomain("openai.com", "api.openai.com") {
		t.Fatal("parent must not match child domain entry")
	}
	if HostMatchesMITMDomain("notopenai.azure.com", "openai.azure.com") {
		t.Fatal("suffix-only false positive")
	}
}

func TestNormalizeSystemProxyConfig_RefillsEmptyDomains(t *testing.T) {
	c := SystemProxyConfig{AllowLANClients: false, ListenAddr: "0.0.0.0", Domains: nil, PathPatterns: nil}
	NormalizeSystemProxyConfig(&c)
	if len(c.Domains) == 0 {
		t.Fatal("empty Domains should be refilled from defaults")
	}
	if len(c.PathPatterns) == 0 {
		t.Fatal("empty PathPatterns should be refilled from defaults")
	}
	found := false
	for _, d := range c.Domains {
		if d == "opencode.ai" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("refilled Domains must include opencode.ai")
	}
}
