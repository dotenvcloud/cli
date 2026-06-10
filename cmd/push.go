package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/formats"
	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	pushProject     string
	pushTarget      string
	pushEnvironment string
	pushForce       bool
	pushClientKey   string
	pushEncrypt     bool
)

var pushCmd = &cobra.Command{
	Use:   "push [project]/[target]/[environment] [file]",
	Short: "Push secrets to DotEnv",
	Long: `Push secrets to DotEnv from local files with support for
client-side encryption.

The hierarchy follows the pattern:
  project -> target -> environment

Secrets are stored as one encrypted .env blob per level (the inverse of pull).
You can push to any level of the hierarchy.`,

	Example: `  # Push .env file to specific environment
  dotenv push myproject/production/api .env

  # Push to project level (applies to all targets/environments)
  dotenv push myproject .env.defaults

  # Push multiple files with hierarchy
  dotenv push myproject --project=.env.project --target=.env.target --env=.env.env

  # Push with client-side encryption
  dotenv push myproject/staging .env --client-key=./encryption.key

  # Force overwrite existing secrets
  dotenv push myproject/staging .env --force`,

	Args: cobra.RangeArgs(1, 2),
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		// Try to refresh organizations if needed
		if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
			// Don't fail the command, just warn
			ui.PrintWarningf("Could not refresh organizations: %v", err)
		}
		return nil
	},
	RunE: runPush,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	pushCmd.Flags().StringVar(&pushProject, "project", "",
		"file containing project-level secrets")
	pushCmd.Flags().StringVar(&pushTarget, "target", "",
		"file containing target-level secrets")
	pushCmd.Flags().StringVar(&pushEnvironment, "env", "",
		"file containing environment-level secrets")
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false,
		"overwrite existing secrets")
	pushCmd.Flags().StringVar(&pushClientKey, "client-key", "",
		"path to a client encryption key file, or the key value itself")
	pushCmd.Flags().BoolVar(&pushEncrypt, "encrypt", true,
		"encrypt secrets before pushing")
}

func runPush(cmd *cobra.Command, args []string) error {
	printActiveIdentity()

	projectSlug, targetSlug, environmentSlug, singleFile, err := parsePushArgs(args)
	if err != nil {
		return err
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	project, resp, err := client.Projects.Get(cmd.Context(), projectSlug)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return fmt.Errorf("project '%s' not found in organization", projectSlug)
		}
		return fmt.Errorf("failed to verify project '%s' exists: %w", projectSlug, err)
	}

	encKey, keyProof, err := resolvePushEncryptionKey(cmd.Context(), client, projectSlug)
	if err != nil {
		return err
	}

	if singleFile != "" {
		return pushSingleFile(cmd.Context(), client, project, targetSlug, environmentSlug,
			singleFile, encKey, keyProof, pushForce)
	}
	return pushMultipleFiles(cmd.Context(), client, project, pushProject, pushTarget,
		pushEnvironment, encKey, keyProof, pushForce)
}

func parsePushArgs(args []string) (projectSlug, targetSlug, environmentSlug, singleFile string, err error) {
	path := args[0]
	parts := strings.Split(path, "/")

	if len(args) == 2 {
		singleFile = args[1]
		switch len(parts) {
		case 1:
			projectSlug = parts[0]
		case 2:
			projectSlug, targetSlug = parts[0], parts[1]
		case 3:
			projectSlug, targetSlug, environmentSlug = parts[0], parts[1], parts[2]
		default:
			err = fmt.Errorf("invalid path format: use project[/target[/environment]]")
		}
		return
	}

	if len(parts) != 1 {
		err = fmt.Errorf("in multi-file mode, specify only the project name (got: %s)", path)
		return
	}
	projectSlug = parts[0]
	if pushProject == "" && pushTarget == "" && pushEnvironment == "" {
		err = fmt.Errorf("no files specified: use --project, --target, or --env flags to specify .env files")
	}
	return
}

