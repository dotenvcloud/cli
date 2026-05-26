package config

import (
	"os"
	"strconv"
	"strings"
)

// Environment variable names
const (
	// EnvAPIKey is the env var name (not a credential itself).
	EnvAPIKey        = "DOTENV_API_KEY" //nolint:gosec // env var name, not a credential
	EnvAPIURL        = "DOTENV_API_URL"
	EnvOrganization  = "DOTENV_ORGANIZATION"
	EnvContext       = "DOTENV_CONTEXT"
	EnvDebug         = "DOTENV_DEBUG"
	EnvNoColor       = "DOTENV_NO_COLOR"
	EnvQuiet         = "DOTENV_QUIET"
	EnvTLSSkipVerify = "DOTENV_TLS_SKIP_VERIFY"
	EnvConfigDir     = "DOTENV_CONFIG_DIR"
)

// EnvConfig provides environment variable overrides
type EnvConfig struct {
	APIKey        string
	APIURL        string
	Organization  string
	Context       string
	Debug         bool
	NoColor       bool
	Quiet         bool
	TLSSkipVerify bool
	ConfigDir     string
}

// LoadEnvConfig loads configuration from environment variables
func LoadEnvConfig() *EnvConfig {
	env := &EnvConfig{
		APIKey:        os.Getenv(EnvAPIKey),
		APIURL:        os.Getenv(EnvAPIURL),
		Organization:  os.Getenv(EnvOrganization),
		Context:       os.Getenv(EnvContext),
		Debug:         parseBool(os.Getenv(EnvDebug)),
		NoColor:       parseBool(os.Getenv(EnvNoColor)),
		Quiet:         parseBool(os.Getenv(EnvQuiet)),
		TLSSkipVerify: parseBool(os.Getenv(EnvTLSSkipVerify)),
		ConfigDir:     os.Getenv(EnvConfigDir),
	}

	// Support development environment
	if env.APIURL == "" && isDevelopment() {
		env.APIURL = "https://dotenv.test"
	}

	return env
}

// HasOverrides checks if any environment overrides are set
func (e *EnvConfig) HasOverrides() bool {
	return e.APIKey != "" || e.APIURL != "" || e.Organization != "" || e.Context != ""
}

// Apply applies environment overrides to a context
func (e *EnvConfig) Apply(ctx *Context) *Context {
	result := *ctx

	if e.APIKey != "" {
		result.APIKey = e.APIKey
	}

	if e.APIURL != "" {
		result.APIURL = e.APIURL
	}

	if e.Organization != "" {
		result.Organization = e.Organization
	}

	return &result
}

// ApplyToConfig applies environment overrides to the entire configuration
func (e *EnvConfig) ApplyToConfig(config *Config) (*Config, error) {
	result := *config

	// Apply context override
	if e.Context != "" {
		if _, exists := result.Contexts[e.Context]; exists {
			result.CurrentContext = e.Context
		}
	}

	// If we have direct environment overrides, create a temporary context
	if e.HasOverrides() && result.CurrentContext != "" {
		ctx, exists := result.Contexts[result.CurrentContext]
		if exists {
			// Apply overrides to current context
			ctx = *e.Apply(&ctx)
			result.Contexts[result.CurrentContext] = ctx
		}
	}

	return &result, nil
}

// parseBool parses a boolean environment variable
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}

	// Check common true values
	if s == "1" || s == "true" || s == "yes" || s == "on" {
		return true
	}

	// Try parsing as bool
	b, _ := strconv.ParseBool(s)
	return b
}

// isDevelopment checks if we're in development mode
func isDevelopment() bool {
	const (
		devEnvName  = "development"
		devEnvShort = "dev"
	)
	// Check common development indicators
	if env := os.Getenv("DOTENV_ENV"); env == devEnvName || env == devEnvShort {
		return true
	}
	if env := os.Getenv("ENV"); env == devEnvName || env == devEnvShort {
		return true
	}
	if env := os.Getenv("NODE_ENV"); env == "development" {
		return true
	}
	return false
}
