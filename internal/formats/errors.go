package formats

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrInvalidFormat indicates the format is invalid or unsupported
	ErrInvalidFormat = errors.New("invalid format")

	// ErrInvalidKey indicates an invalid key name
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue indicates an invalid value
	ErrInvalidValue = errors.New("invalid value")

	// ErrDuplicateKey indicates a duplicate key was found
	ErrDuplicateKey = errors.New("duplicate key")

	// ErrUnclosedQuote indicates an unclosed quote in the content
	ErrUnclosedQuote = errors.New("unclosed quote")

	// ErrCircularReference indicates a circular variable reference
	ErrCircularReference = errors.New("circular variable reference")

	// ErrVariableNotFound indicates a required variable was not found
	ErrVariableNotFound = errors.New("variable not found")

	// ErrMaxDepthExceeded indicates maximum interpolation depth was exceeded
	ErrMaxDepthExceeded = errors.New("maximum interpolation depth exceeded")
)

// ParseError represents a parsing error with line information
type ParseError struct {
	Line    int
	Column  int
	Message string
	Err     error
}

// Error implements the error interface
func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *ParseError) Unwrap() error {
	return e.Err
}

// NewParseError creates a new parse error
func NewParseError(line int, message string, err error) *ParseError {
	return &ParseError{
		Line:    line,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Field != "" && e.Value != "" {
		return fmt.Sprintf("%s '%s': %s", e.Field, e.Value, e.Message)
	}
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// InterpolationError represents an error during variable interpolation
type InterpolationError struct {
	Variable string
	Message  string
	Err      error
}

// Error implements the error interface
func (e *InterpolationError) Error() string {
	if e.Variable != "" {
		return fmt.Sprintf("variable '%s': %s", e.Variable, e.Message)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *InterpolationError) Unwrap() error {
	return e.Err
}
