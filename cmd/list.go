package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

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
  contexts      - List configured contexts`,

	Example: `  # List all projects in current organization
  dotenv list projects

  # List targets in a specific project
  dotenv list targets myproject

  # List environments in a project/target
  dotenv list environments myproject/production

  # List all configured contexts
  dotenv list contexts

  # List projects in a specific organization
  dotenv list projects --organization=acme-corp`,

	ValidArgs: []string{"organizations", "projects", "targets", "environments", "contexts"},
	RunE:      runList,
}

func init() {
	listCmd.Flags().StringVar(&listOrganization, "organization", "",
		"specify organization (overrides current context)")
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

	switch resource {
	case "contexts":
		return listContexts(cmd)

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

func listContexts(cmd *cobra.Command) error {
	cm, err := config.NewContextManager("")
	if err != nil {
		return err
	}

	contexts := cm.List()

	if len(contexts) == 0 {
		ui.PrintWarning("No contexts configured. Run 'dotenv init' to get started.")
		return nil
	}

	switch listFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(contexts)

	case "yaml":
		// Simple YAML output
		for _, ctx := range contexts {
			fmt.Printf("- name: %s\n", ctx.Name)
			fmt.Printf("  organization: %s\n", ctx.Organization)
			fmt.Printf("  api_url: %s\n", ctx.APIURL)
			fmt.Printf("  current: %v\n", ctx.Current)
			if ctx.UserEmail != "" {
				fmt.Printf("  user_email: %s\n", ctx.UserEmail)
			}
			fmt.Println()
		}
		return nil

	default:
		// Table format
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("NAME", "ORGANIZATION", "API URL", "CURRENT")

		for _, ctx := range contexts {
			current := ""
			if ctx.Current {
				current = "*"
			}

			apiURL := ctx.APIURL
			if apiURL == "https://api.dotenv.com" {
				apiURL = "default"
			}

			table.Append([]string{
				ctx.Name,
				ctx.Organization,
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
			ctx, err := getCurrentContext()
			if err != nil {
				return err
			}
			orgSlug = ctx.Organization
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
