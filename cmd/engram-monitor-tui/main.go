package main

import (
	"os"

	"github.com/svg153/engram-monitor-tui/internal/cli"
)

var version = "dev"

func main() {
	runner := cli.NewRunner(version)
	os.Exit(runner.Run(os.Args[1:], os.Stdout, os.Stderr))
}
