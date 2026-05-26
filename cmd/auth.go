package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/ui"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication management",
	Long:  `Manage authentication and view authentication information.`,

	Example: `  # Show current user info
  dotenv auth info

  # Show user info with organization details
  dotenv auth info --verbose`,
}

// Subcommands
var authInfoCmd *cobra.Command

// Flags
var authInfoVerbose bool

func init() {
	// Info command
	authInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show authenticated user information",
		Long: `Display information about the currently authenticated user
including their name, email, and organization memberships.`,
		RunE: runAuthInfo,
	}
	authInfoCmd.Flags().BoolVarP(&authInfoVerbose, "verbose", "v", false,
		"show detailed information including organization permissions")

	// Add subcommands
	authCmd.AddCommand(authInfoCmd)
}

func runAuthInfo(cmd *cobra.Command, args []string) error {
	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Check if using API key
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = os.Getenv("DOTENV_API_KEY")
	}

	if apiKey != "" {
		// For API key authentication, show limited info
		ui.PrintInfo("Authentication: API Key")
		fmt.Printf("Token Prefix: %s...\n", apiKey[:min(12, len(apiKey))])

		// Get organization from config
		org := viper.GetString("organization")
		if org != "" {
			fmt.Printf("Organization: %s\n", org)
		}

		ui.PrintInfo("\nAPI key authentication has limited user information.")
		ui.PrintInfo("Use OAuth authentication for full user details.")
		return nil
	}

	// Get authenticated user info
	ui.PrintInfo("Fetching user information...")

	user, organizations, _, err := client.User.GetAuthenticatedUser(cmd.Context())
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	// Display user info
	fmt.Println()
	ui.PrintSuccess("Authenticated User")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Name:     %s\n", user.Name)
	fmt.Printf("Email:    %s\n", user.Email)
	fmt.Printf("ID:       %s\n", user.ID)
	fmt.Printf("Verified: %v\n", user.IsVerified)
	fmt.Printf("Created:  %s\n", user.CreatedAt.Format("2006-01-02"))

	// Display organizations
	if len(organizations) > 0 {
		fmt.Println()
		ui.PrintSuccess("Organizations")
		fmt.Println(strings.Repeat("─", 40))

		if authInfoVerbose {
			// Detailed table view
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSLUG\tROLE\tID\tJOINED")
			fmt.Fprintln(w, "────\t────\t────\t──\t──────")

			for _, org := range organizations {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					org.Name,
					org.Slug,
					org.Role,
					org.ID,
					org.JoinedAt.Format("2006-01-02"),
				)
			}
			w.Flush()
		} else {
			// Simple list
			for _, org := range organizations {
				fmt.Printf("• %s (%s) - %s\n", org.Name, org.Slug, org.Role)
			}
		}

		// Show current organization
		account, err := getCurrentAccount()
		if err == nil && account.IsOAuth() {
			if currentOrg, err := account.GetCurrentOrganization(); err == nil {
				fmt.Println()
				ui.PrintInfo("Current organization: %s", currentOrg.Name)
			}
		}
	} else {
		fmt.Println()
		ui.PrintWarning("No organization memberships found")
	}

	// Show authentication type
	fmt.Println()
	account, err := getCurrentAccount()
	if err == nil {
		if account.IsOAuth() {
			ui.PrintInfo("Authentication type: OAuth")
			if !account.IsTokenExpired() {
				ui.PrintInfo("Token expires: %s", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))
			} else {
				ui.PrintWarning("Token expired! Run 'dotenv refresh' to renew.")
			}
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
