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
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var (
	listOrganization string
	listFormat       string
	listJSON         bool
	listPaths        bool
	listWithPaths    bool
)

var listCmd = &cobra.Command{
	Use:   "list [resource]",
	Short: "List resources",
	Long: `List DotEnv resources in a hierarchical structure.

Note: When listing organizations:
- OAuth authentication: Shows all organizations you belong to
- API key authentication: Shows only the organization tied to the API key

Resources:
  organizations  - List organizations (behavior depends on auth type)
  projects      - List projects in current organization
  targets       - List targets in a project
  environments  - List environments in a target
  accounts      - List configured accounts
  all           - List all resources in a flat view`,

	Example: `  # List all projects in current organization
  dotenv list projects

  # List targets in a specific project
  dotenv list targets myproject

  # List environments in a project/target
  dotenv list environments myproject/production

  # List all configured accounts
  dotenv list accounts

  # List projects in a specific organization
  dotenv list projects --organization=acme-corp
  
  # List all resources in a flat table
  dotenv list all
  
  # List all resources as paths only
  dotenv list all --paths`,

	ValidArgs: []string{"organizations", "projects", "targets", "environments", "accounts", "all"},
	RunE:      runList,
}

func init() {
	listCmd.Flags().StringVar(&listOrganization, "organization", "",
		"specify organization (overrides current account's organization)")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table",
		"output format (table, json, yaml)")
	listCmd.Flags().BoolVar(&listJSON, "json", false,
		"output as JSON (deprecated, use --format=json)")
	listCmd.Flags().BoolVar(&listPaths, "paths", false,
		"output full paths for easy copy/paste")
	listCmd.Flags().BoolVar(&listWithPaths, "with-paths", false,
		"include paths in JSON/YAML output")
}

