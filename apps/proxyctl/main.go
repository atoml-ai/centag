package main

import (
	"fmt"
	"os"

	"centag/apps/proxyctl/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "centag-proxyctl: %v\n", err)
		os.Exit(1)
	}
}
