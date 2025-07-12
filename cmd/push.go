package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/crypto"
	"github.com/dotenv/cli/internal/crypto/key"
	"github.com/dotenv/cli/internal/formats"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
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
	// Display account/org info unless using API key override
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			// Don't fail if we can't display account info
			ui.PrintWarning("Could not display account info: %v", err)
		}
	}

	// Parse arguments
	path := args[0]
	parts := strings.Split(path, "/")

	var projectSlug, targetSlug, environmentSlug string
	var singleFile string

	// Determine mode: single file or multi-file
	if len(args) == 2 {
		// Single file mode
		singleFile = args[1]

		switch len(parts) {
		case 1:
			projectSlug = parts[0]
		case 2:
			projectSlug = parts[0]
			targetSlug = parts[1]
		case 3:
			projectSlug = parts[0]
			targetSlug = parts[1]
			environmentSlug = parts[2]
		default:
			return fmt.Errorf("invalid path format: use project[/target[/environment]]")
		}
	} else {
		// Multi-file mode
		if len(parts) != 1 {
			return fmt.Errorf("in multi-file mode, specify only the project name (got: %s)", path)
		}
		projectSlug = parts[0]

		if pushProject == "" && pushTarget == "" && pushEnvironment == "" {
			return fmt.Errorf("no files specified: use --project, --target, or --env flags to specify .env files")
		}
	}

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	// Verify project exists
	project, resp, err := client.Projects.Get(context.Background(), projectSlug)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return fmt.Errorf("project '%s' not found in organization", projectSlug)
		}
		return fmt.Errorf("failed to verify project '%s' exists: %w", projectSlug, err)
	}

	// Get encryption key if encrypting
	var encKey []byte
	if pushEncrypt {
		if pushClientKey != "" {
			// Use client-provided key
			keyData, err := os.ReadFile(pushClientKey)
			if err != nil {
				return fmt.Errorf("failed to read client key: %w", err)
			}

			encKey, err = key.ParseKey(string(keyData))
			if err != nil {
				return fmt.Errorf("failed to parse client key: %w", err)
			}
		} else {
			// Get key from server
			encKeyResp, _, err := client.Encryption.GetEncryptionKey(context.Background(), projectSlug)
			if err != nil {
				return fmt.Errorf("failed to get encryption key: %w", err)
			}

			encKey, err = key.ParseKey(encKeyResp.Key)
			if err != nil {
				return fmt.Errorf("failed to parse encryption key: %w", err)
			}
		}
	}

	// Process files
	if singleFile != "" {
		// Single file mode
		return pushSingleFile(context.Background(), client, project, targetSlug, environmentSlug,
			singleFile, encKey, pushForce)
	} else {
		// Multi-file mode
		return pushMultipleFiles(context.Background(), client, project, pushProject, pushTarget,
			pushEnvironment, encKey, pushForce)
	}
}

