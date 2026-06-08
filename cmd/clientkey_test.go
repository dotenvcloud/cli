package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/ui"
)

// newEncKeyServer returns a dotenv client pointed at a server whose
// encryption-key endpoint either serves a server-managed key (content-wrapped,
// matching the real API shape the SDK parses) or returns the
// client_managed_encryption 400 envelope that the SDK maps to
// dotenv.ErrClientManagedEncryption.
func newEncKeyServer(t *testing.T, clientManaged bool, key string) *dotenv.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/encryption-key") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if clientManaged {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "client_managed_encryption",
				"message": "This project uses client-side encryption.",
			})
			return
		}
		content, _ := json.Marshal(map[string]interface{}{
			"key": map[string]interface{}{"key": key, "version": 1},
		})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"type":       "encryption_keys",
				"attributes": map[string]interface{}{"content": string(content), "format": "json"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	return dotenv.NewClient(
		dotenv.WithBaseURL(srv.URL),
		dotenv.WithAPIKey("test-key"),
		dotenv.WithOrganization("test-org"),
	)
}

// captureUI redirects ui.Stdout to a buffer for the test so warning/info output
// can be asserted.
func captureUI(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	old := ui.Stdout
	ui.Stdout = buf
	t.Cleanup(func() { ui.Stdout = old })
	return buf
}

func noPromptT(t *testing.T) func() (string, error) {
	return func() (string, error) {
		t.Error("interactive prompt should not have been called")
		return "", fmt.Errorf("unexpected prompt")
	}
}

func TestLooksLikePath(t *testing.T) {
	cases := map[string]bool{
		"./key":            true,
		"keys/app.key":     true,
		"~/secret":         true,
		`C:\keys\app`:      true,
		"app.pem":          true,
		"secret.txt":       true,
		"data.json":        true,
		"blob.b64":         true,
		".env":             true,
		"abcdef0123456789": false,
		"myrawkeyvalue":    false,
		"PROJECTKEY":       false,
	}
	for in, want := range cases {
		if got := looksLikePath(in); got != want {
			t.Errorf("looksLikePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestInterpretClientKeyFlag(t *testing.T) {
	t.Run("file path is read and trimmed", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "k.key")
		require.NoError(t, os.WriteFile(p, []byte("filekeyvalue\n"), 0o600))
		got, err := interpretClientKeyFlag(p)
		require.NoError(t, err)
		require.Equal(t, "filekeyvalue", got)
	})

	t.Run("missing path-like value errors instead of being used as a key", func(t *testing.T) {
		_, err := interpretClientKeyFlag(filepath.Join(t.TempDir(), "typo.key"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "client key file not found")
	})

	t.Run("literal value is used with a warning", func(t *testing.T) {
		buf := captureUI(t)
		got, err := interpretClientKeyFlag("rawkeyvalue123")
		require.NoError(t, err)
		require.Equal(t, "rawkeyvalue123", got)
		require.Contains(t, buf.String(), "literal argument")
	})
}

func TestResolveEncryptionKey(t *testing.T) {
	ctx := context.Background()

	t.Run("server-managed returns the server key", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "")
		client := newEncKeyServer(t, false, "serverkey123")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "", noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "serverkey123", got)
	})

	t.Run("flag file path bypasses the server", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "k.key")
		require.NoError(t, os.WriteFile(p, []byte("flagfilekey\n"), 0o600))
		client := newEncKeyServer(t, true, "") // client-managed; must NOT be consulted
		got, err := resolveEncryptionKey(ctx, client, "test-project", p, noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "flagfilekey", got)
	})

	t.Run("client-managed uses DOTENV_CLIENT_KEY with a warning", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "envkeyvalue")
		buf := captureUI(t)
		client := newEncKeyServer(t, true, "")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "", noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "envkeyvalue", got)
		require.Contains(t, buf.String(), config.EnvClientKey)
	})

	t.Run("client-managed falls back to the prompt", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "")
		_ = captureUI(t)
		client := newEncKeyServer(t, true, "")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "",
			func() (string, error) { return "promptedkey", nil })
		require.NoError(t, err)
		require.Equal(t, "promptedkey", got)
	})

	t.Run("client-managed prompt error surfaces the client-managed message", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "")
		_ = captureUI(t)
		client := newEncKeyServer(t, true, "")
		_, err := resolveEncryptionKey(ctx, client, "test-project", "",
			func() (string, error) { return "", fmt.Errorf("no tty") })
		require.Error(t, err)
		require.Contains(t, err.Error(), "client-managed encryption")
	})
}

func TestResolvePushEncryptionKeyPlaintextGuard(t *testing.T) {
	ctx := context.Background()

	t.Run("encrypt=false on a client-managed project is blocked", func(t *testing.T) {
		origEnc, origKey := pushEncrypt, pushClientKey
		t.Cleanup(func() { pushEncrypt, pushClientKey = origEnc, origKey })
		pushEncrypt = false
		pushClientKey = ""
		_ = captureUI(t)
		client := newEncKeyServer(t, true, "")
		_, err := resolvePushEncryptionKey(ctx, client, "test-project")
		require.Error(t, err)
		require.Contains(t, err.Error(), "plaintext")
	})

	t.Run("encrypt=false on a server-managed project returns an empty key", func(t *testing.T) {
		origEnc := pushEncrypt
		t.Cleanup(func() { pushEncrypt = origEnc })
		pushEncrypt = false
		client := newEncKeyServer(t, false, "serverkey")
		got, err := resolvePushEncryptionKey(ctx, client, "test-project")
		require.NoError(t, err)
		require.Equal(t, "", got)
	})
}
