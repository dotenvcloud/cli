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

// encKeyTestSalt / encKeyTestIters are the fixed client-managed proof params the
// mock server returns, so tests can derive the expected proof deterministically.
const (
	encKeyTestSalt  = "AAAAAAAAAAAAAAAAAAAAAA==" // 16 zero bytes, base64
	encKeyTestIters = 600000
)

// newEncKeyServer returns a dotenv client pointed at a server whose
// encryption-key endpoint serves the 200 "managed" descriptor: a server-managed
// key (with the raw key) or a client-managed descriptor (proof params only, no
// key), matching the real API shape the SDK parses.
func newEncKeyServer(t *testing.T, clientManaged bool, key string) *dotenv.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/encryption-key") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var keyObj map[string]interface{}
		if clientManaged {
			keyObj = map[string]interface{}{
				"managed":              "client",
				"version":              1,
				"key_check_salt":       encKeyTestSalt,
				"key_check_iterations": encKeyTestIters,
			}
		} else {
			// Server-managed now also returns the data-key salt/iterations (unified
			// PBKDF2 derivation) alongside the key value.
			keyObj = map[string]interface{}{
				"managed":              "server",
				"key":                  key,
				"version":              1,
				"key_check_salt":       encKeyTestSalt,
				"key_check_iterations": encKeyTestIters,
			}
		}
		content, _ := json.Marshal(map[string]interface{}{"key": keyObj})
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
		require.Equal(t, "serverkey123", got.key)
		require.Equal(t, "server", got.managed)
	})

	t.Run("server-managed carries the data-key salt and derives via PBKDF2", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "")
		client := newEncKeyServer(t, false, "serverkey123")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "", noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "server", got.managed)
		require.Equal(t, encKeyTestSalt, got.proofSalt)
		require.Equal(t, encKeyTestIters, got.proofIters)

		// Unified derivation: the AES key is PBKDF2(key, salt, iters) — identical to
		// the browser — NOT the raw server key.
		dk, err := got.dataKey()
		require.NoError(t, err)
		want, err := dotenv.DeriveDataKey("serverkey123", encKeyTestSalt, encKeyTestIters)
		require.NoError(t, err)
		require.Equal(t, string(want), dk)
		require.NotEqual(t, "serverkey123", dk)
	})

	t.Run("client-managed flag file supplies the key and carries the proof params", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "k.key")
		require.NoError(t, os.WriteFile(p, []byte("flagfilekey\n"), 0o600))
		client := newEncKeyServer(t, true, "")
		got, err := resolveEncryptionKey(ctx, client, "test-project", p, noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "flagfilekey", got.key)
		require.Equal(t, "client", got.managed)
		require.Equal(t, encKeyTestSalt, got.proofSalt)
		require.Equal(t, encKeyTestIters, got.proofIters)
	})

	t.Run("client-managed uses DOTENV_CLIENT_KEY with a warning", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "envkeyvalue")
		buf := captureUI(t)
		client := newEncKeyServer(t, true, "")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "", noPromptT(t))
		require.NoError(t, err)
		require.Equal(t, "envkeyvalue", got.key)
		require.Contains(t, buf.String(), config.EnvClientKey)
	})

	t.Run("client-managed falls back to the prompt", func(t *testing.T) {
		t.Setenv(config.EnvClientKey, "")
		_ = captureUI(t)
		client := newEncKeyServer(t, true, "")
		got, err := resolveEncryptionKey(ctx, client, "test-project", "",
			func() (string, error) { return "promptedkey", nil })
		require.NoError(t, err)
		require.Equal(t, "promptedkey", got.key)
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
		_, _, err := resolvePushEncryptionKey(ctx, client, "test-project")
		require.Error(t, err)
		require.Contains(t, err.Error(), "plaintext")
	})

	t.Run("encrypt=false on a server-managed project returns an empty key", func(t *testing.T) {
		origEnc := pushEncrypt
		t.Cleanup(func() { pushEncrypt = origEnc })
		pushEncrypt = false
		client := newEncKeyServer(t, false, "serverkey")
		got, proof, err := resolvePushEncryptionKey(ctx, client, "test-project")
		require.NoError(t, err)
		require.Equal(t, "", got)
		require.Equal(t, "", proof)
	})

	t.Run("client-managed push derives the key proof from the descriptor params", func(t *testing.T) {
		origEnc, origKey := pushEncrypt, pushClientKey
		t.Cleanup(func() { pushEncrypt, pushClientKey = origEnc, origKey })
		pushEncrypt = true
		pushClientKey = "myclientkey" // literal value
		_ = captureUI(t)
		client := newEncKeyServer(t, true, "")

		encKey, proof, err := resolvePushEncryptionKey(ctx, client, "test-project")
		require.NoError(t, err)

		// Client-managed encryption now uses the PBKDF2-derived AES key, not the
		// raw passphrase, so a weak passphrase is salted and stretched.
		wantKey, kerr := dotenv.DeriveDataKey("myclientkey", encKeyTestSalt, encKeyTestIters)
		require.NoError(t, kerr)
		require.Equal(t, string(wantKey), encKey)

		want, derr := dotenv.DeriveKeyProof("myclientkey", encKeyTestSalt, encKeyTestIters)
		require.NoError(t, derr)
		require.Equal(t, want, proof)
	})
}
