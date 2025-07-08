package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
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
	// Display account/org info
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarning("Could not display account info: %v", err)
		}
	}

	searchTerm := args[0]
	
	// Validate flags
	if pathExact && pathRegex {
		return fmt.Errorf("cannot use both --exact and --regex")
	}
	
	// Compile regex if needed
	var searchRegex *regexp.Regexp
	if pathRegex {
		var err error
		searchRegex, err = regexp.Compile(searchTerm)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}
	
	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}
	
	// Search for resources
	matches, err := searchResources(cmd.Context(), client, searchTerm, searchRegex)
	if err != nil {
		return err
	}
	
	// Filter by type if requested
	if pathType != "all" {
		filtered := []resourceMatch{}
		for _, match := range matches {
			if strings.ToLower(match.Type) == strings.ToLower(pathType) {
				filtered = append(filtered, match)
			}
		}
		matches = filtered
	}
	
	// Handle output format
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
			ui.PrintWarning("No matches found for '%s'", searchTerm)
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
	matches := []resourceMatch{}
	
	// Fetch all projects
	projects, _, err := client.Projects.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	
	// Search projects and their children
	for _, project := range projects {
		// Check if project matches
		if matchesSearch(project.Name, project.Slug, searchTerm, searchRegex) {
			matches = append(matches, resourceMatch{
				Type: "Project",
				Path: project.Slug,
				Name: project.Name,
			})
		}
		
		// Fetch targets
		targets, _, err := client.Targets.List(ctx, project.Slug, nil)
		if err != nil {
			ui.PrintDebug("Failed to fetch targets for %s: %v", project.Slug, err)
			continue
		}
		
		for _, target := range targets {
			// Check if target matches
			if matchesSearch(target.Name, target.Slug, searchTerm, searchRegex) {
				matches = append(matches, resourceMatch{
					Type: "Target",
					Path: fmt.Sprintf("%s/%s", project.Slug, target.Slug),
					Name: target.Name,
				})
			}
			
			// Fetch environments
			envs, _, err := client.Environments.List(ctx, project.Slug, target.Slug, nil)
			if err != nil {
				ui.PrintDebug("Failed to fetch environments for %s/%s: %v", project.Slug, target.Slug, err)
				continue
			}
			
			for _, env := range envs {
				// Check if environment matches
				if matchesSearch(env.Name, env.Slug, searchTerm, searchRegex) {
					matches = append(matches, resourceMatch{
						Type: "Environment",
						Path: fmt.Sprintf("%s/%s/%s", project.Slug, target.Slug, env.Slug),
						Name: env.Name,
					})
				}
			}
		}
	}
	
	return matches, nil
}

func matchesSearch(name, slug, searchTerm string, searchRegex *regexp.Regexp) bool {
	// Handle regex search
	if searchRegex != nil {
		return searchRegex.MatchString(name) || searchRegex.MatchString(slug)
	}
	
	// Handle exact match
	if pathExact {
		searchLower := strings.ToLower(searchTerm)
		return strings.ToLower(name) == searchLower || strings.ToLower(slug) == searchLower
	}
	
	// Handle fuzzy match (case-insensitive contains)
	searchLower := strings.ToLower(searchTerm)
	return strings.Contains(strings.ToLower(name), searchLower) ||
		strings.Contains(strings.ToLower(slug), searchLower)
}