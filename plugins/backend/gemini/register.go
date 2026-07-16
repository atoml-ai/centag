//go:build backend_gemini

package gemini

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterBackend("gemini", func(config map[string]interface{}) (interface{}, error) {
		return NewBackend()
	})

	plugin.RegisterBackendMeta(plugin.BackendMeta{
		Type:           "gemini",
		Name:           "Gemini",
		DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		KeyHelp:        "Google AI API Key — 从 Google AI Studio (aistudio.google.com) 获取",
		Capabilities:   []string{"chat", "streaming"},
		AuthSchemes:    []string{"api_key"},
	})
}
