package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	versionsLimit    int
	versionsPage     int
	purgeProjectAll  bool
	purgeForce       bool
	restoreForce     bool
	showVersionID    string
	showOutput       string
	versionClientKey string
	versionOldKeys   []string
	diffShowValues   bool
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

// versionsNotFoundHint maps a 404 from the version endpoints to a friendlier
// message: the ID may be wrong, or the server predates secret versioning.
func versionsNotFoundHint(err error) error {
	if dotenv.IsNotFound(err) {
		return fmt.Errorf("not found — check the version ID, and note that servers older than secret versioning return 404 for these endpoints")
	}
	return HandleAPIError(err, accountForErrorContext())
}

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	versionsCmd := &cobra.Command{
		Use:   "versions <project>[/<target>[/<environment>]]",
		Short: "List the backup version history for a level",
		Args:  cobra.ExactArgs(1),
		RunE:  runSecretVersions,
	}
	versionsCmd.Flags().IntVar(&versionsLimit, "limit", 50, "versions per page (max 100)")
	versionsCmd.Flags().IntVar(&versionsPage, "page", 1, "page number")

	showCmd := &cobra.Command{
		Use:   "show <project>[/<target>[/<environment>]] --version <id>",
		Short: "Decrypt and print a backup version locally",
		Long: `Fetch a backup version and decrypt it locally (plaintext never leaves this
machine). Versions under an OLD client-managed key need that key — pass it with
--old-key (file or value, repeatable) or enter it at the prompt; it is verified
against the stored proof before use.`,
		Args: cobra.ExactArgs(1),
		RunE: runSecretShow,
	}
	showCmd.Flags().StringVar(&showVersionID, "version", "", "version ID (required; see 'dotenv secret versions')")
	showCmd.Flags().StringVarP(&showOutput, "output", "o", "", "write plaintext to a file instead of stdout")
	showCmd.Flags().StringVar(&versionClientKey, "client-key", "", "path to the CURRENT client encryption key file, or the key value itself")
	showCmd.Flags().StringArrayVar(&versionOldKeys, "old-key", nil,
		"old encryption key (file or value; repeatable) for versions under rotated keys")
	_ = showCmd.MarkFlagRequired("version")

	diffCmd := &cobra.Command{
		Use:   "diff <project>[/<target>[/<environment>]] <version> [version-b]",
		Short: "Diff a backup version against the current secret (or another version)",
		Long: `Decrypt a backup version locally and show which keys were added, removed or
changed relative to it. With one version the diff is taken against the LIVE
current secret; pass a second version to diff one version against another. The
diff reads from the first (older) state to the second. Values are masked unless
--show-values is passed. Plaintext never leaves this machine.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: runSecretDiff,
	}
	diffCmd.Flags().StringVar(&versionClientKey, "client-key", "", "path to the CURRENT client encryption key file, or the key value itself")
	diffCmd.Flags().StringArrayVar(&versionOldKeys, "old-key", nil,
		"old encryption key (file or value; repeatable) for versions under rotated keys")
	diffCmd.Flags().BoolVar(&diffShowValues, "show-values", false, "show plaintext values in the diff output")

	restoreCmd := &cobra.Command{
		Use:   "restore <version-id>",
		Short: "Restore a secret to a previous version (append-only)",
		Long: `Restore a secret to a previous version. A new version is recorded; nothing
is lost. List version IDs with 'dotenv secret versions <path>'.

Restoring a version encrypted under an OLD client-managed key requires
re-encrypting it under the current key client-side; use the web dashboard for
that case (CLI support is coming).`,
		Args: cobra.ExactArgs(1),
		RunE: runSecretRestore,
	}
	restoreCmd.Flags().BoolVarP(&restoreForce, "force", "f", false, "skip the confirmation prompt")

	purgeCmd := &cobra.Command{
		Use:   "purge <project>[/<target>[/<environment>]]",
		Short: "Permanently delete backup version history",
		Args:  cobra.ExactArgs(1),
		RunE:  runSecretPurge,
	}
	purgeCmd.Flags().BoolVar(&purgeProjectAll, "project-wide", false, "purge history for the whole project, not just this level")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "skip the confirmation prompt(s)")

	secretCmd.AddCommand(versionsCmd, showCmd, diffCmd, restoreCmd, purgeCmd)
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
		return versionsNotFoundHint(err)
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

// fetchAndDecryptVersion fetches a version and decrypts it locally, resolving
// the current key (active versions) or a rotated old key (proof-verified).
func fetchAndDecryptVersion(
	ctx context.Context,
	client *dotenv.Client,
	projectSlug, versionID, clientKeyFlag string,
	oldKeys []string,
) (string, error) {
	version, resp, err := client.SecretVersions.Get(ctx, versionID)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", versionsNotFoundHint(err)
	}
	if version.Content == "" {
		return "", fmt.Errorf("version %s has no content (delete snapshot of an empty level?)", versionID)
	}

	// Active key descriptor: tells us whether the version is under the current key.
	activeDesc, descResp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if descResp != nil {
		defer descResp.Body.Close()
	}
	if err != nil {
		return "", HandleAPIError(err, accountForErrorContext())
	}

	underActiveKey := version.Key == nil ||
		version.Key.IsActive ||
		version.Key.Version == fmt.Sprintf("%d", activeDesc.Version)

	if underActiveKey {
		rk, rkErr := resolveEncryptionKey(ctx, client, projectSlug, clientKeyFlag, promptForClientKey)
		if rkErr != nil {
			return "", rkErr
		}
		dataKey, dkErr := rk.dataKey()
		if dkErr != nil {
			return "", dkErr
		}
		return dotenv.Decrypt(version.Content, []byte(dataKey))
	}

	// Version under a rotated key.
	if version.Key.Managed == managedServer {
		return "", fmt.Errorf(
			"version %s is encrypted under an old SERVER-managed key, which the API never exposes; "+
				"use 'dotenv secret restore %s' instead — the server re-encrypts it onto the current key",
			versionID, versionID,
		)
	}

	oldKey, err := resolveOldClientKey(version.Key.Version, version.Key, oldKeys, promptForClientKey)
	if err != nil {
		return "", err
	}

	if version.Key.KeyCheckSalt != "" {
		dataKey, dkErr := dotenv.DeriveDataKey(oldKey, version.Key.KeyCheckSalt, version.Key.KeyCheckIterations)
		if dkErr != nil {
			return "", fmt.Errorf("failed to derive old data key: %w", dkErr)
		}
		return dotenv.Decrypt(version.Content, dataKey)
	}

	// Legacy old key without a salt: padKey fallback.
	return dotenv.Decrypt(version.Content, []byte(oldKey))
}

// fetchAndDecryptCurrentLevel fetches the LIVE current secret for the deepest
// requested level and decrypts it under the CURRENT key — the counterpart to
// fetchAndDecryptVersion, used as the default "to" side of a diff. Versions hold
// PRIOR states, so the live secret (not the newest snapshot) is "current".
// Returns an empty string when the level has no live content.
func fetchAndDecryptCurrentLevel(
	ctx context.Context,
	client *dotenv.Client,
	project, target, environment, clientKeyFlag string,
) (string, error) {
	resp, hResp, err := client.Secrets.GetProjectSecrets(ctx, project, target, environment)
	if hResp != nil {
		defer hResp.Body.Close()
	}
	if err != nil {
		return "", HandleAPIError(err, accountForErrorContext())
	}
	if resp == nil {
		return "", nil
	}

	// A version is per-level, so compare against that same level's own live blob
	// (the deepest requested level), not the parent-merged view.
	levelName := resourceProject
	switch {
	case environment != "":
		levelName = resourceEnvironment
	case target != "":
		levelName = resourceTarget
	}

	level, ok := resp.Data.Attributes.Levels[levelName]
	if !ok || level.Content == "" {
		return "", nil
	}
	if !level.Encrypted {
		return level.Content, nil
	}

	rk, err := resolveEncryptionKey(ctx, client, project, clientKeyFlag, promptForClientKey)
	if err != nil {
		return "", err
	}
	dataKey, err := rk.dataKey()
	if err != nil {
		return "", err
	}
	return dotenv.DecryptWithProjectKey(level.Content, dataKey)
}

func runSecretShow(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project, _, _, err := parseSecretPath(args[0])
	if err != nil {
		return err
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	plaintext, err := fetchAndDecryptVersion(cmd.Context(), client, project, showVersionID, versionClientKey, versionOldKeys)
	if err != nil {
		return err
	}

	if showOutput != "" {
		if werr := os.WriteFile(showOutput, []byte(plaintext), 0o600); werr != nil {
			return fmt.Errorf("failed to write %s: %w", showOutput, werr)
		}
		ui.PrintSuccessf("Wrote version %s to %s", showVersionID, showOutput)
		return nil
	}

	fmt.Fprintln(ui.Stdout, plaintext)
	return nil
}

func runSecretDiff(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project, target, environment, err := parseSecretPath(args[0])
	if err != nil {
		return err
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	fromVersion := args[1]

	fromPlain, err := fetchAndDecryptVersion(cmd.Context(), client, project, fromVersion, versionClientKey, versionOldKeys)
	if err != nil {
		return fmt.Errorf("version %s: %w", fromVersion, err)
	}

	toLabel := "current"
	var toPlain string
	if len(args) == 3 {
		// Diff one version against another (older -> newer reads left to right).
		toLabel = args[2]
		toPlain, err = fetchAndDecryptVersion(cmd.Context(), client, project, args[2], versionClientKey, versionOldKeys)
		if err != nil {
			return fmt.Errorf("version %s: %w", args[2], err)
		}
	} else {
		// Default: diff against the LIVE current secret. Versions hold prior
		// states, so the newest snapshot is NOT the current content.
		toPlain, err = fetchAndDecryptCurrentLevel(cmd.Context(), client, project, target, environment, versionClientKey)
		if err != nil {
			return err
		}
	}

	fromMap, err := parseSecretContent(fromPlain, formatENV)
	if err != nil {
		return fmt.Errorf("version %s: %w", fromVersion, err)
	}
	toMap, err := parseSecretContent(toPlain, formatENV)
	if err != nil {
		return fmt.Errorf("%s: %w", toLabel, err)
	}

	printSecretDiff(fromVersion, toLabel, fromMap, toMap, diffShowValues)
	return nil
}

// printSecretDiff prints key-level changes going FROM the `from` state TO the
// `to` state: keys present only in `to` are additions (+), keys present only in
// `from` are removals (-), and keys in both with different values show the
// from -> to transition.
func printSecretDiff(fromLabel, toLabel string, from, to map[string]string, showValues bool) {
	mask := func(v string) string {
		if showValues {
			return v
		}
		return "********"
	}

	keys := map[string]struct{}{}
	for k := range from {
		keys[k] = struct{}{}
	}
	for k := range to {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	ui.PrintInfof("Diff from version %s to %s:", fromLabel, toLabel)
	changes := 0
	for _, k := range sorted {
		vFrom, inFrom := from[k]
		vTo, inTo := to[k]
		switch {
		case !inFrom && inTo:
			fmt.Fprintf(ui.Stdout, "+ %s=%s\n", k, mask(vTo))
			changes++
		case inFrom && !inTo:
			fmt.Fprintf(ui.Stdout, "- %s=%s\n", k, mask(vFrom))
			changes++
		case vFrom != vTo:
			fmt.Fprintf(ui.Stdout, "~ %s: %s -> %s\n", k, mask(vFrom), mask(vTo))
			changes++
		}
	}
	if changes == 0 {
		ui.PrintInfof("No differences.")
	}
}

func runSecretRestore(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	versionID := args[0]

	if !restoreForce {
		confirmed, confirmErr := ui.Confirm(
			fmt.Sprintf("Restore version %s? The current value is preserved as a new version, then overwritten.", versionID),
			false,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ui.PrintInfof("Restore canceled")
			return nil
		}
	}

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
			return fmt.Errorf("this version is encrypted under an old client-managed " +
				"key and must be re-encrypted under the current key before restoring; " +
				"use the web dashboard for now")
		}
		return versionsNotFoundHint(err)
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
		what := fmt.Sprintf("the version history at %q", args[0])
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

		// Project-wide purge is the most destructive operation here: require the
		// project name to be typed back, mirroring the web's typed confirmation.
		if purgeProjectAll {
			typed, terr := ui.Input(fmt.Sprintf("Type the project name (%s) to confirm", project), "", nil)
			if terr != nil {
				return terr
			}
			if typed != project {
				ui.PrintInfof("Purge canceled (name did not match)")
				return nil
			}
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
		return versionsNotFoundHint(err)
	}

	ui.PrintSuccessf("Purged %d version(s)", count)
	return nil
}
