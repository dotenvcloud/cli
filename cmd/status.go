package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current account and organization status",
	Long: `Display the current authentication status, including:
- Current account name and type
- Current organization
- Token expiry (for OAuth)
- Last used timestamp`,
	RunE: runStatus,
}

func runStatus(_ *cobra.Command, _ []string) error {
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
		ui.PrintWarningf("No accounts configured.")
		ui.PrintInfof("Run 'dotenv account add' to add an account.")
		return nil
	}

	account, err := am.GetCurrent()
	if err != nil {
		ui.PrintWarningf("No current account selected.")
		ui.PrintInfof("Available accounts:")
		for _, name := range accounts {
			fmt.Printf("  - %s\n", name)
		}
		ui.PrintInfof("\nRun 'dotenv account use <name>' to select an account.")
		return nil
	}

	printStatusAccount(account)
	printStatusAuth(account)
	printStatusOrganization(account)
	printStatusOtherAccounts(accounts, account.Name)

	return nil
}

func printStatusAccount(account *config.Account) {
	ui.PrintInfof("Current Account")
	fmt.Printf("  Name: %s\n", account.Name)
	fmt.Printf("  Type: %s\n", account.AuthType)
	fmt.Printf("  API URL: %s\n", account.APIURL)
	fmt.Printf("  Created: %s\n", account.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Last Used: %s\n", account.LastUsed.Format("2006-01-02 15:04:05"))
}

func printStatusAuth(account *config.Account) {
	fmt.Println()
	ui.PrintInfof("Authentication")
	if !account.IsOAuth() {
		fmt.Printf("  API Key: %s...%s\n",
			account.Auth.APIKey[:12],
			account.Auth.APIKey[len(account.Auth.APIKey)-4:])
		return
	}

	fmt.Printf("  Token Type: %s\n", account.Auth.TokenType)
	if account.IsTokenExpired() {
		ui.PrintWarningf("  Token Status: EXPIRED")
		fmt.Printf("  Expired At: %s\n", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))
		ui.PrintInfof("  Run 'dotenv account refresh' to refresh the token")
	} else {
		fmt.Printf("  Token Status: Valid\n")
		fmt.Printf("  Expires At: %s\n", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))
		timeUntilExpiry := time.Until(account.Auth.ExpiresAt)
		if timeUntilExpiry < 24*time.Hour {
			ui.PrintWarningf("  Expires In: %s", timeUntilExpiry.Round(time.Minute))
		} else {
			fmt.Printf("  Expires In: %s\n", timeUntilExpiry.Round(time.Hour))
		}
	}

	if account.IsRefreshTokenExpired() {
		ui.PrintWarningf("  Refresh Token: EXPIRED")
	} else {
		fmt.Printf("  Refresh Token: Valid (expires %s)\n",
			account.Auth.RefreshTokenExpiresAt.Format("2006-01-02"))
	}
}

func printStatusOrganization(account *config.Account) {
	fmt.Println()
	ui.PrintInfof("Organization")

	if account.IsOAuth() {
		printOAuthOrgs(account)
		return
	}

	if account.Organization == nil {
		ui.PrintWarningf("  No organization information")
		ui.PrintInfof("  Run 'dotenv org refresh' to fetch organization details")
		return
	}
	fmt.Printf("  Name: %s\n", account.Organization.Name)
	fmt.Printf("  ULID: %s\n", account.Organization.ULID)
	if account.OrganizationFetchedAt != nil {
		fmt.Printf("  Last Fetched: %s ago\n",
			time.Since(*account.OrganizationFetchedAt).Round(time.Minute))
	}
}

func printOAuthOrgs(account *config.Account) {
	if len(account.Organizations) == 0 {
		ui.PrintWarningf("  No organizations available")
		ui.PrintInfof("  Run 'dotenv org refresh' to fetch organizations")
		return
	}
	if org, err := account.GetCurrentOrganization(); err != nil {
		ui.PrintWarningf("  No organization selected")
		ui.PrintInfof("  Run 'dotenv org list' to see available organizations")
		ui.PrintInfof("  Run 'dotenv org use <ulid>' to select one")
	} else {
		fmt.Printf("  Current: %s\n", org.Name)
		fmt.Printf("  ULID: %s\n", org.ULID)
	}
	fmt.Printf("  Total Available: %d\n", len(account.Organizations))
	if account.OrganizationsFetchedAt != nil {
		age := time.Since(*account.OrganizationsFetchedAt)
		if age > 24*time.Hour {
			ui.PrintWarningf("  Last Fetched: %s ago (consider refreshing)", age.Round(time.Hour))
		} else {
			fmt.Printf("  Last Fetched: %s ago\n", age.Round(time.Minute))
		}
	}
}

func printStatusOtherAccounts(accounts []string, currentName string) {
	if len(accounts) <= 1 {
		return
	}
	fmt.Println()
	ui.PrintInfof("Other Accounts")
	for _, name := range accounts {
		if name != currentName {
			fmt.Printf("  - %s\n", name)
		}
	}
	ui.PrintInfof("\nRun 'dotenv account use <name>' to switch accounts")
}
