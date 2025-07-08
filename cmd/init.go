package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

var (
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize DotEnv CLI configuration",
	Long: `Initialize DotEnv CLI by creating a configuration file and setting up
your first account. This command will guide you through:

- API authentication
- Organization selection
- Telemetry preferences
- Default settings`,

	Example: `  # Initialize with interactive prompts
  dotenv init

  # Initialize and overwrite existing configuration
  dotenv init --force`,

	RunE: runInit,
}

func init() {
	// Add flags specific to init command
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if config already exists
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	loader := config.NewLoader(configPath)
	if loader.Exists() && !initForce {
		ui.PrintWarning("Configuration already exists at %s", configPath)

		overwrite, err := ui.Confirm("Do you want to overwrite it?", false)
		if err != nil {
			return err
		}

		if !overwrite {
			ui.PrintInfo("Init cancelled")
			return nil
		}
	}

	// Initialize config early
	cfg := config.DefaultConfig()

	ui.PrintInfo("Welcome to DotEnv CLI setup!")
	fmt.Println()

	// Step 1: API Configuration
	ui.PrintInfo("Step 1: API Configuration")

	// Get default API URL
	defaultAPIURL := getAPIURL()

	apiURL, err := ui.Input("API URL", defaultAPIURL, nil)
	if err != nil {
		return err
	}

	// Validate URL
	validator := config.NewValidator()
	if err := validator.ValidateAPIURL(apiURL); err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}

	// Step 2: Telemetry Configuration
	ui.PrintInfo("\nStep 2: Telemetry")
	ui.PrintInfo("Help us improve DotEnv CLI by sharing anonymous usage data.")
	ui.PrintInfo("No personal information or secret values are ever collected.")
	
	telemetryEnabled, err := ui.Confirm("Enable anonymous telemetry?", true)
	if err != nil {
		return err
	}
	cfg.TelemetryEnabled = telemetryEnabled

	// Step 3: Authentication Method
	ui.PrintInfo("\nStep 3: Authentication")

	authMethod, err := ui.Select("How would you like to authenticate?", []string{
		"Login via browser (recommended)",
		"Enter API key manually",
	})
	if err != nil {
		return err
	}

	var apiKey string
	var organization string
	var deferredLogin bool

	if strings.Contains(authMethod, "browser") {
		// Save the config first before attempting login (including telemetry preference)
		if err := loader.Save(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		// Ask if they want to login now
		loginNow, err := ui.Confirm("Would you like to login now?", true)
		if err != nil {
			return err
		}

		if loginNow {
			ui.PrintInfo("Starting login process...")

			// Create account manager with the new config
			am, err := config.NewAccountManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create account manager: %w", err)
			}

			// Use browser login
			opts := auth.BrowserLoginOptions{
				APIUrl:        apiURL,
				CallbackPort:  "",
				NoBrowser:     false,
				IsInteractive: true,
			}

			if err := auth.DoBrowserLogin(cmd.Context(), am, opts); err != nil {
				ui.PrintError("Login failed: %v", err)
				ui.PrintInfo("You can try again later by running 'dotenv login'")
				deferredLogin = true
			} else {
				// Login successful, reload config to show current account
				cfg, err = loader.Load()
				if err == nil && cfg.CurrentAccount != "" {
					ui.PrintSuccess("Configuration saved and logged in successfully!")
					ui.PrintInfo("Current account: %s", cfg.CurrentAccount)
					ui.PrintInfo("Try 'dotenv list projects' to see your projects")
					return nil // Exit early, OAuth setup is complete
				}
			}
		} else {
			ui.PrintInfo("You can login later by running 'dotenv login'")
			deferredLogin = true
		}
	} else {
		// Manual API key entry
		apiKey, err = ui.Password("Enter your API key")
		if err != nil {
			return err
		}

		// Validate API key format
		if err := validator.ValidateAPIKey(apiKey); err != nil {
			return fmt.Errorf("invalid API key: %w", err)
		}

		// Try to verify API key and get organization info
		tempClient := dotenv.NewClient(
			dotenv.WithAPIKey(apiKey),
			dotenv.WithBaseURL(apiURL),
		)
		if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
			tempClient.SetTLSSkipVerify(true)
		}

		orgs, _, err := tempClient.Organizations.List(context.Background(), nil)
		if err != nil {
			ui.PrintWarning("Could not verify API key: %v", err)

			organization, err = ui.Input("Enter your organization slug", "", nil)
			if err != nil {
				return err
			}
		} else if len(orgs) > 0 {
			// Select organization
			orgNames := make([]string, len(orgs))
			for i, org := range orgs {
				orgNames[i] = fmt.Sprintf("%s (%s)", org.Name, org.Slug)
			}

			selected, err := ui.Select("Select organization", orgNames)
			if err != nil {
				return err
			}

			for _, org := range orgs {
				if strings.Contains(selected, org.Slug) {
					organization = org.Slug
					break
				}
			}
		}
	}

	// Step 4: Account Configuration for API Key
	if apiKey != "" && organization != "" {
		ui.PrintInfo("\nStep 4: Account Configuration")

		// Create account manager
		am, err := config.NewAccountManager(configPath)
		if err != nil {
			return fmt.Errorf("failed to create account manager: %w", err)
		}

		// Default account name is organization slug
		defaultAccountName := organization
		accountName, err := ui.Input("Account name", defaultAccountName, nil)
		if err != nil {
			return err
		}

		if err := validator.ValidateAccountName(accountName); err != nil {
			return fmt.Errorf("invalid account name: %w", err)
		}

		// Create org info
		orgInfo := config.OrgInfo{
			ULID: organization,
			Name: organization,
			Slug: organization,
		}

		// Create API key account
		if err := am.CreateWithAPIKey(accountName, apiURL, apiKey, &orgInfo); err != nil {
			return fmt.Errorf("failed to create account: %w", err)
		}

		// Set as current account
		if err := am.Use(accountName); err != nil {
			return fmt.Errorf("failed to set current account: %w", err)
		}

		ui.PrintSuccess("Configuration saved to %s", configPath)
		ui.PrintSuccess("You're all set!")
		ui.PrintInfo("Current account: %s", accountName)
		ui.PrintInfo("Try 'dotenv list projects' to see your projects")
	} else if deferredLogin {
		// Config was already saved for browser auth
		ui.PrintSuccess("Configuration saved to %s", configPath)
		ui.PrintInfo("Next steps:")
		fmt.Println("1. Run 'dotenv login' to authenticate")
		fmt.Println("2. Run 'dotenv pull <project>' to fetch your secrets")
	}

	return nil
}