func pushSingleFile(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	targetSlug, environmentSlug, filename string, encKey []byte, force bool) error {

	ui.PrintInfo("Reading secrets from %s...", filename)

	// Detect format and parse
	secrets, err := parseSecretsFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	ui.PrintInfo("Found %d secrets", len(secrets))

	// Encrypt if requested
	if encKey != nil {
		encrypted, err := encryptSecretsMap(secrets, encKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt secrets: %w", err)
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
		existing, _, err := client.Secrets.List(ctx, project.Slug, nil)
		if err == nil && len(existing) > 0 {
			ui.PrintWarning("Project already has %d secrets", len(existing))

			overwrite, err := ui.Confirm("Overwrite existing secrets?", false)
			if err != nil {
				return err
			}

			if !overwrite {
				ui.PrintInfo("Push cancelled")
				return nil
			}
		}
	}

	// Push secrets
	ui.PrintInfo("Pushing secrets...")

	created, _, err := client.Secrets.BulkCreate(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push secrets: %w", err)
	}

	ui.PrintSuccess("Successfully pushed %d secrets", len(created))

	return nil
}

func pushMultipleFiles(ctx context.Context, client *dotenv.Client, project *dotenv.Project,
	projectFile, targetFile, envFile string, encKey []byte, force bool) error {

	type secretSet struct {
		level  string
		file   string
		slug   string
		target string
		env    string
	}

	sets := []secretSet{}

	if projectFile != "" {
		sets = append(sets, secretSet{
			level: "project",
			file:  projectFile,
			slug:  project.Slug,
		})
	}

	if targetFile != "" {
		// Need to specify target
		targets, _, err := client.Targets.List(ctx, project.Slug, nil)
		if err != nil {
			return fmt.Errorf("failed to list targets: %w", err)
		}

		if len(targets) == 0 {
			return fmt.Errorf("no targets found in project")
		}

		targetNames := make([]string, len(targets))
		for i, t := range targets {
			targetNames[i] = fmt.Sprintf("%s - %s", t.Name, t.Slug)
		}

		selected, err := ui.Select("Select target for "+targetFile, targetNames)
		if err != nil {
			return err
		}

		var targetSlug string
		for _, t := range targets {
			if strings.Contains(selected, t.Slug) {
				targetSlug = t.Slug
				break
			}
		}

		sets = append(sets, secretSet{
			level:  "target",
			file:   targetFile,
			slug:   project.Slug,
			target: targetSlug,
		})
	}

	if envFile != "" {
		// Need to specify target and environment
		targets, _, err := client.Targets.List(ctx, project.Slug, nil)
		if err != nil {
			return fmt.Errorf("failed to list targets: %w", err)
		}

		if len(targets) == 0 {
			return fmt.Errorf("no targets found in project")
		}

		targetNames := make([]string, len(targets))
		for i, t := range targets {
			targetNames[i] = fmt.Sprintf("%s - %s", t.Name, t.Slug)
		}

		selectedTarget, err := ui.Select("Select target for "+envFile, targetNames)
		if err != nil {
			return err
		}

		var targetSlug string
		for _, t := range targets {
			if strings.Contains(selectedTarget, t.Slug) {
				targetSlug = t.Slug
				break
			}
		}

		// Now get environments for this target
		envs, _, err := client.Environments.List(ctx, project.Slug, targetSlug, nil)
		if err != nil {
			return fmt.Errorf("failed to list environments: %w", err)
		}

		if len(envs) == 0 {
			return fmt.Errorf("no environments found in target")
		}

		envNames := make([]string, len(envs))
		for i, e := range envs {
			envNames[i] = fmt.Sprintf("%s - %s", e.Name, e.Slug)
		}

		selectedEnv, err := ui.Select("Select environment for "+envFile, envNames)
		if err != nil {
			return err
		}

		var envSlug string
		for _, e := range envs {
			if strings.Contains(selectedEnv, e.Slug) {
				envSlug = e.Slug
				break
			}
		}

		sets = append(sets, secretSet{
			level:  "environment",
			file:   envFile,
			slug:   project.Slug,
			target: targetSlug,
			env:    envSlug,
		})
	}

	// Process each file
	totalSecrets := 0

	for _, set := range sets {
		ui.PrintInfo("Processing %s-level secrets from %s...", set.level, set.file)

		secrets, err := parseSecretsFile(set.file)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", set.file, err)
		}

		ui.PrintInfo("Found %d secrets", len(secrets))

		// Encrypt if requested
		if encKey != nil {
			encrypted, err := encryptSecretsMap(secrets, encKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt secrets: %w", err)
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
			if set.target != "" {
				item.TargetSlug = &set.target
			}
			if set.env != "" {
				item.EnvironmentSlug = &set.env
			}
			bulkSecrets = append(bulkSecrets, item)
		}

		req := &dotenv.BulkSecretsRequest{
			ProjectSlug: set.slug,
			Secrets:     bulkSecrets,
		}

		// Push
		created, _, err := client.Secrets.BulkCreate(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to push %s-level secrets: %w", set.level, err)
		}

		totalSecrets += len(created)
	}

	ui.PrintSuccess("Successfully pushed %d total secrets", totalSecrets)

	return nil
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
		content, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		format, err = detector.DetectFormat(content)
		if err != nil {
			// Default to env format
			format = "env"
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
