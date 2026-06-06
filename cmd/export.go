package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dotenvcloud/cli/internal/ui"
)

var (
	exportFormat   string
	exportOutput   string
	exportTemplate string
	exportQuiet    bool
)

var exportCmd = &cobra.Command{
	Use:   "export [project]/[target]/[environment]",
	Short: "Export secrets in various formats",
	Long: `Export secrets in different formats for use with other tools
and deployment systems.

Supported formats:
  env        - Standard .env format (default)
  json       - JSON object
  yaml       - YAML format
  shell      - Shell export statements
  dockerfile - Dockerfile ARG/ENV format`,

	Example: `  # Export as .env file
  dotenv export myproject/production/api

  # Export as JSON
  dotenv export myproject/staging --format=json

  # Export as shell exports
  dotenv export myproject --format=shell > exports.sh

  # Export to file
  dotenv export myproject --output=secrets.json --format=json`,

	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

//nolint:gochecknoinits // cobra subcommand registration is idiomatic in init
func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "env",
		"export format")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "",
		"output file (default: stdout)")
	exportCmd.Flags().StringVar(&exportTemplate, "template", "",
		"custom template file for export (not implemented yet)")
	exportCmd.Flags().BoolVarP(&exportQuiet, "quiet", "q", false,
		"suppress non-error output")
}

func runExport(cmd *cobra.Command, args []string) error {
	// This is essentially the same as pull with different defaults
	// Reuse the pull logic
	pullFormat = exportFormat
	pullOutput = exportOutput
	pullDecrypt = true
	pullResolve = false
	pullQuiet = exportQuiet

	if !exportQuiet {
		ui.PrintInfof("Exporting secrets in %s format...", exportFormat)
	}

	return runPull(cmd, args)
}
