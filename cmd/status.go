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

func runStatus(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	// Check if any accounts exist
	accounts := am.List()
	if len(accounts) == 0 {
		ui.PrintWarning("No accounts configured.")
		ui.PrintInfo("Run 'dotenv account add' to add an account.")
		return nil
	}

	// Get current account
	account, err := am.GetCurrent()
	if err != nil {
		ui.PrintWarning("No current account selected.")
		ui.PrintInfo("Available accounts:")
		for _, name := range accounts {
			fmt.Printf("  - %s\n", name)
		}
		ui.PrintInfo("\nRun 'dotenv account use <name>' to select an account.")
		return nil
	}

	// Display account info
	ui.PrintInfo("Current Account")
	fmt.Printf("  Name: %s\n", account.Name)
	fmt.Printf("  Type: %s\n", account.AuthType)
	fmt.Printf("  API URL: %s\n", account.APIURL)
	fmt.Printf("  Created: %s\n", account.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Last Used: %s\n", account.LastUsed.Format("2006-01-02 15:04:05"))

	// Display authentication info
	fmt.Println()
	ui.PrintInfo("Authentication")
	if account.IsOAuth() {
		fmt.Printf("  Token Type: %s\n", account.Auth.TokenType)

		if account.IsTokenExpired() {
			ui.PrintWarning("  Token Status: EXPIRED")
			fmt.Printf("  Expired At: %s\n", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))
			ui.PrintInfo("  Run 'dotenv account refresh' to refresh the token")
		} else {
			fmt.Printf("  Token Status: Valid\n")
			fmt.Printf("  Expires At: %s\n", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))

			// Show time until expiry
			timeUntilExpiry := time.Until(account.Auth.ExpiresAt)
			if timeUntilExpiry < 24*time.Hour {
				ui.PrintWarning("  Expires In: %s", timeUntilExpiry.Round(time.Minute))
			} else {
				fmt.Printf("  Expires In: %s\n", timeUntilExpiry.Round(time.Hour))
			}
		}

		// Refresh token info
		if account.IsRefreshTokenExpired() {
			ui.PrintWarning("  Refresh Token: EXPIRED")
		} else {
			fmt.Printf("  Refresh Token: Valid (expires %s)\n",
				account.Auth.RefreshTokenExpiresAt.Format("2006-01-02"))
		}
	} else {
		fmt.Printf("  API Key: %s...%s\n",
			account.Auth.APIKey[:12],
			account.Auth.APIKey[len(account.Auth.APIKey)-4:])
	}

	// Display organization info
	fmt.Println()
	ui.PrintInfo("Organization")

	if account.IsOAuth() {
		if len(account.Organizations) == 0 {
			ui.PrintWarning("  No organizations available")
			ui.PrintInfo("  Run 'dotenv org refresh' to fetch organizations")
		} else {
			org, err := account.GetCurrentOrganization()
			if err != nil {
				ui.PrintWarning("  No organization selected")
				ui.PrintInfo("  Run 'dotenv org list' to see available organizations")
				ui.PrintInfo("  Run 'dotenv org use <ulid>' to select one")
			} else {
				fmt.Printf("  Current: %s\n", org.Name)
				fmt.Printf("  ULID: %s\n", org.ULID)
			}

			fmt.Printf("  Total Available: %d\n", len(account.Organizations))

			if account.OrganizationsFetchedAt != nil {
				age := time.Since(*account.OrganizationsFetchedAt)
				if age > 24*time.Hour {
					ui.PrintWarning("  Last Fetched: %s ago (consider refreshing)",
						age.Round(time.Hour))
				} else {
					fmt.Printf("  Last Fetched: %s ago\n", age.Round(time.Minute))
				}
			}
		}
	} else {
		// API key account
		if account.Organization == nil {
			ui.PrintWarning("  No organization information")
			ui.PrintInfo("  Run 'dotenv org refresh' to fetch organization details")
		} else {
			fmt.Printf("  Name: %s\n", account.Organization.Name)
			fmt.Printf("  ULID: %s\n", account.Organization.ULID)

			if account.OrganizationFetchedAt != nil {
				fmt.Printf("  Last Fetched: %s ago\n",
					time.Since(*account.OrganizationFetchedAt).Round(time.Minute))
			}
		}
	}

	// Show command to switch accounts if there are multiple
	if len(accounts) > 1 {
		fmt.Println()
		ui.PrintInfo("Other Accounts")
		for _, name := range accounts {
			if name != account.Name {
				fmt.Printf("  - %s\n", name)
			}
		}
		ui.PrintInfo("\nRun 'dotenv account use <name>' to switch accounts")
	}

	return nil
}
