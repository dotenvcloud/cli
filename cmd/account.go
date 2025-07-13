package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
	"github.com/dotenv/cli/internal/utils"
	dotenv "github.com/dotenv/sdk-go"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage DotEnv accounts",
	Long: `Manage DotEnv accounts for authentication.

Accounts can be either OAuth-based (supporting multiple organizations) or
API key-based (tied to a single organization).`,
	Example: `  # List all accounts
  dotenv account list

  # Switch to a different account
  dotenv account use work@example.com

  # Add a new account interactively
  dotenv account add

  # Remove an account
  dotenv account remove old-account

  # Refresh OAuth tokens
  dotenv account refresh

  # Rename an account
  dotenv account rename old-name new-name`,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured accounts",
	RunE:  runAccountList,
}

var accountUseCmd = &cobra.Command{
	Use:   "use [account-name]",
	Short: "Switch to a different account",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccountUse,
}

var accountAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new account",
	Long: `Add a new account interactively.

You can choose between OAuth login (recommended) or API key authentication.`,
	RunE: runAccountAdd,
}

var accountRemoveCmd = &cobra.Command{
	Use:     "remove [account-name]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an account",
	Args:    cobra.ExactArgs(1),
	RunE:    runAccountRemove,
}

var accountRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh OAuth tokens for the current account",
	RunE:  runAccountRefresh,
}

var accountRenameCmd = &cobra.Command{
	Use:   "rename [old-name] [new-name]",
	Short: "Rename an account",
	Args:  cobra.ExactArgs(2),
	RunE:  runAccountRename,
}

func init() {
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountUseCmd)
	accountCmd.AddCommand(accountAddCmd)
	accountCmd.AddCommand(accountRemoveCmd)
	accountCmd.AddCommand(accountRefreshCmd)
	accountCmd.AddCommand(accountRenameCmd)
}

// getAPIURL returns the API URL with proper precedence:
// 1. From environment variable DOTENV_API_URL
// 2. Default to https://api.dotenv.cloud
func getAPIURL() string {
	if apiURL := os.Getenv("DOTENV_API_URL"); apiURL != "" {
		return apiURL
	}
	return constants.LegacyAPIURL
}
func runAccountList(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	accounts := am.List()
	if len(accounts) == 0 {
		ui.PrintWarning("No accounts configured. Run 'dotenv account add' to add an account.")
		return nil
	}

	current, _ := am.GetCurrent()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CURRENT\tNAME\tTYPE\tORGANIZATION\tLAST USED")

	for _, name := range accounts {
		account, err := am.Get(name)
		if err != nil {
			continue
		}

		current := " "
		if name == currentName {
			current = "*"
		}

		authType := account.AuthType
		if authType == "" {
			authType = constants.AuthTypeAPIKey
		}

		orgInfo := ""
		if account.IsOAuth() {
			org, err := account.GetCurrentOrganization()
			if err == nil {
				orgInfo = fmt.Sprintf("%s (%d orgs)", org.Name, len(account.Organizations))
			} else {
				orgInfo = fmt.Sprintf("(%d orgs)", len(account.Organizations))
			}
		} else if account.Organization != nil {
			orgInfo = account.Organization.Name
		}

		lastUsed := account.LastUsed.Format("2006-01-02 15:04")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", current, name, authType, orgInfo, lastUsed)
	}

	w.Flush()
	return nil
}

func runAccountUse(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	accountName := args[0]
	if err := am.Use(accountName); err != nil {
		return err
	}

	account, err := am.Get(accountName)
	if err != nil {
		return err
	}

	ui.PrintSuccess("Switched to account: %s", accountName)

	// Show organization info
	if account.IsOAuth() {
		org, err := account.GetCurrentOrganization()
		if err == nil {
			ui.PrintInfo("Current organization: %s", org.Name)
		}
	} else if account.Organization != nil {
		ui.PrintInfo("Organization: %s", account.Organization.Name)
	}

	return nil
}

