package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// Manager is the main configuration manager
type Manager struct {
	config         *Config
	loader         *Loader
	envConfig      *EnvConfig
	contextMgr     *ContextManager
	mu             sync.RWMutex
	telemetryOptIn bool
}

// NewManager creates a new configuration manager
func NewManager(configPath string) (*Manager, error) {
	loader := NewLoader(configPath)
	config, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	envConfig := LoadEnvConfig()

	// Apply environment overrides
	if envConfig.HasOverrides() {
		config, err = envConfig.ApplyToConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
		}
	}

	contextMgr, err := NewContextManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create context manager: %w", err)
	}

	return &Manager{
		config:         config,
		loader:         loader,
		envConfig:      envConfig,
		contextMgr:     contextMgr,
		telemetryOptIn: config.TelemetryEnabled,
	}, nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Save saves the current configuration
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loader.Save(m.config)
}

// Reload reloads the configuration from disk
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, err := m.loader.Load()
	if err != nil {
		return err
	}

	// Apply environment overrides
	if m.envConfig.HasOverrides() {
		config, err = m.envConfig.ApplyToConfig(config)
		if err != nil {
			return err
		}
	}

	m.config = config
	return nil
}

// GetCurrentContext returns the current active context with env overrides applied
func (m *Manager) GetCurrentContext() (*Context, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx, err := m.config.GetCurrentContext()
	if err != nil {
		return nil, err
	}

	// Apply environment overrides
	if m.envConfig.HasOverrides() {
		ctx = m.envConfig.Apply(ctx)
	}

	return ctx, nil
}

// GetContextManager returns the context manager
func (m *Manager) GetContextManager() *ContextManager {
	return m.contextMgr
}

// SetTelemetryOptIn sets the telemetry opt-in preference
func (m *Manager) SetTelemetryOptIn(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.TelemetryEnabled = enabled
	m.telemetryOptIn = enabled
	return m.loader.Save(m.config)
}

// GetTelemetryOptIn returns the telemetry opt-in preference
func (m *Manager) GetTelemetryOptIn() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.telemetryOptIn
}

// LoadIntoViper loads configuration into Viper
func (m *Manager) LoadIntoViper() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Set defaults
	viper.SetDefault("telemetry_enabled", m.config.TelemetryEnabled)
	viper.SetDefault("color_output", m.config.Preferences.ColorOutput)
	viper.SetDefault("auto_update", m.config.Preferences.AutoUpdate)
	viper.SetDefault("update_channel", m.config.Preferences.UpdateChannel)
	viper.SetDefault("default_format", m.config.Preferences.DefaultFormat)

	// Load current context
	if ctx, err := m.GetCurrentContext(); err == nil {
		viper.Set("current_context", m.config.CurrentContext)
		viper.Set("api_url", ctx.APIURL)
		viper.Set("organization", ctx.Organization)
		// Don't set API key in viper for security
	}

	// Load environment overrides
	if m.envConfig.Debug {
		viper.Set("debug", true)
	}
	if m.envConfig.NoColor {
		viper.Set("no_color", true)
	}
	if m.envConfig.Quiet {
		viper.Set("quiet", true)
	}
	if m.envConfig.TLSSkipVerify {
		viper.Set("tls_skip_verify", true)
	}

	return nil
}

// GetAPIURL returns the effective API URL
func (m *Manager) GetAPIURL() (string, error) {
	ctx, err := m.GetCurrentContext()
	if err != nil {
		return "", err
	}

	if ctx.APIURL == "" {
		return "https://api.dotenv.com", nil
	}

	return ctx.APIURL, nil
}

// GetAPIKey returns the effective API key
func (m *Manager) GetAPIKey() (string, error) {
	ctx, err := m.GetCurrentContext()
	if err != nil {
		return "", err
	}

	if ctx.APIKey == "" {
		return "", fmt.Errorf("no API key configured for context %s", ctx.Name)
	}

	return ctx.APIKey, nil
}

// GetOrganization returns the effective organization
func (m *Manager) GetOrganization() (string, error) {
	ctx, err := m.GetCurrentContext()
	if err != nil {
		return "", err
	}

	if ctx.Organization == "" {
		return "", fmt.Errorf("no organization configured for context %s", ctx.Name)
	}

	return ctx.Organization, nil
}

// IsDebug returns whether debug mode is enabled
func (m *Manager) IsDebug() bool {
	return m.envConfig.Debug || viper.GetBool("debug")
}

// IsQuiet returns whether quiet mode is enabled
func (m *Manager) IsQuiet() bool {
	return m.envConfig.Quiet || viper.GetBool("quiet")
}

// ShouldSkipTLSVerify returns whether TLS verification should be skipped
func (m *Manager) ShouldSkipTLSVerify() bool {
	return m.envConfig.TLSSkipVerify || viper.GetBool("tls_skip_verify")
}

// GetDefaultFormat returns the default output format
func (m *Manager) GetDefaultFormat() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if format := m.config.Preferences.DefaultFormat; format != "" {
		return format
	}
	return "env"
}

// UpdatePreferences updates user preferences
func (m *Manager) UpdatePreferences(updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, value := range updates {
		switch key {
		case "default_format":
			if v, ok := value.(string); ok {
				m.config.Preferences.DefaultFormat = v
			}
		case "color_output":
			if v, ok := value.(bool); ok {
				m.config.Preferences.ColorOutput = v
			}
		case "auto_update":
			if v, ok := value.(bool); ok {
				m.config.Preferences.AutoUpdate = v
			}
		case "update_channel":
			if v, ok := value.(string); ok {
				m.config.Preferences.UpdateChannel = v
			}
		}
	}

	return m.loader.Save(m.config)
}

// Backup creates a backup of the configuration
func (m *Manager) Backup() error {
	return m.loader.Backup()
}

// RestoreBackup restores the configuration from backup
func (m *Manager) RestoreBackup() error {
	if err := m.loader.RestoreBackup(); err != nil {
		return err
	}
	return m.Reload()
}
