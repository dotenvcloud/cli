package env

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/dotenv/cli/internal/formats"
)

// Parser implements ENV format parsing
type Parser struct {
	options *formats.Options
}

// NewParser creates a new ENV parser
func NewParser(opts *formats.Options) *Parser {
	if opts == nil {
		opts = formats.DefaultOptions()
	}
	return &Parser{options: opts}
}

// parseState tracks multiline parsing state.
type parseState struct {
	currentKey   string
	currentValue strings.Builder
	inMultiline  bool
	quote        string
}

// Parse parses ENV content from a reader
func (p *Parser) Parse(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	lineNum := 0
	st := &parseState{}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if st.inMultiline {
			if err := p.handleMultilineContinuation(line, lineNum, result, st); err != nil {
				return nil, err
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if err := p.parseAssignment(line, lineNum, result, st); err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	if st.inMultiline {
		return nil, fmt.Errorf("unclosed quote for key '%s'", st.currentKey)
	}

	return result, nil
}

func (p *Parser) handleMultilineContinuation(line string, lineNum int, result map[string]string, st *parseState) error {
	if st.quote == "" {
		return nil
	}
	idx := strings.Index(line, st.quote)
	if idx < 0 {
		st.currentValue.WriteString(line)
		st.currentValue.WriteString("\n")
		return nil
	}
	st.currentValue.WriteString(line[:idx])
	result[st.currentKey] = p.processValue(st.currentValue.String())
	closingQuote := st.quote
	st.inMultiline = false
	st.quote = ""
	rest := strings.TrimSpace(line[idx+len(closingQuote):])
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("line %d: unexpected content after closing quote", lineNum)
	}
	return nil
}

func (p *Parser) parseAssignment(line string, lineNum int, result map[string]string, st *parseState) error {
	equals := strings.Index(line, "=")
	if equals < 0 {
		if p.options.StrictMode {
			return fmt.Errorf("line %d: missing '=' in assignment", lineNum)
		}
		return nil
	}

	key := strings.TrimSpace(line[:equals])
	key = strings.TrimPrefix(key, "export ")
	key = strings.TrimSpace(key)
	value := line[equals+1:]

	if !isValidKey(key) {
		return fmt.Errorf("line %d: invalid key '%s'", lineNum, key)
	}
	if p.options.StrictMode {
		if _, exists := result[key]; exists {
			return fmt.Errorf("line %d: duplicate key '%s'", lineNum, key)
		}
	}

	value = strings.TrimLeft(value, " \t")
	if value != "" && (value[0] == '"' || value[0] == '\'') {
		p.handleQuotedValue(key, value, result, st)
		return nil
	}

	if idx := strings.Index(value, " #"); idx >= 0 {
		value = value[:idx]
	}
	if p.options.TrimSpace {
		value = strings.TrimRight(value, " \t")
	}
	result[key] = p.processValue(value)
	return nil
}

func (p *Parser) handleQuotedValue(key, value string, result map[string]string, st *parseState) {
	quote := string(value[0])
	value = value[1:]
	if idx := strings.LastIndex(value, quote); idx >= 0 {
		if idx == 0 || value[idx-1] != '\\' {
			result[key] = p.processValue(value[:idx])
			return
		}
	}
	st.currentKey = key
	st.currentValue.Reset()
	st.currentValue.WriteString(value)
	st.currentValue.WriteString("\n")
	st.inMultiline = true
	st.quote = quote
}

// ParseString parses ENV content from a string
func (p *Parser) ParseString(content string) (map[string]string, error) {
	return p.Parse(strings.NewReader(content))
}

// ParseFile parses ENV content from a file
func (p *Parser) ParseFile(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return p.Parse(file)
}

// processValue handles escape sequences and interpolation
func (p *Parser) processValue(value string) string {
	// Handle escape sequences
	value = processEscapes(value)

	// Handle variable interpolation if enabled
	if p.options.ExpandVariables {
		value = p.interpolateVariables(value)
	}

	return value
}

// processEscapes handles common escape sequences
func processEscapes(value string) string {
	replacements := []struct {
		from string
		to   string
	}{
		{`\n`, "\n"},
		{`\r`, "\r"},
		{`\t`, "\t"},
		{`\\`, "\\"},
		{`\"`, "\""},
		{`\'`, "'"},
	}

	for _, r := range replacements {
		value = strings.ReplaceAll(value, r.from, r.to)
	}

	return value
}

// interpolateVariables expands ${VAR} and $VAR references
func (p *Parser) interpolateVariables(value string) string {
	// Pattern for ${VAR} or ${VAR:-default}
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

	value = re.ReplaceAllStringFunc(value, func(match string) string {
		parts := re.FindStringSubmatch(match)
		varName := parts[1]
		defaultValue := parts[3]

		// Look up variable
		if val, ok := p.options.Variables[varName]; ok {
			return val
		}

		// Check environment
		if val := os.Getenv(varName); val != "" {
			return val
		}

		// Use default if provided
		if defaultValue != "" {
			return defaultValue
		}

		// Keep original if not found
		return match
	})

	// Also handle simple $VAR format
	re2 := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	value = re2.ReplaceAllStringFunc(value, func(match string) string {
		varName := match[1:]

		if val, ok := p.options.Variables[varName]; ok {
			return val
		}

		if val := os.Getenv(varName); val != "" {
			return val
		}

		return match
	})

	return value
}

// isValidKey checks if a key is valid
func isValidKey(key string) bool {
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

// Helper functions
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// ParseExtended parses with additional metadata
func (p *Parser) ParseExtended(r io.Reader) (*formats.ParseResult, error) {
	result := &formats.ParseResult{
		Data:     make(map[string]string),
		Comments: make(map[string]string),
		Order:    make([]string, 0),
		Metadata: make(map[string]interface{}),
	}

	scanner := bufio.NewScanner(r)
	lineNum := 0
	lastComment := ""

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Capture comments
		if strings.HasPrefix(trimmed, "#") {
			lastComment = strings.TrimSpace(trimmed[1:])
			continue
		}

		// Skip empty lines
		if trimmed == "" {
			lastComment = ""
			continue
		}

		// Parse assignment
		if idx := strings.Index(line, "="); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			result.Data[key] = p.processValue(value)
			result.Order = append(result.Order, key)

			if lastComment != "" && p.options.PreserveComments {
				result.Comments[key] = lastComment
			}

			lastComment = ""
		}
	}

	return result, scanner.Err()
}
