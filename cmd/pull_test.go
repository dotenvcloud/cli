package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenv/cli/cmd"
	"github.com/dotenv/cli/test/helpers"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

func TestPullCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		serverResponse map[string]interface{}
		wantOutput     string
		wantError      bool
		errorContains  string
	}{
		{
			name: "successful pull",
			args: []string{"test-project"},
			serverResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"type": "secrets",
					"attributes": map[string]interface{}{
						"encrypted": false,
						"format":    "env",
						"levels": map[string]interface{}{
							"project": map[string]interface{}{
								"encrypted": false,
								"content":   "DATABASE_URL=postgres://localhost/test\nAPI_KEY=test-api-key",
								"source":    "project",
							},
						},
					},
				},
				"meta": map[string]interface{}{
					"hierarchy": map[string]interface{}{
						"project": "test-project",
					},
				},
			},
			wantOutput: "API_KEY=test-api-key\nDATABASE_URL=postgres://localhost/test",
		},
		{
			name: "pull project with target",
			args: []string{"test-project/production"},
			serverResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"type": "secrets",
					"attributes": map[string]interface{}{
						"encrypted": false,
						"format":    "env",
						"levels": map[string]interface{}{
							"project": map[string]interface{}{
								"encrypted": false,
								"content":   "DATABASE_URL=postgres://localhost/test",
								"source":    "project",
							},
							"target": map[string]interface{}{
								"encrypted": false,
								"content":   "PROD_VAR=prod-value",
								"source":    "target",
							},
						},
					},
				},
				"meta": map[string]interface{}{
					"hierarchy": map[string]interface{}{
						"project": "test-project",
						"target":  "production",
					},
				},
			},
			wantOutput: "DATABASE_URL=postgres://localhost/test\nPROD_VAR=prod-value",
		},
		{
			name:          "missing project",
			args:          []string{},
			wantError:     true,
			errorContains: "accepts 1 arg(s), received 0",
		},
		{
			name: "project not found",
			args: []string{"non-existent"},
			serverResponse: map[string]interface{}{
				"error": "Not Found",
			},
			wantError:     true,
			errorContains: "project not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			tc := helpers.NewTestConfig(t)

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Mock encryption key endpoint
				if strings.Contains(r.URL.Path, "/encryption-key") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					response := map[string]interface{}{
						"data": map[string]interface{}{
							"type": "encryption-keys",
							"attributes": map[string]interface{}{
								"key": "YmFzZTY0ZW5jb2RlZDMyYnl0ZWVuY3J5cHRpb25rZXk=", // base64 encoded 32 byte key
							},
						},
					}
					json.NewEncoder(w).Encode(response)
					return
				}

				// Mock secrets endpoint
				if tt.serverResponse != nil {
					if _, ok := tt.serverResponse["error"]; ok {
						w.WriteHeader(http.StatusNotFound)
					} else {
						w.Header().Set("Content-Type", "application/json")
					}
					json.NewEncoder(w).Encode(tt.serverResponse)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			// Set up config with account system
			config := map[string]interface{}{
				"version":         "1.0",
				"current_account": "test-account",
				"accounts": map[string]interface{}{
					"test-account": map[string]interface{}{
						"name":      "Test Account",
						"api_url":   server.URL,
						"auth_type": "api_key",
						"auth": map[string]interface{}{
							"api_key": tc.APIKey,
						},
						"organization": map[string]interface{}{
							"ulid": "01JK3K3C89D4E0YYPKR4AEZ33Q",
							"name": "Test Organization",
							"slug": "test-org",
						},
					},
				},
			}
			tc.WriteConfig(t, config)

			// Create command
			rootCmd := cmd.NewRootCommand()
			rootCmd.SetArgs(append([]string{"pull", "--config", tc.ConfigPath}, tt.args...))

			// Capture output
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)

			// Execute
			err := rootCmd.Execute()

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.wantOutput != "" {
					assert.Contains(t, stdout.String(), tt.wantOutput)
				}
			}
		})
	}
}

