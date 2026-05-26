package cmd

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

var (
	orgListFormat string
	orgNoRefresh  bool
)

var orgCmd = &cobra.Command{
	Use:     "org",
	Aliases: []string{"orgs", "organization", "organizations"},
	Short:   "Manage organizations within accounts",
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
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if !orgNoRefresh {
			if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
				ui.PrintWarningf("Could not refresh organizations: %v", err)
				// Continue anyway with cached data
			}
		}
		return nil
	},
	RunE: runOrgList,
}

var orgUseCmd = &cobra.Command{
	Use:   "use [organization]",
	Short: "Switch to a different organization",
	Long: `Switch to a different organization within the current account.

You can specify the organization by its slug or ULID.
If no organization is specified, an interactive selection will be shown.`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if !orgNoRefresh {
			if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
				ui.PrintWarningf("Could not refresh organizations: %v", err)
				// Continue anyway with cached data
			}
		}
		return nil
	},
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

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgUseCmd)
	orgCmd.AddCommand(orgRefreshCmd)
	orgCmd.AddCommand(orgShowCmd)

	// Add format flag to list command
	orgListCmd.Flags().StringVarP(&orgListFormat, "format", "f", "table",
		"Output format: table, json, yaml")

	// Add no-refresh flag to commands that support auto-refresh
	orgCmd.PersistentFlags().BoolVar(&orgNoRefresh, "no-refresh", false,
		"Skip automatic organization refresh")
}

func runOrgList(cmd *cobra.Command, _ []string) error {
	// Display account/org info
	if err := displayAccountInfo(); err != nil {
		ui.PrintWarningf("Could not display account info: %v", err)
	}

	// Use client without org context since we're listing organizations
	client, err := getAPIClientWithoutOrgContext()
	if err != nil {
		return err
	}

	ui.PrintInfof("Fetching organizations...")

	orgs, resp, err := client.Organizations.List(cmd.Context(), nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 403 {
			return fmt.Errorf("API key authentication only shows the organization tied to the key. Use OAuth for listing all organizations")
		}
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(orgs) == 0 {
		ui.PrintWarningf("No organizations found")
		return nil
	}

	return renderOrgList(orgs, orgListFormat)
}

func runOrgUse(_ *cobra.Command, args []string) error {
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

	if account.IsAPIKey() {
		return fmt.Errorf("cannot switch organizations for API key account")
	}

	// Check if we need to refresh organizations
	if len(account.Organizations) == 0 {
		ui.PrintWarningf("No organizations found. Run 'dotenv org refresh' to fetch.")
		return nil
	}

	var selectedOrg *config.OrgInfo

	// If argument provided, use it
	if len(args) > 0 {
		identifier := args[0]

		// Resolve organization
		org, err := config.ResolveOrganization(identifier, account.Organizations)
		if err != nil {
			return err
		}
		selectedOrg = org
	} else {
		// Interactive selection
		var options []string
		orgMap := make(map[string]*config.OrgInfo)

		// Get current org for highlighting
		currentOrg, _ := account.GetCurrentOrganization()
		currentULID := ""
		if currentOrg != nil {
			currentULID = currentOrg.ULID
		}

		for i := range account.Organizations {
			org := &account.Organizations[i]
			label := fmt.Sprintf("%s (%s)", org.Name, org.ULID)
			if org.ULID == currentULID {
				label = fmt.Sprintf("→ %s", label)
			}
			options = append(options, label)
			orgMap[label] = org
		}

		var selected string
		prompt := &survey.Select{
			Message: "Select an organization:",
			Options: options,
		}

		if err := survey.AskOne(prompt, &selected); err != nil {
			return fmt.Errorf("selection canceled")
		}

		selectedOrg = orgMap[selected]
	}

	// Set as current organization
	if err := am.SetOrganization(account.Name, selectedOrg.ULID); err != nil {
		return err
	}

	ui.PrintSuccessf("Switched to organization: %s", selectedOrg.Name)
	ui.PrintInfof("ULID: %s", selectedOrg.ULID)

	return nil
}

func runOrgRefresh(cmd *cobra.Command, _ []string) error {
	if err := displayAccountInfo(); err != nil {
		ui.PrintWarningf("Could not display account info: %v", err)
	}

	am, account, err := loadCurrentAccount()
	if err != nil {
		return err
	}

	ui.PrintInfof("Refreshing organizations...")

	sdkClient, err := buildRefreshClient(cmd.Context(), account, am)
	if err != nil {
		return err
	}

	orgs, orgResp, err := sdkClient.Organizations.List(cmd.Context(), nil)
	if orgResp != nil {
		defer orgResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to fetch organizations: %w", err)
	}
	if len(orgs) == 0 {
		return fmt.Errorf("no organizations found")
	}

	orgInfos := toOrgInfos(orgs)
	orgRemoved, err := am.RefreshOrganizations(account.Name, orgInfos)
	if err != nil {
		return err
	}
	if orgRemoved {
		ui.PrintWarningf("Current organization no longer exists. Please select a new one with 'dotenv org use'")
	}

	ui.PrintSuccessf("Organizations refreshed successfully!")
	printRefreshSummary(account, orgInfos)
	return nil
}

func loadCurrentAccount() (*config.AccountManager, *config.Account, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, nil, err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return nil, nil, err
	}
	account, err := am.GetCurrent()
	if err != nil {
		return nil, nil, fmt.Errorf("no current account: %w", err)
	}
	return am, account, nil
}

func buildRefreshClient(ctx context.Context, account *config.Account, am *config.AccountManager) (*dotenv.Client, error) {
	factory := client.NewFactory(config.GetAPIURL(""))
	if account.IsOAuth() {
		c, err := factory.RefreshTokenAndCreateClient(ctx, account, am, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create API client: %w", err)
		}
		return c, nil
	}
	c, err := factory.NewClientFromAccount(account, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	return c, nil
}

func toOrgInfos(orgs []*dotenv.Organization) []config.OrgInfo {
	orgInfos := make([]config.OrgInfo, 0, len(orgs))
	for _, org := range orgs {
		ulid := org.ULID
		if ulid == "" && org.ID != "" {
			ulid = org.ID
		}
		orgInfos = append(orgInfos, config.OrgInfo{ULID: ulid, Name: org.Name})
	}
	return orgInfos
}

func printRefreshSummary(account *config.Account, orgInfos []config.OrgInfo) {
	if account.IsOAuth() {
		ui.PrintInfof("Found %d organization(s)", len(orgInfos))
		if currentOrg, err := account.GetCurrentOrganization(); err == nil {
			ui.PrintInfof("Current organization: %s", currentOrg.Name)
		}
		return
	}
	ui.PrintInfof("Organization: %s", orgInfos[0].Name)
}

func runOrgShow(_ *cobra.Command, _ []string) error {
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

	currentOrg, err := account.GetCurrentOrganization()
	if err != nil {
		return fmt.Errorf("no organization selected: %w", err)
	}

	ui.PrintInfof("Current organization details:")
	fmt.Printf("  Name: %s\n", currentOrg.Name)
	fmt.Printf("  ULID: %s\n", currentOrg.ULID)

	// Show when it was last refreshed
	if account.IsOAuth() && account.OrganizationsFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationsFetchedAt.Format("2006-01-02 15:04:05"))
	} else if account.IsAPIKey() && account.OrganizationFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationFetchedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}
