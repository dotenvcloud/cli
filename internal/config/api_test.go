package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dotenv/cli/internal/constants"
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
	// Save original env value
	originalEnv := os.Getenv(EnvTLSSkipVerify)
	defer func() {
		if originalEnv != "" {
			os.Setenv(EnvTLSSkipVerify, originalEnv)
		} else {
			os.Unsetenv(EnvTLSSkipVerify)
		}
	}()

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "returns true when set to 1",
			envValue: "1",
			expected: true,
		},
		{
			name:     "returns true when set to true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "returns true when set to any value",
			envValue: "yes",
			expected: true,
		},
		{
			name:     "returns false when not set",
			envValue: "",
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

			result := ShouldSkipTLSVerify()
			assert.Equal(t, tt.expected, result)
		})
	}
}
