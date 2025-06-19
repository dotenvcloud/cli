package formats

import (
	"io"
)

// Format represents a file format type
type Format string

const (
	FormatENV    Format = "env"
	FormatJSON   Format = "json"
	FormatYAML   Format = "yaml"
	FormatShell  Format = "shell"
	FormatDocker Format = "dockerfile"
)

// Parser defines the interface for parsing different formats
type Parser interface {
	// Parse reads from the reader and returns key-value pairs
	Parse(r io.Reader) (map[string]string, error)

	// ParseString parses a string
	ParseString(content string) (map[string]string, error)

	// ParseFile parses a file
	ParseFile(filename string) (map[string]string, error)
}

// Generator defines the interface for generating different formats
type Generator interface {
	// Generate writes key-value pairs to the writer
	Generate(w io.Writer, data map[string]string) error

	// GenerateString returns the formatted string
	GenerateString(data map[string]string) (string, error)

	// GenerateFile writes to a file
	GenerateFile(filename string, data map[string]string) error
}

// Validator defines the interface for format validation
type Validator interface {
	// Validate checks if the content is valid for the format
	Validate(content []byte) error

	// ValidateKey checks if a key is valid
	ValidateKey(key string) error

	// ValidateValue checks if a value is valid
	ValidateValue(value string) error
}

// Handler combines parser, generator, and validator
type Handler interface {
	Parser
	Generator
	Validator

	// Format returns the format type
	Format() Format

	// Extensions returns file extensions for this format
	Extensions() []string
}

// Options for parsing and generation
type Options struct {
	// Parsing options
	ExpandVariables  bool              // Expand ${VAR} references
	StrictMode       bool              // Strict parsing mode
	PreserveComments bool              // Preserve comments (ENV only)
	TrimSpace        bool              // Trim whitespace from values
	Variables        map[string]string // Variables for interpolation

	// Generation options
	SortKeys        bool   // Sort keys alphabetically
	QuoteValues     bool   // Always quote values
	IndentSize      int    // Indent size for structured formats
	LineEnding      string // Line ending style (\n or \r\n)
	IncludeComments bool   // Include instructional comments
}

// DefaultOptions returns default options
func DefaultOptions() *Options {
	return &Options{
		ExpandVariables:  false,
		StrictMode:       false,
		PreserveComments: true,
		TrimSpace:        true,
		Variables:        make(map[string]string),
		SortKeys:         false,
		QuoteValues:      false,
		IndentSize:       2,
		LineEnding:       "\n",
		IncludeComments:  false,
	}
}

// ParseResult contains parsed data with metadata
type ParseResult struct {
	Data     map[string]string
	Comments map[string]string // Key -> comment mapping
	Order    []string          // Original key order
	Metadata map[string]interface{}
}

// GenerateOptions provides fine-grained control over generation
type GenerateOptions struct {
	*Options
	Header    string            // File header comment
	KeyOrder  []string          // Specific key order
	KeyFilter func(string) bool // Filter keys
}