func TestPullCommand_Formats(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{
			format:   "env",
			expected: "KEY1=value1\nKEY2=value2",
		},
		{
			format:   "json",
			expected: `{"KEY1":"value1","KEY2":"value2"}`,
		},
		{
			format:   "yaml",
			expected: "KEY1: value1\nKEY2: value2",
		},
		{
			format:   "shell",
			expected: `export KEY1="value1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			tc := helpers.NewTestConfig(t)

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []interface{}{
						map[string]interface{}{
							"type": "secrets",
							"attributes": map[string]interface{}{
								"key":   "KEY1",
								"value": "value1",
							},
						},
						map[string]interface{}{
							"type": "secrets",
							"attributes": map[string]interface{}{
								"key":   "KEY2",
								"value": "value2",
							},
						},
					},
				})
			}))
			defer server.Close()

			// Set up config with account system
			config := map[string]interface{}{
				"version":         "1.0",
				"current_account": "test-account",
				"accounts": map[string]interface{}{
					"test-account": map[string]interface{}{
						"name":      "Test Account",
						"api_url":   server.URL,
						"auth_type": "api_key",
						"auth": map[string]interface{}{
							"api_key": tc.APIKey,
						},
						"organization": map[string]interface{}{
							"ulid": "01JK3K3C89D4E0YYPKR4AEZ33Q",
							"name": "Test Organization",
							"slug": "test-org",
						},
					},
				},
			}
			tc.WriteConfig(t, config)

			// Create command
			rootCmd := cmd.NewRootCommand()
			rootCmd.SetArgs([]string{"pull", "test-project", "--format", tt.format, "--config", tc.ConfigPath})

			// Capture output
			var stdout bytes.Buffer
			rootCmd.SetOut(&stdout)

			// Execute
			err := rootCmd.Execute()
			require.NoError(t, err)

			assert.Contains(t, stdout.String(), tt.expected)
		})
	}
}

func TestPullCommand_OutputFile(t *testing.T) {
	tc := helpers.NewTestConfig(t)
	outputFile := filepath.Join(tc.TempDir, ".env")

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"type": "secrets",
					"attributes": map[string]interface{}{
						"key":   "TEST_VAR",
						"value": "test-value",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Set up config
	config := map[string]interface{}{
		"version":         "1.0",
		"current_context": "test",
		"contexts": map[string]interface{}{
			"test": map[string]interface{}{
				"api_url":      server.URL,
				"api_key":      tc.APIKey,
				"organization": "test-org",
			},
		},
	}
	tc.WriteConfig(t, config)

	// Create command
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"pull", "test-project", "--output", outputFile, "--config", tc.ConfigPath})

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Check file was created
	assert.FileExists(t, outputFile)

	// Check content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "TEST_VAR=test-value")
}

func TestPullCommand_Encryption(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	// Generate test encryption key
	encKey, err := dotenv.GenerateKey()
	require.NoError(t, err)
	encodedKey := dotenv.EncodeKey(encKey)

	// Encrypt test value
	encrypted, err := dotenv.Encrypt("decrypted-value", encKey)
	require.NoError(t, err)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/test-project/encryption-key" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"type": "encryption_keys",
					"attributes": map[string]interface{}{
						"key":       encodedKey,
						"is_active": true,
					},
				},
			})
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"type": "secrets",
						"attributes": map[string]interface{}{
							"key":   "ENCRYPTED_VAR",
							"value": encrypted,
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	// Set up config
	config := map[string]interface{}{
		"version":         "1.0",
		"current_context": "test",
		"contexts": map[string]interface{}{
			"test": map[string]interface{}{
				"api_url":      server.URL,
				"api_key":      tc.APIKey,
				"organization": "test-org",
			},
		},
	}
	tc.WriteConfig(t, config)

	// Create command
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"pull", "test-project", "--decrypt", "--config", tc.ConfigPath})

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Execute
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Check decrypted value appears
	assert.Contains(t, stdout.String(), "ENCRYPTED_VAR=decrypted-value")
}
