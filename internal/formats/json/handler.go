package json

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dotenv/cli/internal/formats"
)

// Handler implements JSON format handling
type Handler struct {
	options *formats.Options
}

// NewHandler creates a new JSON handler
func NewHandler(opts *formats.Options) *Handler {
	if opts == nil {
		opts = formats.DefaultOptions()
	}
	return &Handler{options: opts}
}

// Format returns the format type
func (h *Handler) Format() formats.Format {
	return formats.FormatJSON
}

// Extensions returns supported file extensions
func (h *Handler) Extensions() []string {
	return []string{".json"}
}

// Parse parses JSON content
func (h *Handler) Parse(r io.Reader) (map[string]string, error) {
	var raw map[string]interface{}
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
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

// ParseString parses JSON from string
func (h *Handler) ParseString(content string) (map[string]string, error) {
	return h.Parse(strings.NewReader(content))
}

// ParseFile parses JSON from file
func (h *Handler) ParseFile(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return h.Parse(file)
}

// convertToString converts various JSON types to string
func (h *Handler) convertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		// Handle integers without decimal
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), nil
		}
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%v", v), nil
	case nil:
		return "", nil
	default:
		// For complex types, encode back to JSON
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot convert to string: %w", err)
		}
		return string(data), nil
	}
}

// Generate writes JSON format
func (h *Handler) Generate(w io.Writer, data map[string]string) error {
	encoder := json.NewEncoder(w)

	if h.options.IndentSize > 0 {
		encoder.SetIndent("", strings.Repeat(" ", h.options.IndentSize))
	}

	// Sort keys if requested
	if h.options.SortKeys {
		return encoder.Encode(data)
	}

	// Preserve order using ordered map
	ordered := make(map[string]interface{}, len(data))
	for k, v := range data {
		ordered[k] = v
	}

	return encoder.Encode(ordered)
}

// GenerateString generates JSON string
func (h *Handler) GenerateString(data map[string]string) (string, error) {
	var buf strings.Builder
	if err := h.Generate(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// GenerateFile writes JSON to file
func (h *Handler) GenerateFile(filename string, data map[string]string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return h.Generate(file, data)
}

// Validate validates JSON content
func (h *Handler) Validate(content []byte) error {
	var v interface{}
	return json.Unmarshal(content, &v)
}

// ValidateKey validates a JSON key
func (h *Handler) ValidateKey(key string) error {
	// JSON allows any string as key
	return nil
}

// ValidateValue validates a JSON value
func (h *Handler) ValidateValue(value string) error {
	// Any string is valid
	return nil
}

func init() {
	// Register the handler with the default registry
	formats.DefaultRegistry.Register(formats.FormatJSON, NewHandler(nil))
}
