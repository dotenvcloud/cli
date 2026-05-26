package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestConfig holds test configuration
type TestConfig struct {
	TempDir    string
	ConfigPath string
	Server     *httptest.Server
	APIKey     string
}

// RedirectUI swaps ui.Stdout / ui.Stderr to the supplied buffers for the
// lifetime of the test. Tests that assert on Print*-produced text need this
// because cobra's SetOut/SetErr only captures cobra's own writes.
func (tc *TestConfig) RedirectUI(t *testing.T, stdout, stderr *bytes.Buffer) {
	if stdout != nil {
		oldOut := ui.Stdout
		ui.Stdout = stdout
		t.Cleanup(func() { ui.Stdout = oldOut })
	}
	if stderr != nil {
		oldErr := ui.Stderr
		ui.Stderr = stderr
		t.Cleanup(func() { ui.Stderr = oldErr })
	}
}

// NewTestConfig creates a new test configuration.
//
// Sets DOTENV_CONFIG_DIR + HOME to fresh temp paths so the CLI under test
// cannot leak into the developer's real ~/.dotenv account store (which would
// otherwise trigger OAuth refresh against a real API and hit rate limits).
//
// Layout: tempDir/ (HOME) -> .dotenv/ (DOTENV_CONFIG_DIR via UserHomeDir) ->
// config.yaml. This matches the production layout config.ConfigPath() returns.
func NewTestConfig(t testing.TB) *TestConfig {
	tempDir := t.TempDir()
	dotenvDir := filepath.Join(tempDir, ".dotenv")
	if err := os.MkdirAll(dotenvDir, 0700); err != nil {
		t.Fatalf("create .dotenv dir: %v", err)
	}

	t.Setenv("DOTENV_CONFIG_DIR", dotenvDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir) // Windows fallback.
	t.Setenv("DOTENV_API_KEY", "")   // Don't leak real env API keys.

	return &TestConfig{
		TempDir:    tempDir,
		ConfigPath: filepath.Join(dotenvDir, "config.yaml"),
		APIKey:     "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_test123456",
	}
}

// WriteConfig writes a test configuration file.
//
// Legacy compatibility: if the supplied config carries the old `contexts`
// shape, this method translates the first context into the new accounts/
// account_manager format via the production AccountManager so the CLI under
// test can authenticate. Tests written for the modern shape pass through
// unchanged.
func (tc *TestConfig) WriteConfig(t testing.TB, cfg interface{}) {
	if m, ok := cfg.(map[string]interface{}); ok {
		if _, hasContexts := m["contexts"]; hasContexts {
			tc.writeAccountFromLegacy(t, m)
			return
		}
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	err = os.WriteFile(tc.ConfigPath, data, 0600)
	require.NoError(t, err)
}

func (tc *TestConfig) writeAccountFromLegacy(t testing.TB, m map[string]interface{}) {
	contexts, _ := m["contexts"].(map[string]interface{})
	currentName, _ := m["current_context"].(string)
	if currentName == "" {
		for name := range contexts {
			currentName = name
			break
		}
	}

	ctx, _ := contexts[currentName].(map[string]interface{})
	apiURL, _ := ctx["api_url"].(string)
	apiKey, _ := ctx["api_key"].(string)
	orgULID, _ := ctx["organization"].(string)
	if apiKey == "" {
		apiKey = tc.APIKey
	}

	am, err := config.NewAccountManager(tc.ConfigPath)
	require.NoError(t, err)

	orgInfo := &config.OrgInfo{ULID: orgULID, Name: orgULID}
	require.NoError(t, am.CreateWithAPIKey(currentName, apiURL, apiKey, orgInfo))
	require.NoError(t, am.Use(currentName))

	// Some commands (notably `apikeys`) still read the organization via
	// viper rather than from the active account. Bind via env so those
	// commands see the same value the account would supply.
	if orgULID != "" {
		t.Setenv("DOTENV_ORGANIZATION", orgULID)
	}
}

// WriteFixture writes a fixture file
func WriteFixture(t *testing.T, path, content string) {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o750)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

// LoadFixture loads a fixture file
func LoadFixture(t *testing.T, name string) []byte {
	path := filepath.Join("fixtures", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// CompareJSON compares two JSON strings
func CompareJSON(t *testing.T, expected, actual string) {
	var expectedObj, actualObj interface{}

	err := json.Unmarshal([]byte(expected), &expectedObj)
	require.NoError(t, err)

	err = json.Unmarshal([]byte(actual), &actualObj)
	require.NoError(t, err)

	require.Equal(t, expectedObj, actualObj)
}

// CaptureOutput captures stdout and stderr
func CaptureOutput(_ *testing.T, fn func()) (stdout, stderr string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	wOut.Close()
	wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return string(outBytes), string(errBytes)
}

// SetEnv sets environment variables and cleans up after test
func SetEnv(t *testing.T, vars map[string]string) {
	for k, v := range vars {
		old := os.Getenv(k)
		os.Setenv(k, v)
		t.Cleanup(func() {
			if old == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, old)
			}
		})
	}
}

// MockResponse creates a mock HTTP response
type MockResponse struct {
	Status  int
	Body    interface{}
	Headers map[string]string
}

// ServeHTTP implements http.Handler
func (m MockResponse) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	for k, v := range m.Headers {
		w.Header().Set(k, v)
	}

	w.WriteHeader(m.Status)

	if m.Body != nil {
		switch v := m.Body.(type) {
		case string:
			_, _ = w.Write([]byte(v))
		case []byte:
			_, _ = w.Write(v)
		default:
			_ = json.NewEncoder(w).Encode(v)
		}
	}
}

// CreateTestServer creates a test HTTP server with predefined routes
func CreateTestServer(_ *testing.T, routes map[string]http.Handler) *httptest.Server {
	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.Handle(path, handler)
	}
	return httptest.NewServer(mux)
}

// AssertFileContains checks if a file contains a specific string
func AssertFileContains(t *testing.T, path, content string) {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), content)
}

// CreateTestEnvFile creates a test .env file
func CreateTestEnvFile(t *testing.T, path string, vars map[string]string) {
	var content string
	for k, v := range vars {
		content += k + "=" + v + "\n"
	}
	WriteFixture(t, path, content)
}
