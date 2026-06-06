package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	pathExact  bool
	pathRegex  bool
	pathType   string
	pathOutput string
)

var pathCmd = &cobra.Command{
	Use:   "path <search-term>",
	Short: "Find resources by name and get their paths",
	Long: `Search for projects, targets, or environments by name and get their full paths.
    
This command provides a quick way to find resource paths for use with other commands.
Supports exact matching, fuzzy matching, and regular expression patterns.`,

	Example: `  # Find all resources containing "prod"
  dotenv path prod
  
  # Find exact match for "production"
  dotenv path production --exact
  
  # Find using regex pattern
  dotenv path "prod.*api" --regex
  
  # Find only environments
  dotenv path staging --type=environment
  
  # Get first match only
  dotenv path myapp --output=first`,

	Args: cobra.ExactArgs(1),
	RunE: runPath,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	pathCmd.Flags().BoolVar(&pathExact, "exact", false,
		"match exact names only")
	pathCmd.Flags().BoolVar(&pathRegex, "regex", false,
		"treat search term as regex pattern")
	pathCmd.Flags().StringVar(&pathType, "type", "all",
		"resource type to search (all, project, target, environment)")
	pathCmd.Flags().StringVar(&pathOutput, "output", "list",
		"output format (list, first, count)")
}

func runPath(cmd *cobra.Command, args []string) error {
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

	searchTerm := args[0]
	if pathExact && pathRegex {
		return fmt.Errorf("cannot use both --exact and --regex")
	}

	var searchRegex *regexp.Regexp
	if pathRegex {
		var err error
		searchRegex, err = regexp.Compile(searchTerm)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	matches, err := searchResources(cmd.Context(), client, searchTerm, searchRegex)
	if err != nil {
		return err
	}

	if pathType != resourceAll {
		filtered := []resourceMatch{}
		for _, match := range matches {
			if strings.EqualFold(match.Type, pathType) {
				filtered = append(filtered, match)
			}
		}
		matches = filtered
	}

	return printPathMatches(matches, searchTerm)
}

func printPathMatches(matches []resourceMatch, searchTerm string) error {
	switch pathOutput {
	case "first":
		if len(matches) == 0 {
			return fmt.Errorf("no matches found")
		}
		fmt.Println(matches[0].Path)
	case "count":
		fmt.Println(len(matches))
	default: // "list"
		if len(matches) == 0 {
			ui.PrintWarningf("No matches found for '%s'", searchTerm)
			return nil
		}
		for _, match := range matches {
			fmt.Printf("%s\t# %s: %s\n", match.Path, match.Type, match.Name)
		}
	}
	return nil
}

type resourceMatch struct {
	Type string
	Path string
	Name string
}

func searchResources(ctx context.Context, client *dotenv.Client, searchTerm string, searchRegex *regexp.Regexp) ([]resourceMatch, error) {
	projects, projResp, err := client.Projects.List(ctx, nil)
	if projResp != nil {
		defer projResp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	matches := []resourceMatch{}
	for _, project := range projects {
		if matchesSearch(project.Name, project.Slug, searchTerm, searchRegex) {
			matches = append(matches, resourceMatch{
				Type: "Project",
				Path: project.Slug,
				Name: project.Name,
			})
		}
		matches = append(matches, searchProjectTargets(ctx, client, project, searchTerm, searchRegex)...)
	}
	return matches, nil
}

func searchProjectTargets(
	ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	searchTerm string, searchRegex *regexp.Regexp,
) []resourceMatch {
	targets, tgtResp, err := client.Targets.List(ctx, project.Slug, nil)
	if tgtResp != nil {
		defer tgtResp.Body.Close()
	}
	if err != nil {
		ui.PrintDebugf("Failed to fetch targets for %s: %v", project.Slug, err)
		return nil
	}
	var matches []resourceMatch
	for _, target := range targets {
		if matchesSearch(target.Name, target.Slug, searchTerm, searchRegex) {
			matches = append(matches, resourceMatch{
				Type: "Target",
				Path: fmt.Sprintf("%s/%s", project.Slug, target.Slug),
				Name: target.Name,
			})
		}
		matches = append(matches, searchTargetEnvs(ctx, client, project.Slug, target.Slug, searchTerm, searchRegex)...)
	}
	return matches
}

func searchTargetEnvs(
	ctx context.Context, client *dotenv.Client,
	projectSlug, targetSlug, searchTerm string, searchRegex *regexp.Regexp,
) []resourceMatch {
	envs, envResp, err := client.Environments.List(ctx, projectSlug, targetSlug, nil)
	if envResp != nil {
		defer envResp.Body.Close()
	}
	if err != nil {
		ui.PrintDebugf("Failed to fetch environments for %s/%s: %v", projectSlug, targetSlug, err)
		return nil
	}
	var matches []resourceMatch
	for _, env := range envs {
		if matchesSearch(env.Name, env.Slug, searchTerm, searchRegex) {
			matches = append(matches, resourceMatch{
				Type: "Environment",
				Path: fmt.Sprintf("%s/%s/%s", projectSlug, targetSlug, env.Slug),
				Name: env.Name,
			})
		}
	}
	return matches
}

func matchesSearch(name, slug, searchTerm string, searchRegex *regexp.Regexp) bool {
	// Handle regex search
	if searchRegex != nil {
		return searchRegex.MatchString(name) || searchRegex.MatchString(slug)
	}

	// Handle exact match
	if pathExact {
		searchLower := strings.ToLower(searchTerm)
		return strings.EqualFold(name, searchLower) || strings.EqualFold(slug, searchLower)
	}

	// Handle fuzzy match (case-insensitive contains)
	searchLower := strings.ToLower(searchTerm)
	return strings.Contains(strings.ToLower(name), searchLower) ||
		strings.Contains(strings.ToLower(slug), searchLower)
}
