package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/auth"
	"github.com/dotenvcloud/cli/internal/client"
	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/constants"
	"github.com/dotenvcloud/cli/internal/ui"
	"github.com/dotenvcloud/cli/internal/utils"
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

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
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
func runAccountList(_ *cobra.Command, _ []string) error {
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
		ui.PrintWarningf("No accounts configured. Run 'dotenv account add' to add an account.")
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

func runAccountUse(_ *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	accountName := args[0]
	if useErr := am.Use(accountName); useErr != nil {
		return useErr
	}

	account, err := am.Get(accountName)
	if err != nil {
		return err
	}

	ui.PrintSuccessf("Switched to account: %s", accountName)

	// Show organization info
	if account.IsOAuth() {
		org, err := account.GetCurrentOrganization()
		if err == nil {
			ui.PrintInfof("Current organization: %s", org.Name)
		}
	} else if account.Organization != nil {
		ui.PrintInfof("Organization: %s", account.Organization.Name)
	}

	return nil
}

func runAccountAdd(cmd *cobra.Command, _ []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	apiURL := resolveAccountAPIURL(am)

	authMethod, err := ui.Select("How would you like to authenticate?", []string{
		"Login via browser (OAuth)",
		"Enter API key manually",
	})
	if err != nil {
		return err
	}

	if strings.Contains(authMethod, "OAuth") {
		return runAccountAddOAuth(cmd, am, apiURL)
	}
	return runAccountAddAPIKey(cmd, am, apiURL)
}

func resolveAccountAPIURL(am *config.AccountManager) string {
	apiURL := getAPIURL()
	if currentAccount, getErr := am.GetCurrent(); getErr == nil && currentAccount.APIURL != "" {
		apiURL = currentAccount.APIURL
		ui.PrintInfof("Using API URL from current account: %s", apiURL)
	}
	return apiURL
}

func runAccountAddOAuth(cmd *cobra.Command, am *config.AccountManager, apiURL string) error {
	ui.PrintInfof("Starting OAuth login...")
	opts := auth.BrowserLoginOptions{
		APIUrl:        apiURL,
		CallbackPort:  "",
		NoBrowser:     false,
		IsInteractive: true,
	}
	if err := auth.DoBrowserLogin(cmd.Context(), am, opts); err != nil {
		return fmt.Errorf("OAuth login failed: %w", err)
	}
	return nil
}

func runAccountAddAPIKey(cmd *cobra.Command, am *config.AccountManager, apiURL string) error {
	apiKey, err := ui.Password("Enter your API key")
	if err != nil {
		return err
	}

	validator := config.NewValidator()
	if validateErr := validator.ValidateAPIKey(apiKey); validateErr != nil {
		return fmt.Errorf("invalid API key: %w", validateErr)
	}

	org, err := verifyAPIKeyOrg(cmd.Context(), apiKey)
	if err != nil {
		return err
	}

	ulid := org.ULID
	if ulid == "" && org.ID != "" {
		ulid = org.ID
	}
	orgInfo := config.OrgInfo{ULID: ulid, Name: org.Name}

	defaultName := utils.Slugify(org.Name)
	accountName, err := ui.Input("Account name", defaultName, nil)
	if err != nil {
		return err
	}

	if err := am.CreateWithAPIKey(accountName, apiURL, apiKey, &orgInfo); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	if err := am.Use(accountName); err != nil {
		return fmt.Errorf("failed to set current account: %w", err)
	}

	ui.PrintSuccessf("Account created successfully!")
	ui.PrintInfof("Current account: %s", accountName)
	ui.PrintInfof("Organization: %s", org.Name)
	return nil
}

func verifyAPIKeyOrg(ctx context.Context, apiKey string) (*dotenv.Organization, error) {
	factory := client.NewFactory(getAPIURL())
	sdkClient := factory.NewClientFromAPIKey(apiKey, getAPIURL(), "")

	orgs, resp, err := sdkClient.Organizations.List(ctx, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to verify API key: %w", err)
	}
	if len(orgs) == 0 {
		return nil, fmt.Errorf("no organizations found for this API key")
	}
	return orgs[0], nil
}

// revokeOAuthToken best-effort revokes an account's OAuth token on the server
// before its local credentials are deleted, so logging out invalidates the
// session everywhere rather than just on this machine. API-key accounts have
// nothing to revoke. Failures (offline, already-expired token, a server without
// the endpoint) only warn — local removal must always proceed.
func revokeOAuthToken(ctx context.Context, account *config.Account) {
	if account == nil || !account.IsOAuth() || account.Auth.RefreshToken == "" {
		return
	}

	factory := client.NewFactory(account.APIURL)
	sdkClient := factory.NewUnauthenticatedClient(account.APIURL, config.ShouldSkipTLSVerify())

	resp, err := sdkClient.OAuth.RevokeToken(ctx, account.Auth.RefreshToken, constants.OAuthClientID)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		ui.PrintWarningf("Could not revoke token on server (removing locally anyway): %v", err)
		return
	}

	ui.PrintInfof("Revoked token on server")
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
		ui.PrintInfof("Account removal canceled")
		return nil
	}

	revokeOAuthToken(cmd.Context(), account)

	if err := am.Remove(accountName); err != nil {
		return err
	}

	ui.PrintSuccessf("Account removed: %s", accountName)

	// Show new current account if any
	if current, err := am.GetCurrent(); err == nil {
		ui.PrintInfof("Current account: %s", current.Name)
	}

	return nil
}

func runAccountRefresh(cmd *cobra.Command, _ []string) error {
	// Display account/org info
	if err := displayAccountInfo(); err != nil {
		ui.PrintWarningf("Could not display account info: %v", err)
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
		ui.PrintInfof("Token is still valid (expires %s)", account.Auth.ExpiresAt.Format(time.RFC3339))
		return nil
	}

	ui.PrintInfof("Refreshing OAuth token...")

	// Create SDK client without authentication (OAuth token endpoint doesn't require auth)
	factory := client.NewFactory(account.APIURL)
	sdkClient := factory.NewUnauthenticatedClient(account.APIURL, config.ShouldSkipTLSVerify())

	// Refresh token using SDK
	sdkTokenResp, refreshResp, err := sdkClient.OAuth.RefreshToken(cmd.Context(), account.Auth.RefreshToken, constants.OAuthClientID)
	if refreshResp != nil {
		defer refreshResp.Body.Close()
	}
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

	ui.PrintSuccessf("Token refreshed successfully!")
	ui.PrintInfof("New token expires: %s", time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second).Format(time.RFC3339))

	return nil
}

func runAccountRename(_ *cobra.Command, args []string) error {
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

	ui.PrintSuccessf("Account renamed from '%s' to '%s'", oldName, newName)

	// Show if it's the current account
	if current, err := am.GetCurrent(); err == nil && current.Name == newName {
		ui.PrintInfof("This is the current account")
	}

	return nil
}
