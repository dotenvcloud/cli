package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
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

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
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

func runAuthInfo(cmd *cobra.Command, _ []string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = os.Getenv("DOTENV_API_KEY")
	}
	if apiKey != "" {
		printAPIKeyAuthInfo(apiKey)
		return nil
	}

	ui.PrintInfof("Fetching user information...")
	user, organizations, userResp, err := client.User.GetAuthenticatedUser(cmd.Context())
	if userResp != nil {
		defer userResp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	printUserInfo(user)
	printOrganizations(organizations)
	printAuthType()
	return nil
}

func printAPIKeyAuthInfo(apiKey string) {
	ui.PrintInfof("Authentication: API Key")
	fmt.Printf("Token Prefix: %s...\n", apiKey[:min(12, len(apiKey))])
	if org := viper.GetString("organization"); org != "" {
		fmt.Printf("Organization: %s\n", org)
	}
	ui.PrintInfof("\nAPI key authentication has limited user information.")
	ui.PrintInfof("Use OAuth authentication for full user details.")
}

func printUserInfo(user *dotenv.User) {
	fmt.Println()
	ui.PrintSuccessf("Authenticated User")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Name:     %s\n", user.Name)
	fmt.Printf("Email:    %s\n", user.Email)
	fmt.Printf("ID:       %s\n", user.ID)
	fmt.Printf("Verified: %v\n", user.IsVerified)
	fmt.Printf("Created:  %s\n", user.CreatedAt.Format("2006-01-02"))
}

func printOrganizations(organizations []*dotenv.UserOrganization) {
	if len(organizations) == 0 {
		fmt.Println()
		ui.PrintWarningf("No organization memberships found")
		return
	}

	fmt.Println()
	ui.PrintSuccessf("Organizations")
	fmt.Println(strings.Repeat("─", 40))

	if authInfoVerbose {
		w := tabwriter.NewWriter(ui.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSLUG\tROLE\tID\tJOINED")
		fmt.Fprintln(w, "────\t────\t────\t──\t──────")
		for _, org := range organizations {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				org.Name, org.Slug, org.Role, org.ID,
				org.JoinedAt.Format("2006-01-02"),
			)
		}
		_ = w.Flush()
	} else {
		for _, org := range organizations {
			fmt.Printf("• %s (%s) - %s\n", org.Name, org.Slug, org.Role)
		}
	}

	if account, acctErr := getCurrentAccount(); acctErr == nil && account.IsOAuth() {
		if currentOrg, orgErr := account.GetCurrentOrganization(); orgErr == nil {
			fmt.Println()
			ui.PrintInfof("Current organization: %s", currentOrg.Name)
		}
	}
}

func printAuthType() {
	fmt.Println()
	account, err := getCurrentAccount()
	if err != nil || !account.IsOAuth() {
		return
	}
	ui.PrintInfof("Authentication type: OAuth")
	if !account.IsTokenExpired() {
		ui.PrintInfof("Token expires: %s", account.Auth.ExpiresAt.Format("2006-01-02 15:04:05"))
	} else {
		ui.PrintWarningf("Token expired! Run 'dotenv refresh' to renew.")
	}
}
