package formats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Detector detects file formats
type Detector struct {
	handlers map[Format]Handler
}

// NewDetector creates a new format detector
func NewDetector() *Detector {
	return &Detector{
		handlers: make(map[Format]Handler),
	}
}

// DetectFormat detects the format of the content
func (d *Detector) DetectFormat(content []byte) (Format, error) {
	// Try JSON first (most strict)
	if isJSON(content) {
		return FormatJSON, nil
	}

	// Try YAML
	if isYAML(content) {
		return FormatYAML, nil
	}

	// Default to ENV for simple key=value
	if isENV(content) {
		return FormatENV, nil
	}

	return "", fmt.Errorf("unable to detect format")
}

// DetectFormatFromFile detects format from filename
func (d *Detector) DetectFormatFromFile(filename string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".json":
		return FormatJSON, nil
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".env":
		return FormatENV, nil
	default:
		// Check for common patterns
		base := filepath.Base(filename)
		if strings.HasPrefix(base, ".env") || strings.Contains(base, "dotenv") {
			return FormatENV, nil
		}

		// Try content detection if file has no extension
		if ext == "" {
			return "", fmt.Errorf("cannot determine format from filename: %s", filename)
		}

		return "", fmt.Errorf("unknown file extension: %s", ext)
	}
}

// isJSON checks if content is valid JSON
func isJSON(content []byte) bool {
	content = bytes.TrimSpace(content)
	if len(content) == 0 {
		return false
	}

	// Check for JSON object or array markers
	if content[0] != '{' && content[0] != '[' {
		return false
	}

	var js json.RawMessage
	return json.Unmarshal(content, &js) == nil
}

// isYAML checks if content is valid YAML
func isYAML(content []byte) bool {
	content = bytes.TrimSpace(content)
	if len(content) == 0 {
		return false
	}

	// Quick checks for YAML indicators
	str := string(content)
	yamlIndicators := []string{
		"---", "...", ": ", ":\n", ":\r\n",
		"- ", "\n- ", "\r\n- ",
	}

	hasIndicator := false
	for _, indicator := range yamlIndicators {
		if strings.Contains(str, indicator) {
			hasIndicator = true
			break
		}
	}

	if !hasIndicator {
		return false
	}

	var y interface{}
	err := yaml.Unmarshal(content, &y)
	if err != nil {
		return false
	}

	// Ensure it's a map (not array or scalar)
	_, ok := y.(map[string]interface{})
	return ok
}

// isENV checks if content looks like ENV format
func isENV(content []byte) bool {
	lines := strings.Split(string(content), "\n")
	validLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for KEY=VALUE pattern
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && isValidEnvKey(parts[0]) {
				validLines++
			}
		}
	}

	return validLines > 0
}

// isValidEnvKey checks if a string is a valid environment variable key
func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}

	// Must start with letter or underscore
	if !isLetter(rune(key[0])) && key[0] != '_' {
		return false
	}

	// Rest must be alphanumeric or underscore
	for _, r := range key {
		if !isLetter(r) && !isDigit(r) && r != '_' {
			return false
		}
	}

	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
