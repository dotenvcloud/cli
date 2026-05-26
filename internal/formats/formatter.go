package formats

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Formatter provides formatting options for output
type Formatter struct {
	options *FormatterOptions
}

// FormatterOptions configures the formatter
type FormatterOptions struct {
	// Display options
	ShowHeader      bool     // Show header with metadata
	ShowComments    bool     // Show comments for each key
	ShowLineNumbers bool     // Show line numbers
	ColorOutput     bool     // Use ANSI colors
	MaxValueLength  int      // Truncate values longer than this
	MaskSecrets     bool     // Mask sensitive values
	SecretPatterns  []string // Patterns to identify secrets

	// Sorting options
	SortBy          SortOption // How to sort keys
	GroupByPrefix   bool       // Group keys by prefix
	PrefixSeparator string     // Separator for prefix grouping

	// Output format
	Format        OutputFormat // Output format style
	Indent        string       // Indentation string
	KeyValueSep   string       // Separator between key and value
	LineSeparator string       // Line separator
}

// SortOption defines how to sort keys
type SortOption int

const (
	SortNone SortOption = iota
	SortAlphabetical
	SortAlphabeticalReverse
	SortByLength
	SortByCategory
)

// OutputFormat defines the output format style
type OutputFormat int

const (
	OutputDefault OutputFormat = iota
	OutputTable
	OutputExport
	OutputShell
	OutputDockerfile
	OutputCompact
)

// DefaultFormatterOptions returns default formatter options
func DefaultFormatterOptions() *FormatterOptions {
	return &FormatterOptions{
		ShowHeader:      false,
		ShowComments:    false,
		ShowLineNumbers: false,
		ColorOutput:     false,
		MaxValueLength:  0,
		MaskSecrets:     false,
		SecretPatterns:  []string{"PASSWORD", "SECRET", "KEY", "TOKEN", "PRIVATE"},
		SortBy:          SortNone,
		GroupByPrefix:   false,
		PrefixSeparator: "_",
		Format:          OutputDefault,
		Indent:          "  ",
		KeyValueSep:     "=",
		LineSeparator:   "\n",
	}
}

// NewFormatter creates a new formatter
func NewFormatter(opts *FormatterOptions) *Formatter {
	if opts == nil {
		opts = DefaultFormatterOptions()
	}
	return &Formatter{options: opts}
}

// Format formats the key-value data according to options
func (f *Formatter) Format(w io.Writer, data map[string]string) error {
	// Sort keys according to options
	keys := f.sortKeys(data)

	// Group by prefix if requested
	if f.options.GroupByPrefix {
		groups := f.groupByPrefix(keys, data)
		return f.formatGroups(w, groups)
	}

	// Format according to output format
	switch f.options.Format {
	case OutputTable:
		return f.formatTable(w, keys, data)
	case OutputExport:
		return f.formatExport(w, keys, data)
	case OutputShell:
		return f.formatShell(w, keys, data)
	case OutputDockerfile:
		return f.formatDockerfile(w, keys, data)
	case OutputCompact:
		return f.formatCompact(w, keys, data)
	case OutputDefault:
		return f.formatDefault(w, keys, data)
	default:
		return f.formatDefault(w, keys, data)
	}
}

// sortKeys sorts keys according to options
func (f *Formatter) sortKeys(data map[string]string) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	switch f.options.SortBy {
	case SortAlphabetical:
		sort.Strings(keys)
	case SortAlphabeticalReverse:
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	case SortByLength:
		sort.Slice(keys, func(i, j int) bool {
			return len(keys[i]) < len(keys[j])
		})
	case SortByCategory:
		sort.Slice(keys, func(i, j int) bool {
			cat1 := f.getCategory(keys[i])
			cat2 := f.getCategory(keys[j])
			if cat1 == cat2 {
				return keys[i] < keys[j]
			}
			return cat1 < cat2
		})
	case SortNone:
		// Preserve insertion order; no sort.
	}

	return keys
}

// groupByPrefix groups keys by their prefix
func (f *Formatter) groupByPrefix(keys []string, _ map[string]string) map[string][]string {
	groups := make(map[string][]string)

	for _, key := range keys {
		prefix := f.getPrefix(key)
		groups[prefix] = append(groups[prefix], key)
	}

	return groups
}

// getPrefix extracts the prefix from a key
func (f *Formatter) getPrefix(key string) string {
	idx := strings.Index(key, f.options.PrefixSeparator)
	if idx > 0 {
		return key[:idx]
	}
	return ""
}