func runAccountAdd(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	// Get API URL (prefer existing account's URL if available)
	apiURL := getAPIURL()
	if currentAccount, err := am.GetCurrent(); err == nil && currentAccount.APIURL != "" {
		apiURL = currentAccount.APIURL
		ui.PrintInfo("Using API URL from current account: %s", apiURL)
	}

	// Ask for authentication method
	authMethod, err := ui.Select("How would you like to authenticate?", []string{
		"Login via browser (OAuth)",
		"Enter API key manually",
	})
	if err != nil {
		return err
	}

	if strings.Contains(authMethod, "OAuth") {
		// OAuth flow
		ui.PrintInfo("Starting OAuth login...")
		opts := auth.BrowserLoginOptions{
			APIUrl:        apiURL,
			CallbackPort:  "",
			NoBrowser:     false,
			IsInteractive: true,
		}

		if err := auth.DoBrowserLogin(cmd.Context(), am, opts); err != nil {
			return fmt.Errorf("OAuth login failed: %w", err)
		}
	} else {
		// API key flow
		apiKey, err := ui.Password("Enter your API key")
		if err != nil {
			return err
		}

		// Validate API key
		validator := config.NewValidator()
		if err := validator.ValidateAPIKey(apiKey); err != nil {
			return fmt.Errorf("invalid API key: %w", err)
		}

		// Try to fetch organization info
		client := dotenv.NewClient(
			dotenv.WithAPIKey(apiKey),
			dotenv.WithBaseURL(getAPIURL()),
		)
		if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
			client.SetTLSSkipVerify(true)
		}

		orgs, _, err := client.Organizations.List(cmd.Context(), nil)
		if err != nil {
			return fmt.Errorf("failed to verify API key: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("no organizations found for this API key")
		}

		// For API key, we expect single org
		org := orgs[0]
		// Use ID if ULID is empty (API returns ULID in ID field)
		ulid := org.ULID
		if ulid == "" && org.ID != "" {
			ulid = org.ID
		}
		orgInfo := config.OrgInfo{
			ULID: ulid,
			Name: org.Name,
		}

		// Default account name is org name
		defaultName := utils.Slugify(org.Name)
		accountName, err := ui.Input("Account name", defaultName, nil)
		if err != nil {
			return err
		}

		// Create API key account
		if err := am.CreateWithAPIKey(accountName, apiURL, apiKey, &orgInfo); err != nil {
			return fmt.Errorf("failed to create account: %w", err)
		}

		// Set as current
		if err := am.Use(accountName); err != nil {
			return fmt.Errorf("failed to set current account: %w", err)
		}

		ui.PrintSuccess("Account created successfully!")
		ui.PrintInfo("Current account: %s", accountName)
		ui.PrintInfo("Organization: %s", org.Name)
	}

	return nil
}

func runAccountRemove(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	accountName := args[0]

	// Confirm deletion
	account, err := am.Get(accountName)
	if err != nil {
		return err
	}

	orgInfo := ""
	if account.IsOAuth() {
		orgInfo = fmt.Sprintf(" (OAuth, %d organizations)", len(account.Organizations))
	} else if account.Organization != nil {
		orgInfo = fmt.Sprintf(" (API key, %s)", account.Organization.Name)
	}

	confirm, err := ui.Confirm(fmt.Sprintf("Remove account '%s'%s?", accountName, orgInfo), false)
	if err != nil {
		return err
	}

	if !confirm {
		ui.PrintInfo("Account removal cancelled")
		return nil
	}

	if err := am.Remove(accountName); err != nil {
		return err
	}

	ui.PrintSuccess("Account removed: %s", accountName)

	// Show new current account if any
	if current, err := am.GetCurrent(); err == nil {
		ui.PrintInfo("Current account: %s", current.Name)
	}

	return nil
}

func runAccountRefresh(cmd *cobra.Command, args []string) error {
	// Display account/org info
	if err := displayAccountInfo(); err != nil {
		ui.PrintWarning("Could not display account info: %v", err)
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	account, err := am.GetCurrent()
	if err != nil {
		return fmt.Errorf("no current account: %w", err)
	}

	if !account.IsOAuth() {
		return fmt.Errorf("token refresh is only available for OAuth accounts")
	}

	if !account.IsTokenExpired() {
		ui.PrintInfo("Token is still valid (expires %s)", account.Auth.ExpiresAt.Format(time.RFC3339))
		return nil
	}

	ui.PrintInfo("Refreshing OAuth token...")

	// Create SDK client without authentication (OAuth token endpoint doesn't require auth)
	client := dotenv.NewClient(
		dotenv.WithBaseURL(account.APIURL),
	)

	// Check for TLS skip verify (development mode)
	if config.ShouldSkipTLSVerify() {
		client = dotenv.NewClient(
			dotenv.WithBaseURL(account.APIURL),
			dotenv.WithInsecureSkipVerify(),
		)
		}

	// Refresh token using SDK
	sdkTokenResp, _, err := client.OAuth.RefreshToken(cmd.Context(), account.Auth.RefreshToken, constants.OAuthClientID)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update account with new tokens
	tokenResp := config.TokenResponse{
		AccessToken:  sdkTokenResp.AccessToken,
		RefreshToken: sdkTokenResp.RefreshToken,
		TokenType:    sdkTokenResp.TokenType,
		ExpiresIn:    sdkTokenResp.ExpiresIn,
	}

	if err := am.RefreshToken(account.Name, tokenResp); err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	ui.PrintSuccess("Token refreshed successfully!")
	ui.PrintInfo("New token expires: %s", time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second).Format(time.RFC3339))

	return nil
}

func runAccountRename(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	oldName := args[0]
	newName := args[1]

	// Validate new name
	validator := config.NewValidator()
	if err := validator.ValidateAccountName(newName); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	if err := am.Rename(oldName, newName); err != nil {
		return err
	}

	ui.PrintSuccess("Account renamed from '%s' to '%s'", oldName, newName)

	// Show if it's the current account
	if current, err := am.GetCurrent(); err == nil && current.Name == newName {
		ui.PrintInfo("This is the current account")
	}

	return nil
}
