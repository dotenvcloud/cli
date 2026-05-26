package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/olekukonko/tablewriter"
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !orgNoRefresh {
			if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
				ui.PrintWarning("Could not refresh organizations: %v", err)
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !orgNoRefresh {
			if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
				ui.PrintWarning("Could not refresh organizations: %v", err)
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

func runOrgList(cmd *cobra.Command, args []string) error {
	// Display account/org info
	if err := displayAccountInfo(); err != nil {
		ui.PrintWarning("Could not display account info: %v", err)
	}

	// Use client without org context since we're listing organizations
	client, err := getAPIClientWithoutOrgContext()
	if err != nil {
		return err
	}

	ui.PrintInfo("Fetching organizations...")

	orgs, resp, err := client.Organizations.List(cmd.Context(), nil)
	if err != nil {
		// Check if using API key authentication
		if resp != nil && resp.StatusCode == 403 {
			return fmt.Errorf("API key authentication only shows the organization tied to the key. Use OAuth for listing all organizations")
		}
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(orgs) == 0 {
		ui.PrintWarning("No organizations found")
		return nil
	}

	switch orgListFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(orgs)

	case "yaml":
		// Simple YAML output
		// Get current organization for comparison
		account := accountForErrorContext()
		currentOrgID := ""
		if account != nil {
			if account.IsOAuth() && account.CurrentOrganization != "" {
				currentOrgID = account.CurrentOrganization
			} else if account.Organization != nil {
				currentOrgID = account.Organization.ULID
			}
		}

		for _, org := range orgs {
			// Use ID if ULID is empty (API might return ULID in ID field)
			ulid := org.ULID
			if ulid == "" && org.ID != "" {
				ulid = org.ID
			}

			fmt.Printf("- name: %s\n", org.Name)
			fmt.Printf("  ulid: %s\n", ulid)
			fmt.Printf("  plan: %s\n", org.PlanName)
			fmt.Printf("  status: %s\n", org.Status)
			if org.ID == currentOrgID || org.ULID == currentOrgID {
				fmt.Printf("  current: true\n")
			}
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "ULID", "PLAN", "STATUS", "CURRENT")

		// Get current organization for comparison
		account := accountForErrorContext()
		currentOrgID := ""
		if account != nil {
			if account.IsOAuth() && account.CurrentOrganization != "" {
				currentOrgID = account.CurrentOrganization
			} else if account.Organization != nil {
				currentOrgID = account.Organization.ULID
			}
		}

		for _, org := range orgs {
			current := ""
			if org.ID == currentOrgID || org.ULID == currentOrgID {
				current = "*"
			}

			// Use ID if ULID is empty (API might return ULID in ID field)
			ulid := org.ULID
			if ulid == "" && org.ID != "" {
				ulid = org.ID
			}

			table.Append([]string{
				org.Name,
				ulid,
				org.PlanName,
				org.Status,
				current,
			})
		}

		table.Render()
		return nil
	}
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

	// Check if we need to refresh organizations
	if len(account.Organizations) == 0 {
		ui.PrintWarning("No organizations found. Run 'dotenv org refresh' to fetch.")
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
			return fmt.Errorf("selection cancelled")
		}

		selectedOrg = orgMap[selected]
	}

	// Set as current organization
	if err := am.SetOrganization(account.Name, selectedOrg.ULID); err != nil {
		return err
	}

	ui.PrintSuccess("Switched to organization: %s", selectedOrg.Name)
	ui.PrintInfo("ULID: %s", selectedOrg.ULID)

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

	// Use factory to create API client
	factory := client.NewFactory(config.GetAPIURL(""))

	// Handle OAuth token refresh if needed
	var sdkClient *dotenv.Client
	if account.IsOAuth() {
		// Use factory to handle refresh
		sdkClient, err = factory.RefreshTokenAndCreateClient(cmd.Context(), account, am, false)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
	} else {
		// Create client from account (without organization context)
		sdkClient, err = factory.NewClientFromAccount(account, false)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
	}

	// Fetch organizations
	orgs, _, err := sdkClient.Organizations.List(cmd.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to fetch organizations: %w", err)
	}

	if len(orgs) == 0 {
		return fmt.Errorf("no organizations found")
	}

	// Convert to OrgInfo format
	var orgInfos []config.OrgInfo
	for _, org := range orgs {
		// Use ID if ULID is empty (API returns ULID in ID field)
		ulid := org.ULID
		if ulid == "" && org.ID != "" {
			ulid = org.ID
		}
		orgInfos = append(orgInfos, config.OrgInfo{
			ULID: ulid,
			Name: org.Name,
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
	fmt.Printf("  ULID: %s\n", currentOrg.ULID)

	// Show when it was last refreshed
	if account.IsOAuth() && account.OrganizationsFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationsFetchedAt.Format("2006-01-02 15:04:05"))
	} else if account.IsAPIKey() && account.OrganizationFetchedAt != nil {
		fmt.Printf("  Last Updated: %s\n", account.OrganizationFetchedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}
