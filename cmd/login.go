package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
)

var (
	loginNoBrowser    bool
	loginCallbackPort string
	loginManual       bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with DotEnv",
	Long: `Authenticate with DotEnv using a browser-based flow.
This command will:

1. Open your browser to the DotEnv authentication page
2. Allow you to select organizations to access
3. Securely store your API credentials`,

	Example: `  # Login via browser
  dotenv login

  # Login without opening browser
  dotenv login --no-browser
  
  # Enter API key manually
  dotenv login --manual`,

	RunE: runLogin,
}

func init() {
	// Add flags specific to login command
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false,
		"print URL instead of opening browser")
	loginCmd.Flags().StringVar(&loginCallbackPort, "callback-port", "",
		"specify callback port (default: random)")
	loginCmd.Flags().BoolVar(&loginManual, "manual", false,
		"enter API key manually instead of browser auth")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Load current config
	cm, err := config.NewContextManager("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get API URL
	apiURL := viper.GetString("api_url")
	if apiURL == "" {
		apiURL = "https://api.dotenv.com"
		if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
			apiURL = "https://dotenv.test"
		}
	}

	// Handle manual login
	if loginManual {
		return runManualLogin(cm, apiURL)
	}

	// Use browser login
	opts := auth.BrowserLoginOptions{
		APIUrl:        apiURL,
		CallbackPort:  loginCallbackPort,
		NoBrowser:     loginNoBrowser,
		IsInteractive: true,
	}

	return auth.DoBrowserLogin(cmd.Context(), cm, opts)
}

func runManualLogin(cm *config.ContextManager, apiURL string) error {
	ui.PrintInfo("Manual API key entry")

	// Get API key
	apiKey, err := ui.Password("Enter your API key")
	if err != nil {
		return err
	}

	// Validate API key format
	validator := config.NewValidator()
	if err := validator.ValidateAPIKey(apiKey); err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	// Get organization
	organization, err := ui.Input("Enter your organization slug", "", ui.Required)
	if err != nil {
		return err
	}

	// Get context name
	contextName, err := ui.Input("Context name", organization, nil)
	if err != nil {
		return err
	}

	// Create context
	if err := cm.Create(contextName, apiURL, apiKey, organization); err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}

	// Set as current
	if err := cm.Use(contextName); err != nil {
		return fmt.Errorf("failed to set current context: %w", err)
	}

	ui.PrintSuccess("Login successful!")
	ui.PrintInfo("Current context set to: %s", contextName)

	return nil
}
