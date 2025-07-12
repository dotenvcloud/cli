package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations within accounts",
	Long: `Manage organizations for the current account.

For OAuth accounts, you can switch between multiple organizations.
For API key accounts, only the single organization is available.`,
	Example: `  # List organizations for current account
  dotenv org list

  # Switch to a different organization
  dotenv org use acme-corp

  # Refresh organization list
  dotenv org refresh`,
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations for the current account",
	RunE:  runOrgList,
}

var orgUseCmd = &cobra.Command{
	Use:   "use [organization]",
	Short: "Switch to a different organization",
	Long: `Switch to a different organization within the current account.

You can specify the organization by its slug or ULID.`,
	Args: cobra.ExactArgs(1),
	RunE: runOrgUse,
}

var orgRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the organization list",
	Long: `Refresh the organization list for the current account.

This fetches the latest organizations from the API and updates
the local cache.`,
	RunE: runOrgRefresh,
}
var orgShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current organization details",
	RunE:  runOrgShow,
}

func init() {
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgUseCmd)
	orgCmd.AddCommand(orgRefreshCmd)
	orgCmd.AddCommand(orgShowCmd)
}

func runOrgList(cmd *cobra.Command, args []string) error {
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

	if account.IsAPIKey() {
		// API key account - single organization
		if account.Organization == nil {
			ui.PrintWarning("No organization information available. Run 'dotenv org refresh' to fetch.")
			return nil
		}

		ui.PrintInfo("Organization for API key account '%s':", account.Name)
		fmt.Printf("  Name: %s\n", account.Organization.Name)
		fmt.Printf("  Slug: %s\n", account.Organization.Slug)
		fmt.Printf("  ULID: %s\n", account.Organization.ULID)

		if account.OrganizationFetchedAt != nil {
			fmt.Printf("  Last Updated: %s\n", account.OrganizationFetchedAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	}

	// OAuth account - multiple organizations
	if len(account.Organizations) == 0 {
		ui.PrintWarning("No organizations found. Run 'dotenv org refresh' to fetch.")
		return nil
	}

	currentOrg, _ := account.GetCurrentOrganization()
	currentULID := ""
	if currentOrg != nil {
		currentULID = currentOrg.ULID
	}

	ui.PrintInfo("Organizations for OAuth account '%s':", account.Name)

	// Create table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CURRENT\tNAME\tSLUG\tULID")

	for _, org := range account.Organizations {
		current := " "
		if org.ULID == currentULID {
			current = "*"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", current, org.Name, org.Slug, org.ULID)
	}

	w.Flush()

	if account.OrganizationsFetchedAt != nil {
		fmt.Printf("\nLast Updated: %s\n", account.OrganizationsFetchedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runOrgUse(cmd *cobra.Command, args []string) error {
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

	if account.IsAPIKey() {
		return fmt.Errorf("cannot switch organizations for API key account")
	}

	identifier := args[0]

	// Resolve organization
	org, err := config.ResolveOrganization(identifier, account.Organizations)
	if err != nil {
		return err
	}

	// Set as current organization
	if err := am.SetOrganization(account.Name, org.ULID); err != nil {
		return err
	}

	ui.PrintSuccess("Switched to organization: %s", org.Name)
	ui.PrintInfo("Slug: %s", org.Slug)

	return nil
}

func runOrgRefresh(cmd *cobra.Command, args []string) error {
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

	ui.PrintInfo("Refreshing organizations...")

	// Create API client
	var client *dotenv.Client

	if account.IsOAuth() {
		// Check if token needs refresh
		if account.IsTokenExpired() {
			ui.PrintInfo("Token expired, refreshing...")
			// TODO: Implement token refresh here
			return fmt.Errorf("token expired, please run 'dotenv account refresh' first")
		}

		client = dotenv.NewClient(
			dotenv.WithBearerToken(account.Auth.AccessToken),
			dotenv.WithBaseURL(account.APIURL),
		)
	} else {
		// API key account
		client = dotenv.NewClient(
			dotenv.WithAPIKey(account.Auth.APIKey),
			dotenv.WithBaseURL(account.APIURL),
		)
	}

	// Set TLS skip verify for development
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		client.SetTLSSkipVerify(true)
	}

	// Fetch organizations
	orgs, _, err := client.Organizations.List(cmd.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to fetch organizations: %w", err)
	}

	if len(orgs) == 0 {
		return fmt.Errorf("no organizations found")
	}

	// Convert to OrgInfo format
	var orgInfos []config.OrgInfo
	for _, org := range orgs {
		orgInfos = append(orgInfos, config.OrgInfo{
			ULID: org.Slug, // Laravel returns ULID in slug field
			Name: org.Name,
			Slug: org.Slug, // For now, using ULID as slug
		})
	}

	// Update account with new organizations
	orgRemoved, err := am.RefreshOrganizations(account.Name, orgInfos)
	if err != nil {
		return err
	}

	if orgRemoved {
		ui.PrintWarning("Current organization no longer exists. Please select a new one with 'dotenv org use'")
	}

	ui.PrintSuccess("Organizations refreshed successfully!")

	if account.IsOAuth() {
		ui.PrintInfo("Found %d organization(s)", len(orgInfos))

		// Show current organization
		if currentOrg, err := account.GetCurrentOrganization(); err == nil {
			ui.PrintInfo("Current organization: %s", currentOrg.Name)
		}
	} else {
		ui.PrintInfo("Organization: %s", orgInfos[0].Name)
	}

	return nil
}

func runOrgShow(cmd *cobra.Command, args []string) error {
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

	currentOrg, err := account.GetCurrentOrganization()
	if err != nil {
		return fmt.Errorf("no organization selected: %w", err)
	}

	ui.PrintInfo("Current organization details:")
	fmt.Printf("  Name: %s\n", currentOrg.Name)
	fmt.Printf("  Slug: %s\n", currentOrg.Slug)
	fmt.Printf("  ULID: %s\n", currentOrg.ULID)

	// Show when it was last refreshed
	if account.IsOAuth() && account.OrganizationsFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationsFetchedAt.Format("2006-01-02 15:04:05"))
	} else if account.IsAPIKey() && account.OrganizationFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationFetchedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}
