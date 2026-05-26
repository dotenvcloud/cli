package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/crypto"
	"github.com/dotenv/cli/internal/crypto/key"
	"github.com/dotenv/cli/internal/formats/env"
	"github.com/dotenv/cli/internal/formats/interpolation"
	jsonformat "github.com/dotenv/cli/internal/formats/json"
	"github.com/dotenv/cli/internal/formats/yaml"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

var (
	pullOutput    string
	pullResolve   bool
	pullFormat    string
	pullClientKey string
	pullDecrypt   bool
	pullQuiet     bool
	pullMerge     bool
	pullLevelOnly bool
)

var pullCmd = &cobra.Command{
	Use:   "pull [project]/[target]/[environment]",
	Short: "Pull secrets from DotEnv",
	Long: `Pull secrets from DotEnv with support for hierarchical inheritance
and client-side encryption.

The hierarchy follows the pattern:
  project -> target -> environment

Variables are inherited from parent levels and can be overridden
at more specific levels.`,

	Example: `  # Pull all secrets for a project
  dotenv pull myproject

  # Pull secrets for specific environment
  dotenv pull myproject/production/api

  # Output to file
  dotenv pull myproject/staging --output=.env

  # Pull with client-side decryption
  dotenv pull myproject --client-key=./encryption.key

  # Resolve variable interpolation
  dotenv pull myproject --resolve

  # Export as JSON
  dotenv pull myproject --format=json`,

	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		// Try to refresh organizations if needed
		if err := RefreshOrganizationsIfNeeded(cmd.Context()); err != nil {
			// Don't fail the command, just warn
			ui.PrintWarningf("Could not refresh organizations: %v", err)
		}
		return nil
	},
	RunE: runPull,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	pullCmd.Flags().StringVarP(&pullOutput, "output", "o", "",
		"output to file instead of stdout")
	pullCmd.Flags().BoolVarP(&pullResolve, "resolve", "r", false,
		"resolve variable interpolation")
	pullCmd.Flags().StringVarP(&pullFormat, "format", "f", "env",
		"output format (env, json, yaml, shell, dockerfile)")
	pullCmd.Flags().StringVar(&pullClientKey, "client-key", "",
		"path to client encryption key file")
	pullCmd.Flags().BoolVar(&pullDecrypt, "decrypt", true,
		"decrypt secrets (disable for raw encrypted values)")
	pullCmd.Flags().BoolVarP(&pullQuiet, "quiet", "q", false,
		"suppress output (exit code only)")
	pullCmd.Flags().BoolVarP(&pullMerge, "merge", "m", true,
		"merge secrets from all hierarchy levels (default: true)")
	pullCmd.Flags().BoolVar(&pullLevelOnly, "level-only", false,
		"only show secrets from the specified level, not merged with parent levels")
}

func runPull(cmd *cobra.Command, args []string) error {
	if pullLevelOnly {
		pullMerge = false
	}

	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			ui.PrintWarningf("Could not display account info: %v", err)
		}
	}

	projectSlug, targetSlug, environmentSlug, err := parsePullPath(args[0])
	if err != nil {
		return err
	}

	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !pullQuiet {
		ui.PrintInfof("Pulling secrets from %s...", args[0])
	}

	secrets, err := fetchAndProcessSecrets(cmd.Context(), client, projectSlug, targetSlug, environmentSlug)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		if !pullQuiet {
			ui.PrintWarningf("No secrets found")
		}
		return nil
	}

	if pullResolve {
		secrets = resolveInterpolation(secrets)
	}

	output, err := formatSecrets(secrets, pullFormat)
	if err != nil {
		return fmt.Errorf("failed to format secrets as %s: %w", pullFormat, err)
	}

	return writePullOutput(cmd, output)
}

