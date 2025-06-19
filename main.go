package main

import (
	"fmt"
	"os"

	"github.com/dotenv/cli/cmd"
	"github.com/dotenv/cli/internal/build"
)

// Build variables set by ldflags
var (
	version   = "dev"
	commit    = "none"
	date      = "unknown"
	goVersion = "unknown"
)

func main() {
	// Set build info
	build.SetInfo(build.Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: goVersion,
	})

	// Execute root command
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
