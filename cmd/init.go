package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/auth"
	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
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

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	// Add flags specific to init command
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing configuration")
}

func runInit(cmd *cobra.Command, _ []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	loader := config.NewLoader(configPath)
	ok, confirmErr := confirmInitOverwrite(loader, configPath)
	if !ok || confirmErr != nil {
		return confirmErr
	}

	cfg := config.DefaultConfig()
	ui.PrintInfof("Welcome to DotEnv CLI setup!")
	fmt.Println()

	apiURL, err := promptAPIURL()
	if err != nil {
		return err
	}

	telemetryEnabled, err := promptTelemetry()
	if err != nil {
		return err
	}
	cfg.TelemetryEnabled = telemetryEnabled

	authMethod, err := ui.Select("\nStep 3: Authentication\nHow would you like to authenticate?", []string{
		"Login via browser (recommended)",
		"Enter API key manually",
	})
	if err != nil {
		return err
	}

	if strings.Contains(authMethod, "browser") {
		return runInitBrowserAuth(cmd, loader, cfg, configPath, apiURL)
	}
	return runInitAPIKeyAuth(cmd, configPath, apiURL)
}

func confirmInitOverwrite(loader *config.Loader, configPath string) (bool, error) {
	if !loader.Exists() || initForce {
		return true, nil
	}
	ui.PrintWarningf("Configuration already exists at %s", configPath)
	overwrite, confirmErr := ui.Confirm("Do you want to overwrite it?", false)
	if confirmErr != nil {
		return false, confirmErr
	}
	if !overwrite {
		ui.PrintInfof("Init canceled")
		return false, nil
	}
	return true, nil
}

func promptAPIURL() (string, error) {
	ui.PrintInfof("Step 1: API Configuration")
	apiURL, err := ui.Input("API URL", getAPIURL(), nil)
	if err != nil {
		return "", err
	}
	validator := config.NewValidator()
	if validateErr := validator.ValidateAPIURL(apiURL); validateErr != nil {
		return "", fmt.Errorf("invalid API URL: %w", validateErr)
	}
	return apiURL, nil
}

func promptTelemetry() (bool, error) {
	ui.PrintInfof("\nStep 2: Telemetry")
	ui.PrintInfof("Help us improve DotEnv CLI by sharing anonymous usage data.")
	ui.PrintInfof("No personal information or secret values are ever collected.")
	return ui.Confirm("Enable anonymous telemetry?", true)
}

func runInitBrowserAuth(cmd *cobra.Command, loader *config.Loader, cfg *config.Config, configPath, apiURL string) error {
	if saveErr := loader.Save(cfg); saveErr != nil {
		return fmt.Errorf("failed to save configuration: %w", saveErr)
	}

	loginNow, loginErr := ui.Confirm("Would you like to login now?", true)
	if loginErr != nil {
		return loginErr
	}

	if !loginNow {
		ui.PrintInfof("You can login later by running 'dotenv login'")
		printDeferredLoginNextSteps(configPath)
		return nil
	}

	ui.PrintInfof("Starting login process...")
	am, amErr := config.NewAccountManager(configPath)
	if amErr != nil {
		return fmt.Errorf("failed to create account manager: %w", amErr)
	}
	opts := auth.BrowserLoginOptions{
		APIUrl:        apiURL,
		CallbackPort:  "",
		NoBrowser:     false,
		IsInteractive: true,
	}
	if loginErr := auth.DoBrowserLogin(cmd.Context(), am, opts); loginErr != nil {
		ui.PrintErrorf("Login failed: %v", loginErr)
		ui.PrintInfof("You can try again later by running 'dotenv login'")
		printDeferredLoginNextSteps(configPath)
		return nil
	}

	cfg, err := loader.Load()
	if err == nil && cfg.CurrentAccount != "" {
		ui.PrintSuccessf("Configuration saved and logged in successfully!")
		ui.PrintInfof("Current account: %s", cfg.CurrentAccount)
		ui.PrintInfof("Try 'dotenv list projects' to see your projects")
	}
	return nil
}

func printDeferredLoginNextSteps(configPath string) {
	ui.PrintSuccessf("Configuration saved to %s", configPath)
	ui.PrintInfof("Next steps:")
	fmt.Println("1. Run 'dotenv login' to authenticate")
	fmt.Println("2. Run 'dotenv pull <project>' to fetch your secrets")
}

func runInitAPIKeyAuth(cmd *cobra.Command, configPath, apiURL string) error {
	validator := config.NewValidator()
	apiKey, err := ui.Password("Enter your API key")
	if err != nil {
		return err
	}
	if vErr := validator.ValidateAPIKey(apiKey); vErr != nil {
		return fmt.Errorf("invalid API key: %w", vErr)
	}

	organization, err := resolveInitOrganization(cmd.Context(), apiURL, apiKey)
	if err != nil {
		return err
	}
	if organization == "" {
		return nil
	}

	return createInitAccount(configPath, apiURL, apiKey, organization, validator)
}

func resolveInitOrganization(ctx context.Context, apiURL, apiKey string) (string, error) {
	factory := client.NewFactory(apiURL)
	tempClient := factory.NewClientFromAPIKey(apiKey, apiURL, "")

	orgs, orgResp, err := tempClient.Organizations.List(ctx, nil)
	if orgResp != nil {
		defer orgResp.Body.Close()
	}
	if err != nil {
		ui.PrintWarningf("Could not verify API key: %v", err)
		return ui.Input("Enter your organization slug", "", nil)
	}
	if len(orgs) == 0 {
		return "", nil
	}
	orgNames := make([]string, len(orgs))
	for i, org := range orgs {
		orgNames[i] = fmt.Sprintf("%s (%s)", org.Name, org.Slug)
	}
	selected, err := ui.Select("Select organization", orgNames)
	if err != nil {
		return "", err
	}
	for _, org := range orgs {
		if strings.Contains(selected, org.Slug) {
			return org.Slug, nil
		}
	}
	return "", nil
}

func createInitAccount(configPath, apiURL, apiKey, organization string, validator *config.Validator) error {
	ui.PrintInfof("\nStep 4: Account Configuration")
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return fmt.Errorf("failed to create account manager: %w", err)
	}

	accountName, err := ui.Input("Account name", organization, nil)
	if err != nil {
		return err
	}
	if err := validator.ValidateAccountName(accountName); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	orgInfo := config.OrgInfo{ULID: organization, Name: organization}
	if err := am.CreateWithAPIKey(accountName, apiURL, apiKey, &orgInfo); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	if err := am.Use(accountName); err != nil {
		return fmt.Errorf("failed to set current account: %w", err)
	}

	ui.PrintSuccessf("Configuration saved to %s", configPath)
	ui.PrintSuccessf("You're all set!")
	ui.PrintInfof("Current account: %s", accountName)
	ui.PrintInfof("Try 'dotenv list projects' to see your projects")
	return nil
}
