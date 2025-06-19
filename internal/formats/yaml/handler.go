package yaml

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dotenv/cli/internal/formats"
	"gopkg.in/yaml.v3"
)

// Handler implements YAML format handling
type Handler struct {
	options *formats.Options
}

// NewHandler creates a new YAML handler
func NewHandler(opts *formats.Options) *Handler {
	if opts == nil {
		opts = formats.DefaultOptions()
	}
	return &Handler{options: opts}
}

// Format returns the format type
func (h *Handler) Format() formats.Format {
	return formats.FormatYAML
}

// Extensions returns supported file extensions
func (h *Handler) Extensions() []string {
	return []string{".yaml", ".yml"}
}

// Parse parses YAML content
func (h *Handler) Parse(r io.Reader) (map[string]string, error) {
	var raw map[string]interface{}
	decoder := yaml.NewDecoder(r)

	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	result := make(map[string]string)
	for key, value := range raw {
		strValue, err := h.convertToString(value)
		if err != nil {
			return nil, fmt.Errorf("key '%s': %w", key, err)
		}
		result[key] = strValue
	}

	return result, nil
}

// ParseString parses YAML from string
func (h *Handler) ParseString(content string) (map[string]string, error) {
	return h.Parse(strings.NewReader(content))
}

// ParseFile parses YAML from file
func (h *Handler) ParseFile(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return h.Parse(file)
}

// convertToString converts various YAML types to string
func (h *Handler) convertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%v", v), nil
	case nil:
		return "", nil
	default:
		// For complex types, encode back to YAML
		data, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot convert to string: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
}

// Generate writes YAML format
func (h *Handler) Generate(w io.Writer, data map[string]string) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(h.options.IndentSize)
	defer encoder.Close()

	// Convert to proper types
	yamlData := make(map[string]interface{})
	for k, v := range data {
		yamlData[k] = h.convertValue(v)
	}

	return encoder.Encode(yamlData)
}

// GenerateString generates YAML string
func (h *Handler) GenerateString(data map[string]string) (string, error) {
	var buf strings.Builder
	if err := h.Generate(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// GenerateFile writes YAML to file
func (h *Handler) GenerateFile(filename string, data map[string]string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return h.Generate(file, data)
}

// convertValue converts string values to appropriate YAML types
func (h *Handler) convertValue(value string) interface{} {
	// Preserve multiline strings
	if strings.Contains(value, "\n") {
		return value
	}

	// Try to parse as number
	if strings.TrimSpace(value) == value {
		// Integer
		if n, err := fmt.Sscanf(value, "%d", new(int64)); err == nil && n == 1 {
			var i int64
			fmt.Sscanf(value, "%d", &i)
			return i
		}

		// Float
		if n, err := fmt.Sscanf(value, "%g", new(float64)); err == nil && n == 1 {
			var f float64
			fmt.Sscanf(value, "%g", &f)
			return f
		}

		// Boolean
		lower := strings.ToLower(value)
		if lower == "true" || lower == "yes" || lower == "on" {
			return true
		}
		if lower == "false" || lower == "no" || lower == "off" {
			return false
		}
	}

	return value
}

// Validate validates YAML content
func (h *Handler) Validate(content []byte) error {
	var v interface{}
	return yaml.Unmarshal(content, &v)
}

// ValidateKey validates a YAML key
func (h *Handler) ValidateKey(key string) error {
	// YAML allows any string as key
	return nil
}

// ValidateValue validates a YAML value
func (h *Handler) ValidateValue(value string) error {
	// Any string is valid
	return nil
}

func init() {
	// Register the handler with the default registry
	formats.DefaultRegistry.Register(formats.FormatYAML, NewHandler(nil))
}
