package env

import (
	"fmt"
	"io"

	"github.com/dotenvcloud/cli/internal/formats"
)

// Handler implements the ENV format handler
type Handler struct {
	parser    *Parser
	generator *Generator
	options   *formats.Options
}

// NewHandler creates a new ENV handler
func NewHandler(opts *formats.Options) *Handler {
	if opts == nil {
		opts = formats.DefaultOptions()
	}
	return &Handler{
		parser:    NewParser(opts),
		generator: NewGenerator(opts),
		options:   opts,
	}
}

// Format returns the format type
func (h *Handler) Format() formats.Format {
	return formats.FormatENV
}

// Extensions returns supported file extensions
func (h *Handler) Extensions() []string {
	return []string{".env", ".env.local", ".env.production", ".env.development", ".env.test"}
}

// Parse implements Parser interface
func (h *Handler) Parse(r io.Reader) (map[string]string, error) {
	return h.parser.Parse(r)
}

// ParseString implements Parser interface
func (h *Handler) ParseString(content string) (map[string]string, error) {
	return h.parser.ParseString(content)
}

// ParseFile implements Parser interface
func (h *Handler) ParseFile(filename string) (map[string]string, error) {
	return h.parser.ParseFile(filename)
}

// Generate implements Generator interface
func (h *Handler) Generate(w io.Writer, data map[string]string) error {
	return h.generator.Generate(w, data)
}

// GenerateString implements Generator interface
func (h *Handler) GenerateString(data map[string]string) (string, error) {
	return h.generator.GenerateString(data)
}

// GenerateFile implements Generator interface
func (h *Handler) GenerateFile(filename string, data map[string]string) error {
	return h.generator.GenerateFile(filename, data)
}

// Validate implements Validator interface
func (h *Handler) Validate(content []byte) error {
	_, err := h.ParseString(string(content))
	return err
}

// ValidateKey implements Validator interface
func (h *Handler) ValidateKey(key string) error {
	if !isValidKey(key) {
		return &formats.ValidationError{
			Field:   "key",
			Value:   key,
			Message: "must start with letter or underscore and contain only alphanumeric characters and underscores",
		}
	}
	return nil
}

// ValidateValue implements Validator interface
func (h *Handler) ValidateValue(_ string) error {
	// ENV format accepts any string value
	return nil
}

// ParseWithComments parses ENV content preserving comments
func (h *Handler) ParseWithComments(r io.Reader) (*formats.ParseResult, error) {
	return h.parser.ParseExtended(r)
}

// GenerateWithComments generates ENV content with comments
func (h *Handler) GenerateWithComments(w io.Writer, result *formats.ParseResult, opts *formats.GenerateOptions) error {
	return h.generator.GenerateExtended(w, result, opts)
}

//nolint:gochecknoinits // format registry self-registration is idiomatic for plugin-style handlers
func init() {
	// Register the handler with the default registry
	if err := formats.DefaultRegistry.Register(formats.FormatENV, NewHandler(nil)); err != nil {
		panic(fmt.Sprintf("env handler: failed to register: %v", err))
	}
}
