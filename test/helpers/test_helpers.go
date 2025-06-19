package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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

// NewTestConfig creates a new test configuration
func NewTestConfig(t *testing.T) *TestConfig {
	tempDir := t.TempDir()

	return &TestConfig{
		TempDir:    tempDir,
		ConfigPath: filepath.Join(tempDir, "config.yaml"),
		APIKey:     "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_test123456",
	}
}

// WriteConfig writes a test configuration file
func (tc *TestConfig) WriteConfig(t *testing.T, config interface{}) {
	data, err := yaml.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(tc.ConfigPath, data, 0600)
	require.NoError(t, err)
}

// WriteFixture writes a fixture file
func WriteFixture(t *testing.T, path string, content string) {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0644)
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
func CaptureOutput(t *testing.T, fn func()) (stdout, stderr string) {
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
func (m MockResponse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k, v := range m.Headers {
		w.Header().Set(k, v)
	}

	w.WriteHeader(m.Status)

	if m.Body != nil {
		switch v := m.Body.(type) {
		case string:
			w.Write([]byte(v))
		case []byte:
			w.Write(v)
		default:
			json.NewEncoder(w).Encode(v)
		}
	}
}

// CreateTestServer creates a test HTTP server with predefined routes
func CreateTestServer(t *testing.T, routes map[string]http.Handler) *httptest.Server {
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
