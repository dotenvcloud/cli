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
your first context. This command will guide you through:

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

	// Check if we're in development mode
	apiURL := "https://api.dotenv.com"
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		apiURL = "https://dotenv.test"
	}

	apiURL, err = ui.Input("API URL", apiURL, nil)
	if err != nil {
		return err
	}

	// Validate URL
	validator := config.NewValidator()
	if err := validator.ValidateAPIURL(apiURL); err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}

	// Step 2: Authentication Method
	ui.PrintInfo("\nStep 2: Authentication")

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
		// Save the config first before attempting login
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

			// Create context manager with the new config
			cm, err := config.NewContextManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create context manager: %w", err)
			}

			// Use browser login
			opts := auth.BrowserLoginOptions{
				APIUrl:        apiURL,
				CallbackPort:  "",
				NoBrowser:     false,
				IsInteractive: true,
			}

			if err := auth.DoBrowserLogin(cmd.Context(), cm, opts); err != nil {
				ui.PrintError("Login failed: %v", err)
				ui.PrintInfo("You can try again later by running 'dotenv login'")
				deferredLogin = true
			} else {
				// Login successful, reload config to show current context
				cfg, err = loader.Load()
				if err == nil && cfg.CurrentContext != "" {
					ui.PrintSuccess("Configuration saved and logged in successfully!")
					ui.PrintInfo("Current context: %s", cfg.CurrentContext)
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
		tempClient := dotenv.NewClient(apiKey, dotenv.WithBaseURL(apiURL))
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

	// Step 3: Context Name
	ui.PrintInfo("\nStep 3: Context Configuration")

	defaultContextName := "default"
	if organization != "" {
		defaultContextName = organization
	}

	contextName, err := ui.Input("Context name", defaultContextName, nil)
	if err != nil {
		return err
	}

	if err := validator.ValidateContextName(contextName); err != nil {
		return fmt.Errorf("invalid context name: %w", err)
	}

	// Add context if we have manual credentials
	if apiKey != "" && organization != "" {
		ctx := config.Context{
			APIURL:       apiURL,
			APIKey:       apiKey,
			Organization: organization,
		}

		if err := cfg.AddContext(contextName, ctx); err != nil {
			return fmt.Errorf("failed to add context: %w", err)
		}

		cfg.CurrentContext = contextName

		// Save configuration
		if err := loader.Save(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		ui.PrintSuccess("Configuration saved to %s", configPath)
		ui.PrintSuccess("You're all set!")
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
