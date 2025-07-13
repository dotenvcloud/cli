package cmd

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var organizationsCmd = &cobra.Command{
	Use:     "organizations",
	Aliases: []string{"orgs"},
	Short:   "Manage organizations",
	Long: `Manage DotEnv organizations.

For OAuth accounts, you can switch between organizations you have access to.
For API key accounts, you are limited to the organization tied to the key.`,
	Example: `  # List all organizations
  dotenv organizations list

  # Switch to a different organization
  dotenv organizations use 01HQNWK1XQXQY1XQXQY1XQXQY1

  # Switch interactively
  dotenv organizations use

  # Show current organization
  dotenv organizations current`,
}

var organizationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all organizations",
	Long:  "List all organizations you have access to",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Just call the existing list organizations command
		return listOrganizations(cmd)
	},
}

var organizationUseCmd = &cobra.Command{
	Use:   "use [ulid]",
	Short: "Switch to a different organization",
	Long: `Switch to a different organization by ULID.

If no ULID is provided, an interactive selection will be shown.
This command only works with OAuth authentication.`,
	RunE: runOrganizationUse,
}

var organizationCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current organization",
	RunE:  runOrganizationCurrent,
}

func init() {
	organizationsCmd.AddCommand(organizationListCmd)
	organizationsCmd.AddCommand(organizationUseCmd)
	organizationsCmd.AddCommand(organizationCurrentCmd)
}

func runOrganizationUse(cmd *cobra.Command, args []string) error {
	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return fmt.Errorf("failed to get current account: %w", err)
	}

	// Check if using OAuth
	if !account.IsOAuth() {
		return fmt.Errorf("organization switching requires OAuth authentication. API keys are tied to a single organization")
	}

	// Get API client without organization context (since we're listing/selecting organizations)
	client, err := getAPIClientWithoutOrgContext()
	if err != nil {
		return err
	}

	// Fetch organizations
	ui.PrintInfo("Fetching organizations...")
	orgs, _, err := client.Organizations.List(context.Background(), nil)
	if err != nil {
		return HandleAPIError(err, account)
	}

	if len(orgs) == 0 {
		return fmt.Errorf("no organizations found")
	}

	var selectedOrg *dotenv.Organization

	// If ULID provided, find it
	if len(args) > 0 {
		ulid := args[0]
		for _, org := range orgs {
			// Check both ULID and ID fields since API might return ULID in ID field
			if org.ULID == ulid || org.ID == ulid {
				selectedOrg = org
				break
			}
		}
		if selectedOrg == nil {
			return fmt.Errorf("organization not found or you don't have access: %s", ulid)
		}
	} else {
		// Interactive selection
		var options []string
		orgMap := make(map[string]*dotenv.Organization)

		// Get current org ID for highlighting
		currentOrgID := ""
		if account.CurrentOrganization != "" {
			currentOrgID = account.CurrentOrganization
		}

		for _, org := range orgs {
			// Use ID if ULID is empty (API might return ULID in ID field)
			ulid := org.ULID
			if ulid == "" && org.ID != "" {
				ulid = org.ID
			}
			label := fmt.Sprintf("%s (%s) - %s", org.Name, ulid, org.PlanName)
			if org.ID == currentOrgID || org.ULID == currentOrgID {
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

	// Update account configuration
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	// Update the current organization using the account manager
	// Use ID if ULID is empty (API might return ULID in ID field)
	orgID := selectedOrg.ULID
	if orgID == "" && selectedOrg.ID != "" {
		orgID = selectedOrg.ID
	}
	if err := am.SetOrganization(account.Name, orgID); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	ui.PrintSuccess("Switched to organization: %s", selectedOrg.Name)
	return nil
}

func runOrganizationCurrent(cmd *cobra.Command, args []string) error {
	account, err := getCurrentAccount()
	if err != nil {
		return fmt.Errorf("failed to get current account: %w", err)
	}

	if account.IsOAuth() {
		currentOrg, err := account.GetCurrentOrganization()
		if err != nil {
			fmt.Println("No organization selected")
			return nil
		}
		fmt.Printf("Current organization: %s (%s)\n", currentOrg.Name, currentOrg.ULID)
	} else if account.Organization != nil {
		fmt.Printf("Organization: %s (%s)\n", account.Organization.Name, account.Organization.ULID)
		fmt.Println("Note: API key authentication is tied to this organization")
	} else {
		fmt.Println("No organization information available")
	}

	return nil
}