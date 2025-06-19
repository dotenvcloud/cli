package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dotenv/cli/internal/config/crypto"
	"gopkg.in/yaml.v3"
)

// Loader handles configuration loading and saving
type Loader struct {
	path   string
	crypto *crypto.SimpleCrypto
}

// NewLoader creates a new configuration loader
func NewLoader(path string) *Loader {
	if path == "" {
		// Check for DOTENV_CONFIG_DIR environment variable
		if configDir := os.Getenv("DOTENV_CONFIG_DIR"); configDir != "" {
			path = filepath.Join(configDir, "config.yaml")
		} else {
			path, _ = ConfigPath()
		}
	}

	return &Loader{
		path:   path,
		crypto: crypto.NewSimpleCrypto(),
	}
}

// Load reads and decrypts the configuration
func (l *Loader) Load() (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(l.path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	// Read file
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Decrypt API keys
	for name, ctx := range config.Contexts {
		if ctx.APIKey != "" {
			decrypted, err := l.crypto.Decrypt(ctx.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt API key for context %s: %w", name, err)
			}
			ctx.APIKey = decrypted
			config.Contexts[name] = ctx
		}
	}

	// Validate configuration
	if err := l.validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save encrypts and writes the configuration
func (l *Loader) Save(config *Config) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create a copy to avoid modifying the original
	configCopy := *config
	configCopy.Contexts = make(map[string]Context)

	// Encrypt API keys
	for name, ctx := range config.Contexts {
		ctxCopy := ctx
		if ctx.APIKey != "" {
			encrypted, err := l.crypto.Encrypt(ctx.APIKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key for context %s: %w", name, err)
			}
			ctxCopy.APIKey = encrypted
		}
		configCopy.Contexts[name] = ctxCopy
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&configCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temporary file first
	tmpPath := l.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, l.path); err != nil {
		os.Remove(tmpPath) // Clean up
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Exists checks if configuration file exists
func (l *Loader) Exists() bool {
	_, err := os.Stat(l.path)
	return err == nil
}

// validate ensures the configuration is valid
func (l *Loader) validate(config *Config) error {
	if config.Version == "" {
		return fmt.Errorf("missing version")
	}

	// Validate contexts
	for name, ctx := range config.Contexts {
		if ctx.Organization == "" {
			return fmt.Errorf("context %s: missing organization", name)
		}
		if ctx.APIURL == "" {
			// Set default API URL if not specified
			ctx.APIURL = "https://api.dotenv.com"
			config.Contexts[name] = ctx
		}
	}

	// Validate current context
	if config.CurrentContext != "" {
		if _, exists := config.Contexts[config.CurrentContext]; !exists {
			return fmt.Errorf("current context '%s' not found", config.CurrentContext)
		}
	}

	return nil
}

// Backup creates a backup of the current configuration
func (l *Loader) Backup() error {
	if !l.Exists() {
		return nil // Nothing to backup
	}

	source, err := os.Open(l.path)
	if err != nil {
		return err
	}
	defer source.Close()

	backupPath := l.path + ".backup"
	dest, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

// RestoreBackup restores configuration from backup
func (l *Loader) RestoreBackup() error {
	backupPath := l.path + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	return os.Rename(backupPath, l.path)
}

// Path returns the configuration file path
func (l *Loader) Path() string {
	return l.path
}
