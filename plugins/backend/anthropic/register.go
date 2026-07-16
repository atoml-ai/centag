//go:build backend_anthropic

package anthropic

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterBackend("anthropic", func(config map[string]interface{}) (interface{}, error) {
		return NewBackend()
	})

	plugin.RegisterBackendMeta(plugin.BackendMeta{
		Type:           "anthropic",
		Name:           "Anthropic",
		DefaultBaseURL: "https://api.anthropic.com",
		KeyHelp:        "Anthropic API Key (sk-ant-...)",
		Capabilities:   []string{"chat", "streaming", "function_calling"},
		AuthSchemes:    []string{"x-api-key"},
	})
}