func runList(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a resource to list: accounts, organizations, projects, targets, environments, or all")
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
			return fmt.Errorf("project name required: use 'dotenv list targets <project>'")
		}
		return listTargets(cmd, args[1])

	case "environments":
		if len(args) < 2 {
			return fmt.Errorf("project and target required: use 'dotenv list environments <project>/<target>'")
		}
		parts := strings.Split(args[1], "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid format '%s': expected 'project/target'", args[1])
		}
		return listEnvironments(cmd, parts[0], parts[1])

	case "all":
		return listAll(cmd)

	default:
		return fmt.Errorf("unknown resource '%s': valid resources are accounts, organizations, projects, targets, environments, all, text", resource)
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
			Name         string `json:"name"`
			Type         string `json:"type"`
			Organization string `json:"organization"`
			APIURL       string `json:"api_url"`
			Current      bool   `json:"current"`
			UserEmail    string `json:"user_email,omitempty"`
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
				authType = constants.AuthTypeAPIKey
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
			if apiURL == constants.LegacyAPIURL {
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
	// Use client without org context since we're listing organizations
	client, err := getAPIClientWithoutOrgContext()
	if err != nil {
		return err
	}

	ui.PrintInfo("Fetching organizations...")

	orgs, resp, err := client.Organizations.List(context.Background(), nil)
	if err != nil {
		// Check if using API key authentication
		if resp != nil && resp.StatusCode == 403 {
			return fmt.Errorf("API key authentication only shows the organization tied to the key. Use OAuth for listing all organizations")
		}
		account, _ := getCurrentAccount()
		return HandleAPIError(err, account)
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
		// Get current organization for comparison
		account, _ := getCurrentAccount()
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
		account, _ := getCurrentAccount()
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

func listProjects(cmd *cobra.Command, orgSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Note: organization context is already set in the client via getAPIClient()

	ui.PrintInfo("Fetching projects...")

	projects, _, err := client.Projects.List(context.Background(), nil)
	if err != nil {
		account, _ := getCurrentAccount()
		return HandleAPIError(err, account)
	}

	if len(projects) == 0 {
		ui.PrintWarning("No projects found")
		return nil
	}

	// Handle --paths flag
	if listPaths {
		for _, proj := range projects {
			fmt.Println(proj.Slug)
		}
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if listWithPaths {
			// Create enhanced output with paths
			type projectWithPath struct {
				*dotenv.Project
				Path string `json:"path"`
			}
			enhanced := make([]projectWithPath, len(projects))
			for i, p := range projects {
				enhanced[i] = projectWithPath{
					Project: p,
					Path:    p.Slug,
				}
			}
			return encoder.Encode(enhanced)
		}
		return encoder.Encode(projects)

	case "yaml":
		// Simple YAML output
		for _, proj := range projects {
			fmt.Printf("- name: %s\n", proj.Name)
			fmt.Printf("  slug: %s\n", proj.Slug)
			if listWithPaths {
				fmt.Printf("  path: %s\n", proj.Slug)
			}
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

	// Handle --paths flag
	if listPaths {
		for _, target := range targets {
			fmt.Printf("%s/%s\n", projectSlug, target.Slug)
		}
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if listWithPaths {
			// Create enhanced output with paths
			type targetWithPath struct {
				*dotenv.Target
				Path string `json:"path"`
			}
			enhanced := make([]targetWithPath, len(targets))
			for i, t := range targets {
				enhanced[i] = targetWithPath{
					Target: t,
					Path:   fmt.Sprintf("%s/%s", projectSlug, t.Slug),
				}
			}
			return encoder.Encode(enhanced)
		}
		return encoder.Encode(targets)

	case "yaml":
		// Simple YAML output
		for _, target := range targets {
			fmt.Printf("- name: %s\n", target.Name)
			fmt.Printf("  slug: %s\n", target.Slug)
			if listWithPaths {
				fmt.Printf("  path: %s/%s\n", projectSlug, target.Slug)
			}
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

	// Handle --paths flag
	if listPaths {
		for _, env := range envs {
			fmt.Printf("%s/%s/%s\n", projectSlug, targetSlug, env.Slug)
		}
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if listWithPaths {
			// Create enhanced output with paths
			type envWithPath struct {
				*dotenv.Environment
				Path string `json:"path"`
			}
			enhanced := make([]envWithPath, len(envs))
			for i, e := range envs {
				enhanced[i] = envWithPath{
					Environment: e,
					Path:        fmt.Sprintf("%s/%s/%s", projectSlug, targetSlug, e.Slug),
				}
			}
			return encoder.Encode(enhanced)
		}
		return encoder.Encode(envs)

	case "yaml":
		// Simple YAML output
		for _, env := range envs {
			fmt.Printf("- name: %s\n", env.Name)
			fmt.Printf("  slug: %s\n", env.Slug)
			if listWithPaths {
				fmt.Printf("  path: %s/%s/%s\n", projectSlug, targetSlug, env.Slug)
			}
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

func listAll(cmd *cobra.Command) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	account, err := getCurrentAccount()
	if err != nil {
		return err
	}

	// Get organization
	orgIdentifier, err := account.GetOrganizationIdentifier()
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	ui.PrintInfo("Fetching all resources from %s...", orgIdentifier)

	// Fetch all projects
	projects, _, err := client.Projects.List(context.Background(), nil)
	if err != nil {
		account, _ := getCurrentAccount()
		return HandleAPIError(err, account)
	}

	// Handle --paths flag
	if listPaths {
		// Just output paths
		for _, project := range projects {
			fmt.Println(project.Slug)

			// Fetch targets for this project
			targets, _, err := client.Targets.List(context.Background(), project.Slug, nil)
			if err != nil {
				ui.PrintWarning("Failed to fetch targets for %s: %v", project.Slug, err)
				continue
			}

			for _, target := range targets {
				fmt.Printf("%s/%s\n", project.Slug, target.Slug)

				// Fetch environments for this target
				envs, _, err := client.Environments.List(context.Background(), project.Slug, target.Slug, nil)
				if err != nil {
					ui.PrintWarning("Failed to fetch environments for %s/%s: %v", project.Slug, target.Slug, err)
					continue
				}

				for _, env := range envs {
					fmt.Printf("%s/%s/%s\n", project.Slug, target.Slug, env.Slug)
				}
			}
		}
		return nil
	}

	// Collect all resources
	type resourceInfo struct {
		Type        string
		Path        string
		Name        string
		Description string
		Status      string
		Count       int
	}

	var resources []resourceInfo

	// Add projects
	for _, project := range projects {
		resources = append(resources, resourceInfo{
			Type:  "Project",
			Path:  project.Slug,
			Name:  project.Name,
			Count: project.SecretCount,
		})

		// Fetch targets for this project
		targets, _, err := client.Targets.List(context.Background(), project.Slug, nil)
		if err != nil {
			ui.PrintWarning("Failed to fetch targets for %s: %v", project.Slug, err)
			continue
		}

		for _, target := range targets {
			resources = append(resources, resourceInfo{
				Type:        "Target",
				Path:        fmt.Sprintf("%s/%s", project.Slug, target.Slug),
				Name:        target.Name,
				Description: target.Description,
			})

			// Fetch environments for this target
			envs, _, err := client.Environments.List(context.Background(), project.Slug, target.Slug, nil)
			if err != nil {
				ui.PrintWarning("Failed to fetch environments for %s/%s: %v", project.Slug, target.Slug, err)
				continue
			}

			for _, env := range envs {
				resources = append(resources, resourceInfo{
					Type:        "Environment",
					Path:        fmt.Sprintf("%s/%s/%s", project.Slug, target.Slug, env.Slug),
					Name:        env.Name,
					Description: env.Description,
					Status:      env.Status,
				})
			}
		}
	}

	if len(resources) == 0 {
		ui.PrintWarning("No resources found")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(resources)

	case "yaml":
		// Simple YAML output
		for _, r := range resources {
			fmt.Printf("- type: %s\n", r.Type)
			fmt.Printf("  path: %s\n", r.Path)
			fmt.Printf("  name: %s\n", r.Name)
			if r.Description != "" {
				fmt.Printf("  description: %s\n", r.Description)
			}
			if r.Status != "" {
				fmt.Printf("  status: %s\n", r.Status)
			}
			if r.Count > 0 {
				fmt.Printf("  secrets: %d\n", r.Count)
			}
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("TYPE", "PATH", "NAME", "STATUS", "DETAILS")

		for _, r := range resources {
			details := r.Description
			if r.Count > 0 {
				details = fmt.Sprintf("%d secrets", r.Count)
			}
			if len(details) > 40 {
				details = details[:37] + "..."
			}

			status := r.Status
			if status == "" && r.Type != "Environment" {
				status = "-"
			}

			table.Append([]string{
				r.Type,
				r.Path,
				r.Name,
				status,
				details,
			})
		}

		table.Render()
		return nil
	}
}
