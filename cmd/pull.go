package cmd

import (
	"context"
	"encoding/json"
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
	dotenv "github.com/dotenv/sdk-go"
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
	RunE: runPull,
}

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
	// If level-only is set, disable merge
	if pullLevelOnly {
		pullMerge = false
	}

	// Display account/org info unless using API key override
	if viper.GetString("api_key") == "" && os.Getenv("DOTENV_API_KEY") == "" {
		if err := displayAccountInfo(); err != nil {
			// Don't fail if we can't display account info
			ui.PrintWarning("Could not display account info: %v", err)
		}
	}

	// Parse hierarchy path
	path := args[0]
	parts := strings.Split(path, "/")

	var projectSlug, targetSlug, environmentSlug string

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

	// Get API client
	client, err := getAPIClient()
	if err != nil {
		return err
	}

	if !pullQuiet {
		ui.PrintInfo("Pulling secrets from %s...", path)
	}

	// Retrieve secrets using GetProjectSecrets
	hierarchyResp, _, err := client.Secrets.GetProjectSecrets(context.Background(), projectSlug, targetSlug, environmentSlug)
	if err != nil {
		// Get current account for better error messages
		account, _ := getCurrentAccount()
		return HandleAPIError(err, account)
	}

	// Check if we got any levels
	if hierarchyResp == nil || hierarchyResp.Data.Attributes.Levels == nil || len(hierarchyResp.Data.Attributes.Levels) == 0 {
		if !pullQuiet {
			ui.PrintWarning("No secrets found")
		}
		return nil
	}

	// Process the hierarchical response
	secrets, err := processHierarchicalSecrets(hierarchyResp, pullMerge, pullDecrypt, pullClientKey, projectSlug, client)
	if err != nil {
		return err
	}

	if len(secrets) == 0 {
		if !pullQuiet {
			ui.PrintWarning("No secrets found")
		}
		return nil
	}

	// Resolve interpolation if requested
	if pullResolve {
		interpolator := interpolation.NewInterpolator(secrets, nil)
		resolved, err := interpolator.InterpolateMap(secrets)
		if err != nil {
			if !pullQuiet {
				ui.PrintWarning("Failed to resolve some variables: %v", err)
			}
		} else {
			secrets = resolved
		}
	}

	// Format output
	output, err := formatSecrets(secrets, pullFormat)
	if err != nil {
		return fmt.Errorf("failed to format secrets as %s: %w", pullFormat, err)
	}

	// Write output
	if pullOutput != "" {
		// Ensure directory exists
		dir := filepath.Dir(pullOutput)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dir, err)
		}

		// Check if file exists and create backup
		if _, err := os.Stat(pullOutput); err == nil {
			// File exists, create backup
			backupPath := pullOutput + ".backup"
			
			// Ask for confirmation
			if !pullQuiet {
				confirm, err := ui.Confirm(fmt.Sprintf("File %s exists. Create backup at %s?", pullOutput, backupPath), true)
				if err != nil {
					return fmt.Errorf("failed to get user confirmation for backup: %w", err)
				}
				if !confirm {
					return fmt.Errorf("operation cancelled by user")
				}
			}
			
			// Copy existing file to backup
			existingData, err := os.ReadFile(pullOutput)
			if err != nil {
				return fmt.Errorf("failed to read existing file '%s' for backup: %w", pullOutput, err)
			}
			
			if err := os.WriteFile(backupPath, existingData, 0600); err != nil {
				return fmt.Errorf("failed to create backup file '%s': %w", backupPath, err)
			}
			
			if !pullQuiet {
				ui.PrintInfo("Created backup at %s", backupPath)
			}
		}

		// Write file
		if err := os.WriteFile(pullOutput, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write secrets to file '%s': %w", pullOutput, err)
		}

		if !pullQuiet {
			ui.PrintSuccess("Secrets written to %s", pullOutput)
		}
	} else if !pullQuiet {
		// Print to stdout
		fmt.Print(output)
	}

	return nil
}

// processHierarchicalSecrets processes the hierarchical response from the API
func processHierarchicalSecrets(resp *dotenv.SecretsHierarchyResponse, merge bool, decrypt bool, clientKeyPath string, projectSlug string, client *dotenv.Client) (map[string]string, error) {
	// Get encryption key if decryption is requested
	var encKey []byte
	if decrypt {
		var err error
		encKey, err = getEncryptionKey(clientKeyPath, projectSlug, client)
		if err != nil {
			// If this is a client-managed key project and no key was provided, prompt for it
			if err == ErrClientManagedKey && clientKeyPath == "" {
				ui.PrintInfo("This project uses client-managed encryption. Please provide your encryption key.")
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
func getEncryptionKey(clientKeyPath string, projectSlug string, client *dotenv.Client) ([]byte, error) {
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
	encKeyResp, resp, err := client.Encryption.GetEncryptionKey(context.Background(), projectSlug)
	if err != nil {
		// Check if it's a 400 error (likely client-managed key)
		if resp != nil && resp.StatusCode == 400 {
			return nil, ErrClientManagedKey
		}
		
		// Provide specific error for encryption key failures
		if dotenv.IsNotFound(err) {
			return nil, fmt.Errorf("encryption key not found for project '%s'. The project may not have encryption enabled", projectSlug)
		}
		account, _ := getCurrentAccount()
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
func processSecretLevels(resp *dotenv.SecretsHierarchyResponse, merge bool, decrypt bool, encKey []byte, targetLevel string) (map[string]string, error) {
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
			ui.PrintWarning("Failed to process %s level: %v", levelName, err)
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
func parseSecretContent(content string, format string) (map[string]string, error) {
	switch format {
	case "env", "":
		// Use the existing env parser for proper .env format handling
		parser := env.NewParser(nil)
		return parser.Parse(strings.NewReader(content))
	case "json":
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
	case "env":
		gen := env.NewGenerator(nil)
		return gen.GenerateString(secrets)

	case "json":
		gen := jsonformat.NewHandler(nil)
		return gen.GenerateString(secrets)

	case "yaml", "yml":
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
			lines = append(lines, fmt.Sprintf(`export %s="%s"`, k, escaped))
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
			lines = append(lines, fmt.Sprintf(`ENV %s="%s"`, k, escaped))
		}
		return strings.Join(lines, "\n") + "\n", nil

	default:
		return "", fmt.Errorf("unsupported output format '%s': valid formats are env, json, yaml, shell, dockerfile", format)
	}
}
