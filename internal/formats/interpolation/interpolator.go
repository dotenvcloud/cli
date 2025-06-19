package interpolation

import (
	"fmt"
	"os"
	"regexp"
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

// interpolateWithDepth handles recursive interpolation with cycle detection
func (i *Interpolator) interpolateWithDepth(text string, depth int, visiting map[string]bool) (string, error) {
	if depth > i.options.MaxDepth {
		return "", fmt.Errorf("maximum interpolation depth exceeded")
	}

	// Track if we made any substitutions
	changed := false
	var lastError error

	// Pattern for ${VAR} or ${VAR:-default} or ${VAR:?error}
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:([-?])([^}]*))?\}`)

	result := re.ReplaceAllStringFunc(text, func(match string) string {
		changed = true
		parts := re.FindStringSubmatch(match)

		varName := parts[1]
		operator := parts[3]
		operand := parts[4]

		// Check for circular reference
		if visiting[varName] {
			lastError = fmt.Errorf("circular reference detected for variable '%s'", varName)
			return match
		}

		// Look up variable
		value := i.lookupVariable(varName)

		// Handle operators
		switch operator {
		case "-": // Use default if not set
			if value == "" {
				value = operand
			}
		case "?": // Error if not set
			if value == "" {
				if operand != "" {
					// Custom error message
					lastError = fmt.Errorf("variable %s: %s", varName, operand)
				} else {
					lastError = fmt.Errorf("variable %s is not set", varName)
				}
				if i.options.FailOnMissing {
					return match
				}
			}
		default:
			// No operator
			if value == "" {
				if i.options.FailOnMissing {
					lastError = fmt.Errorf("variable %s is not set", varName)
					return match
				}
				if i.options.KeepUnresolved {
					return match // Keep original
				}
				return "" // Replace with empty string
			}
		}

		return value
	})

	// Return early if we had an error
	if lastError != nil {
		return "", lastError
	}

	// Also handle simple $VAR format
	re2 := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:]

		// Check for circular reference
		if visiting[varName] {
			lastError = fmt.Errorf("circular reference detected for variable '%s'", varName)
			return match
		}

		value := i.lookupVariable(varName)

		if value != "" {
			changed = true
			return value
		}

		if i.options.FailOnMissing {
			lastError = fmt.Errorf("variable %s is not set", varName)
			return match
		}

		if i.options.KeepUnresolved {
			return match
		}

		changed = true
		return ""
	})

	// Return early if we had an error
	if lastError != nil {
		return "", lastError
	}

	// Handle recursive interpolation
	if changed && i.options.RecursiveResolve && depth < i.options.MaxDepth {
		// Create a new visiting map for the next level
		newVisiting := make(map[string]bool)
		for k, v := range visiting {
			newVisiting[k] = v
		}

		// Mark all variables in the current text as visiting
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			newVisiting[match[1]] = true
		}
		matches2 := re2.FindAllStringSubmatch(text, -1)
		for _, match := range matches2 {
			newVisiting[match[1]] = true
		}

		return i.interpolateWithDepth(result, depth+1, newVisiting)
	}

	return result, nil
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
