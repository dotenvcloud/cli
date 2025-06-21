package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var (
	listOrganization string
	listFormat       string
	listJSON         bool
)

var listCmd = &cobra.Command{
	Use:   "list [resource]",
	Short: "List resources",
	Long: `List DotEnv resources in a hierarchical structure.

Resources:
  organizations  - List all organizations
  projects      - List projects in current organization
  targets       - List targets in a project
  environments  - List environments in a target
  accounts      - List configured accounts`,

	Example: `  # List all projects in current organization
  dotenv list projects

  # List targets in a specific project
  dotenv list targets myproject

  # List environments in a project/target
  dotenv list environments myproject/production

  # List all configured accounts
  dotenv list accounts

  # List projects in a specific organization
  dotenv list projects --organization=acme-corp`,

	ValidArgs: []string{"organizations", "projects", "targets", "environments", "accounts"},
	RunE:      runList,
}

func init() {
	listCmd.Flags().StringVar(&listOrganization, "organization", "",
		"specify organization (overrides current account's organization)")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table",
		"output format (table, json, yaml)")
	listCmd.Flags().BoolVar(&listJSON, "json", false,
		"output as JSON (deprecated, use --format=json)")
}

func runList(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a resource to list")
	}

	// Handle deprecated --json flag
	if listJSON {
		listFormat = "json"
	}

	resource := args[0]

	// Display account/org info for resources that use API
	// (not for "accounts" since that's local config)
	if resource != "accounts" && viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			// Don't fail if we can't display account info
			ui.PrintWarning("Could not display account info: %v", err)
		}
	}

	switch resource {
	case "accounts":
		return listAccounts(cmd)

	case "organizations":
		return listOrganizations(cmd)

	case "projects":
		return listProjects(cmd, "")

	case "targets":
		if len(args) < 2 {
			return fmt.Errorf("specify project: dotenv list targets <project>")
		}
		return listTargets(cmd, args[1])

	case "environments":
		if len(args) < 2 {
			return fmt.Errorf("specify project/target: dotenv list environments <project>/<target>")
		}
		parts := strings.Split(args[1], "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid format: use project/target")
		}
		return listEnvironments(cmd, parts[0], parts[1])

	default:
		return fmt.Errorf("unknown resource: %s", resource)
	}
}

func listAccounts(cmd *cobra.Command) error {
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
		ui.PrintWarning("No accounts configured. Run 'dotenv init' to get started.")
		return nil
	}

	current, _ := am.GetCurrent()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	switch listFormat {
	case "json":
		// Build account details for JSON output
		type accountInfo struct {
			Name                string `json:"name"`
			Type                string `json:"type"`
			Organization        string `json:"organization"`
			APIURL              string `json:"api_url"`
			Current             bool   `json:"current"`
			UserEmail           string `json:"user_email,omitempty"`
		}
		
		accountList := []accountInfo{}
		for _, name := range accounts {
			account, err := am.Get(name)
			if err != nil {
				continue
			}
			
			info := accountInfo{
				Name:    name,
				Type:    account.AuthType,
				APIURL:  account.APIURL,
				Current: name == currentName,
			}
			
			if account.IsOAuth() {
				org, err := account.GetCurrentOrganization()
				if err == nil {
					info.Organization = org.Name
				}
				// TODO: Store user email in account metadata
			} else if account.Organization != nil {
				info.Organization = account.Organization.Name
			}
			
			accountList = append(accountList, info)
		}
		
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(accountList)

	case "yaml":
		// Simple YAML output
		for _, name := range accounts {
			account, err := am.Get(name)
			if err != nil {
				continue
			}
			
			fmt.Printf("- name: %s\n", name)
			fmt.Printf("  type: %s\n", account.AuthType)
			
			if account.IsOAuth() {
				org, err := account.GetCurrentOrganization()
				if err == nil {
					fmt.Printf("  organization: %s\n", org.Name)
				}
				// TODO: Store and display user email in account metadata
			} else if account.Organization != nil {
				fmt.Printf("  organization: %s\n", account.Organization.Name)
			}
			
			fmt.Printf("  api_url: %s\n", account.APIURL)
			fmt.Printf("  current: %v\n", name == currentName)
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "TYPE", "ORGANIZATION", "API URL", "CURRENT")

		for _, name := range accounts {
			account, err := am.Get(name)
			if err != nil {
				continue
			}
			
			current := ""
			if name == currentName {
				current = "*"
			}

			authType := account.AuthType
			if authType == "" {
				authType = "api_key"
			}

			orgName := ""
			if account.IsOAuth() {
				org, err := account.GetCurrentOrganization()
				if err == nil {
					orgName = org.Name
				}
			} else if account.Organization != nil {
				orgName = account.Organization.Name
			}

			apiURL := account.APIURL
			if apiURL == "https://api.dotenv.cloud" {
				apiURL = "default"
			}

			table.Append([]string{
				name,
				authType,
				orgName,
				apiURL,
				current,
			})
		}

		table.Render()
		return nil
	}
}

