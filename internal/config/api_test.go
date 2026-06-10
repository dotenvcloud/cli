package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dotenvcloud/cli/internal/constants"
)

func TestGetAPIURL(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvAPIURL)
	defer func() {
		if originalEnv != "" {
			os.Setenv(EnvAPIURL, originalEnv)
		} else {
			os.Unsetenv(EnvAPIURL)
		}
	}()

	tests := []struct {
		name       string
		envValue   string
		defaultURL string
		expected   string
	}{
		{
			name:       "environment variable takes precedence",
			envValue:   "https://custom.api.url",
			defaultURL: "https://account.api.url",
			expected:   "https://custom.api.url",
		},
		{
			name:       "uses provided default when no env",
			envValue:   "",
			defaultURL: "https://account.api.url",
			expected:   "https://account.api.url",
		},
		{
			name:       "falls back to default API URL",
			envValue:   "",
			defaultURL: "",
			expected:   constants.DefaultAPIURL,
		},
		{
			name:       "staging URL from env",
			envValue:   constants.TestAPIURL,
			defaultURL: "",
			expected:   constants.TestAPIURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(EnvAPIURL, tt.envValue)
			} else {
				os.Unsetenv(EnvAPIURL)
			}

			result := GetAPIURL(tt.defaultURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultAPIURL(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvAPIURL)
	defer func() {
		if originalEnv != "" {
			os.Setenv(EnvAPIURL, originalEnv)
		} else {
			os.Unsetenv(EnvAPIURL)
		}
	}()

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns env value when set",
			envValue: "https://test.api.url",
			expected: "https://test.api.url",
		},
		{
			name:     "returns default when no env",
			envValue: "",
			expected: constants.DefaultAPIURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(EnvAPIURL, tt.envValue)
			} else {
				os.Unsetenv(EnvAPIURL)
			}

			result := GetDefaultAPIURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldSkipTLSVerify(t *testing.T) {
	// Save original env values
	originalSkip := os.Getenv(EnvTLSSkipVerify)
	originalURL := os.Getenv(EnvAPIURL)
	defer func() {
		restoreEnv(EnvTLSSkipVerify, originalSkip)
		restoreEnv(EnvAPIURL, originalURL)
	}()

	// The override is only honored for local development hosts so a stray or
	// injected env var cannot silently disable TLS verification against a real
	// endpoint and enable a man-in-the-middle.
	tests := []struct {
		name     string
		envValue string
		apiURL   string
		expected bool
	}{
		{
			name:     "honored for localhost",
			envValue: "1",
			apiURL:   "http://localhost:8000",
			expected: true,
		},
		{
			name:     "honored for 127.0.0.1",
			envValue: "true",
			apiURL:   "https://127.0.0.1:8443",
			expected: true,
		},
		{
			name:     "honored for .test host",
			envValue: "yes",
			apiURL:   "https://api.dotenv.test",
			expected: true,
		},
		{
			name:     "ignored for production host even when set",
			envValue: "1",
			apiURL:   "https://api.dotenv.cloud",
			expected: false,
		},
		{
			name:     "ignored for arbitrary remote host even when set",
			envValue: "true",
			apiURL:   "https://evil.example.com",
			expected: false,
		},
		{
			name:     "returns false when not set",
			envValue: "",
			apiURL:   "http://localhost:8000",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(EnvTLSSkipVerify, tt.envValue)
			} else {
				os.Unsetenv(EnvTLSSkipVerify)
			}
			os.Setenv(EnvAPIURL, tt.apiURL)

			result := ShouldSkipTLSVerify()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func restoreEnv(key, value string) {
	if value != "" {
		os.Setenv(key, value)
	} else {
		os.Unsetenv(key)
	}
}
