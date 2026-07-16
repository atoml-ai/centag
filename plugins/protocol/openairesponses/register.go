//go:build protocol_openairesponses

package openairesponses

import "centag/core/pkg/plugin"

func init() {
	plugin.RegisterProtocol("responses", func(config map[string]interface{}) (interface{}, error) {
		return NewProtocol()
	})
}
