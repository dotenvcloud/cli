//go:build integration
// +build integration

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationFullWorkflow(t *testing.T) {
	// Skip if not integration test
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create manager
	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Get context manager
	cm := mgr.GetContextManager()

	// Create first context
	err = cm.Create("dev", "https://api.dotenv.test", "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_dev123", "dev-org")
	require.NoError(t, err)

	// Create second context
	err = cm.Create("prod", "https://api.dotenv.cloud", "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_prod456", "prod-org")
	require.NoError(t, err)

	// Switch to prod
	err = cm.Use("prod")
	require.NoError(t, err)

	// Reload manager
	err = mgr.Reload()
	require.NoError(t, err)

	// Verify current context
	ctx, err := mgr.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "prod-org", ctx.Organization)
	assert.Equal(t, "https://api.dotenv.cloud", ctx.APIURL)

	// Update telemetry
	err = mgr.SetTelemetryOptIn(true)
	require.NoError(t, err)

	// Test environment override
	os.Setenv("DOTENV_API_KEY", "override-key")
	defer os.Unsetenv("DOTENV_API_KEY")

	// Reload to pick up env changes
	mgr2, err := NewManager(configPath)
	require.NoError(t, err)

	ctx2, err := mgr2.GetCurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "override-key", ctx2.APIKey)

	// List all contexts
	contexts := cm.List()
	assert.Len(t, contexts, 2)

	// Rename context
	err = cm.Rename("dev", "development")
	require.NoError(t, err)

	// Delete context
	err = cm.Delete("development")
	require.NoError(t, err)

	// Verify only one context remains
	contexts = cm.List()
	assert.Len(t, contexts, 1)
	assert.Equal(t, "prod", contexts[0].Name)
}