func listOrganizations(cmd *cobra.Command) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	ui.PrintInfo("Fetching organizations...")

	orgs, _, err := client.Organizations.List(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	if len(orgs) == 0 {
		ui.PrintWarning("No organizations found")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(orgs)

	case "yaml":
		// Simple YAML output
		for _, org := range orgs {
			fmt.Printf("- name: %s\n", org.Name)
			fmt.Printf("  slug: %s\n", org.Slug)
			fmt.Printf("  plan: %s\n", org.PlanName)
			fmt.Printf("  status: %s\n", org.Status)
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "SLUG", "PLAN", "STATUS")

		for _, org := range orgs {
			table.Append([]string{
				org.Name,
				org.Slug,
				org.PlanName,
				org.Status,
			})
		}

		table.Render()
		return nil
	}
}

func listProjects(cmd *cobra.Command, orgSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Get organization if not specified
	if orgSlug == "" {
		if listOrganization != "" {
			orgSlug = listOrganization
		} else {
			account, err := getCurrentAccount()
			if err != nil {
				return err
			}
			
			// Get the organization slug based on account type
			if account.IsOAuth() {
				org, err := account.GetCurrentOrganization()
				if err != nil {
					return fmt.Errorf("no organization selected for current account. Use 'dotenv org use' to select one")
				}
				orgSlug = org.Slug
			} else if account.Organization != nil {
				orgSlug = account.Organization.Slug
			} else {
				return fmt.Errorf("no organization configured for current account")
			}
		}
	}

	ui.PrintInfo("Fetching projects from %s...", orgSlug)

	projects, _, err := client.Projects.List(context.Background(), orgSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		ui.PrintWarning("No projects found")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(projects)

	case "yaml":
		// Simple YAML output
		for _, proj := range projects {
			fmt.Printf("- name: %s\n", proj.Name)
			fmt.Printf("  slug: %s\n", proj.Slug)
			fmt.Printf("  secrets: %d\n", proj.SecretCount)
			fmt.Printf("  targets: %d\n", proj.TargetCount)
			fmt.Printf("  environments: %d\n", proj.EnvironmentCount)
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "SLUG", "SECRETS", "TARGETS", "ENVIRONMENTS")

		for _, proj := range projects {
			table.Append([]string{
				proj.Name,
				proj.Slug,
				fmt.Sprintf("%d", proj.SecretCount),
				fmt.Sprintf("%d", proj.TargetCount),
				fmt.Sprintf("%d", proj.EnvironmentCount),
			})
		}

		table.Render()
		return nil
	}
}

func listTargets(cmd *cobra.Command, projectSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	ui.PrintInfo("Fetching targets from %s...", projectSlug)

	targets, _, err := client.Targets.List(context.Background(), projectSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	if len(targets) == 0 {
		ui.PrintWarning("No targets found")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(targets)

	case "yaml":
		// Simple YAML output
		for _, target := range targets {
			fmt.Printf("- name: %s\n", target.Name)
			fmt.Printf("  slug: %s\n", target.Slug)
			if target.Description != "" {
				fmt.Printf("  description: %s\n", target.Description)
			}
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "SLUG", "DESCRIPTION")

		for _, target := range targets {
			desc := target.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}

			table.Append([]string{
				target.Name,
				target.Slug,
				desc,
			})
		}

		table.Render()
		return nil
	}
}

func listEnvironments(cmd *cobra.Command, projectSlug, targetSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	ui.PrintInfo("Fetching environments from %s/%s...", projectSlug, targetSlug)

	envs, _, err := client.Environments.List(context.Background(), projectSlug, targetSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		ui.PrintWarning("No environments found")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(envs)

	case "yaml":
		// Simple YAML output
		for _, env := range envs {
			fmt.Printf("- name: %s\n", env.Name)
			fmt.Printf("  slug: %s\n", env.Slug)
			fmt.Printf("  status: %s\n", env.Status)
			if env.Description != "" {
				fmt.Printf("  description: %s\n", env.Description)
			}
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "SLUG", "STATUS", "DESCRIPTION")

		for _, env := range envs {
			desc := env.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}

			table.Append([]string{
				env.Name,
				env.Slug,
				env.Status,
				desc,
			})
		}

		table.Render()
		return nil
	}
}
