package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	debug        bool
	quiet        bool
	noColor      bool
	globalAPIKey string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "dotenv",
	Short: "DotEnv CLI - Secure environment variable management",
	Long: `DotEnv CLI provides secure management of environment variables
across projects, targets, and environments with client-side encryption support.

For more information, visit: https://dotenv.com/docs/cli`,

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
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.dotenv/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"enable debug output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false,
		"suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"disable colored output")
	rootCmd.PersistentFlags().StringVar(&globalAPIKey, "api-key", "",
		"API key for authentication (overrides account system)")

	// Bind flags to viper
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))

	// Add all commands
	rootCmd.AddCommand(
		initCmd,
		loginCmd,
		pullCmd,
		pushCmd,
		listCmd,
		exportCmd,
		accountCmd,  // New account management
		orgCmd,      // Updated org management
		statusCmd,   // New status command
		refreshCmd,
		updateCmd,
		versionCmd,
	)
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
