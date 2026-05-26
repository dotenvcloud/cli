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

// apiClientFactory is the indirection seam tests use to swap in a fake
// client. Default is buildAPIClient; tests override via t.Cleanup.
var apiClientFactory = buildAPIClient

// getAPIClient returns a configured API client. Behavior is identical to
// the (now-deleted) getAPIClientWithoutOrgContext when withOrg is false.
func getAPIClient() (*dotenv.Client, error) {
	return apiClientFactory(true)
}

// getAPIClientWithoutOrgContext returns a client without enforcing that an
// organization is selected — used by commands like "org list" that pre-date
// org selection.
func getAPIClientWithoutOrgContext() (*dotenv.Client, error) {
	return apiClientFactory(false)
}

func buildAPIClient(withOrg bool) (*dotenv.Client, error) {
	factory := client.NewFactory(config.GetAPIURL(""))

	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = os.Getenv(config.EnvAPIKey)
	}

	// API key path bypasses account system (CI/CD).
	if apiKey != "" {
		apiURL := config.GetAPIURL("")
		org := ""
		if withOrg {
			org = os.Getenv(config.EnvOrganization)
		}
		return factory.NewClientFromAPIKey(apiKey, apiURL, org), nil
	}

	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	if withOrg {
		if account.GetCurrentOrganizationULID() == "" {
			return nil, fmt.Errorf("No organization selected. Run 'dotenv org list' to see available organizations.")
		}
	}

	if account.IsOAuth() {
		configPath, err := config.ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate config directory: %w", err)
		}
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize account manager: %w", err)
		}

		ctx := context.Background()
		return factory.RefreshTokenAndCreateClient(ctx, account, am, withOrg)
	}

	return factory.NewClientFromAccount(account, withOrg)
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

// accountForErrorContext returns the current account if available, surfacing a
// warning when it cannot be loaded. Use in error-handling paths where the
// account is needed only to enrich a subsequent error message.
func accountForErrorContext() *config.Account {
	account, err := getCurrentAccount()
	if err != nil {
		ui.PrintWarning("account context unavailable: %v", err)
	}
	return account
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

// getUnauthenticatedSDKClient returns an SDK client without authentication
// This is useful for OAuth token operations that don't require authentication
func getUnauthenticatedSDKClient(baseURL string) *dotenv.Client {
	if baseURL == "" {
		baseURL = config.GetAPIURL("")
	}

	factory := client.NewFactory(baseURL)
	return factory.NewUnauthenticatedClient(baseURL, config.ShouldSkipTLSVerify())
}
