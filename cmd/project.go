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
	projectStorage     string
	projectClientKey   string
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
	projectCreateCmd.Flags().StringVar(&projectStorage, "storage", "", "encryption storage mode: client (default) or server")
	projectCreateCmd.Flags().StringVar(&projectClientKey, "client-key", "",
		"client encryption key file or value (client-managed); prompted if omitted")

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

	opts, err := buildProjectCreateOptions()
	if err != nil {
		return err
	}

	project, resp, err := client.Projects.Create(cmd.Context(), &dotenv.Project{
		Name:         args[0],
		Description:  projectDescription,
		SecretFormat: projectFormat,
	}, opts)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Created project %s (%s)", project.Name, project.Slug)
	if opts.StorageMode == "client" {
		ui.PrintInfof("Client-managed encryption established. Keep your key safe — it is required to push and pull and is never stored on the server.")
	}
	return nil
}

// buildProjectCreateOptions resolves the encryption setup for project creation.
// The default is client-managed (the server holds no key): the CLI resolves a
// key (flag/env/prompt), derives a PBKDF2 proof, and registers only the proof —
// never the key. --storage server defers to a server-generated key (no proof).
func buildProjectCreateOptions() (*dotenv.ProjectCreateOptions, error) {
	storage := projectStorage
	if storage == "" {
		storage = "client"
	}
	if storage != "client" && storage != "server" {
		return nil, fmt.Errorf("invalid --storage %q: use 'client' or 'server'", storage)
	}

	opts := &dotenv.ProjectCreateOptions{StorageMode: storage}
	if storage != "client" {
		return opts, nil
	}

	key, err := resolveClientKeyValue(
		projectClientKey, promptForClientKey,
		"This project will use client-managed encryption. Choose an encryption key to use for it.",
	)
	if err != nil {
		return nil, err
	}

	salt, proof, iters, err := dotenv.GenerateKeyProof(key)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key proof: %w", err)
	}
	opts.KeyCheck = proof
	opts.KeyCheckSalt = salt
	opts.KeyCheckIterations = iters
	return opts, nil
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
