package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "1.0", cfg.Version)
	assert.False(t, cfg.TelemetryEnabled)
	assert.Empty(t, cfg.CurrentContext)
	assert.NotNil(t, cfg.Contexts)
	assert.Equal(t, "env", cfg.Preferences.DefaultFormat)
	assert.True(t, cfg.Preferences.ColorOutput)
}

func TestConfigContextManagement(t *testing.T) {
	cfg := DefaultConfig()

	// Add context
	ctx := Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "test-key",
		Organization: "test-org",
	}

	err := cfg.AddContext("test", ctx)
	require.NoError(t, err)

	// Should be set as current
	assert.Equal(t, "test", cfg.CurrentContext)

	// Get current context
	current, err := cfg.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "test-org", current.Organization)

	// Add another context
	err = cfg.AddContext("prod", Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "prod-key",
		Organization: "prod-org",
	})
	require.NoError(t, err)

	// Current should still be "test"
	assert.Equal(t, "test", cfg.CurrentContext)

	// Switch context
	err = cfg.SetCurrentContext("prod")
	require.NoError(t, err)
	assert.Equal(t, "prod", cfg.CurrentContext)

	// List contexts
	contexts := cfg.ListContexts()
	assert.Len(t, contexts, 2)
	assert.Contains(t, contexts, "test")
	assert.Contains(t, contexts, "prod")

	// Remove context
	err = cfg.RemoveContext("test")
	require.NoError(t, err)
	assert.Len(t, cfg.Contexts, 1)

	// Try to remove non-existent
	err = cfg.RemoveContext("test")
	assert.Error(t, err)
}

func TestConfigRenameContext(t *testing.T) {
	cfg := DefaultConfig()

	// Add context
	ctx := Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "test-key",
		Organization: "test-org",
	}

	err := cfg.AddContext("test", ctx)
	require.NoError(t, err)

	// Rename context
	err = cfg.RenameContext("test", "production")
	require.NoError(t, err)

	// Old name should not exist
	_, exists := cfg.Contexts["test"]
	assert.False(t, exists)

	// New name should exist
	renamedCtx, exists := cfg.Contexts["production"]
	assert.True(t, exists)
	assert.Equal(t, "production", renamedCtx.Name)
	assert.Equal(t, "test-org", renamedCtx.Organization)

	// Current context should be updated
	assert.Equal(t, "production", cfg.CurrentContext)

	// Try to rename to existing name
	err = cfg.AddContext("staging", Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "staging-key",
		Organization: "staging-org",
	})
	require.NoError(t, err)

	err = cfg.RenameContext("staging", "production")
	assert.Error(t, err)
}

func TestLoaderSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	loader := NewLoader(configPath)

	// Create config
	cfg := DefaultConfig()
	cfg.AddContext("test", Context{
		APIURL:       "https://api.dotenv.test",
		APIKey:       "test-api-key",
		Organization: "test-org",
	})

	// Save
	err := loader.Save(cfg)
	require.NoError(t, err)

	// Verify file permissions
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load
	loaded, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, cfg.Version, loaded.Version)
	assert.Equal(t, cfg.CurrentContext, loaded.CurrentContext)
	assert.Len(t, loaded.Contexts, 1)

	// Verify encryption worked
	ctx, _ := loaded.GetCurrentContext()
	assert.Equal(t, "test-api-key", ctx.APIKey)
}

