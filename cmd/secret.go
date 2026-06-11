package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotenvcloud/cli/internal/ui"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets at a hierarchy level",
	Long: `Manage the secrets stored at a project/target/environment level.

Use 'dotenv pull' to read and 'dotenv push' to write secrets; 'dotenv secret
delete' clears the encrypted blob stored at a level.`,
	Example: `  dotenv secret delete my-app/production/api`,
}

var (
	secretForce    bool
	secretNoBackup bool
)

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	secretDeleteCmd := &cobra.Command{
		Use:   "delete [project]/[target]/[environment]",
		Short: "Delete (clear) the secrets stored at a level",
		Args:  cobra.ExactArgs(1),
		RunE:  runSecretDelete,
	}
	secretDeleteCmd.Flags().BoolVarP(&secretForce, "force", "f", false, "skip the confirmation prompt")
	secretDeleteCmd.Flags().BoolVar(&secretNoBackup, "no-backup", false,
		"leave no trace: also purge version history and hard-delete")

	secretCmd.AddCommand(secretDeleteCmd)
}

func runSecretDelete(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	parts := strings.Split(args[0], "/")
	if len(parts) > 3 {
		return fmt.Errorf("invalid path: use project[/target[/environment]]")
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("invalid path %q: empty segment (use project[/target[/environment]])", args[0])
		}
	}
	project := parts[0]
	var target, environment string
	if len(parts) >= 2 {
		target = parts[1]
	}
	if len(parts) == 3 {
		environment = parts[2]
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !secretForce {
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Delete the secrets stored at %q? This cannot be undone.", args[0]),
			false,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ui.PrintInfof("Deletion canceled")
			return nil
		}
	}

	var resp *http.Response
	if secretNoBackup {
		resp, err = client.Secrets.DeleteSecretLevelWithOptions(cmd.Context(), project, target, environment, true)
	} else {
		resp, err = client.Secrets.DeleteSecretLevel(cmd.Context(), project, target, environment)
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Deleted secrets at %s", args[0])
	return nil
}
