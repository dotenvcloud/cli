package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/telemetry"
)

var (
	cfgFile         string
	debug           bool
	quiet           bool
	noColor         bool
	globalAPIKey    string
	telemetryClient *telemetry.Client
	commandStart    time.Time
)

// rootCmd represents the base command
var rootCmd *cobra.Command

// NewRootCommand creates a new root command instance.
// This is useful for testing as it returns a fresh command tree.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotenv",
		Short: "DotEnv CLI - Secure environment variable management",
		Long: `DotEnv CLI provides secure management of environment variables
across projects, targets, and environments with client-side encryption support.

For more information, visit: https://dotenv.cloud/docs/cli`,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Disable color if requested
			if noColor {
				color.NoColor = true
			}

			// Set debug mode
			if debug {
				viper.Set("debug", true)
			}

			// Set quiet mode
			if quiet {
				viper.Set("quiet", true)
			}

			// Record command start time
			commandStart = time.Now()

			// Initialize telemetry if enabled
			initTelemetry()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Track command execution
			if telemetryClient != nil && telemetryClient.IsEnabled() {
				// Get command name for telemetry
				commandName := cmd.CommandPath()

				// Calculate duration
				duration := time.Since(commandStart)

				// Command considered successful if no error occurred
				success := cmd.Context().Err() == nil

				// Track the command
				telemetryClient.TrackCommand(commandName, duration, success)
			}
		},
	}

	// Setup persistent flags
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.dotenv/config.yaml)")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"enable debug output")
	cmd.PersistentFlags().BoolVar(&quiet, "quiet", false,
		"suppress non-error output")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"disable colored output")
	cmd.PersistentFlags().StringVar(&globalAPIKey, "api-key", "",
		"API key for authentication (overrides account system)")

	// Bind flags to viper
	viper.BindPFlag("debug", cmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("quiet", cmd.PersistentFlags().Lookup("quiet"))
	viper.BindPFlag("api_key", cmd.PersistentFlags().Lookup("api-key"))

	// Add all commands
	cmd.AddCommand(
		initCmd,
		loginCmd,
		pullCmd,
		pushCmd,
		listCmd,
		exportCmd,
		accountCmd, // New account management
		orgCmd,     // Updated org management
		statusCmd,  // New status command
		refreshCmd,
		updateCmd,
		versionCmd,
		treeCmd,       // New tree command
		exploreCmd,    // New explore command
		pathCmd,       // New path command
		apikeysCmd,    // API key management
		authCmd,       // Auth info and management
		completionCmd, // Shell completion support
	)

	// Register shell completion functions after all commands are added
	registerResourcePathCompletions()

	return cmd
}

// Execute runs the root command
func Execute() error {
	err := rootCmd.Execute()

	// Close telemetry client if initialized
	if telemetryClient != nil {
		telemetryClient.Close()
	}

	return err
}

func init() {
	cobra.OnInitialize(initConfig)

	// Initialize the root command
	rootCmd = NewRootCommand()
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory
		configPath := filepath.Join(home, ".dotenv")
		viper.AddConfigPath(configPath)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")

		// Create config directory if it doesn't exist
		if err := os.MkdirAll(configPath, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		}
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("DOTENV")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is not an error for most commands
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		}
	}
}

// initTelemetry initializes the telemetry client based on configuration
func initTelemetry() {
	// Load config to check telemetry settings
	configPath, err := config.ConfigPath()
	if err != nil {
		return
	}

	loader := config.NewLoader(configPath)
	if !loader.Exists() {
		return
	}

	cfg, err := loader.Load()
	if err != nil {
		return
	}

	// Check if telemetry is enabled
	if !cfg.TelemetryEnabled {
		return
	}

	// Get API URL
	apiURL := constants.DefaultAPIURL
	if cfg.CurrentAccount != "" {
		if acct, ok := cfg.Accounts[cfg.CurrentAccount]; ok && acct.APIURL != "" {
			apiURL = acct.APIURL
		}
	}

	// Get analytics ID from preferences
	analyticsID := cfg.Preferences.AnalyticsID
	if analyticsID == "" {
		// Generate new analytics ID if not present
		analyticsID = generateAnalyticsID()
		cfg.Preferences.AnalyticsID = analyticsID
		// Save config with new analytics ID
		loader.Save(cfg)
	}

	// Create unauthenticated SDK client for telemetry
	sdkClient := getUnauthenticatedSDKClient(apiURL)

	// Create telemetry client
	// Note: We're not using an API key for telemetry as it's anonymous
	telemetryClient = telemetry.NewClient(sdkClient, analyticsID)
	telemetryClient.SetEnabled(true)
}

// generateAnalyticsID generates a new anonymous analytics ID
func generateAnalyticsID() string {
	// Use a simple timestamp-based ID for anonymity
	return fmt.Sprintf("cli_%d", time.Now().Unix())
}
