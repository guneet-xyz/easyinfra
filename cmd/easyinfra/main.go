// Package main is the entry point for the easyinfra CLI application.
package main

import (
	"github.com/guneet/easyinfra/internal/cli"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cli.Execute(version, commit, date)
}
