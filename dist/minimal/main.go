package main

import (
	"os"

	"centag/core/pkg/entrypoint"

	_ "centag/plugins/backend/anthropic"
	_ "centag/plugins/backend/ollama"
	_ "centag/plugins/backend/openai"

	_ "centag/plugins/protocol/anthropic"
	_ "centag/plugins/protocol/openai"
)

var (
	// Version is injected at build time via -ldflags.
	Version = "dev"
	// BuildTime is injected at build time via -ldflags.
	BuildTime = "unknown"
)

func main() {
	if entrypoint.HandleVersionCommand(Version, BuildTime, os.Args) {
		return
	}
	entrypoint.Run(Version, BuildTime)
}
