//go:build protocol_anthropic

package anthropic

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterProtocol("anthropic", func(config map[string]interface{}) (interface{}, error) {
		return NewProtocol()
	})
}
