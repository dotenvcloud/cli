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

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenv/cli/internal/hierarchy"
	"github.com/dotenv/cli/internal/interactive"
	"github.com/dotenv/cli/internal/ui"
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

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	exploreCmd.Flags().StringVar(&exploreAction, "action", "select",
		"default action when selecting a resource (select, copy, pull)")
}

func runExplore(cmd *cobra.Command, args []string) error {
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

	if len(args) > 0 {
		exploreStartPath = args[0]
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	builder := hierarchy.NewBuilder(client)
	ctx := cmd.Context()

	startNode, err := buildStartNode(ctx, builder)
	if err != nil {
		return err
	}

	explorer := interactive.NewExplorer(startNode, builder, client)
	return runExplorerLoop(ctx, cmd, explorer, client)
}

func buildStartNode(ctx context.Context, builder *hierarchy.Builder) (*hierarchy.Node, error) {
	if exploreStartPath != "" {
		return buildStartNodeFromPath(ctx, builder, exploreStartPath)
	}
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}
	orgIdentifier, err := account.GetOrganizationIdentifier()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	ui.PrintInfof("Loading organization resources...")
	startNode, err := builder.Build(ctx, orgIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization: %w", err)
	}
	return startNode, nil
}

func buildStartNodeFromPath(ctx context.Context, builder *hierarchy.Builder, path string) (*hierarchy.Node, error) {
	parts := strings.Split(path, "/")
	switch len(parts) {
	case 1:
		n, err := builder.BuildProject(ctx, parts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to load project %s: %w", parts[0], err)
		}
		return n, nil
	case 2:
		n, err := builder.BuildTarget(ctx, parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to load target %s/%s: %w", parts[0], parts[1], err)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("invalid starting path: %s (expected project or project/target)", path)
	}
}

func runExplorerLoop(ctx context.Context, cmd *cobra.Command, explorer *interactive.Explorer, client *dotenv.Client) error {
	for {
		selectedPath, action, err := explorer.Run()
		if err != nil {
			if err == interactive.ErrExit {
				ui.PrintInfof("Exiting explorer")
				return nil
			}
			return err
		}

		done, err := handleExplorerAction(ctx, cmd, client, action, selectedPath)
		if done {
			return err
		}
	}
}

func handleExplorerAction(
	ctx context.Context, cmd *cobra.Command, client *dotenv.Client,
	action interactive.Action, selectedPath string,
) (bool, error) {
	switch action {
	case interactive.ActionCopy:
		handleExploreCopy(selectedPath)
		return false, nil
	case interactive.ActionPull:
		return handleExplorePull(cmd, selectedPath, false)
	case interactive.ActionPullLevelOnly:
		return handleExplorePull(cmd, selectedPath, true)
	case interactive.ActionPush:
		return handleExplorePush(cmd, selectedPath)
	case interactive.ActionView:
		if err := showResourceDetails(ctx, client, selectedPath); err != nil {
			ui.PrintErrorf("Failed to get details: %v", err)
		}
		return false, nil
	case interactive.ActionSelect:
		ui.PrintSuccessf("Selected: %s", selectedPath)
		return true, nil
	case interactive.ActionExit:
		return true, nil
	case interactive.ActionBack:
		return false, nil
	default:
		return false, nil
	}
}

func handleExploreCopy(selectedPath string) {
	if err := copyToClipboard(selectedPath); err != nil {
		ui.PrintWarningf("Failed to copy to clipboard: %v", err)
		ui.PrintInfof("Path: %s", selectedPath)
		return
	}
	ui.PrintSuccessf("Copied to clipboard: %s", selectedPath)
}

func handleExplorePull(cmd *cobra.Command, selectedPath string, levelOnly bool) (bool, error) {
	outputFile, err := ui.InputWithHelp(
		"Enter output file path (or press Enter for terminal output)",
		"",
		"Specify a file path to save secrets (e.g., .env, secrets.env), or press Enter to display in terminal",
		nil,
	)
	if err != nil {
		ui.PrintErrorf("Failed to get output file: %v", err)
		return false, nil
	}

	pullOutput = outputFile
	pullLevelOnly = levelOnly

	flagSuffix := ""
	if levelOnly {
		flagSuffix = " --level-only"
	}
	if outputFile != "" {
		ui.PrintInfof("Running: dotenv pull %s%s --output=%s", selectedPath, flagSuffix, outputFile)
	} else {
		ui.PrintInfof("Running: dotenv pull %s%s", selectedPath, flagSuffix)
	}

	err = runPull(cmd, []string{selectedPath})
	pullOutput = ""
	pullLevelOnly = false

	if err != nil {
		ShowErrorWithHelp(err)
		return false, nil
	}
	return true, nil
}

func handleExplorePush(cmd *cobra.Command, selectedPath string) (bool, error) {
	ui.PrintInfof("Running: dotenv push for %s", selectedPath)
	file, err := ui.Input("Enter file to push", ".env", nil)
	if err != nil {
		ui.PrintErrorf("Failed to get file input: %v", err)
		return false, nil
	}
	return true, runPush(cmd, []string{file, selectedPath})
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
	case platformWindows:
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

	var err error
	switch len(parts) {
	case 1:
		err = showProjectDetails(ctx, client, parts[0])
	case 2:
		err = showTargetDetails(ctx, client, parts[0], parts[1])
	case 3:
		err = showEnvironmentDetails(ctx, client, parts[0], parts[1], parts[2], path)
	}
	if err != nil {
		return err
	}

	fmt.Println() // Empty line after details
	ui.PrintInfof("Press Enter to continue...")
	_, _ = fmt.Scanln() // Wait for user input; ignore EOF/empty input errors

	return nil
}

func showProjectDetails(ctx context.Context, client *dotenv.Client, projectSlug string) error {
	project, getResp, err := client.Projects.Get(ctx, projectSlug)
	if getResp != nil {
		defer getResp.Body.Close()
	}
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
	return nil
}

func showTargetDetails(ctx context.Context, client *dotenv.Client, projectSlug, targetSlug string) error {
	target, getResp, err := client.Targets.Get(ctx, projectSlug, targetSlug)
	if getResp != nil {
		defer getResp.Body.Close()
	}
	if err != nil {
		return err
	}
	ui.PrintKeyValue("Type", "Target")
	ui.PrintKeyValue("Name", target.Name)
	ui.PrintKeyValue("Slug", target.Slug)
	if target.Description != "" {
		ui.PrintKeyValue("Description", target.Description)
	}
	ui.PrintKeyValue("Project", projectSlug)
	ui.PrintKeyValue("Created", target.CreatedAt.Format("2006-01-02 15:04:05"))
	ui.PrintKeyValue("Updated", target.UpdatedAt.Format("2006-01-02 15:04:05"))

	envs, envResp, err := client.Environments.List(ctx, projectSlug, targetSlug, nil)
	if envResp != nil {
		defer envResp.Body.Close()
	}
	if err == nil && len(envs) > 0 {
		ui.PrintSubheader("Environments:")
		for _, env := range envs {
			fmt.Printf("  • %s (%s)\n", env.Name, env.Status)
		}
	}
	return nil
}

func showEnvironmentDetails(ctx context.Context, client *dotenv.Client, projectSlug, targetSlug, envSlug, path string) error {
	env, envResp, err := client.Environments.Get(ctx, projectSlug, targetSlug, envSlug)
	if envResp != nil {
		defer envResp.Body.Close()
	}
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
	ui.PrintKeyValue("Project", projectSlug)
	ui.PrintKeyValue("Target", targetSlug)
	ui.PrintKeyValue("Created", env.CreatedAt.Format("2006-01-02 15:04:05"))
	ui.PrintKeyValue("Updated", env.UpdatedAt.Format("2006-01-02 15:04:05"))
	ui.PrintInfof("\nUse 'dotenv pull %s' to retrieve secrets from this environment", path)
	return nil
}
