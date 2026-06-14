package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/auth"
	"github.com/dotenvcloud/cli/internal/client"
	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/ui"
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
	env := config.LoadEnvConfig()
	flagAPIKey := viper.GetString("api_key")

	// Environment / --api-key credentials take precedence and bypass the
	// account store entirely (CI/CD).
	if config.UsingEnvCredential(flagAPIKey) {
		auth := config.ResolveAuth(flagAPIKey, env, nil)
		factory := client.NewFactory(auth.APIURL)
		org := ""
		if withOrg {
			org = auth.Organization
		}
		return factory.NewClientFromAPIKey(auth.APIKey, auth.APIURL, org), nil
	}

	// Otherwise use the current stored account.
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}
	auth := config.ResolveAuth(flagAPIKey, env, account)
	factory := client.NewFactory(auth.APIURL)

	if withOrg {
		if account.GetCurrentOrganizationULID() == "" {
			return nil, fmt.Errorf("no organization selected. Run 'dotenv org list' to see available organizations")
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
	// Environment / --api-key credentials bypass the account store; there is no
	// stored account to refresh, and consulting one would surface stale-account
	// warnings during CI/CD runs.
	if config.UsingEnvCredential(viper.GetString("api_key")) {
		return nil
	}

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
		ui.PrintWarningf("Current organization no longer exists. Please select a new one with 'dotenv org use'")
	}

	return nil
}

// accountForErrorContext returns the current account if available, surfacing a
// warning when it cannot be loaded. Use in error-handling paths where the
// account is needed only to enrich a subsequent error message.
func accountForErrorContext() *config.Account {
	account, err := getCurrentAccount()
	if err != nil {
		ui.PrintWarningf("account context unavailable: %v", err)
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
		return nil, fmt.Errorf("no accounts configured. Run 'dotenv login' to add an account")
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
		ui.PrintInfof("[Account: %s | Organization: %s]", account.Name, orgName)
	} else {
		ui.PrintInfof("[Account: %s]", account.Name)
	}

	return nil
}

// printActiveIdentity shows which credentials a command is using so it is always
// clear who you are and which organization you are acting on. Environment /
// --api-key credentials are announced explicitly (CI/CD); otherwise the current
// stored account is shown. This is the single place commands report identity.
func printActiveIdentity() {
	if config.UsingEnvCredential(viper.GetString("api_key")) {
		if org := os.Getenv(config.EnvOrganization); org != "" {
			ui.PrintInfof("[Using environment credentials | Organization: %s]", org)
		} else {
			ui.PrintInfof("[Using environment credentials]")
		}
		return
	}

	if err := displayAccountInfo(); err != nil {
		ui.PrintWarningf("Could not display account info: %v", err)
	}
}
