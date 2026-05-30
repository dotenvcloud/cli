package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/dotenv/cli/internal/hierarchy"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenvcloud/sdk-go"
)

var (
	treeProject    string
	treeTarget     string
	treeFull       bool
	treeMaxDepth   int
	treeShowCounts bool
	treeFormat     string
)

var treeCmd = &cobra.Command{
	Use:   "tree [project]",
	Short: "Display resource hierarchy as a tree",
	Long: `Display the project/target/environment hierarchy in a tree format.
    
This command provides a visual representation of your DotEnv resources,
making it easy to understand the structure and navigate the hierarchy.`,

	Example: `  # Show tree for all projects in current organization
  dotenv tree
  
  # Show tree for a specific project
  dotenv tree myproject
  
  # Show full tree including environments
  dotenv tree --full
  
  # Show tree with resource counts
  dotenv tree --show-counts
  
  # Limit tree depth
  dotenv tree --max-depth=2`,

	RunE: runTree,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	treeCmd.Flags().StringVarP(&treeProject, "project", "p", "",
		"show tree for specific project")
	treeCmd.Flags().StringVarP(&treeTarget, "target", "t", "",
		"show tree for specific target (requires --project)")
	treeCmd.Flags().BoolVar(&treeFull, "full", false,
		"show full tree including environments")
	treeCmd.Flags().IntVar(&treeMaxDepth, "max-depth", 0,
		"maximum depth to display (0 = unlimited)")
	treeCmd.Flags().BoolVar(&treeShowCounts, "show-counts", false,
		"show resource counts at each level")
	treeCmd.Flags().StringVar(&treeFormat, "format", "tree",
		"output format (tree, json, yaml)")
}

func runTree(cmd *cobra.Command, args []string) error {
	// Display account/org info
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Handle project argument
	projectSlug := treeProject
	if len(args) > 0 {
		projectSlug = args[0]
	}

	// Validate target flag requires project
	if treeTarget != "" && projectSlug == "" {
		return fmt.Errorf("--target requires --project or a project argument")
	}

	// Build hierarchy
	builder := hierarchy.NewBuilder(client)

	var root *hierarchy.Node
	ctx := cmd.Context()

	if projectSlug != "" {
		// Build tree for specific project or target
		if treeTarget != "" {
			root, err = builder.BuildTarget(ctx, projectSlug, treeTarget)
		} else {
			root, err = builder.BuildProject(ctx, projectSlug)
		}
	} else {
		// Build tree for entire organization
		account, acctErr := getCurrentAccount()
		if acctErr != nil {
			return acctErr
		}

		orgIdentifier, orgErr := account.GetOrganizationIdentifier()
		if orgErr != nil {
			return fmt.Errorf("failed to get organization: %w", orgErr)
		}

		root, err = builder.Build(ctx, orgIdentifier)
	}

	if err != nil {
		return fmt.Errorf("failed to build hierarchy: %w", err)
	}

	// Sort the tree for consistent output
	root.SortChildren()

	// Render based on format
	switch treeFormat {
	case "json":
		return renderTreeJSON(root, os.Stdout)
	case "yaml":
		return renderTreeYAML(root, os.Stdout)
	default:
		return renderTree(root, os.Stdout, 0, treeMaxDepth, treeFull, treeShowCounts)
	}
}

func renderTree(node *hierarchy.Node, w io.Writer, depth, maxDepth int, showFull, showCounts bool) error {
	if node == nil {
		return nil
	}
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}
	if node.Type == hierarchy.NodeTypeEnvironment && !showFull {
		return nil
	}

	if node.Type != hierarchy.NodeTypeOrganization {
		fmt.Fprintln(w, formatTreeNode(node, depth, showFull, showCounts))
	}

	childDepth := depth
	if node.Type != hierarchy.NodeTypeOrganization {
		childDepth = depth + 1
	}
	for _, child := range node.Children {
		if child.Type == hierarchy.NodeTypeEnvironment && !showFull {
			continue
		}
		if err := renderTree(child, w, childDepth, maxDepth, showFull, showCounts); err != nil {
			return err
		}
	}
	return nil
}

func formatTreeNode(node *hierarchy.Node, depth int, showFull, showCounts bool) string {
	display := node.Name
	if node.Slug != node.Name {
		display = fmt.Sprintf("%s (%s)", node.Name, node.Slug)
	}
	if showCounts {
		if count := childCountFor(node, showFull); count > 0 {
			display += fmt.Sprintf(" [%d]", count)
		}
	}
	if node.Type == hierarchy.NodeTypeEnvironment {
		if env, ok := node.Metadata.(*dotenv.Environment); ok && env.Status != "" {
			display += fmt.Sprintf(" (%s)", env.Status)
		}
	}
	return treePrefix(depth) + typeIndicator(node.Type) + display
}

func treePrefix(depth int) string {
	if depth == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < depth-1; i++ {
		b.WriteString("│   ")
	}
	b.WriteString("├── ")
	return b.String()
}

func typeIndicator(t hierarchy.NodeType) string {
	switch t {
	case hierarchy.NodeTypeProject:
		return "📁 "
	case hierarchy.NodeTypeTarget:
		return "🎯 "
	case hierarchy.NodeTypeEnvironment:
		return "🌿 "
	case hierarchy.NodeTypeOrganization:
		return ""
	}
	return ""
}

func childCountFor(node *hierarchy.Node, showFull bool) int {
	switch node.Type {
	case hierarchy.NodeTypeProject:
		proj, ok := node.Metadata.(*dotenv.Project)
		if !ok {
			return 0
		}
		if showFull {
			return proj.TargetCount + proj.EnvironmentCount
		}
		return proj.TargetCount
	case hierarchy.NodeTypeTarget:
		return len(node.Children)
	case hierarchy.NodeTypeOrganization, hierarchy.NodeTypeEnvironment:
		return 0
	}
	return 0
}

func renderTreeJSON(node *hierarchy.Node, w io.Writer) error {
	// Convert hierarchy to JSON structure
	data := nodeToMap(node)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func renderTreeYAML(node *hierarchy.Node, w io.Writer) error {
	// Convert hierarchy to YAML structure
	data := nodeToMap(node)

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// nodeToMap converts a hierarchy node to a map for JSON/YAML output
func nodeToMap(node *hierarchy.Node) map[string]interface{} {
	result := map[string]interface{}{
		"type": string(node.Type),
		"name": node.Name,
		"slug": node.Slug,
		"path": node.Path,
	}

	// Add metadata based on type
	switch node.Type {
	case hierarchy.NodeTypeProject:
		if proj, ok := node.Metadata.(*dotenv.Project); ok {
			result["target_count"] = proj.TargetCount
			result["environment_count"] = proj.EnvironmentCount
			result["secret_count"] = proj.SecretCount
			result["has_secrets"] = proj.HasSecrets
		}
	case hierarchy.NodeTypeTarget:
		if target, ok := node.Metadata.(*dotenv.Target); ok {
			if target.Description != "" {
				result["description"] = target.Description
			}
		}
	case hierarchy.NodeTypeEnvironment:
		if env, ok := node.Metadata.(*dotenv.Environment); ok {
			result["status"] = env.Status
			if env.Description != "" {
				result["description"] = env.Description
			}
		}
	case hierarchy.NodeTypeOrganization:
		// No extra metadata for organization nodes.
	}

	// Add children
	if len(node.Children) > 0 {
		children := make([]map[string]interface{}, len(node.Children))
		for i, child := range node.Children {
			children[i] = nodeToMap(child)
		}
		result["children"] = children
	}

	return result
}
