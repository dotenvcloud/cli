package config

import (
	"os"

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

// ShouldSkipTLSVerify returns true if TLS verification should be skipped
// This should only be used in development environments
func ShouldSkipTLSVerify() bool {
	return os.Getenv(EnvTLSSkipVerify) != ""
}
