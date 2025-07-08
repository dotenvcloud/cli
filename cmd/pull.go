package cmd

import (
	"context"
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
	"github.com/dotenv/cli/internal/formats/json"
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
	pullRaw       bool
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
	pullCmd.Flags().BoolVarP(&pullMerge, "merge", "m", false,
		"merge secrets from all hierarchy levels")
	pullCmd.Flags().BoolVar(&pullRaw, "raw", false,
		"get raw secret values (requires --merge and --decrypt)")
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate flags: --raw requires --merge and --decrypt
	if pullRaw && (!pullMerge || !pullDecrypt) {
		return fmt.Errorf("--raw flag requires both --merge and --decrypt flags")
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

	// Build request
	req := dotenv.RetrieveParams{
		Project: projectSlug,
		Raw:     pullRaw,
	}

	if targetSlug != "" {
		req.Target = targetSlug
	}
	if environmentSlug != "" {
		req.Environment = environmentSlug
	}
	
	// Set merge action if requested
	if pullMerge {
		req.Merge = "deep"
	}

	if !pullQuiet {
		ui.PrintInfo("Pulling secrets from %s...", path)
	}

	// Retrieve secrets
	secrets, _, err := client.Secrets.RetrieveSecrets(context.Background(), req)
	if err != nil {
		// Get current account for better error messages
		account, _ := getCurrentAccount()
		return HandleAPIError(err, account)
	}

	if len(secrets) == 0 {
		if !pullQuiet {
			ui.PrintWarning("No secrets found")
		}
		return nil
	}

	// Decrypt if requested
	if pullDecrypt {
		var encKey []byte

		// Check for client-provided key
		if pullClientKey != "" {
			keyData, err := os.ReadFile(pullClientKey)
			if err != nil {
				return fmt.Errorf("failed to read client key: %w", err)
			}

			encKey, err = key.ParseKey(string(keyData))
			if err != nil {
				return fmt.Errorf("failed to parse client key: %w", err)
			}
		} else {
			// Get encryption key from server
			encKeyResp, _, err := client.Encryption.GetEncryptionKey(context.Background(), projectSlug)
			if err != nil {
				account, _ := getCurrentAccount()
				return HandleAPIError(err, account)
			}

			encKey, err = key.ParseKey(encKeyResp.Key)
			if err != nil {
				return fmt.Errorf("failed to parse encryption key: %w", err)
			}
		}

		// Decrypt all secrets
		decrypted := make(map[string]string)
		for k, v := range secrets {
			if crypto.IsEncrypted(v) {
				dec, err := crypto.DecryptString(v, encKey)
				if err != nil {
					if !pullQuiet {
						ui.PrintWarning("Failed to decrypt %s: %v", k, err)
					}
					decrypted[k] = v // Keep encrypted value
				} else {
					decrypted[k] = dec
				}
			} else {
				decrypted[k] = v
			}
		}
		secrets = decrypted
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
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if pullOutput != "" {
		// Ensure directory exists
		dir := filepath.Dir(pullOutput)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Write file
		if err := os.WriteFile(pullOutput, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
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

func formatSecrets(secrets map[string]string, format string) (string, error) {
	switch strings.ToLower(format) {
	case "env":
		gen := env.NewGenerator(nil)
		return gen.GenerateString(secrets)

	case "json":
		gen := json.NewHandler(nil)
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
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}
