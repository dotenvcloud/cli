package interpolation

import (
	"fmt"
	"os"
	"strings"
)

// Interpolator handles variable interpolation
type Interpolator struct {
	variables map[string]string
	options   *Options
}

// Options for interpolation
type Options struct {
	FailOnMissing    bool // Fail if variable not found
	KeepUnresolved   bool // Keep ${VAR} if not found
	RecursiveResolve bool // Resolve variables in values
	MaxDepth         int  // Max recursion depth
}

// DefaultOptions returns default options
func DefaultOptions() *Options {
	return &Options{
		FailOnMissing:    false,
		KeepUnresolved:   true,
		RecursiveResolve: true,
		MaxDepth:         10,
	}
}

// NewInterpolator creates a new interpolator
func NewInterpolator(vars map[string]string, opts *Options) *Interpolator {
	if opts == nil {
		opts = DefaultOptions()
	}
	if opts.MaxDepth <= 0 {
		// MaxDepth zero-value means "caller didn't set it" — fall back to
		// the default so deep but legitimate chains still resolve.
		opts.MaxDepth = DefaultOptions().MaxDepth
	}
	if vars == nil {
		vars = make(map[string]string)
	}

	return &Interpolator{
		variables: vars,
		options:   opts,
	}
}

// Interpolate expands variables in text
func (i *Interpolator) Interpolate(text string) (string, error) {
	return i.interpolateWithDepth(text, 0, make(map[string]bool))
}

// interpolateWithDepth handles recursive interpolation with cycle detection.
//
// Order of checks per pass:
//  1. depth >= MaxDepth → "maximum interpolation depth exceeded"
//  2. refs in `visiting` set → "circular reference detected for variable …"
//  3. substitute once; if anything actually changed, recurse with refs added
//     to visiting set.
func (i *Interpolator) interpolateWithDepth(text string, depth int, visiting map[string]bool) (string, error) {
	if depth >= i.options.MaxDepth {
		return "", fmt.Errorf("maximum interpolation depth exceeded")
	}

	refs := extractRefs(text)
	for _, name := range refs {
		if visiting[name] {
			return "", fmt.Errorf("circular reference detected for variable '%s'", name)
		}
	}

	result, changed, err := i.substitute(text, visiting)
	if err != nil {
		return "", err
	}
	if !changed || !i.options.RecursiveResolve {
		return result, nil
	}

	newVisiting := make(map[string]bool, len(visiting)+len(refs))
	for k, v := range visiting {
		newVisiting[k] = v
	}
	for _, name := range refs {
		newVisiting[name] = true
	}
	return i.interpolateWithDepth(result, depth+1, newVisiting)
}

// substitute walks text and replaces ${VAR[:op operand]} and $VAR refs.
// Returns the new string, whether anything actually changed (i.e. a value
// was substituted), and any error.
//
// Uses brace-counting (not regex) so nested ${...} inside operands works.
func (i *Interpolator) substitute(text string, visiting map[string]bool) (string, bool, error) {
	var b strings.Builder
	changed := false
	n := len(text)
	for pos := 0; pos < n; {
		c := text[pos]
		// Preserve escaped dollar: \$ is emitted literally.
		if c == '\\' && pos+1 < n && text[pos+1] == '$' {
			b.WriteByte('\\')
			b.WriteByte('$')
			pos += 2
			continue
		}
		if c != '$' {
			b.WriteByte(c)
			pos++
			continue
		}
		// $...
		if pos+1 < n && text[pos+1] == '{' {
			// find matching `}` with brace counting
			depth := 1
			end := pos + 2
			for end < n {
				switch text[end] {
				case '{':
					depth++
				case '}':
					depth--
				}
				if depth == 0 {
					break
				}
				end++
			}
			if depth != 0 {
				// unterminated; emit literal and continue
				b.WriteByte('$')
				pos++
				continue
			}
			inner := text[pos+2 : end]
			match := text[pos : end+1]
			value, sub, err := i.evalBracedRef(inner, match, visiting)
			if err != nil {
				return "", false, err
			}
			if sub {
				changed = true
			}
			b.WriteString(value)
			pos = end + 1
			continue
		}
		// $VAR (simple)
		if pos+1 < n && isVarStart(text[pos+1]) {
			j := pos + 1
			for j < n && isVarChar(text[j]) {
				j++
			}
			varName := text[pos+1 : j]
			if visiting[varName] {
				return "", false, fmt.Errorf("circular reference detected for variable '%s'", varName)
			}
			value := i.lookupVariable(varName)
			if value != "" {
				b.WriteString(value)
				changed = true
			} else if i.options.FailOnMissing {
				return "", false, fmt.Errorf("variable %s is not set", varName)
			} else if i.options.KeepUnresolved {
				b.WriteString(text[pos:j])
			} else {
				changed = true
			}
			pos = j
			continue
		}
		b.WriteByte(c)
		pos++
	}
	return b.String(), changed, nil
}

