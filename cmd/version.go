package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/build"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print detailed version information about DotEnv CLI.`,

	Example: `  # Show full version information
  dotenv version

  # Show just the version number
  dotenv version --short`,

	Run: func(cmd *cobra.Command, _ []string) {
		info := build.GetInfo()

		if short, _ := cmd.Flags().GetBool("short"); short {
			fmt.Println(info.ShortString())
		} else {
			fmt.Println(info.String())
		}
	},
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	versionCmd.Flags().BoolP("short", "s", false, "print just the version number")
}
