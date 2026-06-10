package config

import (
	"net/url"
	"os"
	"strings"

	"github.com/dotenvcloud/cli/internal/constants"
)

// GetAPIURL returns the API URL with proper precedence:
// 1. From environment variable DOTENV_API_URL
// 2. From provided default (e.g., from current account)
// 3. Default to https://api.dotenv.cloud
func GetAPIURL(defaultURL string) string {
	// First priority: environment variable
	if apiURL := os.Getenv(EnvAPIURL); apiURL != "" {
		return apiURL
	}

	// Second priority: provided default (e.g., from account)
	if defaultURL != "" {
		return defaultURL
	}

	// Final fallback: default API URL
	return constants.DefaultAPIURL
}

// GetDefaultAPIURL returns the API URL from environment or default
// This is a convenience function when no account-specific URL is available
func GetDefaultAPIURL() string {
	return GetAPIURL("")
}

// ShouldSkipTLSVerify returns true if TLS verification should be skipped.
//
// The DOTENV_TLS_SKIP_VERIFY override is only honored when the effective API
// URL points at a local development host. This prevents a stray or injected
// environment variable from silently disabling certificate verification
// against a real endpoint (e.g. api.dotenv.cloud) and enabling a
// man-in-the-middle that would expose bearer tokens and secrets in transit.
func ShouldSkipTLSVerify() bool {
	if os.Getenv(EnvTLSSkipVerify) == "" {
		return false
	}

	return isLocalAPIHost(GetDefaultAPIURL())
}

// isLocalAPIHost reports whether the given URL targets a loopback or local
// development host where skipping TLS verification is acceptable.
func isLocalAPIHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := parsed.Hostname()
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}

	return strings.HasSuffix(host, ".test") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".localhost")
}