// getCategory determines the category of a key
func (f *Formatter) getCategory(key string) string {
	lower := strings.ToLower(key)

	// Common categories
	if strings.Contains(lower, "database") || strings.Contains(lower, "db_") {
		return "database"
	}
	if strings.Contains(lower, "redis") || strings.Contains(lower, "cache") {
		return "cache"
	}
	if strings.Contains(lower, "mail") || strings.Contains(lower, "smtp") {
		return "mail"
	}
	if strings.Contains(lower, "aws") || strings.Contains(lower, "s3") {
		return "aws"
	}
	if strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") {
		return "api"
	}

	return "general"
}

// formatDefault formats in default style
func (f *Formatter) formatDefault(w io.Writer, keys []string, data map[string]string) error {
	for i, key := range keys {
		value := f.formatValue(key, data[key])

		if f.options.ShowLineNumbers {
			fmt.Fprintf(w, "%4d: ", i+1)
		}

		fmt.Fprintf(w, "%s%s%s", key, f.options.KeyValueSep, value)

		if i < len(keys)-1 {
			fmt.Fprint(w, f.options.LineSeparator)
		}
	}

	return nil
}

// formatTable formats as a table
func (f *Formatter) formatTable(w io.Writer, keys []string, data map[string]string) error {
	// Find max key length
	maxKeyLen := 0
	for _, key := range keys {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}

	// Print header
	if f.options.ShowHeader {
		fmt.Fprintf(w, "%-*s | %s\n", maxKeyLen, "KEY", "VALUE")
		fmt.Fprintf(w, "%s-+-%s\n", strings.Repeat("-", maxKeyLen), strings.Repeat("-", 40))
	}

	// Print rows
	for _, key := range keys {
		value := f.formatValue(key, data[key])
		fmt.Fprintf(w, "%-*s | %s\n", maxKeyLen, key, value)
	}

	return nil
}

// formatExport formats as shell export statements
func (f *Formatter) formatExport(w io.Writer, keys []string, data map[string]string) error {
	for _, key := range keys {
		value := data[key]
		// Escape for shell
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "\"", "\\\"")
		value = strings.ReplaceAll(value, "$", "\\$")
		value = strings.ReplaceAll(value, "`", "\\`")

		fmt.Fprintf(w, "export %s=\"%s\"\n", key, value)
	}

	return nil
}

// formatShell formats for shell evaluation
func (f *Formatter) formatShell(w io.Writer, keys []string, data map[string]string) error {
	fmt.Fprintln(w, "#!/bin/sh")
	fmt.Fprintln(w, "# Generated by DotEnv CLI")
	fmt.Fprintln(w)

	return f.formatExport(w, keys, data)
}

// formatDockerfile formats as Dockerfile ENV statements
func (f *Formatter) formatDockerfile(w io.Writer, keys []string, data map[string]string) error {
	fmt.Fprintln(w, "# Environment variables")

	for _, key := range keys {
		value := data[key]
		// Escape for Dockerfile
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "\"", "\\\"")

		fmt.Fprintf(w, "ENV %s=\"%s\"\n", key, value)
	}

	return nil
}

// formatCompact formats in compact style
func (f *Formatter) formatCompact(w io.Writer, keys []string, data map[string]string) error {
	parts := make([]string, 0, len(keys))

	for _, key := range keys {
		value := f.formatValue(key, data[key])
		parts = append(parts, fmt.Sprintf("%s%s%s", key, f.options.KeyValueSep, value))
	}

	fmt.Fprint(w, strings.Join(parts, " "))

	return nil
}

// formatGroups formats grouped keys
func (f *Formatter) formatGroups(w io.Writer, groups map[string][]string) error {
	// Sort group names
	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for i, groupName := range groupNames {
		if groupName != "" {
			fmt.Fprintf(w, "# %s\n", strings.ToUpper(groupName))
		}

		keys := groups[groupName]
		sort.Strings(keys)

		for _, key := range keys {
			fmt.Fprintf(w, "%s\n", key)
		}

		if i < len(groupNames)-1 {
			fmt.Fprintln(w)
		}
	}

	return nil
}

// formatValue formats a value according to options
func (f *Formatter) formatValue(key, value string) string {
	// Mask secrets if requested
	if f.options.MaskSecrets && f.isSecret(key) {
		return "********"
	}

	// Truncate if requested
	if f.options.MaxValueLength > 0 && len(value) > f.options.MaxValueLength {
		return value[:f.options.MaxValueLength] + "..."
	}

	return value
}

// isSecret checks if a key represents a secret
func (f *Formatter) isSecret(key string) bool {
	upper := strings.ToUpper(key)

	for _, pattern := range f.options.SecretPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}

	return false
}
