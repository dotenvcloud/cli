package cmd

import (
	"fmt"

	"github.com/dotenv/cli/internal/build"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print detailed version information about DotEnv CLI.`,

	Example: `  # Show full version information
  dotenv version

  # Show just the version number
  dotenv version --short`,

	Run: func(cmd *cobra.Command, args []string) {
		info := build.GetInfo()

		if short, _ := cmd.Flags().GetBool("short"); short {
			fmt.Println(info.ShortString())
		} else {
			fmt.Println(info.String())
		}
	},
}

func init() {
	versionCmd.Flags().BoolP("short", "s", false, "print just the version number")
}
