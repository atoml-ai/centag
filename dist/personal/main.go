package main

import (
	"centag/core/pkg/entrypoint"

	_ "centag/plugins/backend/anthropic"
	_ "centag/plugins/backend/azure"
	_ "centag/plugins/backend/gemini"
	_ "centag/plugins/backend/ollama"
	_ "centag/plugins/backend/openai"

	_ "centag/plugins/protocol/anthropic"
	_ "centag/plugins/protocol/gemini"
	_ "centag/plugins/protocol/openai"
	_ "centag/plugins/protocol/openairesponses"

	_ "centag/plugins/database/postgresql"
	_ "centag/plugins/database/sqlite"

	_ "centag/plugins/storage/chroma"
	_ "centag/plugins/storage/elasticsearch"
	_ "centag/plugins/storage/file"
	_ "centag/plugins/storage/postgresql"
	_ "centag/plugins/storage/redis"
)

func main() {
	entrypoint.Run("dev", "unknown")
}
