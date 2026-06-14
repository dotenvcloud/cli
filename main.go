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
	// telemetrySecret HMAC-signs CLI telemetry. Injected from a private CI
	// secret at release build time; empty in local/dev builds (telemetry then
	// goes out unsigned).
	telemetrySecret = ""
)

func main() {
	// Set build info
	build.SetInfo(build.Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: goVersion,
	})
	build.SetTelemetrySecret(telemetrySecret)

	// Execute root command
	if err := cmd.Execute(); err != nil {
		cmd.ShowErrorWithHelp(err)
		os.Exit(1)
	}
}
