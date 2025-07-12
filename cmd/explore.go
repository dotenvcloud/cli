package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"github.com/dotenv/cli/internal/hierarchy"
	"github.com/dotenv/cli/internal/interactive"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var (
	exploreStartPath string
	exploreAction    string
)

var exploreCmd = &cobra.Command{
	Use:   "explore [starting-path]",
	Short: "Interactively explore projects, targets, and environments",
	Long: `Navigate through your DotEnv resources interactively.
    
This command provides an interactive interface to browse through your
projects, targets, and environments, with options to copy paths or
execute commands on selected resources.`,
	
	Example: `  # Start exploring from organization root
  dotenv explore
  
  # Start exploring from a specific project
  dotenv explore myproject
  
  # Start exploring from a specific target
  dotenv explore myproject/production`,
	
	RunE: runExplore,
}

func init() {
	exploreCmd.Flags().StringVar(&exploreAction, "action", "select",
		"default action when selecting a resource (select, copy, pull)")
}

func runExplore(cmd *cobra.Command, args []string) error {
	// Display account/org info
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarning("Could not display account info: %v", err)
		}
	}
	
	// Get starting path from args
	if len(args) > 0 {
		exploreStartPath = args[0]
	}
	
	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}
	
	// Build initial hierarchy
	builder := hierarchy.NewBuilder(client)
	ctx := cmd.Context()
	
	// Determine starting point
	var startNode *hierarchy.Node
	if exploreStartPath != "" {
		// Parse path and build from that point
		parts := strings.Split(exploreStartPath, "/")
		switch len(parts) {
		case 1:
			// Project level
			startNode, err = builder.BuildProject(ctx, parts[0])
			if err != nil {
				return fmt.Errorf("failed to load project %s: %w", parts[0], err)
			}
		case 2:
			// Target level
			startNode, err = builder.BuildTarget(ctx, parts[0], parts[1])
			if err != nil {
				return fmt.Errorf("failed to load target %s/%s: %w", parts[0], parts[1], err)
			}
		default:
			return fmt.Errorf("invalid starting path: %s (expected project or project/target)", exploreStartPath)
		}
	} else {
		// Start from organization root
		account, err := getCurrentAccount()
		if err != nil {
			return err
		}
		
		orgIdentifier, err := account.GetOrganizationIdentifier()
		if err != nil {
			return fmt.Errorf("failed to get organization: %w", err)
		}
		
		ui.PrintInfo("Loading organization resources...")
		startNode, err = builder.Build(ctx, orgIdentifier)
		if err != nil {
			return fmt.Errorf("failed to load organization: %w", err)
		}
	}
	
	// Create and run explorer
	explorer := interactive.NewExplorer(startNode, builder, client)
	
	for {
		selectedPath, action, err := explorer.Run()
		if err != nil {
			if err == interactive.ErrExit {
				ui.PrintInfo("Exiting explorer")
				return nil
			}
			return err
		}
		
		// Handle action
		switch action {
		case interactive.ActionCopy:
			if err := copyToClipboard(selectedPath); err != nil {
				ui.PrintWarning("Failed to copy to clipboard: %v", err)
				ui.PrintInfo("Path: %s", selectedPath)
			} else {
				ui.PrintSuccess("Copied to clipboard: %s", selectedPath)
			}
			// Continue exploring
			
		case interactive.ActionPull:
			ui.PrintInfo("Running: dotenv pull %s", selectedPath)
			// Exit explorer and run pull command
			return runPull(cmd, []string{selectedPath})
			
		case interactive.ActionPullLevelOnly:
			ui.PrintInfo("Running: dotenv pull %s --level-only", selectedPath)
			// Set the flag and run pull command
			pullLevelOnly = true
			return runPull(cmd, []string{selectedPath})
			
		case interactive.ActionPush:
			ui.PrintInfo("Running: dotenv push for %s", selectedPath)
			// Prompt for file
			file, err := ui.Input("Enter file to push", ".env", nil)
			if err != nil {
				ui.PrintError("Failed to get file input: %v", err)
				continue
			}
			// Exit explorer and run push command
			return runPush(cmd, []string{file, selectedPath})
			
		case interactive.ActionView:
			// Show details about the selected resource
			if err := showResourceDetails(ctx, client, selectedPath); err != nil {
				ui.PrintError("Failed to get details: %v", err)
			}
			// Continue exploring
			
		case interactive.ActionSelect:
			ui.PrintSuccess("Selected: %s", selectedPath)
			return nil
			
		case interactive.ActionExit:
			return nil
		}
	}
}

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try different clipboard commands
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("no clipboard command found (install xclip, xsel, or wl-clipboard)")
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	// Write text to command's stdin
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	
	if err := cmd.Start(); err != nil {
		return err
	}
	
	if _, err := in.Write([]byte(text)); err != nil {
		return err
	}
	
	if err := in.Close(); err != nil {
		return err
	}
	
	return cmd.Wait()
}

