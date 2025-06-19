package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var (
	refreshContext string
	refreshAll     bool
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh API credentials and permissions",
	Long: `Refresh your API credentials to update permissions and organization access.
This is useful when:

- Your organization permissions have changed
- You've been added to new organizations
- Your API key permissions have been updated
- You need to sync the latest organization structure`,

	Example: `  # Refresh credentials for current context
  dotenv refresh

  # Refresh specific context
  dotenv refresh --context=production

  # Refresh all contexts
  dotenv refresh --all`,

	RunE: runRefresh,
}

func init() {
	refreshCmd.Flags().StringVar(&refreshContext, "context", "",
		"refresh specific context")
	refreshCmd.Flags().BoolVar(&refreshAll, "all", false,
		"refresh all contexts")
}

func runRefresh(cmd *cobra.Command, args []string) error {
	cm, err := config.NewContextManager("")
	if err != nil {
		return err
	}

	// Determine which context(s) to refresh
	var contextsToRefresh []string

	if refreshAll {
		// Get all contexts
		contexts := cm.List()
		for _, ctx := range contexts {
			contextsToRefresh = append(contextsToRefresh, ctx.Name)
		}
		if len(contextsToRefresh) == 0 {
			return fmt.Errorf("no contexts found")
		}
	} else if refreshContext != "" {
		// Specific context
		if err := cm.ValidateContext(refreshContext); err != nil {
			return err
		}
		contextsToRefresh = []string{refreshContext}
	} else {
		// Current context
		ctx, err := cm.GetCurrent()
		if err != nil {
			return fmt.Errorf("no current context: %w", err)
		}
		contextsToRefresh = []string{ctx.Name}
	}

	ui.PrintInfo("Refreshing %d context(s)...", len(contextsToRefresh))

	// For now, just prompt to re-login
	// In the future, this could refresh tokens automatically
	for _, contextName := range contextsToRefresh {
		ui.PrintInfo("Context: %s", contextName)
		ui.PrintInfo("Please re-authenticate to refresh permissions")

		// Set specific context for login
		viper.Set("refresh_context", contextName)

		// Run login with manual flag
		loginManual = true
		if err := runLogin(cmd, args); err != nil {
			ui.PrintError("Failed to refresh %s: %v", contextName, err)
			continue
		}
	}

	ui.PrintSuccess("Refresh complete!")
	return nil
}
