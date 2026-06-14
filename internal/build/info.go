package build

import (
	"fmt"
	"runtime"
)

// Info contains build information
type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
}

var info Info

// telemetrySecret is the HMAC secret used to sign CLI telemetry requests,
// injected at build time via ldflags and forwarded here from main. Empty in
// local/dev builds, in which case telemetry is sent unsigned.
var telemetrySecret string

// SetInfo sets the build information
func SetInfo(i Info) {
	info = i
	if info.GoVersion == "unknown" {
		info.GoVersion = runtime.Version()
	}
}

// SetTelemetrySecret stores the build-time telemetry signing secret.
func SetTelemetrySecret(secret string) {
	telemetrySecret = secret
}

// TelemetrySecret returns the build-time telemetry signing secret (empty when
// not injected at build time).
func TelemetrySecret() string {
	return telemetrySecret
}

// GetInfo returns the build information
func GetInfo() Info {
	return info
}

// String returns a formatted version string
func (i Info) String() string {
	return fmt.Sprintf("DotEnv CLI %s (commit: %s, built: %s, go: %s)",
		i.Version, i.Commit, i.Date, i.GoVersion)
}

// ShortString returns just the version
func (i Info) ShortString() string {
	return fmt.Sprintf("v%s", i.Version)
}
