package server

import (
	"strings"
	"testing"
)

// 回归：replaceManifestVersion 必须按行首锚定顶层 version 行，
// 不能命中首行 "api_version: " 的尾缀子串导致 schema 版本被写坏。
func TestReplaceManifestVersion(t *testing.T) {
	in := []byte(`api_version: centag.agent-skill/v1alpha1
implementation: custom.agent-skill-x
name: x
kind: agent.skill
version: feishu/1.0.0
`)
	out := replaceManifestVersion(in, "feishu")

	if !strings.Contains(string(out), "api_version: centag.agent-skill/v1alpha1") {
		t.Fatalf("api_version must be preserved, got:\n%s", out)
	}
	if !strings.Contains(string(out), "version: feishu\n") {
		t.Fatalf("top-level version must be replaced with marker, got:\n%s", out)
	}
}
