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
	CurrentAccount   string             `yaml:"current_account"`
	Accounts         map[string]Account `yaml:"accounts"`
	Preferences      Preferences        `yaml:"preferences,omitempty"`
	LastUpdateCheck  *time.Time         `yaml:"last_update_check,omitempty"`
	// Legacy fields for migration detection
	CurrentContext string             `yaml:"current_context,omitempty"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
}

// Context represents a legacy DotEnv context (for migration detection)
type Context struct {
	Name                string             `yaml:"name"`
	APIURL              string             `yaml:"api_url"`
	APIKey              string             `yaml:"api_key,omitempty"`
	Organization        string             `yaml:"organization,omitempty"`
	AuthType            string             `yaml:"auth_type"`
	Auth                AuthData           `yaml:"auth,omitempty"`
	Organizations       []OrganizationInfo `yaml:"organizations,omitempty"`
	CurrentOrganization string             `yaml:"current_organization,omitempty"`
	CreatedAt           time.Time          `yaml:"created_at"`
	UpdatedAt           time.Time          `yaml:"updated_at"`
	LastUpdate          time.Time          `yaml:"last_update"`
	Metadata            Metadata           `yaml:"metadata,omitempty"`
}

// AuthData holds authentication credentials
type AuthData struct {
	// For API key auth
	APIKey string `yaml:"api_key,omitempty"`

	// For OAuth auth
	AccessToken           string    `yaml:"access_token,omitempty"`
	RefreshToken          string    `yaml:"refresh_token,omitempty"`
	TokenType             string    `yaml:"token_type,omitempty"`
	ExpiresAt             time.Time `yaml:"expires_at,omitempty"`
	RefreshTokenExpiresAt time.Time `yaml:"refresh_token_expires_at,omitempty"`
}

// OrganizationInfo represents an organization the user has access to
type OrganizationInfo struct {
	Slug string `yaml:"slug"`
	Name string `yaml:"name"`
	ID   int64  `yaml:"id,omitempty"`
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
		Accounts:         make(map[string]Account),
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

// GetCurrentAccount returns the current active account
func (c *Config) GetCurrentAccount() (*Account, error) {
	if c.CurrentAccount == "" {
		return nil, fmt.Errorf("no account selected")
	}

	acct, exists := c.Accounts[c.CurrentAccount]
	if !exists {
		return nil, fmt.Errorf("account '%s' not found", c.CurrentAccount)
	}

	return &acct, nil
}

// GetCurrentContext returns the current active context (legacy, for compatibility)
func (c *Config) GetCurrentContext() (*Context, error) {
	// This is now a legacy method that should trigger migration
	return nil, fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
}

// AddAccount adds or updates an account
func (c *Config) AddAccount(name string, account Account) error {
	if name == "" {
		return fmt.Errorf("account name cannot be empty")
	}

	account.Name = name
	account.UpdatedAt = time.Now()
	account.LastUsed = time.Now()

	if _, exists := c.Accounts[name]; !exists {
		account.CreatedAt = time.Now()
	}

	c.Accounts[name] = account

	// Set as current if it's the first account
	if len(c.Accounts) == 1 {
		c.CurrentAccount = name
	}

	return nil
}

// AddContext adds or updates a context (legacy)
func (c *Config) AddContext(name string, context Context) error {
	return fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
}

// RemoveAccount removes an account
func (c *Config) RemoveAccount(name string) error {
	if _, exists := c.Accounts[name]; !exists {
		return fmt.Errorf("Account not found: %s", name)
	}

	delete(c.Accounts, name)

	// Clear current account if it was removed
	if c.CurrentAccount == name {
		c.CurrentAccount = ""
		// Set to first available account
		for k := range c.Accounts {
			c.CurrentAccount = k
			break
		}
	}

	return nil
}

// RemoveContext removes a context (legacy)
func (c *Config) RemoveContext(name string) error {
	return fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
}

// RenameAccount renames an account
func (c *Config) RenameAccount(oldName, newName string) error {
	if oldName == "" || newName == "" {
		return fmt.Errorf("account names cannot be empty")
	}

	if oldName == newName {
		return nil
	}

	acct, exists := c.Accounts[oldName]
	if !exists {
		return fmt.Errorf("account '%s' not found", oldName)
	}

	if _, exists := c.Accounts[newName]; exists {
		return fmt.Errorf("account '%s' already exists", newName)
	}

	// Copy account with new name
	acct.Name = newName
	acct.UpdatedAt = time.Now()
	c.Accounts[newName] = acct

	// Delete old account
	delete(c.Accounts, oldName)

	// Update current account if needed
	if c.CurrentAccount == oldName {
		c.CurrentAccount = newName
	}

	return nil
}

// RenameContext renames a context (legacy)
func (c *Config) RenameContext(oldName, newName string) error {
	return fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
}

// SetCurrentAccount sets the current active account
func (c *Config) SetCurrentAccount(name string) error {
	if _, exists := c.Accounts[name]; !exists {
		return fmt.Errorf("account '%s' not found", name)
	}

	c.CurrentAccount = name
	return nil
}

// SetCurrentContext sets the current active context (legacy)
func (c *Config) SetCurrentContext(name string) error {
	return fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
}

// ListAccounts returns a list of all account names
func (c *Config) ListAccounts() []string {
	names := make([]string, 0, len(c.Accounts))
	for name := range c.Accounts {
		names = append(names, name)
	}
	return names
}

// ListContexts returns a list of all context names (legacy)
func (c *Config) ListContexts() []string {
	// Return empty list for legacy method
	return []string{}
}

// GetAPIURL returns the API URL for the current account
func (c *Config) GetAPIURL() (string, error) {
	acct, err := c.GetCurrentAccount()
	if err != nil {
		return "", err
	}

	if acct.APIURL == "" {
		return "https://api.dotenv.com", nil
	}

	return acct.APIURL, nil
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

// IsOAuthContext returns true if this is an OAuth context
func (c *Context) IsOAuthContext() bool {
	return c.AuthType == "oauth"
}

// IsAPIKeyContext returns true if this is an API key context
func (c *Context) IsAPIKeyContext() bool {
	return c.AuthType == "api_key" || c.AuthType == ""
}

// IsTokenExpired returns true if the OAuth token is expired
func (c *Context) IsTokenExpired() bool {
	if !c.IsOAuthContext() {
		return false
	}
	return time.Now().After(c.Auth.ExpiresAt)
}

// GetEffectiveAPIKey returns the API key (for backward compatibility)
func (c *Context) GetEffectiveAPIKey() string {
	if c.IsAPIKeyContext() {
		// Check new location first, then fall back to old location
		if c.Auth.APIKey != "" {
			return c.Auth.APIKey
		}
		return c.APIKey
	}
	return ""
}

// GetEffectiveOrganization returns the current organization
func (c *Context) GetEffectiveOrganization() string {
	if c.IsOAuthContext() {
		return c.CurrentOrganization
	}
	return c.Organization
}

// HasLegacyConfig checks if the config has old context-based structure
func (c *Config) HasLegacyConfig() bool {
	return len(c.Contexts) > 0 || c.CurrentContext != ""
}
