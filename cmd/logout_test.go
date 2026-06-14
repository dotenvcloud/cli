package cmd_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenvcloud/cli/cmd"
	"github.com/dotenvcloud/cli/internal/config"
)

// newRevokeServer returns an httptest server that counts /api/oauth/revoke hits.
func newRevokeServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/revoke" {
			atomic.AddInt32(hits, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// newAccountManagerInTemp points config resolution at a temp HOME and returns a
// fresh AccountManager rooted there.
func newAccountManagerInTemp(t *testing.T) *config.AccountManager {
	t.Helper()
	t.Setenv("DOTENV_CONFIG_DIR", "")
	t.Setenv("HOME", t.TempDir())

	configPath, err := config.ConfigPath()
	require.NoError(t, err)
	am, err := config.NewAccountManager(configPath)
	require.NoError(t, err)
	return am
}

func addOAuthAccount(t *testing.T, am *config.AccountManager, name, apiURL string) {
	t.Helper()
	err := am.CreateWithOAuth(
		name,
		apiURL,
		config.TokenResponse{
			AccessToken:  "access-" + name,
			RefreshToken: "refresh-" + name,
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		},
		[]config.OrgInfo{{ULID: "01hzzzzzzzzzzzzzzzzzzzzzz1", Name: "Org"}},
		"01hzzzzzzzzzzzzzzzzzzzzzz1",
	)
	require.NoError(t, err)
}

func TestLogoutCommand_RevokesAndRemovesCurrent(t *testing.T) {
	var hits int32
	server := newRevokeServer(t, &hits)
	defer server.Close()

	am := newAccountManagerInTemp(t)
	addOAuthAccount(t, am, "work", server.URL)
	require.NoError(t, am.Use("work"))

	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"logout", "--force", "--all=false"})
	require.NoError(t, rootCmd.Execute())

	// Server token was revoked.
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	// Account is gone locally.
	configPath, _ := config.ConfigPath()
	reloaded, err := config.NewAccountManager(configPath)
	require.NoError(t, err)
	assert.Empty(t, reloaded.List())
}

func TestLogoutCommand_All(t *testing.T) {
	var hits int32
	server := newRevokeServer(t, &hits)
	defer server.Close()

	am := newAccountManagerInTemp(t)
	addOAuthAccount(t, am, "work", server.URL)
	addOAuthAccount(t, am, "personal", server.URL)

	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"logout", "--force", "--all=true"})
	require.NoError(t, rootCmd.Execute())

	// Both tokens revoked.
	assert.Equal(t, int32(2), atomic.LoadInt32(&hits))

	configPath, _ := config.ConfigPath()
	reloaded, err := config.NewAccountManager(configPath)
	require.NoError(t, err)
	assert.Empty(t, reloaded.List())
}

func TestLogoutCommand_NamedAccount(t *testing.T) {
	var hits int32
	server := newRevokeServer(t, &hits)
	defer server.Close()

	am := newAccountManagerInTemp(t)
	addOAuthAccount(t, am, "work", server.URL)
	addOAuthAccount(t, am, "personal", server.URL)
	require.NoError(t, am.Use("personal"))

	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"logout", "work", "--force", "--all=false"})
	require.NoError(t, rootCmd.Execute())

	// Only the named account's token revoked.
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	configPath, _ := config.ConfigPath()
	reloaded, err := config.NewAccountManager(configPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"personal"}, reloaded.List())
}
