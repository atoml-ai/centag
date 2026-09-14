// Package agentcatalog exposes the Agent registry as a proxy-launch app
// catalog for the wrap CLI and local desktop shell, without coupling the
// standalone apps/wrap module to centag core.
package agentcatalog

import (
	"encoding/json"

	"centag/core/internal/agent"
)

// Catalog is the wire payload shared with `centag wrap apps` and
// `GET /api/v1/wrap/apps`.
type Catalog struct {
	Apps []agent.ProxyApp `json:"apps"`
}

// Build derives the catalog from the default Agent template registry.
func Build() Catalog {
	apps := agent.ProxyApps(agent.NewTemplateRegistry())
	if apps == nil {
		apps = []agent.ProxyApp{}
	}
	return Catalog{Apps: apps}
}

// JSON marshals the catalog for injection into the wrap CLI.
func JSON() ([]byte, error) {
	return json.Marshal(Build())
}
