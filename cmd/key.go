package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	keyRotatePolicy  string
	keyRotateForce   bool
	keyOldKeys       []string
	keyCurrentClient string
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage a project's encryption keys",
	Long:  `Inspect and manage the encryption keys for a project, including the key version history.`,
}

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	keyHistoryCmd := &cobra.Command{
		Use:   "history <project>",
		Short: "List the encryption key version history for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runKeyHistory,
	}

	keyRotateCmd := &cobra.Command{
		Use:   "rotate <project>",
		Short: "Rotate a SERVER-managed encryption key",
		Long: `Rotate a server-managed encryption key. The server generates a new random key
and re-encrypts the current secrets.

--history-policy controls what happens to backup versions: 'keep' (default,
per the project setting) leaves them under the old key so they stay restorable
with it; 're_encrypt' moves them onto the new key (use after a key exposure).

Client-managed keys cannot be rotated from the CLI: rotation requires
re-encrypting every secret client-side against secret IDs the public API does
not expose. Use the web dashboard's rotation flow instead.`,
		Args: cobra.ExactArgs(1),
		RunE: runKeyRotate,
	}
	keyRotateCmd.Flags().StringVar(&keyRotatePolicy, "history-policy", "", "what happens to backup versions: keep | re_encrypt (default: project setting)")
	keyRotateCmd.Flags().BoolVarP(&keyRotateForce, "force", "f", false, "skip the confirmation prompt")

	keyReencryptCmd := &cobra.Command{
		Use:   "re-encrypt-history <project>",
		Short: "Re-encrypt historical backup versions onto the current client-managed key",
		Long: `Move backup versions still encrypted under OLD client-managed keys onto the
project's current key. Versions are decrypted locally with the old key(s) and
re-encrypted locally with the current key — plaintext never leaves this machine.

Old keys are supplied with --old-key (file or value, repeatable) or entered at
the prompt; each is verified against its stored proof before use. The loop is
resumable: re-run the command to continue where it stopped.

Server-managed projects do not need this command — the server re-encrypts
history itself when a rotation runs with --history-policy=re_encrypt.`,
		Args: cobra.ExactArgs(1),
		RunE: runKeyReencryptHistory,
	}
	keyReencryptCmd.Flags().StringArrayVar(&keyOldKeys, "old-key", nil, "old encryption key (file or value; repeatable)")
	keyReencryptCmd.Flags().StringVar(&keyCurrentClient, "client-key", "", "path to the CURRENT client encryption key file, or the key value itself")

	keyCmd.AddCommand(keyHistoryCmd, keyRotateCmd, keyReencryptCmd)
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
		return versionsNotFoundHint(err)
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

func runKeyRotate(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project := args[0]

	if keyRotatePolicy != "" && keyRotatePolicy != "keep" && keyRotatePolicy != "re_encrypt" {
		return fmt.Errorf("invalid --history-policy %q: use keep or re_encrypt", keyRotatePolicy)
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if projectIsClientManaged(cmd.Context(), client, project) {
		return fmt.Errorf(
			"project %q uses CLIENT-managed encryption; rotation re-encrypts every secret "+
				"client-side against secret IDs the public API does not expose, so it must be "+
				"done from the web dashboard's rotation flow", project,
		)
	}

	if !keyRotateForce {
		warning := fmt.Sprintf("Rotate the encryption key for %q? The server generates a new key and re-encrypts the current secrets.", project)
		if keyRotatePolicy == "re_encrypt" {
			warning += " Backup versions will also be re-encrypted — recovering them with the OLD key will no longer be possible."
		}
		confirmed, confirmErr := ui.Confirm(warning, false)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ui.PrintInfof("Rotation canceled")
			return nil
		}
	}

	var opts *dotenv.RotateOptions
	if keyRotatePolicy != "" {
		opts = &dotenv.RotateOptions{HistoryPolicy: keyRotatePolicy}
	}

	newKey, resp, err := client.Encryption.RotateEncryptionKey(cmd.Context(), project, opts)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return HandleAPIError(err, accountForErrorContext())
	}

	ui.PrintSuccessf("Encryption key rotated; the active key is now v%d", newKey.Version)
	return nil
}

