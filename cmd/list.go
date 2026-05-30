package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
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
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		// Try to refresh organizations if needed
		if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
			// Don't fail the command, just warn
			ui.PrintWarningf("Could not refresh organizations: %v", err)
		}
		return nil
	},
	RunE: runList,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
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
		listFormat = formatJSON
	}

	resource := args[0]

	// Display account/org info for resources that use API
	// (not for "accounts" since that's local config)
	if resource != resourceAccounts && viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			// Don't fail if we can't display account info
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

	return dispatchListResource(cmd, resource, args)
}

func dispatchListResource(cmd *cobra.Command, resource string, args []string) error {
	switch resource {
	case resourceAccounts:
		return listAccounts(cmd)
	case "organizations":
		return listOrganizations(cmd)
	case "projects":
		return listProjects(cmd, "")
	case resourceTargets:
		if len(args) < 2 {
			return fmt.Errorf("project name required: use 'dotenv list targets <project>'")
		}
		return listTargets(cmd, args[1])
	case resourceEnvironments:
		return listEnvironmentsFromArgs(cmd, args)
	case resourceAll:
		return listAll(cmd)
	default:
		return fmt.Errorf("unknown resource '%s': valid resources are accounts, organizations, projects, targets, environments, all", resource)
	}
}

func listEnvironmentsFromArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("project and target required: use 'dotenv list environments <project>/<target>'")
	}
	parts := strings.Split(args[1], "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format '%s': expected 'project/target'", args[1])
	}
	return listEnvironments(cmd, parts[0], parts[1])
}

func listAccounts(_ *cobra.Command) error {
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
		ui.PrintWarningf("No accounts configured. Run 'dotenv init' to get started.")
		return nil
	}

	current, _ := am.GetCurrent()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	switch listFormat {
	case formatJSON:
		return renderAccountsJSON(am, accounts, currentName)
	case formatYAML:
		return renderAccountsYAML(am, accounts, currentName)
	default:
		return renderAccountsTable(am, accounts, currentName)
	}
}

type accountInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Organization string `json:"organization"`
	APIURL       string `json:"api_url"`
	Current      bool   `json:"current"`
	UserEmail    string `json:"user_email,omitempty"`
}

func accountOrgName(account *config.Account) string {
	if account.IsOAuth() {
		if org, err := account.GetCurrentOrganization(); err == nil {
			return org.Name
		}
		return ""
	}
	if account.Organization != nil {
		return account.Organization.Name
	}
	return ""
}

func renderAccountsJSON(am *config.AccountManager, accounts []string, currentName string) error {
	accountList := []accountInfo{}
	for _, name := range accounts {
		account, err := am.Get(name)
		if err != nil {
			continue
		}
		accountList = append(accountList, accountInfo{
			Name:         name,
			Type:         account.AuthType,
			APIURL:       account.APIURL,
			Current:      name == currentName,
			Organization: accountOrgName(account),
		})
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(accountList)
}

func renderAccountsYAML(am *config.AccountManager, accounts []string, currentName string) error {
	for _, name := range accounts {
		account, err := am.Get(name)
		if err != nil {
			continue
		}
		fmt.Printf("- name: %s\n", name)
		fmt.Printf("  type: %s\n", account.AuthType)
		if org := accountOrgName(account); org != "" {
			fmt.Printf("  organization: %s\n", org)
		}
		fmt.Printf("  api_url: %s\n", account.APIURL)
		fmt.Printf("  current: %v\n", name == currentName)
		fmt.Println()
	}
	return nil
}

func renderAccountsTable(am *config.AccountManager, accounts []string, currentName string) error {
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

		apiURL := account.APIURL
		if apiURL == constants.LegacyAPIURL {
			apiURL = "default"
		}

		if err := table.Append([]string{
			name,
			authType,
			accountOrgName(account),
			apiURL,
			current,
		}); err != nil {
			return fmt.Errorf("append account row: %w", err)
		}
	}

	if err := table.Render(); err != nil {
		return fmt.Errorf("render account table: %w", err)
	}
	return nil
}

func listOrganizations(cmd *cobra.Command) error {
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
		// Check if using API key authentication
		if resp != nil && resp.StatusCode == 403 {
			return fmt.Errorf("API key authentication only shows the organization tied to the key. Use OAuth for listing all organizations")
		}
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(orgs) == 0 {
		ui.PrintWarningf("No organizations found")
		return nil
	}

	return renderOrgList(orgs, listFormat)
}

