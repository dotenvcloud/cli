package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var environmentCmd = &cobra.Command{
	Use:     "environment",
	Aliases: []string{"env"},
	Short:   "Manage environments within a target",
	Long:    `Create, update and delete environments. Use 'dotenv list environments <project>/<target>' to view them.`,
	Example: `  dotenv environment create my-app production api --status production
  dotenv environment update my-app production api --name "API"
  dotenv environment delete my-app production api`,
}

var (
	environmentCreateCmd *cobra.Command
	environmentUpdateCmd *cobra.Command
	environmentDeleteCmd *cobra.Command
)

var (
	environmentDescription string
	environmentName        string
	environmentNewSlug     string
	environmentStatus      string
	environmentForce       bool
)

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	environmentCreateCmd = &cobra.Command{
		Use:   "create [project] [target] [name]",
		Short: "Create an environment",
		Args:  cobra.ExactArgs(3),
		RunE:  runEnvironmentCreate,
	}
	environmentCreateCmd.Flags().StringVar(&environmentStatus, "status", "", "status (production, staging, development, local)")
	environmentCreateCmd.Flags().StringVar(&environmentDescription, "description", "", "environment description")

	environmentUpdateCmd = &cobra.Command{
		Use:   "update [project] [target] [environment]",
		Short: "Update an environment",
		Args:  cobra.ExactArgs(3),
		RunE:  runEnvironmentUpdate,
	}
	environmentUpdateCmd.Flags().StringVar(&environmentName, "name", "", "new environment name")
	environmentUpdateCmd.Flags().StringVar(&environmentStatus, "status", "", "new status")
	environmentUpdateCmd.Flags().StringVar(&environmentDescription, "description", "", "new description")
	environmentUpdateCmd.Flags().StringVar(&environmentNewSlug, "slug", "", "new slug")

	environmentDeleteCmd = &cobra.Command{
		Use:   "delete [project] [target] [environment]",
		Short: "Delete an environment and all of its secrets",
		Args:  cobra.ExactArgs(3),
		RunE:  runEnvironmentDelete,
	}
	environmentDeleteCmd.Flags().BoolVarP(&environmentForce, "force", "f", false, "skip the confirmation prompt")

	environmentCmd.AddCommand(environmentCreateCmd, environmentUpdateCmd, environmentDeleteCmd)
}

func runEnvironmentCreate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	env, resp, err := client.Environments.Create(cmd.Context(), args[0], args[1], &dotenv.Environment{
		Name:        args[2],
		Status:      environmentStatus,
		Description: environmentDescription,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Created environment %s (%s) in %s/%s", env.Name, env.Slug, args[0], args[1])
	return nil
}

func runEnvironmentUpdate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	env, resp, err := client.Environments.Update(cmd.Context(), args[0], args[1], args[2], &dotenv.Environment{
		Name:        environmentName,
		Status:      environmentStatus,
		Description: environmentDescription,
		Slug:        environmentNewSlug,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Updated environment %s (%s)", env.Name, env.Slug)
	return nil
}

func runEnvironmentDelete(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !environmentForce {
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Delete environment %q in %s/%s and ALL its secrets? This cannot be undone.", args[2], args[0], args[1]),
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

	resp, err := client.Environments.Delete(cmd.Context(), args[0], args[1], args[2])
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Deleted environment %s", args[2])
	return nil
}