// resolvePushEncryptionKey returns the project key as a RAW STRING (see
// dotenv.DeriveProjectKey) — never hex/base64-decoded — and, for client-managed
// projects, the base64 key proof to send with the write. An empty encKey means
// "do not encrypt". Key resolution (file/value/env/prompt) is shared with pull
// via resolveEncryptionKey; this wrapper adds push's --encrypt=false handling,
// the proof derivation, and a guard against silently uploading plaintext to a
// client-managed project.
func resolvePushEncryptionKey(ctx context.Context, client *dotenv.Client, projectSlug string) (encKey, keyProof string, err error) {
	if !pushEncrypt {
		if projectIsClientManaged(ctx, client, projectSlug) {
			ui.PrintWarningf("--encrypt=false on a client-managed project would upload PLAINTEXT secrets, defeating client-side encryption.")
			ok, confirmErr := ui.Confirm("Push plaintext anyway?", false)
			if confirmErr != nil {
				return "", "", fmt.Errorf("refusing to push plaintext to a client-managed project; remove --encrypt=false (or run interactively to confirm)")
			}
			if !ok {
				return "", "", fmt.Errorf("push canceled")
			}
		}
		return "", "", nil
	}

	rk, rerr := resolveEncryptionKey(ctx, client, projectSlug, pushClientKey, promptForClientKey)
	if rerr != nil {
		return "", "", rerr
	}

	// Client-managed: derive the proof the server verifies against its stored
	// proof. Without proof params the server has no verification configured.
	if rk.managed == "client" {
		if rk.proofSalt == "" {
			return "", "", fmt.Errorf(
				"this project's client-managed key has no verification configured; re-establish its encryption key (web key setup) or recreate it with `dotenv project create --storage client`",
			)
		}
		proof, perr := dotenv.DeriveKeyProof(rk.key, rk.proofSalt, rk.proofIters)
		if perr != nil {
			return "", "", fmt.Errorf("failed to derive key proof: %w", perr)
		}
		keyProof = proof
	}

	// For client-managed projects this returns the PBKDF2-derived AES key (not
	// the raw passphrase) so secrets are encrypted under a salted, stretched
	// key; server-managed projects return the server key unchanged.
	dk, derr := rk.dataKey()
	if derr != nil {
		return "", "", derr
	}

	return dk, keyProof, nil
}

// slugForLabel looks up the slug for the label the user picked. Exact-match
// avoids the substring bug where one slug was a prefix of another.
func slugForLabel(selected string, labels []string, slugAt func(int) string) (string, error) {
	for i, label := range labels {
		if label == selected {
			return slugAt(i), nil
		}
	}
	return "", fmt.Errorf("internal: selected label %q not found in options", selected)
}

func pushSingleFile(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	targetSlug, environmentSlug, filename string, encKey, keyProof string, force bool,
) error {
	return storeSecretLevel(ctx, client, project.Slug, targetSlug, environmentSlug, filename, encKey, keyProof, force)
}

type secretSet struct {
	level  string
	file   string
	slug   string
	target string
	env    string
}

func pushMultipleFiles(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	projectFile, targetFile, envFile string, encKey, keyProof string, force bool,
) error {
	sets, err := buildSecretSets(ctx, client, project, projectFile, targetFile, envFile)
	if err != nil {
		return err
	}

	for i := range sets {
		set := &sets[i]
		if err := storeSecretLevel(ctx, client, set.slug, set.target, set.env, set.file, encKey, keyProof, force); err != nil {
			return err
		}
	}

	ui.PrintSuccessf("Successfully pushed %d level(s)", len(sets))
	return nil
}

func buildSecretSets(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	projectFile, targetFile, envFile string,
) ([]secretSet, error) {
	var sets []secretSet

	if projectFile != "" {
		sets = append(sets, secretSet{level: "project", file: projectFile, slug: project.Slug})
	}

	if targetFile != "" {
		targetSlug, err := promptForTarget(ctx, client, project.Slug, "Select target for "+targetFile)
		if err != nil {
			return nil, err
		}
		sets = append(sets, secretSet{
			level: "target", file: targetFile, slug: project.Slug, target: targetSlug,
		})
	}

	if envFile != "" {
		targetSlug, err := promptForTarget(ctx, client, project.Slug, "Select target for "+envFile)
		if err != nil {
			return nil, err
		}
		envSlug, err := promptForEnvironment(ctx, client, project.Slug, targetSlug, "Select environment for "+envFile)
		if err != nil {
			return nil, err
		}
		sets = append(sets, secretSet{
			level: "environment", file: envFile, slug: project.Slug,
			target: targetSlug, env: envSlug,
		})
	}

	return sets, nil
}

func promptForTarget(ctx context.Context, client *dotenv.Client, projectSlug, prompt string) (string, error) {
	targets, resp, err := client.Targets.List(ctx, projectSlug, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("failed to list targets: %w", err)
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("no targets found in project")
	}

	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = fmt.Sprintf("%s - %s", t.Name, t.Slug)
	}
	selected, err := ui.Select(prompt, names)
	if err != nil {
		return "", err
	}
	return slugForLabel(selected, names, func(i int) string { return targets[i].Slug })
}

