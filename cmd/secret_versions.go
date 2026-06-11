package cmd

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	versionsLimit   int
	versionsPage    int
	purgeProjectAll bool
	purgeForce      bool
)

// parseSecretPath splits a "project[/target[/environment]]" argument.
func parseSecretPath(arg string) (project, target, environment string, err error) {
	parts := strings.Split(arg, "/")
	if len(parts) > 3 {
		return "", "", "", fmt.Errorf("invalid path: use project[/target[/environment]]")
	}
	for _, p := range parts {
		if p == "" {
			return "", "", "", fmt.Errorf("invalid path %q: empty segment (use project[/target[/environment]])", arg)
		}
	}
	project = parts[0]
	if len(parts) >= 2 {
		target = parts[1]
	}
	if len(parts) == 3 {
		environment = parts[2]
	}
	return project, target, environment, nil
}

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	versionsCmd := &cobra.Command{
		Use:   "versions [project]/[target]/[environment]",
		Short: "List the backup version history for a level",
		Args:  cobra.ExactArgs(1),
		RunE:  runSecretVersions,
	}
	versionsCmd.Flags().IntVar(&versionsLimit, "limit", 50, "versions per page (max 100)")
	versionsCmd.Flags().IntVar(&versionsPage, "page", 1, "page number")

	restoreCmd := &cobra.Command{
		Use:   "restore [version-id]",
		Short: "Restore a secret to a previous version (append-only)",
		Long: `Restore a secret to a previous version. A new version is recorded; nothing
is lost. List version IDs with 'dotenv secret versions <path>'.

Restoring a version encrypted under an OLD client-managed key requires
re-encrypting it under the current key client-side; use the web dashboard for
that case (CLI support is coming).`,
		Args: cobra.ExactArgs(1),
		RunE: runSecretRestore,
	}

	purgeCmd := &cobra.Command{
		Use:   "purge [project]/[target]/[environment]",
		Short: "Permanently delete backup version history",
		Args:  cobra.ExactArgs(1),
		RunE:  runSecretPurge,
	}
	purgeCmd.Flags().BoolVar(&purgeProjectAll, "project-wide", false, "purge history for the whole project, not just this level")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "skip the confirmation prompt")

	secretCmd.AddCommand(versionsCmd, restoreCmd, purgeCmd)
}

func runSecretVersions(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project, target, environment, err := parseSecretPath(args[0])
	if err != nil {
		return err
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	versions, meta, resp, err := client.SecretVersions.List(cmd.Context(), project, target, environment, &dotenv.VersionListOptions{
		Page:    versionsPage,
		PerPage: versionsLimit,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(versions) == 0 {
		ui.PrintInfof("No version history for %s", args[0])
		return nil
	}

	w := tabwriter.NewWriter(ui.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tACTION\tSIZE\tKEY\tCREATED\tBY")
	fmt.Fprintln(w, "───────\t──────\t────\t───\t───────\t──")
	for _, v := range versions {
		by := "system"
		if v.CreatedBy != nil && v.CreatedBy.Name != "" {
			by = v.CreatedBy.Name
		}
		keyVer := v.KeyVersion
		if keyVer == "" {
			keyVer = v.EncryptionKeyVersion
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", v.ID, v.Action, v.SizeBytes, keyVer, v.CreatedAt, by)
	}
	w.Flush()

	if meta != nil && meta.LastPage > 1 {
		ui.PrintInfof("Page %d of %d (%d total)", meta.CurrentPage, meta.LastPage, meta.Total)
	}
	return nil
}

func runSecretRestore(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	versionID := args[0]

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	resp, err := client.SecretVersions.Restore(cmd.Context(), versionID, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if errors.Is(err, dotenv.ErrContentRequiredForOldKey) {
			return fmt.Errorf("this version is encrypted under an old client-managed key and must be re-encrypted under the current key before restoring; use the web dashboard for now")
		}
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Restored version %s", versionID)
	return nil
}

func runSecretPurge(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project, target, environment, err := parseSecretPath(args[0])
	if err != nil {
		return err
	}

	scope := "level"
	target2, environment2 := target, environment
	if purgeProjectAll {
		scope = "project"
		target2, environment2 = "", ""
	}

	if !purgeForce {
		what := fmt.Sprintf("the %s history at %q", scope, args[0])
		if purgeProjectAll {
			what = fmt.Sprintf("ALL version history for project %q", project)
		}
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Permanently delete %s? This cannot be undone.", what),
			false,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ui.PrintInfof("Purge canceled")
			return nil
		}
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	count, resp, err := client.SecretVersions.PurgeHistory(cmd.Context(), dotenv.PurgeHistoryRequest{
		Project:     project,
		Target:      target2,
		Environment: environment2,
		Scope:       scope,
		Confirmed:   true,
	})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Purged %d version(s)", count)
	return nil
}
