package main

import "embed"

//go:embed all:static
var staticContent embed.FS
