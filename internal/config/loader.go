package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dotenv/cli/internal/config/crypto"
	"github.com/dotenv/cli/internal/constants"
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

	// Check for legacy configuration
	if config.HasLegacyConfig() {
		return nil, fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
	}

	// Decrypt API keys in accounts
	for name, account := range config.Accounts {
		if account.Auth.APIKey != "" {
			decrypted, err := l.crypto.Decrypt(account.Auth.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt API key for account %s: %w", name, err)
			}
			account.Auth.APIKey = decrypted
			config.Accounts[name] = account
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
	configCopy.Accounts = make(map[string]Account)

	// Encrypt API keys in accounts
	for name, account := range config.Accounts {
		accountCopy := account
		if account.Auth.APIKey != "" {
			encrypted, err := l.crypto.Encrypt(account.Auth.APIKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key for account %s: %w", name, err)
			}
			accountCopy.Auth.APIKey = encrypted
		}
		configCopy.Accounts[name] = accountCopy
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

	// Validate accounts
	for name, account := range config.Accounts {
		// Validate auth type
		if account.AuthType != constants.AuthTypeOAuth && account.AuthType != constants.AuthTypeAPIKey {
			return fmt.Errorf("account %s: invalid auth type '%s'", name, account.AuthType)
		}

		// Set default API URL if not specified
		if account.APIURL == "" {
			account.APIURL = constants.LegacyAPIURL
			config.Accounts[name] = account
		}

		// Validate OAuth accounts
		if account.AuthType == constants.AuthTypeOAuth {
			if len(account.Organizations) == 0 {
				return fmt.Errorf("account %s: OAuth account has no organizations", name)
			}
			if account.CurrentOrganization != "" {
				// Verify current org exists
				found := false
				for _, org := range account.Organizations {
					if org.ULID == account.CurrentOrganization {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("account %s: current organization '%s' not found", name, account.CurrentOrganization)
				}
			}
		}

		// Validate API key accounts
		if account.AuthType == constants.AuthTypeAPIKey {
			if account.Organization == nil {
				return fmt.Errorf("account %s: API key account missing organization info", name)
			}
			if account.Auth.APIKey == "" {
				return fmt.Errorf("account %s: API key account missing API key", name)
			}
		}
	}

	// Validate current account
	if config.CurrentAccount != "" {
		if _, exists := config.Accounts[config.CurrentAccount]; !exists {
			return fmt.Errorf("current account '%s' not found", config.CurrentAccount)
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
