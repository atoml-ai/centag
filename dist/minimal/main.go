package main

import (
	"os"

	wrapcli "centag/apps/wrap/cli"
	"centag/core/pkg/entrypoint"

	_ "centag/plugins/backend/anthropic"
	_ "centag/plugins/backend/ollama"
	_ "centag/plugins/backend/openai"

	_ "centag/plugins/protocol/anthropic"
	_ "centag/plugins/protocol/openai"
	_ "centag/plugins/protocol/openairesponses"
)

var (
	// Version is injected at build time via -ldflags.
	Version = "dev"
	// BuildTime is injected at build time via -ldflags.
	BuildTime = "unknown"
)

func init() {
	wrapcli.SetProgramName("centag wrap")
	entrypoint.SetWrapCLI(wrapcli.Run)
}

func main() {
	if entrypoint.HandleVersionCommand(Version, BuildTime, os.Args) {
		return
	}
	if entrypoint.HandleHelpCommand(os.Args) {
		return
	}
	if entrypoint.HandleWrapCommand(os.Args) {
		return
	}
	entrypoint.Run(Version, BuildTime)
}
