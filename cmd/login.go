package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var (
	loginNoBrowser    bool
	loginCallbackPort string
	loginAPIKey       bool
	loginStatus       bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with DotEnv",
	Long: `Authenticate with DotEnv using either OAuth2 or Organization API keys.

OAuth2 (default for interactive sessions):
- Opens your browser for authentication
- Provides access to all organizations you're a member of
- Supports automatic token refresh
- Ideal for local development

Organization API Key (for CI/CD):
- Manual entry of organization-specific API key
- Access limited to one organization
- No browser required
- Perfect for automation and CI/CD`,

	Example: `  # OAuth2 login (default)
  dotenv login

  # OAuth2 without opening browser
  dotenv login --no-browser
  
  # Organization API key login
  dotenv login --api-key
  
  # Check authentication status
  dotenv login --status`,

	RunE: runLogin,
}

func init() {
	// Add flags specific to login command
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false,
		"print URL instead of opening browser (OAuth2 only)")
	loginCmd.Flags().StringVar(&loginCallbackPort, "callback-port", "",
		"specify callback port for OAuth2 (default: auto)")
	loginCmd.Flags().BoolVar(&loginAPIKey, "api-key", false,
		"use organization API key authentication instead of OAuth2")
	loginCmd.Flags().BoolVar(&loginStatus, "status", false,
		"show current authentication status")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Show status if requested
	if loginStatus {
		return runStatus(cmd, args)
	}

	// Load account manager
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get API URL
	apiURL := viper.GetString("api_url")
	if apiURL == "" {
		// Check if there's a current account and use its API URL
		if currentAccount, err := am.GetCurrent(); err == nil && currentAccount.APIURL != "" {
			apiURL = currentAccount.APIURL
			ui.PrintInfo("Using API URL from current account: %s", apiURL)
		} else {
			// Fall back to default
			apiURL = getAPIURL()
		}
	}

	// Determine authentication method
	// Use API key if explicitly requested or in non-interactive environment
	if loginAPIKey || os.Getenv("CI") != "" || !isInteractive() {
		return runAPIKeyLogin(am, apiURL)
	}

	// Default to OAuth2 for interactive sessions
	opts := auth.BrowserLoginOptions{
		APIUrl:        apiURL,
		CallbackPort:  loginCallbackPort,
		NoBrowser:     loginNoBrowser,
		IsInteractive: true,
	}

	return auth.DoBrowserLogin(cmd.Context(), am, opts)
}

func runAPIKeyLogin(am *config.AccountManager, apiURL string) error {
	ui.PrintInfo("Organization API Key Authentication")
	ui.PrintInfo("Use this method for CI/CD and automated workflows")

	// Get API key
	apiKey, err := ui.Password("Enter your organization API key")
	if err != nil {
		return err
	}

	// Validate API key format
	validator := config.NewValidator()
	if err := validator.ValidateAPIKey(apiKey); err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	// Get organization
	organization, err := ui.Input("Enter your organization slug", "", ui.Required)
	if err != nil {
		return err
	}

	// Get account name
	defaultAccountName := fmt.Sprintf("%s-api", organization)
	accountName, err := ui.Input("Account name", defaultAccountName, nil)
	if err != nil {
		return err
	}

	// Create org info
	orgInfo := config.OrgInfo{
		ULID: organization,
		Name: organization,
		Slug: organization,
	}

	// Create API key account
	if err := am.CreateWithAPIKey(accountName, apiURL, apiKey, &orgInfo); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	// Set as current
	if err := am.Use(accountName); err != nil {
		return fmt.Errorf("failed to set current account: %w", err)
	}

	ui.PrintSuccess("API key authentication successful!")
	ui.PrintInfo("Current account: %s", accountName)
	ui.PrintInfo("Organization: %s", organization)

	return nil
}

// showAuthStatus is now handled by the status command

func isInteractive() bool {
	// Check if stdin is a terminal
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}