func promptForEnvironment(ctx context.Context, client *dotenv.Client, projectSlug, targetSlug, prompt string) (string, error) {
	envs, resp, err := client.Environments.List(ctx, projectSlug, targetSlug, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("failed to list environments: %w", err)
	}
	if len(envs) == 0 {
		return "", fmt.Errorf("no environments found in target")
	}

	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = fmt.Sprintf("%s - %s", e.Name, e.Slug)
	}
	selected, err := ui.Select(prompt, names)
	if err != nil {
		return "", err
	}
	return slugForLabel(selected, names, func(i int) string { return envs[i].Slug })
}

// storeSecretLevel pushes one file as the encrypted .env blob for a single
// level (project, target or environment — the deepest non-empty slug). The
// whole file content is encrypted as one blob, mirroring how the web stores
// secrets and how pull reads them back.
func storeSecretLevel(ctx context.Context, client *dotenv.Client,
	projectSlug, targetSlug, environmentSlug, filename, encKey, keyProof string, force bool,
) error {
	level := deepestLevel(targetSlug, environmentSlug)
	ui.PrintInfof("Reading %s-level secrets from %s...", level, filename)

	content, count, err := readFileBlob(filename)
	if err != nil {
		return err
	}
	ui.PrintInfof("Found %d secrets", count)

	if !force {
		exists, existErr := levelHasSecrets(ctx, client, projectSlug, targetSlug, environmentSlug)
		if existErr == nil && exists {
			ui.PrintWarningf("%s level already has secrets", level)
			overwrite, confirmErr := ui.Confirm("Overwrite existing secrets?", false)
			if confirmErr != nil {
				return confirmErr
			}
			if !overwrite {
				ui.PrintInfof("Push canceled")
				return nil
			}
		}
	}

	blob := content
	if encKey != "" {
		blob, err = dotenv.EncryptWithProjectKey(content, encKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt secrets: %w", err)
		}
	}

	ui.PrintInfof("Pushing secrets...")
	resp, err := client.Secrets.StoreSecrets(ctx, projectSlug, targetSlug, environmentSlug, blob, keyProof)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if errors.Is(err, dotenv.ErrKeyProofMismatch) {
			return fmt.Errorf("the encryption key does not match project '%s' — refusing to push secrets encrypted with a different key (a mistyped or wrong key would orphan this level). Use the project's established key", projectSlug)
		}
		if errors.Is(err, dotenv.ErrKeyProofRequired) {
			return fmt.Errorf("project '%s' has no client-key verification configured; re-establish its encryption key before pushing", projectSlug)
		}
		return fmt.Errorf("failed to push secrets: %w", err)
	}

	ui.PrintSuccessf("Successfully pushed %d secrets to %s level", count, level)
	return nil
}

// deepestLevel returns the level name implied by the provided slugs.
func deepestLevel(targetSlug, environmentSlug string) string {
	if environmentSlug != "" {
		return "environment"
	}
	if targetSlug != "" {
		return "target"
	}
	return "project"
}

// levelHasSecrets reports whether the deepest provided level already has a
// secrets blob stored.
func levelHasSecrets(ctx context.Context, client *dotenv.Client, projectSlug, targetSlug, environmentSlug string) (bool, error) {
	resp, hResp, err := client.Secrets.GetProjectSecrets(ctx, projectSlug, targetSlug, environmentSlug)
	if hResp != nil {
		defer hResp.Body.Close()
	}
	if err != nil || resp == nil {
		return false, err
	}
	lvl, ok := resp.Data.Attributes.Levels[deepestLevel(targetSlug, environmentSlug)]
	return ok && lvl.Content != "", nil
}

// readFileBlob reads the raw file content (the blob to store) and best-effort
// counts the keys for the progress message.
func readFileBlob(filename string) (content string, keyCount int, err error) {
	data, readErr := os.ReadFile(filename)
	if readErr != nil {
		return "", 0, fmt.Errorf("failed to read file %s: %w", filename, readErr)
	}
	if parsed, parseErr := parseSecretsFile(filename); parseErr == nil {
		keyCount = len(parsed)
	}
	return string(data), keyCount, nil
}

func parseSecretsFile(filename string) (map[string]string, error) {
	// Check if file exists
	if _, err := os.Stat(filename); err != nil {
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	// Detect format
	detector := formats.NewDetector()
	format, err := detector.DetectFormatFromFile(filename)
	if err != nil {
		// Try content detection
		content, readErr := os.ReadFile(filename)
		if readErr != nil {
			return nil, readErr
		}

		format, err = detector.DetectFormat(content)
		if err != nil {
			// Default to env format
			format = formatENV
		}
	}

	// Get appropriate parser
	handler, err := formats.DefaultRegistry.Get(format)
	if err != nil {
		return nil, err
	}

	// Parse file
	return handler.ParseFile(filename)
}