func listProjects(cmd *cobra.Command, _ string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Note: organization context is already set in the client via getAPIClient()

	ui.PrintInfof("Fetching projects...")

	projects, projResp, err := client.Projects.List(cmd.Context(), nil)
	if projResp != nil {
		defer projResp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(projects) == 0 {
		ui.PrintWarningf("No projects found")
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
	case formatJSON:
		return renderProjectsJSON(projects)
	case formatYAML:
		renderProjectsYAML(projects)
		return nil
	default:
		return renderProjectsTable(projects)
	}
}

func renderProjectsJSON(projects []*dotenv.Project) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if !listWithPaths {
		return encoder.Encode(projects)
	}
	type projectWithPath struct {
		*dotenv.Project
		Path string `json:"path"`
	}
	enhanced := make([]projectWithPath, len(projects))
	for i, p := range projects {
		enhanced[i] = projectWithPath{Project: p, Path: p.Slug}
	}
	return encoder.Encode(enhanced)
}

func renderProjectsYAML(projects []*dotenv.Project) {
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
}

func renderProjectsTable(projects []*dotenv.Project) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "SLUG", "SECRETS", "TARGETS", "ENVIRONMENTS")
	for _, proj := range projects {
		if err := table.Append([]string{
			proj.Name,
			proj.Slug,
			fmt.Sprintf("%d", proj.SecretCount),
			fmt.Sprintf("%d", proj.TargetCount),
			fmt.Sprintf("%d", proj.EnvironmentCount),
		}); err != nil {
			return fmt.Errorf("append project row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("render project table: %w", err)
	}
	return nil
}

func listTargets(cmd *cobra.Command, projectSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	ui.PrintInfof("Fetching targets from %s...", projectSlug)

	targets, tgtResp, err := client.Targets.List(cmd.Context(), projectSlug, nil)
	if tgtResp != nil {
		defer tgtResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	if len(targets) == 0 {
		ui.PrintWarningf("No targets found")
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
	case formatJSON:
		return renderTargetsJSON(targets, projectSlug)
	case formatYAML:
		renderTargetsYAML(targets, projectSlug)
		return nil
	default:
		return renderTargetsTable(targets)
	}
}

func renderTargetsJSON(targets []*dotenv.Target, projectSlug string) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if listWithPaths {
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
}

func renderTargetsYAML(targets []*dotenv.Target, projectSlug string) {
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
}

func renderTargetsTable(targets []*dotenv.Target) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "SLUG", "DESCRIPTION")
	for _, target := range targets {
		desc := target.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if err := table.Append([]string{
			target.Name,
			target.Slug,
			desc,
		}); err != nil {
			return fmt.Errorf("append target row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("render target table: %w", err)
	}
	return nil
}

func listEnvironments(cmd *cobra.Command, projectSlug, targetSlug string) error {
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	ui.PrintInfof("Fetching environments from %s/%s...", projectSlug, targetSlug)

	envs, envResp, err := client.Environments.List(cmd.Context(), projectSlug, targetSlug, nil)
	if envResp != nil {
		defer envResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		ui.PrintWarningf("No environments found")
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
	case formatJSON:
		return renderEnvsJSON(envs, projectSlug, targetSlug)
	case formatYAML:
		renderEnvsYAML(envs, projectSlug, targetSlug)
		return nil
	default:
		return renderEnvsTable(envs)
	}
}

func renderEnvsJSON(envs []*dotenv.Environment, projectSlug, targetSlug string) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if listWithPaths {
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
}

func renderEnvsYAML(envs []*dotenv.Environment, projectSlug, targetSlug string) {
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
}

func renderEnvsTable(envs []*dotenv.Environment) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "SLUG", "STATUS", "DESCRIPTION")
	for _, env := range envs {
		desc := env.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		if err := table.Append([]string{
			env.Name,
			env.Slug,
			env.Status,
			desc,
		}); err != nil {
			return fmt.Errorf("append env row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("render env table: %w", err)
	}
	return nil
}

type resourceInfo struct {
	Type        string
	Path        string
	Name        string
	Description string
	Status      string
	Count       int
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

	orgIdentifier, err := account.GetOrganizationIdentifier()
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	ui.PrintInfof("Fetching all resources from %s...", orgIdentifier)

	projects, projResp, err := client.Projects.List(cmd.Context(), nil)
	if projResp != nil {
		defer projResp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	if listPaths {
		printAllPaths(cmd, client, projects)
		return nil
	}

	resources := collectAllResources(cmd, client, projects)
	if len(resources) == 0 {
		ui.PrintWarningf("No resources found")
		return nil
	}

	switch listFormat {
	case formatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(resources)
	case formatYAML:
		renderAllYAML(resources)
		return nil
	default:
		return renderAllTable(resources)
	}
}

func printAllPaths(cmd *cobra.Command, client *dotenv.Client, projects []*dotenv.Project) {
	for _, project := range projects {
		fmt.Println(project.Slug)
		if err := printProjectPaths(cmd, client, project); err != nil {
			ui.PrintWarningf("%v", err)
		}
	}
}

func printProjectPaths(cmd *cobra.Command, client *dotenv.Client, project *dotenv.Project) error {
	targets, tgtResp, err := client.Targets.List(cmd.Context(), project.Slug, nil)
	if tgtResp != nil {
		defer tgtResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to fetch targets for %s: %w", project.Slug, err)
	}
	for _, target := range targets {
		fmt.Printf("%s/%s\n", project.Slug, target.Slug)
		printTargetEnvPaths(cmd, client, project.Slug, target.Slug)
	}
	return nil
}

func printTargetEnvPaths(cmd *cobra.Command, client *dotenv.Client, projectSlug, targetSlug string) {
	envs, envResp, envErr := client.Environments.List(cmd.Context(), projectSlug, targetSlug, nil)
	if envResp != nil {
		defer envResp.Body.Close()
	}
	if envErr != nil {
		ui.PrintWarningf("Failed to fetch environments for %s/%s: %v", projectSlug, targetSlug, envErr)
		return
	}
	for _, env := range envs {
		fmt.Printf("%s/%s/%s\n", projectSlug, targetSlug, env.Slug)
	}
}

func collectAllResources(cmd *cobra.Command, client *dotenv.Client, projects []*dotenv.Project) []resourceInfo {
	var resources []resourceInfo
	for _, project := range projects {
		resources = append(resources, resourceInfo{
			Type:  "Project",
			Path:  project.Slug,
			Name:  project.Name,
			Count: project.SecretCount,
		})
		resources = append(resources, collectProjectTargets(cmd, client, project)...)
	}
	return resources
}

func collectProjectTargets(cmd *cobra.Command, client *dotenv.Client, project *dotenv.Project) []resourceInfo {
	targets, tgtResp, err := client.Targets.List(cmd.Context(), project.Slug, nil)
	if tgtResp != nil {
		defer tgtResp.Body.Close()
	}
	if err != nil {
		ui.PrintWarningf("Failed to fetch targets for %s: %v", project.Slug, err)
		return nil
	}
	var resources []resourceInfo
	for _, target := range targets {
		resources = append(resources, resourceInfo{
			Type:        "Target",
			Path:        fmt.Sprintf("%s/%s", project.Slug, target.Slug),
			Name:        target.Name,
			Description: target.Description,
		})
		resources = append(resources, collectTargetEnvs(cmd, client, project.Slug, target.Slug)...)
	}
	return resources
}

func collectTargetEnvs(cmd *cobra.Command, client *dotenv.Client, projectSlug, targetSlug string) []resourceInfo {
	envs, envResp, envErr := client.Environments.List(cmd.Context(), projectSlug, targetSlug, nil)
	if envResp != nil {
		defer envResp.Body.Close()
	}
	if envErr != nil {
		ui.PrintWarningf("Failed to fetch environments for %s/%s: %v", projectSlug, targetSlug, envErr)
		return nil
	}
	var resources []resourceInfo
	for _, env := range envs {
		resources = append(resources, resourceInfo{
			Type:        "Environment",
			Path:        fmt.Sprintf("%s/%s/%s", projectSlug, targetSlug, env.Slug),
			Name:        env.Name,
			Description: env.Description,
			Status:      env.Status,
		})
	}
	return resources
}

func renderAllYAML(resources []resourceInfo) {
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
}

func renderAllTable(resources []resourceInfo) error {
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
		if err := table.Append([]string{
			r.Type,
			r.Path,
			r.Name,
			status,
			details,
		}); err != nil {
			return fmt.Errorf("append resource row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("render resource table: %w", err)
	}
	return nil
}
