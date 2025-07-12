package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenv/cli/cmd"
	"github.com/dotenv/cli/test/helpers"
	dotenv "github.com/dotenv/sdk-go"
)

func TestAPIKeysListCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		mockKeys       []dotenv.APIKey
		mockStatusCode int
		wantOutput     string
		wantError      bool
		errorContains  string
	}{
		{
			name: "successful list with keys",
			args: []string{},
			mockKeys: []dotenv.APIKey{
				{
					ID:          "key-123",
					Name:        "Production Key",
					TokenPrefix: "dotenv_api_",
					Abilities:   []string{"secrets:read", "projects:read"},
					CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LastUsedAt:  timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
				},
				{
					ID:          "key-456",
					Name:        "CI/CD Key",
					TokenPrefix: "dotenv_api_",
					Abilities:   []string{"secrets:read"},
					CreatedAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					LastUsedAt:  nil,
				},
			},
			mockStatusCode: http.StatusOK,
			wantOutput:     "key-123\tProduction Key\tdotenv_api_...\tsecrets:read, projects:read\t2024-01-15\t2024-01-01",
		},
		{
			name:           "empty list",
			args:           []string{},
			mockKeys:       []dotenv.APIKey{},
			mockStatusCode: http.StatusOK,
			wantOutput:     "No API keys found",
		},
		{
			name:           "unauthorized",
			args:           []string{},
			mockKeys:       nil,
			mockStatusCode: http.StatusUnauthorized,
			wantError:      true,
			errorContains:  "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			tc := helpers.NewTestConfig(t)

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/test-org/api-keys", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockStatusCode == http.StatusOK {
					response := dotenv.JSONAPIResponse{
						Data: tt.mockKeys,
					}
					json.NewEncoder(w).Encode(response)
				} else {
					errorResp := dotenv.JSONAPIResponse{
						Errors: []dotenv.JSONAPIError{
							{
								Status: http.StatusText(tt.mockStatusCode),
								Title:  http.StatusText(tt.mockStatusCode),
								Detail: "Error occurred",
							},
						},
					}
					json.NewEncoder(w).Encode(errorResp)
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
			rootCmd.SetArgs(append([]string{"apikeys", "list", "--config", tc.ConfigPath}, tt.args...))

			// Capture output
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)

			// Execute
			err := rootCmd.Execute()

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					errStr := err.Error()
					if errStr == "" {
						errStr = stderr.String()
					}
					assert.Contains(t, errStr, tt.errorContains)
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

func TestAPIKeysCreateCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		mockResponse   *dotenv.APIKeyCreateResponse
		mockStatusCode int
		wantOutput     string
		wantError      bool
		errorContains  string
	}{
		{
			name: "successful create",
			args: []string{"CI/CD Key", "--abilities", "secrets:read,projects:read"},
			mockResponse: &dotenv.APIKeyCreateResponse{
				ID:    "key-789",
				Token: "dotenv_api_test_token_123",
				APIKey: &dotenv.APIKey{
					ID:          "key-789",
					Name:        "CI/CD Key",
					TokenPrefix: "dotenv_api_",
					Abilities:   []string{"secrets:read", "projects:read"},
					CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			mockStatusCode: http.StatusCreated,
			wantOutput:     "Token: dotenv_api_test_token_123",
		},
		{
			name:          "missing name",
			args:          []string{},
			wantError:     true,
			errorContains: "accepts 1 arg(s), received 0",
		},
		{
			name:          "missing abilities",
			args:          []string{"Test Key"},
			wantError:     true,
			errorContains: "required flag(s) \"abilities\" not set",
		},
		{
			name:           "invalid abilities",
			args:           []string{"Bad Key", "--abilities", "invalid:ability"},
			mockResponse:   nil,
			mockStatusCode: http.StatusBadRequest,
			wantError:      true,
			errorContains:  "bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			tc := helpers.NewTestConfig(t)

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/test-org/api-keys", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockResponse != nil {
					response := dotenv.JSONAPIResponse{
						Data: tt.mockResponse,
					}
					json.NewEncoder(w).Encode(response)
				} else if tt.mockStatusCode >= 400 {
					errorResp := dotenv.JSONAPIResponse{
						Errors: []dotenv.JSONAPIError{
							{
								Status: http.StatusText(tt.mockStatusCode),
								Title:  http.StatusText(tt.mockStatusCode),
								Detail: "Error occurred",
							},
						},
					}
					json.NewEncoder(w).Encode(errorResp)
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
			rootCmd.SetArgs(append([]string{"apikeys", "create", "--config", tc.ConfigPath}, tt.args...))

			// Capture output
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)

			// Execute
			err := rootCmd.Execute()

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					errStr := err.Error()
					if errStr == "" {
						errStr = stderr.String()
					}
					assert.Contains(t, errStr, tt.errorContains)
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

func TestAPIKeysUpdateCommand(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	mockKey := &dotenv.APIKey{
		ID:          "key-123",
		Name:        "Updated Key Name",
		TokenPrefix: "dotenv_api_",
		Abilities:   []string{"secrets:read"},
		CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/test-org/api-keys/key-123", r.URL.Path)
		assert.Equal(t, "PATCH", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := dotenv.JSONAPIResponse{
			Data: mockKey,
		}
		json.NewEncoder(w).Encode(response)
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
	rootCmd.SetArgs([]string{"apikeys", "update", "key-123", "--name", "Updated Key Name", "--config", tc.ConfigPath})

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Check output
	assert.Contains(t, stdout.String(), "API key updated successfully")
	assert.Contains(t, stdout.String(), "Updated Key Name")
}

func TestAPIKeysDeleteCommand(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	var requestReceived bool

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			assert.Equal(t, "/api/v1/test-org/api-keys/key-123", r.URL.Path)
			requestReceived = true
			w.WriteHeader(http.StatusNoContent)
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

	// Create command - set stdin to simulate "n" response to confirmation
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"apikeys", "delete", "key-123", "--config", tc.ConfigPath})
	
	// Set stdin to provide "n" for the confirmation prompt
	rootCmd.SetIn(bytes.NewReader([]byte("n\n")))

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Check that request was not sent (user cancelled)
	assert.False(t, requestReceived)
	assert.Contains(t, stdout.String(), "Deletion cancelled")
}

func TestAPIKeysRotateCommand(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	mockResponse := &dotenv.APIKeyCreateResponse{
		ID:    "key-123",
		Token: "dotenv_api_new_token_456",
		APIKey: &dotenv.APIKey{
			ID:          "key-123",
			Name:        "Rotated Key",
			TokenPrefix: "dotenv_api_",
			Abilities:   []string{"secrets:read"},
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/test-org/api-keys/key-123/rotate" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			response := dotenv.JSONAPIResponse{
				Data: mockResponse,
			}
			json.NewEncoder(w).Encode(response)
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

	// Create command - set stdin to simulate "y" response to confirmation
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"apikeys", "rotate", "key-123", "--config", tc.ConfigPath})
	
	// Set stdin to provide "y" for the confirmation prompt
	rootCmd.SetIn(bytes.NewReader([]byte("y\n")))

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Check output
	assert.Contains(t, stdout.String(), "API key rotated successfully")
	assert.Contains(t, stdout.String(), "Token: dotenv_api_new_token_456")
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}