package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "Manage targets within a project",
	Long:  `Create, update and delete targets. Use 'dotenv list targets <project>' to view them.`,
	Example: `  dotenv target create my-app production
  dotenv target update my-app production --name "Prod"
  dotenv target delete my-app production`,
}

var (
	targetCreateCmd *cobra.Command
	targetUpdateCmd *cobra.Command
	targetDeleteCmd *cobra.Command
)

var (
	targetDescription string
	targetName        string
	targetNewSlug     string
	targetForce       bool
)

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	targetCreateCmd = &cobra.Command{
		Use:   "create [project] [name]",
		Short: "Create a target",
		Args:  cobra.ExactArgs(2),
		RunE:  runTargetCreate,
	}
	targetCreateCmd.Flags().StringVar(&targetDescription, "description", "", "target description")

	targetUpdateCmd = &cobra.Command{
		Use:   "update [project] [target]",
		Short: "Update a target",
		Args:  cobra.ExactArgs(2),
		RunE:  runTargetUpdate,
	}
	targetUpdateCmd.Flags().StringVar(&targetName, "name", "", "new target name")
	targetUpdateCmd.Flags().StringVar(&targetDescription, "description", "", "new description")
	targetUpdateCmd.Flags().StringVar(&targetNewSlug, "slug", "", "new slug")

	targetDeleteCmd = &cobra.Command{
		Use:   "delete [project] [target]",
		Short: "Delete a target and all of its environments and secrets",
		Args:  cobra.ExactArgs(2),
		RunE:  runTargetDelete,
	}
	targetDeleteCmd.Flags().BoolVarP(&targetForce, "force", "f", false, "skip the confirmation prompt")

	targetCmd.AddCommand(targetCreateCmd, targetUpdateCmd, targetDeleteCmd)
}

func runTargetCreate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	target, resp, err := client.Targets.Create(cmd.Context(), args[0], &dotenv.Target{
		Name:        args[1],
		Description: targetDescription,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Created target %s (%s) in project %s", target.Name, target.Slug, args[0])
	return nil
}

func runTargetUpdate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	target, resp, err := client.Targets.Update(cmd.Context(), args[0], args[1], &dotenv.Target{
		Name:        targetName,
		Description: targetDescription,
		Slug:        targetNewSlug,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Updated target %s (%s)", target.Name, target.Slug)
	return nil
}

func runTargetDelete(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !targetForce {
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Delete target %q in project %q and ALL its environments and secrets? This cannot be undone.", args[1], args[0]),
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

	resp, err := client.Targets.Delete(cmd.Context(), args[0], args[1])
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Deleted target %s", args[1])
	return nil
}
