package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dotenvcloud/cli/internal/config/crypto"
	"github.com/dotenvcloud/cli/internal/constants"
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

	// Decrypt sensitive credentials in accounts. OAuth tokens are at least as
	// sensitive as API keys (a refresh token grants 60 days of access) so they
	// are encrypted at rest too. Token decryption is tolerant of plaintext
	// values left by configs written before tokens were encrypted, so upgrading
	// the CLI never bricks an existing login.
	for name := range config.Accounts {
		account := config.Accounts[name]
		if account.Auth.APIKey != "" {
			decrypted, err := l.crypto.Decrypt(account.Auth.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt API key for account %s: %w", name, err)
			}
			account.Auth.APIKey = decrypted
		}
		account.Auth.AccessToken = l.tryDecrypt(account.Auth.AccessToken)
		account.Auth.RefreshToken = l.tryDecrypt(account.Auth.RefreshToken)
		config.Accounts[name] = account
	}

	// Validate configuration
	if err := l.validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save encrypts and writes the configuration. Writes are guarded by an
// advisory file lock so concurrent CLI invocations don't race the atomic
// rename and silently drop one writer's edits.
func (l *Loader) Save(config *Config) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	lock, err := acquireFileLock(l.path+".lock", 5*time.Second, 30*time.Second)
	if err != nil {
		return err
	}
	defer lock.release()

	// Create a copy to avoid modifying the original
	configCopy := *config
	configCopy.Accounts = make(map[string]Account)

	// Encrypt sensitive credentials in accounts (API key + OAuth tokens).
	for name := range config.Accounts {
		account := config.Accounts[name]
		accountCopy := account
		if account.Auth.APIKey != "" {
			encrypted, encErr := l.crypto.Encrypt(account.Auth.APIKey)
			if encErr != nil {
				return fmt.Errorf("failed to encrypt API key for account %s: %w", name, encErr)
			}
			accountCopy.Auth.APIKey = encrypted
		}
		if account.Auth.AccessToken != "" {
			encrypted, encErr := l.crypto.Encrypt(account.Auth.AccessToken)
			if encErr != nil {
				return fmt.Errorf("failed to encrypt access token for account %s: %w", name, encErr)
			}
			accountCopy.Auth.AccessToken = encrypted
		}
		if account.Auth.RefreshToken != "" {
			encrypted, encErr := l.crypto.Encrypt(account.Auth.RefreshToken)
			if encErr != nil {
				return fmt.Errorf("failed to encrypt refresh token for account %s: %w", name, encErr)
			}
			accountCopy.Auth.RefreshToken = encrypted
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

	for name := range config.Accounts {
		account := config.Accounts[name]
		if err := l.validateAccount(name, &account); err != nil {
			return err
		}
		config.Accounts[name] = account
	}

	if config.CurrentAccount != "" {
		if _, exists := config.Accounts[config.CurrentAccount]; !exists {
			return fmt.Errorf("current account '%s' not found", config.CurrentAccount)
		}
	}

	return nil
}

func (l *Loader) validateAccount(name string, account *Account) error {
	if account.AuthType != constants.AuthTypeOAuth && account.AuthType != constants.AuthTypeAPIKey {
		return fmt.Errorf("account %s: invalid auth type '%s'", name, account.AuthType)
	}

	if account.APIURL == "" {
		account.APIURL = constants.LegacyAPIURL
	}

	if account.AuthType == constants.AuthTypeOAuth {
		return validateOAuthAccount(name, account)
	}
	return validateAPIKeyAccount(name, account)
}

func validateOAuthAccount(name string, account *Account) error {
	if len(account.Organizations) == 0 {
		return fmt.Errorf("account %s: OAuth account has no organizations", name)
	}
	if account.CurrentOrganization == "" {
		return nil
	}
	for _, org := range account.Organizations {
		if org.ULID == account.CurrentOrganization {
			return nil
		}
	}
	return fmt.Errorf("account %s: current organization '%s' not found", name, account.CurrentOrganization)
}

func validateAPIKeyAccount(name string, account *Account) error {
	if account.Organization == nil {
		return fmt.Errorf("account %s: API key account missing organization info", name)
	}
	if account.Auth.APIKey == "" {
		return fmt.Errorf("account %s: API key account missing API key", name)
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
	// 0600: the backup is a full copy of the config (encrypted credentials and
	// all) and must not be world-readable like os.Create's umask default.
	dest, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
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

// tryDecrypt decrypts a stored credential, returning the original value
// unchanged when it cannot be decrypted (e.g. a plaintext value written by a
// config from before OAuth tokens were encrypted). This makes the move to
// encrypting tokens backward-compatible without a migration step.
func (l *Loader) tryDecrypt(value string) string {
	if value == "" {
		return value
	}
	if decrypted, err := l.crypto.Decrypt(value); err == nil {
		return decrypted
	}
	return value
}

// Path returns the configuration file path
func (l *Loader) Path() string {
	return l.path
}
