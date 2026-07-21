package main

import (
	"fmt"
	"os"

	"centag/apps/wrap/cli"
)

func main() {
	// Standalone binary brand; embedded path uses SetProgramName("centag wrap").
	cli.SetProgramName("centag-wrap")
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "centag-wrap: %v\n", err)
		os.Exit(1)
	}
}
