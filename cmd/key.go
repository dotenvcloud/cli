package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dotenvcloud/cli/internal/ui"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage a project's encryption keys",
	Long:  `Inspect and manage the encryption keys for a project, including the key version history.`,
}

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	keyHistoryCmd := &cobra.Command{
		Use:   "history [project]",
		Short: "List the encryption key version history for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runKeyHistory,
	}

	keyCmd.AddCommand(keyHistoryCmd)
}

func runKeyHistory(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project := args[0]

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	keys, resp, err := client.Encryption.GetKeyHistory(cmd.Context(), project)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	if len(keys) == 0 {
		ui.PrintInfof("No encryption keys found for %s", project)
		return nil
	}

	w := tabwriter.NewWriter(ui.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tMANAGED\tACTIVE\tCREATED\tROTATED")
	fmt.Fprintln(w, "───────\t───────\t──────\t───────\t───────")
	for _, k := range keys {
		active := ""
		if k.IsActive {
			active = "✓"
		}
		rotated := k.RotatedAt
		if rotated == "" {
			rotated = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", k.Version, k.Managed, active, k.CreatedAt, rotated)
	}
	w.Flush()

	return nil
}