// showResourceDetails displays detailed information about a resource
func showResourceDetails(ctx context.Context, client *dotenv.Client, path string) error {
	parts := strings.Split(path, "/")
	
	ui.PrintHeader("Resource Details")
	ui.PrintKeyValue("Path", path)
	
	switch len(parts) {
	case 1:
		// Project details
		project, _, err := client.Projects.Get(ctx, parts[0])
		if err != nil {
			return err
		}
		
		ui.PrintKeyValue("Type", "Project")
		ui.PrintKeyValue("Name", project.Name)
		ui.PrintKeyValue("Slug", project.Slug)
		if project.Description != "" {
			ui.PrintKeyValue("Description", project.Description)
		}
		ui.PrintKeyValue("Secrets", fmt.Sprintf("%d", project.SecretCount))
		ui.PrintKeyValue("Targets", fmt.Sprintf("%d", project.TargetCount))
		ui.PrintKeyValue("Environments", fmt.Sprintf("%d", project.EnvironmentCount))
		ui.PrintKeyValue("Created", project.CreatedAt.Format("2006-01-02 15:04:05"))
		ui.PrintKeyValue("Updated", project.UpdatedAt.Format("2006-01-02 15:04:05"))
		
	case 2:
		// Target details
		target, _, err := client.Targets.Get(ctx, parts[0], parts[1])
		if err != nil {
			return err
		}
		
		ui.PrintKeyValue("Type", "Target")
		ui.PrintKeyValue("Name", target.Name)
		ui.PrintKeyValue("Slug", target.Slug)
		if target.Description != "" {
			ui.PrintKeyValue("Description", target.Description)
		}
		ui.PrintKeyValue("Project", parts[0])
		ui.PrintKeyValue("Created", target.CreatedAt.Format("2006-01-02 15:04:05"))
		ui.PrintKeyValue("Updated", target.UpdatedAt.Format("2006-01-02 15:04:05"))
		
		// List environments in this target
		envs, _, err := client.Environments.List(ctx, parts[0], parts[1], nil)
		if err == nil && len(envs) > 0 {
			ui.PrintSubheader("Environments:")
			for _, env := range envs {
				fmt.Printf("  • %s (%s)\n", env.Name, env.Status)
			}
		}
		
	case 3:
		// Environment details
		env, _, err := client.Environments.Get(ctx, parts[0], parts[1], parts[2])
		if err != nil {
			return err
		}
		
		ui.PrintKeyValue("Type", "Environment")
		ui.PrintKeyValue("Name", env.Name)
		ui.PrintKeyValue("Slug", env.Slug)
		if env.Description != "" {
			ui.PrintKeyValue("Description", env.Description)
		}
		ui.PrintKeyValue("Status", env.Status)
		ui.PrintKeyValue("Project", parts[0])
		ui.PrintKeyValue("Target", parts[1])
		ui.PrintKeyValue("Created", env.CreatedAt.Format("2006-01-02 15:04:05"))
		ui.PrintKeyValue("Updated", env.UpdatedAt.Format("2006-01-02 15:04:05"))
		
		// Could show secret count if available
		ui.PrintInfo("\nUse 'dotenv pull %s' to retrieve secrets from this environment", path)
	}
	
	fmt.Println() // Empty line after details
	ui.PrintInfo("Press Enter to continue...")
	fmt.Scanln() // Wait for user input
	
	return nil
}