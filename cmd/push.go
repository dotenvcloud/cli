package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenv/cli/internal/crypto"
	"github.com/dotenv/cli/internal/crypto/key"
	"github.com/dotenv/cli/internal/formats"
	"github.com/dotenv/cli/internal/ui"
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

// encryptSecretsMap encrypts each value in a map
func encryptSecretsMap(secrets map[string]string, key []byte) (map[string]string, error) {
	encryptor := crypto.NewGCMEncryptor()
	encrypted := make(map[string]string)

	for k, v := range secrets {
		enc, err := encryptor.Encrypt([]byte(v), key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt %s: %w", k, err)
		}
		encrypted[k] = enc
	}

	return encrypted, nil
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
		"path to client encryption key file")
	pushCmd.Flags().BoolVar(&pushEncrypt, "encrypt", true,
		"encrypt secrets before pushing")
}

func runPush(cmd *cobra.Command, args []string) error {
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

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

	encKey, err := resolvePushEncryptionKey(cmd.Context(), client, projectSlug)
	if err != nil {
		return err
	}

	if singleFile != "" {
		return pushSingleFile(cmd.Context(), client, project, targetSlug, environmentSlug,
			singleFile, encKey, pushForce)
	}
	return pushMultipleFiles(cmd.Context(), client, project, pushProject, pushTarget,
		pushEnvironment, encKey, pushForce)
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

func resolvePushEncryptionKey(ctx context.Context, client *dotenv.Client, projectSlug string) ([]byte, error) {
	if !pushEncrypt {
		return nil, nil
	}
	if pushClientKey != "" {
		keyData, err := os.ReadFile(pushClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key: %w", err)
		}
		encKey, err := key.ParseKey(string(keyData))
		if err != nil {
			return nil, fmt.Errorf("failed to parse client key: %w", err)
		}
		return encKey, nil
	}
	encKeyResp, encResp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if encResp != nil {
		defer encResp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	encKey, err := key.ParseKey(encKeyResp.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse encryption key: %w", err)
	}
	return encKey, nil
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
	targetSlug, environmentSlug, filename string, encKey []byte, force bool) error {
	ui.PrintInfof("Reading secrets from %s...", filename)

	// Detect format and parse
	secrets, err := parseSecretsFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	ui.PrintInfof("Found %d secrets", len(secrets))

	// Encrypt if requested
	if encKey != nil {
		encrypted, encErr := encryptSecretsMap(secrets, encKey)
		if encErr != nil {
			return fmt.Errorf("failed to encrypt secrets: %w", encErr)
		}
		secrets = encrypted
	}

	// Build request - convert map to slice of BulkSecretItem
	bulkSecrets := make([]dotenv.BulkSecretItem, 0, len(secrets))
	for key, value := range secrets {
		item := dotenv.BulkSecretItem{
			Key:         key,
			Value:       value,
			IsEncrypted: encKey != nil,
		}
		if targetSlug != "" {
			item.TargetSlug = &targetSlug
		}
		if environmentSlug != "" {
			item.EnvironmentSlug = &environmentSlug
		}
		bulkSecrets = append(bulkSecrets, item)
	}

	req := &dotenv.BulkSecretsRequest{
		ProjectSlug: project.Slug,
		Secrets:     bulkSecrets,
	}

	// Check existing secrets if not forcing
	if !force {
		existing, existResp, listErr := client.Secrets.List(ctx, project.Slug, nil)
		if existResp != nil {
			defer existResp.Body.Close()
		}
		if listErr == nil && len(existing) > 0 {
			ui.PrintWarningf("Project already has %d secrets", len(existing))

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

	// Push secrets
	ui.PrintInfof("Pushing secrets...")

	created, bulkResp, err := client.Secrets.BulkCreate(ctx, req)
	if bulkResp != nil {
		defer bulkResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to push secrets: %w", err)
	}

	ui.PrintSuccessf("Successfully pushed %d secrets", len(created))

	return nil
}

type secretSet struct {
	level  string
	file   string
	slug   string
	target string
	env    string
}

func pushMultipleFiles(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	projectFile, targetFile, envFile string, encKey []byte, _ bool) error {
	sets, err := buildSecretSets(ctx, client, project, projectFile, targetFile, envFile)
	if err != nil {
		return err
	}

	totalSecrets := 0
	for i := range sets {
		count, err := processSecretSet(ctx, client, &sets[i], encKey)
		if err != nil {
			return err
		}
		totalSecrets += count
	}

	ui.PrintSuccessf("Successfully pushed %d total secrets", totalSecrets)
	return nil
}

func buildSecretSets(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	projectFile, targetFile, envFile string) ([]secretSet, error) {
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

func processSecretSet(ctx context.Context, client *dotenv.Client, set *secretSet, encKey []byte) (int, error) {
	ui.PrintInfof("Processing %s-level secrets from %s...", set.level, set.file)

	secrets, err := parseSecretsFile(set.file)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s: %w", set.file, err)
	}
	ui.PrintInfof("Found %d secrets", len(secrets))

	if encKey != nil {
		encrypted, encErr := encryptSecretsMap(secrets, encKey)
		if encErr != nil {
			return 0, fmt.Errorf("failed to encrypt secrets: %w", encErr)
		}
		secrets = encrypted
	}

	bulkSecrets := buildBulkSecrets(secrets, set, encKey != nil)
	req := &dotenv.BulkSecretsRequest{ProjectSlug: set.slug, Secrets: bulkSecrets}

	created, bulkResp, err := client.Secrets.BulkCreate(ctx, req)
	if bulkResp != nil {
		defer bulkResp.Body.Close()
	}
	if err != nil {
		return 0, fmt.Errorf("failed to push %s-level secrets: %w", set.level, err)
	}
	return len(created), nil
}

func buildBulkSecrets(secrets map[string]string, set *secretSet, encrypted bool) []dotenv.BulkSecretItem {
	bulkSecrets := make([]dotenv.BulkSecretItem, 0, len(secrets))
	for k, value := range secrets {
		item := dotenv.BulkSecretItem{
			Key:         k,
			Value:       value,
			IsEncrypted: encrypted,
		}
		if set.target != "" {
			tgt := set.target
			item.TargetSlug = &tgt
		}
		if set.env != "" {
			env := set.env
			item.EnvironmentSlug = &env
		}
		bulkSecrets = append(bulkSecrets, item)
	}
	return bulkSecrets
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
