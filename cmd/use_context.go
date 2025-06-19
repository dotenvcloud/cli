package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var (
	useContextCurrent bool
	useContextList    bool
)

var useContextCmd = &cobra.Command{
	Use:     "use-context [context-name]",
	Aliases: []string{"context", "ctx"},
	Short:   "Switch between contexts",
	Long: `Switch between configured contexts (organizations).

A context represents a connection to a specific organization
with its associated API credentials and settings.`,

	Example: `  # Switch to a different context
  dotenv use-context production

  # Show current context
  dotenv use-context --current

  # List all available contexts
  dotenv use-context --list`,

	Args: cobra.MaximumNArgs(1),
	RunE: runUseContext,
}

func init() {
	useContextCmd.Flags().BoolVar(&useContextCurrent, "current", false,
		"show current context")
	useContextCmd.Flags().BoolVar(&useContextList, "list", false,
		"list all available contexts")
}

func runUseContext(cmd *cobra.Command, args []string) error {
	cm, err := config.NewContextManager("")
	if err != nil {
		return err
	}

	// Handle flags
	if useContextCurrent {
		ctx, err := cm.GetCurrent()
		if err != nil {
			return fmt.Errorf("no current context set")
		}
		ui.PrintInfo("Current context: %s", ctx.Name)
		return nil
	}

	if useContextList {
		contexts := cm.List()
		if len(contexts) == 0 {
			ui.PrintWarning("No contexts configured. Run 'dotenv init' to get started.")
			return nil
		}

		ui.PrintInfo("Available contexts:")
		for _, ctx := range contexts {
			if ctx.Current {
				fmt.Printf("  * %s (organization: %s)\n", ctx.Name, ctx.Organization)
			} else {
				fmt.Printf("    %s (organization: %s)\n", ctx.Name, ctx.Organization)
			}
		}
		return nil
	}

	// Switch context
	if len(args) == 0 {
		return fmt.Errorf("specify a context name, or use --current or --list")
	}

	contextName := args[0]

	// Validate context exists
	if err := cm.ValidateContext(contextName); err != nil {
		return err
	}

	// Set as current
	if err := cm.Use(contextName); err != nil {
		return err
	}

	ui.PrintSuccess("Switched to context: %s", contextName)

	// Show organization info
	ctx, err := cm.GetContext(contextName)
	if err == nil {
		ui.PrintInfo("Organization: %s", ctx.Organization)
		if ctx.APIURL != "https://api.dotenv.com" {
			ui.PrintInfo("API URL: %s", ctx.APIURL)
		}
	}

	return nil
}