func TestValidator(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		fn      func() error
		wantErr bool
	}{
		{
			name:    "valid API key",
			fn:      func() error { return v.ValidateAPIKey("dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_abcdef123456") },
			wantErr: false,
		},
		{
			name:    "invalid API key",
			fn:      func() error { return v.ValidateAPIKey("invalid-key") },
			wantErr: true,
		},
		{
			name:    "valid organization",
			fn:      func() error { return v.ValidateOrganization("my-org-123") },
			wantErr: false,
		},
		{
			name:    "invalid organization with spaces",
			fn:      func() error { return v.ValidateOrganization("my org") },
			wantErr: true,
		},
		{
			name:    "organization too short",
			fn:      func() error { return v.ValidateOrganization("ab") },
			wantErr: true,
		},
		{
			name:    "organization with consecutive hyphens",
			fn:      func() error { return v.ValidateOrganization("my--org") },
			wantErr: true,
		},
		{
			name:    "valid API URL",
			fn:      func() error { return v.ValidateAPIURL("https://api.dotenv.com") },
			wantErr: false,
		},
		{
			name:    "valid dev API URL",
			fn:      func() error { return v.ValidateAPIURL("https://dotenv.test") },
			wantErr: false,
		},
		{
			name:    "invalid API URL",
			fn:      func() error { return v.ValidateAPIURL("not-a-url") },
			wantErr: true,
		},
		{
			name:    "valid context name",
			fn:      func() error { return v.ValidateContextName("my-context_123.prod") },
			wantErr: false,
		},
		{
			name:    "invalid context name with spaces",
			fn:      func() error { return v.ValidateContextName("my context") },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("DOTENV_API_KEY", "env-key")
	os.Setenv("DOTENV_ORGANIZATION", "env-org")
	os.Setenv("DOTENV_DEBUG", "true")
	os.Setenv("DOTENV_CONFIG_DIR", "/custom/path")
	defer func() {
		os.Unsetenv("DOTENV_API_KEY")
		os.Unsetenv("DOTENV_ORGANIZATION")
		os.Unsetenv("DOTENV_DEBUG")
		os.Unsetenv("DOTENV_CONFIG_DIR")
	}()

	env := LoadEnvConfig()
	assert.Equal(t, "env-key", env.APIKey)
	assert.Equal(t, "env-org", env.Organization)
	assert.True(t, env.Debug)
	assert.Equal(t, "/custom/path", env.ConfigDir)
	assert.True(t, env.HasOverrides())

	// Apply to context
	ctx := &Context{
		APIKey:       "original-key",
		Organization: "original-org",
		APIURL:       "https://api.dotenv.com",
	}

	updated := env.Apply(ctx)
	assert.Equal(t, "env-key", updated.APIKey)
	assert.Equal(t, "env-org", updated.Organization)
}

func TestContextManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewContextManager(configPath)
	require.NoError(t, err)

	// Create context
	err = cm.Create("", "https://api.dotenv.com", "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_test123", "test-org")
	require.NoError(t, err)

	// Should auto-name from org
	contexts := cm.List()
	assert.Len(t, contexts, 1)
	assert.Equal(t, "test-org", contexts[0].Name)

	// Update context
	err = cm.Update("test-org", map[string]interface{}{
		"api_url": "https://api.dotenv.test",
	})
	require.NoError(t, err)

	// Get updated context
	ctx, err := cm.GetContext("test-org")
	require.NoError(t, err)
	assert.Equal(t, "https://api.dotenv.test", ctx.APIURL)

	// Rename context
	err = cm.Rename("test-org", "production")
	require.NoError(t, err)

	// Verify rename
	contexts = cm.List()
	assert.Len(t, contexts, 1)
	assert.Equal(t, "production", contexts[0].Name)
}

func TestManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create initial config
	loader := NewLoader(configPath)
	cfg := DefaultConfig()
	cfg.AddContext("test", Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "test-key",
		Organization: "test-org",
	})
	err := loader.Save(cfg)
	require.NoError(t, err)

	// Create manager
	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Get current context
	ctx, err := mgr.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "test-org", ctx.Organization)

	// Update telemetry
	err = mgr.SetTelemetryOptIn(true)
	require.NoError(t, err)
	assert.True(t, mgr.GetTelemetryOptIn())

	// Update preferences
	err = mgr.UpdatePreferences(map[string]interface{}{
		"default_format": "json",
		"color_output":   false,
	})
	require.NoError(t, err)

	// Reload and verify
	err = mgr.Reload()
	require.NoError(t, err)
	assert.Equal(t, "json", mgr.GetDefaultFormat())
}

func TestConfigPathWithEnvOverride(t *testing.T) {
	// Set custom config dir
	customDir := "/custom/config/dir"
	os.Setenv("DOTENV_CONFIG_DIR", customDir)
	defer os.Unsetenv("DOTENV_CONFIG_DIR")

	loader := NewLoader("")
	expectedPath := filepath.Join(customDir, "config.yaml")
	assert.Equal(t, expectedPath, loader.Path())
}

func TestBackupRestore(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	loader := NewLoader(configPath)

	// Create and save config
	cfg := DefaultConfig()
	cfg.AddContext("test", Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "test-key",
		Organization: "test-org",
	})
	err := loader.Save(cfg)
	require.NoError(t, err)

	// Create backup
	err = loader.Backup()
	require.NoError(t, err)

	// Modify config
	cfg.AddContext("prod", Context{
		APIURL:       "https://api.dotenv.com",
		APIKey:       "prod-key",
		Organization: "prod-org",
	})
	err = loader.Save(cfg)
	require.NoError(t, err)

	// Restore backup
	err = loader.RestoreBackup()
	require.NoError(t, err)

	// Load and verify
	restored, err := loader.Load()
	require.NoError(t, err)
	assert.Len(t, restored.Contexts, 1)
	assert.Contains(t, restored.Contexts, "test")
	assert.NotContains(t, restored.Contexts, "prod")
}

func TestContextInfo(t *testing.T) {
	info := ContextInfo{
		Name:         "production",
		Organization: "my-org",
		APIURL:       "https://api.dotenv.com",
		UserEmail:    "user@example.com",
		Current:      true,
		LastUpdate:   time.Now(),
	}

	str := info.String()
	assert.Contains(t, str, "Name: production (current)")
	assert.Contains(t, str, "Organization: my-org")
	assert.Contains(t, str, "User: user@example.com")
	assert.NotContains(t, str, "API URL:") // Default URL should not be shown

	// Custom API URL
	info.APIURL = "https://api.dotenv.test"
	str = info.String()
	assert.Contains(t, str, "API URL: https://api.dotenv.test")
}
