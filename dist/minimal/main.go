package main

import (
	"centag/core/pkg/entrypoint"

	_ "centag/plugins/backend/anthropic"
	_ "centag/plugins/backend/ollama"
	_ "centag/plugins/backend/openai"

	_ "centag/plugins/protocol/anthropic"
	_ "centag/plugins/protocol/openai"
)

func main() {
	entrypoint.Run("dev", "unknown")
}
