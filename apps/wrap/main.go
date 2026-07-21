package main

import (
	"fmt"
	"os"

	"centag/apps/wrap/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "centag-wrap: %v\n", err)
		os.Exit(1)
	}
}
