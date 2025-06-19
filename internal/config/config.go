package config

import (
	"fmt"
	"path/filepath"
	"time"
)

// Config represents the complete CLI configuration
type Config struct {
	Version          string             `yaml:"version"`
	TelemetryEnabled bool               `yaml:"telemetry_enabled"`
	CurrentContext   string             `yaml:"current_context"`
	Contexts         map[string]Context `yaml:"contexts"`
	Preferences      Preferences        `yaml:"preferences,omitempty"`
	LastUpdateCheck  *time.Time         `yaml:"last_update_check,omitempty"`
}

// Context represents a DotEnv context (organization)
type Context struct {
	Name         string    `yaml:"name"`
	APIURL       string    `yaml:"api_url"`
	APIKey       string    `yaml:"api_key"` // Encrypted
	Organization string    `yaml:"organization"`
	CreatedAt    time.Time `yaml:"created_at"`
	UpdatedAt    time.Time `yaml:"updated_at"`
	LastUpdate   time.Time `yaml:"last_update"` // Added as per requirements
	Metadata     Metadata  `yaml:"metadata,omitempty"`
}

// Preferences holds user preferences
type Preferences struct {
	DefaultFormat      string            `yaml:"default_format,omitempty"`
	ColorOutput        bool              `yaml:"color_output"`
	AutoUpdate         bool              `yaml:"auto_update"`
	UpdateChannel      string            `yaml:"update_channel,omitempty"`
	AnalyticsID        string            `yaml:"analytics_id,omitempty"`
	CustomHeaders      map[string]string `yaml:"custom_headers,omitempty"`
	DefaultPullOptions PullOptions       `yaml:"default_pull_options,omitempty"`
}

// PullOptions represents default options for pull command
type PullOptions struct {
	ResolveVariables bool   `yaml:"resolve_variables"`
	OutputFormat     string `yaml:"output_format"`
}

// Metadata holds additional context information
type Metadata struct {
	UserEmail        string   `yaml:"user_email,omitempty"`
	OrganizationID   string   `yaml:"organization_id,omitempty"`
	OrganizationName string   `yaml:"organization_name,omitempty"`
	OrganizationPlan string   `yaml:"organization_plan,omitempty"`
	Permissions      []string `yaml:"permissions,omitempty"`
}

// DefaultConfig returns a new default configuration
func DefaultConfig() *Config {
	return &Config{
		Version:          "1.0",
		TelemetryEnabled: false,
		Contexts:         make(map[string]Context),
		Preferences: Preferences{
			DefaultFormat: "env",
			ColorOutput:   true,
			AutoUpdate:    true,
			UpdateChannel: "stable",
			DefaultPullOptions: PullOptions{
				ResolveVariables: false,
				OutputFormat:     "env",
			},
		},
	}
}

// GetCurrentContext returns the current active context
func (c *Config) GetCurrentContext() (*Context, error) {
	if c.CurrentContext == "" {
		return nil, fmt.Errorf("no context selected")
	}

	ctx, exists := c.Contexts[c.CurrentContext]
	if !exists {
		return nil, fmt.Errorf("context '%s' not found", c.CurrentContext)
	}

	return &ctx, nil
}

// AddContext adds or updates a context
func (c *Config) AddContext(name string, context Context) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}

	context.Name = name
	context.UpdatedAt = time.Now()
	context.LastUpdate = time.Now() // Set last_update timestamp

	if _, exists := c.Contexts[name]; !exists {
		context.CreatedAt = time.Now()
	}

	c.Contexts[name] = context

	// Set as current if it's the first context
	if len(c.Contexts) == 1 {
		c.CurrentContext = name
	}

	return nil
}

// RemoveContext removes a context
func (c *Config) RemoveContext(name string) error {
	if _, exists := c.Contexts[name]; !exists {
		return fmt.Errorf("context '%s' not found", name)
	}

	delete(c.Contexts, name)

	// Clear current context if it was removed
	if c.CurrentContext == name {
		c.CurrentContext = ""
		// Set to first available context
		for k := range c.Contexts {
			c.CurrentContext = k
			break
		}
	}

	return nil
}

// RenameContext renames a context
func (c *Config) RenameContext(oldName, newName string) error {
	if oldName == "" || newName == "" {
		return fmt.Errorf("context names cannot be empty")
	}

	if oldName == newName {
		return nil
	}

	ctx, exists := c.Contexts[oldName]
	if !exists {
		return fmt.Errorf("context '%s' not found", oldName)
	}

	if _, exists := c.Contexts[newName]; exists {
		return fmt.Errorf("context '%s' already exists", newName)
	}

	// Copy context with new name
	ctx.Name = newName
	ctx.UpdatedAt = time.Now()
	ctx.LastUpdate = time.Now()
	c.Contexts[newName] = ctx

	// Delete old context
	delete(c.Contexts, oldName)

	// Update current context if needed
	if c.CurrentContext == oldName {
		c.CurrentContext = newName
	}

	return nil
}

// SetCurrentContext sets the current active context
func (c *Config) SetCurrentContext(name string) error {
	if _, exists := c.Contexts[name]; !exists {
		return fmt.Errorf("context '%s' not found", name)
	}

	c.CurrentContext = name
	return nil
}

// ListContexts returns a list of all context names
func (c *Config) ListContexts() []string {
	names := make([]string, 0, len(c.Contexts))
	for name := range c.Contexts {
		names = append(names, name)
	}
	return names
}

// GetAPIURL returns the API URL for the current context
func (c *Config) GetAPIURL() (string, error) {
	ctx, err := c.GetCurrentContext()
	if err != nil {
		return "", err
	}

	if ctx.APIURL == "" {
		return "https://api.dotenv.com", nil
	}

	return ctx.APIURL, nil
}

// ConfigPath returns the default config file path
func ConfigPath() (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".dotenv", "config.yaml"), nil
}

// ConfigDir returns the config directory path
func ConfigDir() (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".dotenv"), nil
}
