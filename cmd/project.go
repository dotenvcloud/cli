package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Create, update and delete projects. Use 'dotenv list projects' to view them.`,
	Example: `  dotenv project create my-app --description "Main application"
  dotenv project update my-app --name "My App"
  dotenv project delete my-app`,
}

var (
	projectCreateCmd *cobra.Command
	projectUpdateCmd *cobra.Command
	projectDeleteCmd *cobra.Command
)

var (
	projectDescription string
	projectName        string
	projectNewSlug     string
	projectFormat      string
	projectForce       bool
)

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	projectCreateCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectCreate,
	}
	projectCreateCmd.Flags().StringVar(&projectDescription, "description", "", "project description")
	projectCreateCmd.Flags().StringVar(&projectFormat, "format", "", "secret format (env, json, yaml, text)")

	projectUpdateCmd = &cobra.Command{
		Use:   "update [project]",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectUpdate,
	}
	projectUpdateCmd.Flags().StringVar(&projectName, "name", "", "new project name")
	projectUpdateCmd.Flags().StringVar(&projectDescription, "description", "", "new description")
	projectUpdateCmd.Flags().StringVar(&projectNewSlug, "slug", "", "new slug")

	projectDeleteCmd = &cobra.Command{
		Use:   "delete [project]",
		Short: "Delete a project and all of its targets, environments and secrets",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectDelete,
	}
	projectDeleteCmd.Flags().BoolVarP(&projectForce, "force", "f", false, "skip the confirmation prompt")

	projectCmd.AddCommand(projectCreateCmd, projectUpdateCmd, projectDeleteCmd)
}

func runProjectCreate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	project, resp, err := client.Projects.Create(cmd.Context(), &dotenv.Project{
		Name:         args[0],
		Description:  projectDescription,
		SecretFormat: projectFormat,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Created project %s (%s)", project.Name, project.Slug)
	return nil
}

func runProjectUpdate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	project, resp, err := client.Projects.Update(cmd.Context(), args[0], &dotenv.Project{
		Name:        projectName,
		Description: projectDescription,
		Slug:        projectNewSlug,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Updated project %s (%s)", project.Name, project.Slug)
	return nil
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	printActiveIdentity()
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !projectForce {
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Delete project %q and ALL its targets, environments and secrets? This cannot be undone.", args[0]),
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

	resp, err := client.Projects.Delete(cmd.Context(), args[0])
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Deleted project %s", args[0])
	return nil
}
