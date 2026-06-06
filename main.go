package main

import (
	"os"

	"github.com/dotenvcloud/cli/cmd"
	"github.com/dotenvcloud/cli/internal/build"
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
		cmd.ShowErrorWithHelp(err)
		os.Exit(1)
	}
}
