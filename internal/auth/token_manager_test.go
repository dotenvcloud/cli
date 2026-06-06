package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/constants"
)

// setupTestConfig creates a test configuration directory with a default config file
func setupTestConfig(t *testing.T) *config.AccountManager {
	configDir := t.TempDir()

	// Create a default config file
	defaultConfig := config.DefaultConfig()
	configFile := filepath.Join(configDir, constants.ConfigFileName)
	data, err := yaml.Marshal(defaultConfig)
	assert.NoError(t, err)
	err = os.WriteFile(configFile, data, 0600)
	assert.NoError(t, err)

	// Pass the full path to the config file, not just the directory
	am, err := config.NewAccountManager(configFile)
	assert.NoError(t, err)

	return am
}

func TestTokenManager_NewTokenManager(t *testing.T) {
	am := setupTestConfig(t)

	tm := NewTokenManager(am)
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.accountManager)
}

func TestTokenManager_RefreshTokenIfNeeded_APIKey(t *testing.T) {
	ctx := context.Background()
	am := setupTestConfig(t)

	tm := NewTokenManager(am)

	// Test with API key account - should not need refresh
	account := &config.Account{
		AuthType: constants.AuthTypeAPIKey,
		Auth: config.AuthData{
			APIKey: "test-api-key",
		},
	}

	err := tm.RefreshTokenIfNeeded(ctx, account)
	assert.NoError(t, err)
}

func TestTokenManager_RefreshTokenIfNeeded_ValidToken(t *testing.T) {
	ctx := context.Background()
	am := setupTestConfig(t)

	tm := NewTokenManager(am)

	// Test with valid OAuth token - should not need refresh
	account := &config.Account{
		AuthType: constants.AuthTypeOAuth,
		Auth: config.AuthData{
			AccessToken: "valid-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
	}

	err := tm.RefreshTokenIfNeeded(ctx, account)
	assert.NoError(t, err)
}

func TestTokenManager_RefreshTokenIfNeeded_ExpiredRefreshToken(t *testing.T) {
	ctx := context.Background()
	am := setupTestConfig(t)

	tm := NewTokenManager(am)

	// Test with expired refresh token
	account := &config.Account{
		AuthType: constants.AuthTypeOAuth,
		Auth: config.AuthData{
			AccessToken:           "expired-token",
			RefreshToken:          "expired-refresh",
			ExpiresAt:             time.Now().Add(-1 * time.Hour),
			RefreshTokenExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	err := tm.RefreshTokenIfNeeded(ctx, account)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")
}

func TestTokenManager_RefreshOrganizationsIfNeeded_NoRefreshNeeded(t *testing.T) {
	ctx := context.Background()
	am := setupTestConfig(t)

	tm := NewTokenManager(am)

	// Test with recently fetched organizations
	fetchedAt := time.Now().Add(-1 * time.Hour)
	account := &config.Account{
		AuthType: constants.AuthTypeOAuth,
		Auth: config.AuthData{
			AccessToken: "valid-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
		OrganizationsFetchedAt: &fetchedAt,
	}

	orgRemoved, err := tm.RefreshOrganizationsIfNeeded(ctx, account)
	assert.NoError(t, err)
	assert.False(t, orgRemoved)
}