func parsePullPath(path string) (projectSlug, targetSlug, environmentSlug string, err error) {
	parts := strings.Split(path, "/")
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

func fetchAndProcessSecrets(
	ctx context.Context, client *dotenv.Client,
	projectSlug, targetSlug, environmentSlug string,
) (map[string]string, error) {
	hierarchyResp, hResp, err := client.Secrets.GetProjectSecrets(ctx, projectSlug, targetSlug, environmentSlug)
	if hResp != nil {
		defer hResp.Body.Close()
	}
	if err != nil {
		return nil, HandleAPIError(err, accountForErrorContext())
	}
	if hierarchyResp == nil || len(hierarchyResp.Data.Attributes.Levels) == 0 {
		return nil, nil
	}
	return processHierarchicalSecrets(ctx, hierarchyResp, pullMerge, pullDecrypt, pullClientKey, projectSlug, client)
}

func resolveInterpolation(secrets map[string]string) map[string]string {
	interpolator := interpolation.NewInterpolator(secrets, nil)
	resolved, interpErr := interpolator.InterpolateMap(secrets)
	if interpErr != nil {
		if !pullQuiet {
			ui.PrintWarningf("Failed to resolve some variables: %v", interpErr)
		}
		return secrets
	}
	return resolved
}

func writePullOutput(cmd *cobra.Command, output string) error {
	if pullOutput == "" {
		if !pullQuiet {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out)
			fmt.Fprint(out, output)
		}
		return nil
	}

	dir := filepath.Dir(pullOutput)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	if err := backupExistingFile(pullOutput); err != nil {
		return err
	}

	if err := os.WriteFile(pullOutput, []byte(output), 0o600); err != nil {
		return fmt.Errorf("failed to write secrets to file '%s': %w", pullOutput, err)
	}
	if !pullQuiet {
		ui.PrintSuccessf("Secrets written to %s", pullOutput)
	}
	return nil
}

func backupExistingFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	backupPath := path + ".backup"
	if !pullQuiet {
		confirm, err := ui.Confirm(fmt.Sprintf("File %s exists. Create backup at %s?", path, backupPath), true)
		if err != nil {
			return fmt.Errorf("failed to get user confirmation for backup: %w", err)
		}
		if !confirm {
			return fmt.Errorf("operation canceled by user")
		}
	}
	existingData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing file '%s' for backup: %w", path, err)
	}
	if err := os.WriteFile(backupPath, existingData, 0o600); err != nil {
		return fmt.Errorf("failed to create backup file '%s': %w", backupPath, err)
	}
	if !pullQuiet {
		ui.PrintInfof("Created backup at %s", backupPath)
	}
	return nil
}

