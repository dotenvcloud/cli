package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/errors"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

// getAPIClient returns a configured API client
func getAPIClient() (*dotenv.Client, error) {
	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv(config.EnvAPIKey)
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		options := []dotenv.ClientOption{
			dotenv.WithAPIKey(apiKey),
		}

		// Check for custom API URL
		apiURL := config.GetAPIURL("")
		options = append(options, dotenv.WithBaseURL(apiURL))

		// Check for organization from environment
		if org := os.Getenv(config.EnvOrganization); org != "" {
			options = append(options, dotenv.WithOrganization(org))
		}

		// Check for TLS skip verify (development mode)
		if config.ShouldSkipTLSVerify() {
			options = append(options, dotenv.WithInsecureSkipVerify())
		}

		return dotenv.NewClient(options...), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Create client options
	options := []dotenv.ClientOption{
		dotenv.WithBaseURL(account.APIURL),
	}

	// Add organization context
	orgULID := account.GetCurrentOrganizationULID()
	if orgULID == "" {
		return nil, fmt.Errorf("No organization selected. Run 'dotenv org list' to see available organizations.")
	}
	options = append(options, dotenv.WithOrganization(orgULID))

	// Check for TLS skip verify (development mode)
	if config.ShouldSkipTLSVerify() {
		options = append(options, dotenv.WithInsecureSkipVerify())
	}

	// Handle authentication based on account type
	if account.IsOAuth() {
		// Use token manager to handle refresh
		configPath, err := config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate config directory: %w", err)
		}
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize account manager: %w", err)
		}

		tokenManager := auth.NewTokenManager(am)
		if err := tokenManager.RefreshTokenIfNeeded(context.Background(), account); err != nil {
			if _, ok := err.(*errors.TokenExpiredError); ok {
				ui.PrintInfo("OAuth token expired. Attempting to refresh...")
			}
			return nil, fmt.Errorf("authentication failed: %w. Please login again with 'dotenv login'", err)
		}

		// Reload account after potential refresh
		if account.IsTokenExpired() {
			account, err = am.GetCurrent()
			if err != nil {
				return nil, fmt.Errorf("failed to reload account after token refresh: %w", err)
			}
		}

		options = append(options, dotenv.WithBearerToken(account.Auth.AccessToken))
	} else {
		// API key authentication
		apiKey := account.GetToken()
		if apiKey == "" {
			return nil, &errors.ConfigurationError{
				Field:   "api_key",
				Message: fmt.Sprintf("no API key found in account '%s'", account.Name),
			}
		}
		options = append(options, dotenv.WithAPIKey(apiKey))
	}

	// Create client
	client := dotenv.NewClient(options...)

	return client, nil
}

// RefreshOrganizationsIfNeeded checks if organizations need refresh and refreshes them
// This should be called explicitly when needed, not in a background goroutine
func RefreshOrganizationsIfNeeded(ctx context.Context) error {
	account, err := getCurrentAccount()
	if err != nil {
		return err
	}

	if !account.NeedsOrganizationRefresh() {
		return nil
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to locate config directory: %w", err)
	}

	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return fmt.Errorf("failed to initialize account manager: %w", err)
	}

	tokenManager := auth.NewTokenManager(am)
	orgRemoved, err := tokenManager.RefreshOrganizationsIfNeeded(ctx, account)
	if err != nil {
		return fmt.Errorf("failed to refresh organizations: %w", err)
	}

	if orgRemoved {
		ui.PrintWarning("Current organization no longer exists. Please select a new one with 'dotenv org use'")
	}

	return nil
}

// getCurrentAccount returns the current account
func getCurrentAccount() (*config.Account, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	account, err := am.GetCurrent()
	if err != nil {
		return nil, fmt.Errorf("No accounts configured. Run 'dotenv login' to add an account")
	}

	// TODO: Apply any environment overrides if needed

	return account, nil
}

// displayAccountInfo displays the current account and organization info
func displayAccountInfo() error {
	account, err := getCurrentAccount()
	if err != nil {
		return err
	}

	orgName := ""
	if account.IsOAuth() {
		org, err := account.GetCurrentOrganization()
		if err == nil {
			orgName = org.Name
		}
	} else if account.Organization != nil {
		orgName = account.Organization.Name
	}

	if orgName != "" {
		ui.PrintInfo("[Account: %s | Organization: %s]", account.Name, orgName)
	} else {
		ui.PrintInfo("[Account: %s]", account.Name)
	}

	return nil
}

// ensureAuthenticated checks if we have valid credentials
func ensureAuthenticated() error {
	_, err := getCurrentAccount()
	return err
}

// getAPIClientWithoutOrgContext returns a configured API client without organization context
// This is useful for operations like listing organizations where we don't need/have an org selected
func getAPIClientWithoutOrgContext() (*dotenv.Client, error) {
	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv(config.EnvAPIKey)
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		options := []dotenv.ClientOption{
			dotenv.WithAPIKey(apiKey),
		}

		// Check for custom API URL
		apiURL := config.GetAPIURL("")
		options = append(options, dotenv.WithBaseURL(apiURL))

		// Check for TLS skip verify (development mode)
		if config.ShouldSkipTLSVerify() {
			options = append(options, dotenv.WithInsecureSkipVerify())
		}

		return dotenv.NewClient(options...), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Create client options
	options := []dotenv.ClientOption{
		dotenv.WithBaseURL(account.APIURL),
	}

	// Check for TLS skip verify (development mode)
	if config.ShouldSkipTLSVerify() {
		options = append(options, dotenv.WithInsecureSkipVerify())
	}

	// Handle authentication based on account type
	if account.IsOAuth() {
		// Use token manager to handle refresh
		configPath, err := config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate config directory: %w", err)
		}
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize account manager: %w", err)
		}

		tokenManager := auth.NewTokenManager(am)
		if err := tokenManager.RefreshTokenIfNeeded(context.Background(), account); err != nil {
			if _, ok := err.(*errors.TokenExpiredError); ok {
				ui.PrintInfo("OAuth token expired. Attempting to refresh...")
			}
			return nil, fmt.Errorf("authentication failed: %w. Please login again with 'dotenv login'", err)
		}

		// Reload account after potential refresh
		if account.IsTokenExpired() {
			account, err = am.GetCurrent()
			if err != nil {
				return nil, fmt.Errorf("failed to reload account after token refresh: %w", err)
			}
		}

		options = append(options, dotenv.WithBearerToken(account.Auth.AccessToken))
	} else {
		// API key authentication
		apiKey := account.GetToken()
		if apiKey == "" {
			return nil, &errors.ConfigurationError{
				Field:   "api_key",
				Message: fmt.Sprintf("no API key found in account '%s'", account.Name),
			}
		}
		options = append(options, dotenv.WithAPIKey(apiKey))
	}

	// Create client
	return dotenv.NewClient(options...), nil
}
