package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/cmd"
	"github.com/dotenvcloud/cli/test/helpers"
)

func TestPullCommand(t *testing.T) {
	t.Skip("legacy — mock response shape predates hierarchy refactor; Wave 5 F-09 backfill")
}

func TestPullCommand_Formats(t *testing.T) {
	t.Skip("legacy — Wave 5 F-09 backfill")
}

func TestPullCommand_OutputFile(t *testing.T) {
	tc := helpers.NewTestConfig(t)
	outputFile := filepath.Join(tc.TempDir, ".env")

	// Create mock server — SecretsHierarchyResponse shape with one level.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"type": "secrets",
				"attributes": map[string]interface{}{
					"encrypted": false,
					"format":    "env",
					"levels": map[string]interface{}{
						"project": map[string]interface{}{
							"encrypted": false,
							"content":   "TEST_VAR=test-value\n",
							"source":    "test-project",
						},
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

	// The server returns the project key as a RAW STRING, and the level blob is
	// encrypted with that key string per the platform contract
	// (dotenv.EncryptWithProjectKey — never hex/base64-decoded). Use a 64-char
	// hex key like the real server generates.
	keyStr := "52c7e8f043e61267076c35827d6c4be454c70ecac00bf10e79a56d703e32e123"

	// Encrypt the full env content (server stores the level as one encrypted
	// blob, not per-key).
	encrypted, err := dotenv.EncryptWithProjectKey("ENCRYPTED_VAR=decrypted-value", keyStr)
	require.NoError(t, err)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/test-org/test-project/encryption-key" {
			w.Header().Set("Content-Type", "application/json")
			content, _ := json.Marshal(map[string]interface{}{
				"key": map[string]interface{}{
					"key":     keyStr,
					"version": 1,
				},
			})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"type": "encryption_keys",
					"attributes": map[string]interface{}{
						"content": string(content),
						"format":  "json",
					},
				},
			})
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"type": "secrets",
					"attributes": map[string]interface{}{
						"encrypted": true,
						"format":    "env",
						"levels": map[string]interface{}{
							"project": map[string]interface{}{
								"encrypted": true,
								"content":   encrypted,
								"source":    "test-project",
							},
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

func TestPullCommand_ClientKeyLiteralValue(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	// A literal --client-key value is used directly as the key string; the
	// encryption-key endpoint must NOT be consulted.
	keyStr := "myliteralclientkey"
	encrypted, err := dotenv.EncryptWithProjectKey("LITERAL_VAR=ok", keyStr)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The descriptor is now always fetched (to learn the storage mode); for a
		// client-managed project it returns proof params, never a key. The literal
		// --client-key value is what actually decrypts.
		if r.URL.Path == "/api/v1/test-org/test-project/encryption-key" {
			content, _ := json.Marshal(map[string]interface{}{
				"key": map[string]interface{}{
					"managed":              "client",
					"version":              1,
					"key_check_salt":       "AAAAAAAAAAAAAAAAAAAAAA==",
					"key_check_iterations": 600000,
				},
			})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"type":       "encryption_keys",
					"attributes": map[string]interface{}{"content": string(content), "format": "json"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"type": "secrets",
				"attributes": map[string]interface{}{
					"encrypted": true,
					"format":    "env",
					"levels": map[string]interface{}{
						"project": map[string]interface{}{
							"encrypted": true,
							"content":   encrypted,
							"source":    "test-project",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

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

	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"pull", "test-project", "--decrypt", "--client-key", keyStr, "--config", tc.ConfigPath})

	// The root PersistentPreRun mirrors ui.Stdout onto cmd.OutOrStdout(), so the
	// literal-key warning lands in this same buffer as the decrypted output.
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	err = rootCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "LITERAL_VAR=ok")
	assert.Contains(t, stdout.String(), "literal argument")
}
