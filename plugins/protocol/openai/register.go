//go:build protocol_openai

package openai

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterProtocol("openai", func(config map[string]interface{}) (interface{}, error) {
		return NewProtocol()
	})
}
