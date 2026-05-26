package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/viper"
)

// Stdout is the destination for Print* helpers. Defaults to os.Stdout; tests
// swap to a buffer to capture output.
var Stdout io.Writer = os.Stdout

// Stderr is the destination for PrintError. Defaults to os.Stderr; tests swap.
var Stderr io.Writer = os.Stderr

// PrintSuccessf prints a success message
func PrintSuccessf(format string, args ...interface{}) {
	if !viper.GetBool("quiet") {
		fmt.Fprintf(Stdout, "%s %s\n", Success("✓"), fmt.Sprintf(format, args...))
	}
}

// PrintErrorf prints an error message
func PrintErrorf(format string, args ...interface{}) {
	fmt.Fprintf(Stderr, "%s %s\n", Error("✗"), fmt.Sprintf(format, args...))
}

// PrintWarningf prints a warning message
func PrintWarningf(format string, args ...interface{}) {
	if !viper.GetBool("quiet") {
		fmt.Fprintf(Stdout, "%s %s\n", Warning("!"), fmt.Sprintf(format, args...))
	}
}

// PrintInfof prints an info message
func PrintInfof(format string, args ...interface{}) {
	if !viper.GetBool("quiet") {
		fmt.Fprintf(Stdout, "%s %s\n", Info("ℹ"), fmt.Sprintf(format, args...))
	}
}

// PrintDebugf prints a debug message
func PrintDebugf(format string, args ...interface{}) {
	if viper.GetBool("debug") {
		fmt.Fprintf(Stdout, "%s %s\n", "[DEBUG]", fmt.Sprintf(format, args...))
	}
}

// StartSpinner starts a loading spinner with a message
func StartSpinner(message string) *spinner.Spinner {
	if viper.GetBool("quiet") || viper.GetBool("no-color") {
		return nil
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + message
	_ = s.Color("cyan")
	s.Start()
	return s
}

// StopSpinner stops a spinner with a success message
func StopSpinner(s *spinner.Spinner, message string) {
	if s != nil {
		s.Stop()
		PrintSuccessf("%s", message)
	}
}

// StopSpinnerError stops a spinner with an error message
func StopSpinnerError(s *spinner.Spinner, message string) {
	if s != nil {
		s.Stop()
		PrintErrorf("%s", message)
	}
}

// PrintHeader prints a section header
func PrintHeader(text string) {
	if !viper.GetBool("quiet") {
		fmt.Println()
		fmt.Println(Bold(text))
		fmt.Println(Bold("─────────────────────────────────────────"))
	}
}

// PrintSubheader prints a subsection header
func PrintSubheader(text string) {
	if !viper.GetBool("quiet") {
		fmt.Println()
		fmt.Println(Bold(text))
	}
}

// PrintKeyValue prints a key-value pair
func PrintKeyValue(key, value string) {
	if !viper.GetBool("quiet") {
		fmt.Printf("%s: %s\n", Bold(key), value)
	}
}

// PrintList prints a list of items
func PrintList(items []string) {
	if !viper.GetBool("quiet") {
		for _, item := range items {
			fmt.Printf("  • %s\n", item)
		}
	}
}

// PrintNumberedList prints a numbered list of items
func PrintNumberedList(items []string) {
	if !viper.GetBool("quiet") {
		for i, item := range items {
			fmt.Printf("  %d. %s\n", i+1, item)
		}
	}
}
