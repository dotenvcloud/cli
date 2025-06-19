package formats

import (
	"fmt"
	"sync"
)

// Registry manages format handlers
type Registry struct {
	mu       sync.RWMutex
	handlers map[Format]Handler
}

// DefaultRegistry is the global format registry
var DefaultRegistry = NewRegistry()

// NewRegistry creates a new format registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[Format]Handler),
	}
}

// Register registers a format handler
func (r *Registry) Register(format Format, handler Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[format]; exists {
		return fmt.Errorf("format %s already registered", format)
	}

	r.handlers[format] = handler
	return nil
}

// Get returns a handler for the format
func (r *Registry) Get(format Format) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[format]
	if !exists {
		return nil, fmt.Errorf("no handler registered for format %s", format)
	}

	return handler, nil
}

// List returns all registered formats
func (r *Registry) List() []Format {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formats := make([]Format, 0, len(r.handlers))
	for format := range r.handlers {
		formats = append(formats, format)
	}

	return formats
}

// Parse parses content with the appropriate handler
func (r *Registry) Parse(format Format, content string) (map[string]string, error) {
	handler, err := r.Get(format)
	if err != nil {
		return nil, err
	}

	return handler.ParseString(content)
}

// Generate generates content with the appropriate handler
func (r *Registry) Generate(format Format, data map[string]string) (string, error) {
	handler, err := r.Get(format)
	if err != nil {
		return "", err
	}

	return handler.GenerateString(data)
}

// init registers default handlers
func init() {
	// Register will be called from individual packages
}
