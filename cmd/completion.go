package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(dotenv completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ dotenv completion bash > /etc/bash_completion.d/dotenv
  # macOS:
  $ dotenv completion bash > /usr/local/etc/bash_completion.d/dotenv

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ dotenv completion zsh > "${fpath[1]}/_dotenv"

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ dotenv completion fish | source

  # To load completions for each session, execute once:
  $ dotenv completion fish > ~/.config/fish/completions/dotenv.fish

PowerShell:

  PS> dotenv completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> dotenv completion powershell > dotenv.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// registerResourcePathCompletions sets up dynamic completions for resource paths
func registerResourcePathCompletions() {
	// For pull command - completes project/target/environment paths
	pullCmd.RegisterFlagCompletionFunc("project", projectCompletion)
	pullCmd.ValidArgsFunction = resourcePathCompletion

	// For push command - completes project/target/environment paths
	pushCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// First arg is file, second is resource path
		if len(args) == 0 {
			// File completion
			return nil, cobra.ShellCompDirectiveDefault
		}
		// Resource path completion
		return resourcePathCompletion(cmd, args[1:], toComplete)
	}

	// For list commands that take resource arguments
	listCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			// First argument is the resource type
			return []string{
				"organizations\tList all organizations",
				"projects\tList projects in current organization",
				"targets\tList targets in a project",
				"environments\tList environments in a target",
				"accounts\tList configured accounts",
				"all\tList all resources in a flat view",
			}, cobra.ShellCompDirectiveNoFileComp
		}

		// For targets and environments, provide path completion
		if len(args) == 1 {
			switch args[0] {
			case "targets":
				return projectCompletion(cmd, args[1:], toComplete)
			case "environments":
				return targetPathCompletion(cmd, args[1:], toComplete)
			}
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// For tree command
	treeCmd.ValidArgsFunction = projectCompletion
	treeCmd.RegisterFlagCompletionFunc("project", projectCompletion)
	treeCmd.RegisterFlagCompletionFunc("target", targetCompletion)

	// For explore command
	exploreCmd.ValidArgsFunction = resourcePathCompletion

	// For path command - no completion needed as it's searching
}

// resourcePathCompletion provides completion for full resource paths
func resourcePathCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Don't complete if we already have a full path
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Parse what's been typed so far
	parts := strings.Split(toComplete, "/")

	switch len(parts) {
	case 1:
		// Complete projects
		return projectCompletion(cmd, args, toComplete)
	case 2:
		// Complete targets for the given project
		return targetCompletionForProject(cmd, parts[0], parts[1])
	case 3:
		// Complete environments for the given project/target
		return environmentCompletionForTarget(cmd, parts[0], parts[1], parts[2])
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// projectCompletion provides completion for project names
func projectCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get cached or fresh project list
	projects := getCachedProjects()
	if projects == nil {
		// Try to fetch fresh list
		client, err := getAPIClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Organization context is already set in the client

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		projectList, _, err := client.Projects.List(ctx, nil)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		projects = make([]string, 0, len(projectList))
		for _, p := range projectList {
			desc := fmt.Sprintf("%s (%d secrets)", p.Name, p.SecretCount)
			projects = append(projects, fmt.Sprintf("%s\t%s", p.Slug, desc))
		}

		// Cache for future use
		cacheProjects(projects)
	}

	// Filter by prefix
	if toComplete != "" {
		filtered := make([]string, 0)
		for _, p := range projects {
			slug := strings.Split(p, "\t")[0]
			if strings.HasPrefix(slug, toComplete) {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	return projects, cobra.ShellCompDirectiveNoFileComp
}

// targetCompletion provides completion for target names (requires --project flag)
func targetCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	projectFlag, _ := cmd.Flags().GetString("project")
	if projectFlag == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return targetCompletionForProject(cmd, projectFlag, toComplete)
}

// targetCompletionForProject provides completion for targets in a specific project
func targetCompletionForProject(cmd *cobra.Command, project, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := getAPIClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	targets, _, err := client.Targets.List(ctx, project, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]string, 0, len(targets))
	for _, t := range targets {
		if toComplete == "" || strings.HasPrefix(t.Slug, toComplete) {
			desc := t.Name
			if t.Description != "" {
				desc = fmt.Sprintf("%s - %s", t.Name, t.Description)
			}
			completions = append(completions, fmt.Sprintf("%s\t%s", t.Slug, desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// targetPathCompletion provides completion for project/target paths
func targetPathCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Don't complete if we already have an argument
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Check if we're completing a partial path
	if strings.Contains(toComplete, "/") {
		parts := strings.Split(toComplete, "/")
		if len(parts) == 2 {
			return targetCompletionForProject(cmd, parts[0], parts[1])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Otherwise, complete projects with hint that target is needed
	projects := getCachedProjects()
	if projects == nil {
		return projectCompletion(cmd, args, toComplete)
	}

	// Modify to show that a target is needed
	completions := make([]string, 0, len(projects))
	for _, p := range projects {
		parts := strings.Split(p, "\t")
		if len(parts) == 2 && (toComplete == "" || strings.HasPrefix(parts[0], toComplete)) {
			completions = append(completions, fmt.Sprintf("%s/\tProject: %s (add target)", parts[0], parts[1]))
		}
	}

	return completions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// environmentCompletionForTarget provides completion for environments in a project/target
func environmentCompletionForTarget(cmd *cobra.Command, project, target, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := getAPIClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	envs, _, err := client.Environments.List(ctx, project, target, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]string, 0, len(envs))
	for _, e := range envs {
		if toComplete == "" || strings.HasPrefix(e.Slug, toComplete) {
			desc := fmt.Sprintf("%s [%s]", e.Name, e.Status)
			if e.Description != "" {
				desc = fmt.Sprintf("%s - %s", desc, e.Description)
			}
			completions = append(completions, fmt.Sprintf("%s\t%s", e.Slug, desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// Simple in-memory cache for completion data
var (
	projectCacheMu   sync.RWMutex
	projectCache     []string
	projectCacheTime time.Time
	cacheDuration    = 5 * time.Minute
)

func getCachedProjects() []string {
	projectCacheMu.RLock()
	defer projectCacheMu.RUnlock()
	if time.Since(projectCacheTime) > cacheDuration {
		return nil
	}
	return projectCache
}

func cacheProjects(projects []string) {
	projectCacheMu.Lock()
	defer projectCacheMu.Unlock()
	projectCache = projects
	projectCacheTime = time.Now()
}
