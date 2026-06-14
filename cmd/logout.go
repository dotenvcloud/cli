package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	logoutAll   bool
	logoutForce bool
)

var logoutCmd = &cobra.Command{
	Use:   "logout [account-name]",
	Short: "Log out of an account",
	Long: `Log out of a DotEnv account.

For OAuth accounts this revokes the token on the server, so the session is
invalidated everywhere — not just on this machine. API key accounts are only
removed locally (the key itself keeps working until you delete it).

With no arguments it logs out the current account. Pass an account name to log
out a specific one, or --all to log out every account.`,
	Example: `  # Log out the current account
  dotenv logout

  # Log out a specific account
  dotenv logout work@example.com

  # Log out every account
  dotenv logout --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogout,
}

//nolint:gochecknoinits // cobra flag registration is idiomatic in init
func init() {
	logoutCmd.Flags().BoolVar(&logoutAll, "all", false, "log out of all accounts")
	logoutCmd.Flags().BoolVarP(&logoutForce, "force", "f", false, "skip the confirmation prompt")
}

func runLogout(cmd *cobra.Command, args []string) error {
	if logoutAll && len(args) > 0 {
		return fmt.Errorf("cannot combine --all with an account name")
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return err
	}

	targets, err := resolveLogoutTargets(am, args)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		ui.PrintInfof("No accounts to log out")
		return nil
	}

	if !logoutForce {
		msg := fmt.Sprintf("Log out of account %q?", targets[0])
		if logoutAll {
			msg = fmt.Sprintf("Log out of ALL %d accounts?", len(targets))
		}
		confirmed, confirmErr := ui.Confirm(msg, false)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ui.PrintInfof("Logout canceled")
			return nil
		}
	}

	for _, name := range targets {
		account, getErr := am.Get(name)
		if getErr != nil {
			ui.PrintWarningf("Skipping %s: %v", name, getErr)
			continue
		}

		revokeOAuthToken(cmd.Context(), account)

		if removeErr := am.Remove(name); removeErr != nil {
			ui.PrintErrorf("Failed to remove %s: %v", name, removeErr)
			continue
		}
		ui.PrintSuccessf("Logged out: %s", name)
	}

	// Show the new current account, if any remains.
	if current, getErr := am.GetCurrent(); getErr == nil {
		ui.PrintInfof("Current account: %s", current.Name)
	}

	return nil
}

// resolveLogoutTargets decides which account names a logout invocation applies
// to: every account with --all, the named account, or otherwise the current one.
func resolveLogoutTargets(am *config.AccountManager, args []string) ([]string, error) {
	switch {
	case logoutAll:
		return am.List(), nil
	case len(args) == 1:
		return []string{args[0]}, nil
	default:
		current, err := am.GetCurrent()
		if err != nil {
			return nil, fmt.Errorf("no current account to log out: %w", err)
		}
		return []string{current.Name}, nil
	}
}
