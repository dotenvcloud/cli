package cmd

import (
	"fmt"
	"os"

	"github.com/dotenv/cli/internal/config"
	dotenv "github.com/dotenv/sdk-go"
)

// getAPIClient returns a configured API client
func getAPIClient() (*dotenv.Client, error) {
	// Get current context
	ctx, err := getCurrentContext()
	if err != nil {
		return nil, err
	}

	// Create client options
	options := []dotenv.ClientOption{
		dotenv.WithBaseURL(ctx.APIURL),
	}

	// Check for TLS skip verify (development mode)
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		options = append(options, dotenv.WithInsecureSkipVerify())
	}

	// Create client
	client := dotenv.NewClient(ctx.APIKey, options...)

	return client, nil
}

// getCurrentContext returns the current context
func getCurrentContext() (*config.Context, error) {
	cm, err := config.NewContextManager("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	ctx, err := cm.GetCurrent()
	if err != nil {
		return nil, fmt.Errorf("no current context. Run 'dotenv init' to get started")
	}

	// Apply any environment overrides
	env := config.LoadEnvConfig()
	if env.HasOverrides() {
		ctx = env.Apply(ctx)
	}

	return ctx, nil
}

// ensureAuthenticated checks if we have valid credentials
func ensureAuthenticated() error {
	_, err := getCurrentContext()
	return err
}