// evalBracedRef evaluates the contents of a ${...} block. `match` is the
// full original match (including the braces) used when we want to preserve
// the source text for "keep unresolved" mode.
func (i *Interpolator) evalBracedRef(inner, match string, visiting map[string]bool) (string, bool, error) {
	// Parse VAR followed by optional :op operand.
	j := 0
	for j < len(inner) && isVarChar(inner[j]) {
		j++
	}
	if j == 0 {
		// no var name; treat as literal
		return match, false, nil
	}
	varName := inner[:j]
	if visiting[varName] {
		return "", false, fmt.Errorf("circular reference detected for variable '%s'", varName)
	}

	value := i.lookupVariable(varName)
	isSet := value != ""

	if j == len(inner) {
		// No operator
		if isSet {
			return value, true, nil
		}
		if i.options.FailOnMissing {
			return "", false, fmt.Errorf("variable %s is not set", varName)
		}
		if i.options.KeepUnresolved {
			return match, false, nil
		}
		return "", true, nil
	}

	if inner[j] != ':' {
		// Unknown format; emit literal.
		return match, false, nil
	}
	if j+1 >= len(inner) {
		return match, false, nil
	}
	op := inner[j+1]
	operand := inner[j+2:]

	// Recursively interpolate the operand so nested ${...} resolves.
	newVisiting := make(map[string]bool, len(visiting)+1)
	for k, v := range visiting {
		newVisiting[k] = v
	}
	newVisiting[varName] = true
	operandResolved, err := i.interpolateWithDepth(operand, 0, newVisiting)
	if err != nil {
		return "", false, err
	}

	switch op {
	case '-': // use default if unset/empty
		if !isSet {
			return operandResolved, true, nil
		}
		return value, true, nil
	case '+': // use operand if set, empty otherwise
		if isSet {
			return operandResolved, true, nil
		}
		return "", true, nil
	case '?': // error if unset
		if !isSet {
			if operandResolved != "" {
				return "", false, fmt.Errorf("variable %s: %s", varName, operandResolved)
			}
			return "", false, fmt.Errorf("variable %s is not set", varName)
		}
		return value, true, nil
	default:
		// Unknown operator; emit literal.
		return match, false, nil
	}
}

// extractRefs returns the top-level variable names referenced by ${VAR}
// or $VAR in text. Operand-nested refs aren't included.
func extractRefs(text string) []string {
	var refs []string
	n := len(text)
	for pos := 0; pos < n; {
		c := text[pos]
		if c == '\\' && pos+1 < n && text[pos+1] == '$' {
			pos += 2
			continue
		}
		if c != '$' {
			pos++
			continue
		}
		if pos+1 < n && text[pos+1] == '{' {
			// var name follows {
			j := pos + 2
			for j < n && isVarChar(text[j]) {
				j++
			}
			if j > pos+2 {
				refs = append(refs, text[pos+2:j])
			}
			// skip to matching brace
			depth := 1
			for j < n && depth > 0 {
				switch text[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				j++
			}
			pos = j
			continue
		}
		if pos+1 < n && isVarStart(text[pos+1]) {
			j := pos + 1
			for j < n && isVarChar(text[j]) {
				j++
			}
			refs = append(refs, text[pos+1:j])
			pos = j
			continue
		}
		pos++
	}
	return refs
}

func isVarStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}
func isVarChar(b byte) bool {
	return isVarStart(b) || (b >= '0' && b <= '9')
}

// lookupVariable looks up a variable value
func (i *Interpolator) lookupVariable(name string) string {
	// Check provided variables first
	if value, ok := i.variables[name]; ok {
		return value
	}

	// Check environment
	return os.Getenv(name)
}

// InterpolateMap interpolates all values in a map
func (i *Interpolator) InterpolateMap(data map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	// First pass: add all variables to the interpolator
	for key, value := range data {
		i.variables[key] = value
	}

	// Second pass: interpolate all values
	for key, value := range data {
		interpolated, err := i.Interpolate(value)
		if err != nil {
			return nil, fmt.Errorf("key '%s': %w", key, err)
		}
		result[key] = interpolated
	}

	return result, nil
}

// SetVariable sets a variable value
func (i *Interpolator) SetVariable(name, value string) {
	i.variables[name] = value
}

// SetVariables sets multiple variables
func (i *Interpolator) SetVariables(vars map[string]string) {
	for k, v := range vars {
		i.variables[k] = v
	}
}

// GetVariables returns all variables
func (i *Interpolator) GetVariables() map[string]string {
	result := make(map[string]string)
	for k, v := range i.variables {
		result[k] = v
	}
	return result
}

// Clear removes all variables
func (i *Interpolator) Clear() {
	i.variables = make(map[string]string)
}
