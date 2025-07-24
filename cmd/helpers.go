package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

// getAPIClient returns a configured API client
func getAPIClient() (*dotenv.Client, error) {
	// Create factory
	factory := client.NewFactory(config.GetAPIURL(""))

	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv(config.EnvAPIKey)
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		apiURL := config.GetAPIURL("")
		org := os.Getenv(config.EnvOrganization)

		return factory.NewClientFromAPIKey(apiKey, apiURL, org), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Check organization context
	orgULID := account.GetCurrentOrganizationULID()
	if orgULID == "" {
		return nil, fmt.Errorf("No organization selected. Run 'dotenv org list' to see available organizations.")
	}

	// Handle OAuth token refresh if needed
	if account.IsOAuth() {
		configPath, err := config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate config directory: %w", err)
		}
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize account manager: %w", err)
		}

		// Use factory's refresh method
		ctx := context.Background()
		return factory.RefreshTokenAndCreateClient(ctx, account, am, true)
	}

	// Create client from account (with organization context)
	return factory.NewClientFromAccount(account, true)
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
	// Create factory
	factory := client.NewFactory(config.GetAPIURL(""))

	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv(config.EnvAPIKey)
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		apiURL := config.GetAPIURL("")
		// Note: Not passing organization since this is for "without org context"
		return factory.NewClientFromAPIKey(apiKey, apiURL, ""), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Handle OAuth token refresh if needed
	if account.IsOAuth() {
		configPath, err := config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate config directory: %w", err)
		}
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize account manager: %w", err)
		}

		// Use factory's refresh method (without org context)
		ctx := context.Background()
		return factory.RefreshTokenAndCreateClient(ctx, account, am, false)
	}

	// Create client from account (without organization context)
	return factory.NewClientFromAccount(account, false)
}

// getUnauthenticatedSDKClient returns an SDK client without authentication
// This is useful for OAuth token operations that don't require authentication
func getUnauthenticatedSDKClient(baseURL string) *dotenv.Client {
	if baseURL == "" {
		baseURL = config.GetAPIURL("")
	}

	factory := client.NewFactory(baseURL)
	return factory.NewUnauthenticatedClient(baseURL, config.ShouldSkipTLSVerify())
}