// processHierarchicalSecrets processes the hierarchical response from the API
func processHierarchicalSecrets(
	ctx context.Context,
	resp *dotenv.SecretsHierarchyResponse,
	merge, decrypt bool,
	clientKeyPath, projectSlug string,
	client *dotenv.Client,
) (map[string]string, error) {
	// Get encryption key if decryption is requested AND at least one level
	// actually carries encrypted content. Plaintext-only responses don't need
	// a server round-trip for the key.
	needsKey := false
	if resp != nil {
		for _, lvl := range resp.Data.Attributes.Levels {
			if lvl.Encrypted {
				needsKey = true
				break
			}
		}
	}

	var encKey []byte
	if decrypt && needsKey {
		var err error
		encKey, err = getEncryptionKey(ctx, clientKeyPath, projectSlug, client)
		if err != nil {
			// If this is a client-managed key project and no key was provided, prompt for it
			if err == ErrClientManagedKey && clientKeyPath == "" {
				ui.PrintInfof("This project uses client-managed encryption. Please provide your encryption key.")
				encKey, err = promptForClientKey()
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	// Determine target level for non-merge operations
	targetLevel := ""
	if !merge {
		targetLevel = determineTargetLevel(resp.Meta.Hierarchy)
	}

	// Process levels and merge secrets
	return processSecretLevels(resp, merge, decrypt, encKey, targetLevel)
}

// getEncryptionKey retrieves the encryption key from client file or server
func getEncryptionKey(ctx context.Context, clientKeyPath, projectSlug string, client *dotenv.Client) ([]byte, error) {
	if clientKeyPath != "" {
		// Use client-provided key
		keyData, err := os.ReadFile(clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key from %s: %w", clientKeyPath, err)
		}

		encKey, err := key.ParseKey(string(keyData))
		if err != nil {
			return nil, fmt.Errorf("invalid client key format in %s: %w", clientKeyPath, err)
		}
		return encKey, nil
	}

	// Get encryption key from server
	encKeyResp, encResp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if encResp != nil {
		defer encResp.Body.Close()
	}
	if err != nil {
		// SDK now exposes a typed sentinel for the client-managed envelope
		// (F-19); prefer that over HTTP-400 sniffing.
		if errors.Is(err, dotenv.ErrClientManagedEncryption) {
			return nil, ErrClientManagedKey
		}

		// Provide specific error for encryption key failures
		if dotenv.IsNotFound(err) {
			return nil, fmt.Errorf("encryption key not found for project '%s'. The project may not have encryption enabled", projectSlug)
		}
		account := accountForErrorContext()
		return nil, HandleAPIError(err, account)
	}

	encKey, err := key.ParseKey(encKeyResp.Key)
	if err != nil {
		return nil, fmt.Errorf("server provided invalid encryption key format: %w", err)
	}
	return encKey, nil
}

// determineTargetLevel determines which level to use when not merging
func determineTargetLevel(hierarchy struct {
	Project     string  `json:"project"`
	Target      *string `json:"target"`
	Environment *string `json:"environment"`
}) string {
	if hierarchy.Environment != nil && *hierarchy.Environment != "" {
		return "environment"
	}
	if hierarchy.Target != nil && *hierarchy.Target != "" {
		return "target"
	}
	return "project"
}

// processSecretLevels processes each level and returns merged or single-level secrets
func processSecretLevels(
	resp *dotenv.SecretsHierarchyResponse,
	merge, decrypt bool,
	encKey []byte,
	targetLevel string,
) (map[string]string, error) {
	allSecrets := make(map[string]string)

	// Order matters for merging: project -> target -> environment
	levelOrder := []string{"project", "target", "environment"}

	for _, levelName := range levelOrder {
		level, exists := resp.Data.Attributes.Levels[levelName]
		if !exists || level.Content == "" {
			continue
		}

		// Skip this level if we're only getting a specific level
		if !merge && levelName != targetLevel {
			continue
		}

		// Process this level
		levelSecrets, err := processLevel(levelName, level, decrypt, encKey, resp.Data.Attributes.Format)
		if err != nil {
			// Log warning but continue processing other levels
			ui.PrintWarningf("Failed to process %s level: %v", levelName, err)
			continue
		}

		if merge {
			// Merge with existing secrets (later levels override earlier ones)
			for k, v := range levelSecrets {
				allSecrets[k] = v
			}
		} else {
			// Just use this level's secrets
			allSecrets = levelSecrets
		}
	}

	return allSecrets, nil
}

// processLevel processes a single level and returns its secrets
func processLevel(levelName string, level dotenv.SecretLevel, decrypt bool, encKey []byte, format string) (map[string]string, error) {
	content := level.Content

	// Decrypt if necessary
	if decrypt && level.Encrypted {
		if len(encKey) == 0 {
			return nil, fmt.Errorf("cannot decrypt %s level: encryption key not provided", levelName)
		}

		decrypted, err := crypto.DecryptString(content, encKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt %s level secrets: %w", levelName, err)
		}
		content = decrypted
	}

	// Parse the content
	return parseSecretContent(content, format)
}

// parseSecretContent parses the decrypted content based on format
func parseSecretContent(content, format string) (map[string]string, error) {
	switch format {
	case formatENV, "":
		// Use the existing env parser for proper .env format handling
		parser := env.NewParser(nil)
		return parser.Parse(strings.NewReader(content))
	case formatJSON:
		// Parse JSON format
		var jsonSecrets map[string]string
		if err := json.Unmarshal([]byte(content), &jsonSecrets); err != nil {
			return nil, fmt.Errorf("invalid JSON format in secrets: %w", err)
		}
		return jsonSecrets, nil
	default:
		return nil, fmt.Errorf("unsupported secret format '%s': expected 'env' or 'json'", format)
	}
}

// promptForClientKey prompts the user to enter their encryption key
func promptForClientKey() ([]byte, error) {
	keyStr, err := ui.Password("Enter encryption key")
	if err != nil {
		return nil, fmt.Errorf("failed to read encryption key: %w", err)
	}

	if keyStr == "" {
		return nil, fmt.Errorf("encryption key cannot be empty")
	}

	encKey, err := key.ParseKey(keyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid key format: %w", err)
	}

	return encKey, nil
}

func formatSecrets(secrets map[string]string, format string) (string, error) {
	switch strings.ToLower(format) {
	case formatENV:
		gen := env.NewGenerator(nil)
		return gen.GenerateString(secrets)

	case formatJSON:
		gen := jsonformat.NewHandler(nil)
		return gen.GenerateString(secrets)

	case formatYAML, "yml":
		gen := yaml.NewHandler(nil)
		return gen.GenerateString(secrets)

	case "shell":
		// Sort keys for consistent output
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var lines []string
		for _, k := range keys {
			v := secrets[k]
			// Properly escape for shell
			escaped := strings.ReplaceAll(v, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, `$`, `\$`)
			escaped = strings.ReplaceAll(escaped, "`", "\\`")
			lines = append(lines, `export `+k+`="`+escaped+`"`)
		}
		return strings.Join(lines, "\n") + "\n", nil

	case "dockerfile":
		// Sort keys for consistent output
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var lines []string
		for _, k := range keys {
			v := secrets[k]
			// Escape quotes for Dockerfile
			escaped := strings.ReplaceAll(v, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, `\`, `\\`)
			lines = append(lines, `ENV `+k+`="`+escaped+`"`)
		}
		return strings.Join(lines, "\n") + "\n", nil

	default:
		return "", fmt.Errorf("unsupported output format '%s': valid formats are env, json, yaml, shell, dockerfile", format)
	}
}
