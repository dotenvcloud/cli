package cmd

import (
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh OAuth tokens for the current account",
	Long: `Refresh OAuth tokens for the current account. This is an alias for 'dotenv account refresh'.

This is useful when:
- Your OAuth token has expired
- You need to refresh your access token

For API key accounts, re-authentication is required as API keys cannot be refreshed.`,

	Example: `  # Refresh OAuth token for current account
  dotenv refresh`,

	RunE: runRefresh,
}

func runRefresh(cmd *cobra.Command, args []string) error {
	// This is just an alias for 'dotenv account refresh'
	return runAccountRefresh(cmd, args)
}