func runKeyReencryptHistory(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	project := args[0]
	ctx := cmd.Context()

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Current key: client-managed projects resolve it locally; server-managed
	// projects don't need this command at all.
	if !projectIsClientManaged(ctx, client, project) {
		return fmt.Errorf(
			"project %q is SERVER-managed; the server re-encrypts history itself — rotate "+
				"with 'dotenv key rotate %s --history-policy=re_encrypt' instead", project, project,
		)
	}

	current, err := resolveEncryptionKey(ctx, client, project, keyCurrentClient, promptForClientKey)
	if err != nil {
		return err
	}
	currentDataKey, err := current.dataKey()
	if err != nil {
		return err
	}
	currentProof, err := dotenv.DeriveKeyProof(current.key, current.proofSalt, current.proofIters)
	if err != nil {
		return fmt.Errorf("failed to derive the current key proof: %w", err)
	}

	history, histResp, err := client.Encryption.GetKeyHistory(ctx, project)
	if histResp != nil {
		defer histResp.Body.Close()
	}
	if err != nil {
		return versionsNotFoundHint(err)
	}
	byVersion := map[string]*dotenv.EncryptionKeyVersion{}
	for _, k := range history {
		byVersion[k.Version] = k
	}

	// Old keys are resolved (and proof-verified) once per key version.
	resolvedOld := map[string]string{}

	totalUpdated := 0
	for {
		pending, remaining, resp, perr := client.Encryption.ListPendingReencrypt(ctx, project, 100)
		if resp != nil {
			resp.Body.Close()
		}
		if perr != nil {
			return versionsNotFoundHint(perr)
		}
		if len(pending) == 0 {
			break
		}

		batch := make([]dotenv.ReencryptedVersion, 0, len(pending))
		for _, p := range pending {
			oldKeyValue, ok := resolvedOld[p.KeyVersion]
			if !ok {
				hist := byVersion[p.KeyVersion]
				oldKeyValue, perr = resolveOldClientKey(p.KeyVersion, hist, keyOldKeys, promptForClientKey)
				if perr != nil {
					return perr
				}
				resolvedOld[p.KeyVersion] = oldKeyValue
			}

			hist := byVersion[p.KeyVersion]
			var plaintext string
			if hist != nil && hist.KeyCheckSalt != "" {
				oldData, derr := dotenv.DeriveDataKey(oldKeyValue, hist.KeyCheckSalt, hist.KeyCheckIterations)
				if derr != nil {
					return fmt.Errorf("failed to derive old data key v%s: %w", p.KeyVersion, derr)
				}
				plaintext, derr = dotenv.Decrypt(p.Content, oldData)
				if derr != nil {
					ui.PrintWarningf("version %d could not be decrypted with key v%s — skipping", p.ID, p.KeyVersion)
					continue
				}
			} else {
				var derr error
				plaintext, derr = dotenv.Decrypt(p.Content, []byte(oldKeyValue))
				if derr != nil {
					ui.PrintWarningf("version %d could not be decrypted with key v%s — skipping", p.ID, p.KeyVersion)
					continue
				}
			}

			cipher, eerr := dotenv.Encrypt(plaintext, []byte(currentDataKey))
			if eerr != nil {
				return fmt.Errorf("failed to re-encrypt version %d: %w", p.ID, eerr)
			}
			batch = append(batch, dotenv.ReencryptedVersion{ID: p.ID, Content: cipher})
		}

		if len(batch) == 0 {
			return fmt.Errorf("%d version(s) remain but none could be decrypted with the supplied keys; provide the right --old-key values", remaining)
		}

		updated, left, subResp, serr := client.Encryption.SubmitReencryptedHistory(ctx, project, batch, currentProof)
		if subResp != nil {
			subResp.Body.Close()
		}
		if serr != nil {
			return HandleAPIError(serr, accountForErrorContext())
		}

		totalUpdated += updated
		ui.PrintInfof("Re-encrypted %d version(s), %d remaining", totalUpdated, left)

		if left == 0 {
			break
		}
	}

	ui.PrintSuccessf("History re-encryption complete (%d version(s) updated)", totalUpdated)
	return nil
}
