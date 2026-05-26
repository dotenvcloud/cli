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
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/test/helpers"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

func TestAuthInfoCommand(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		mockUser          *dotenv.User
		mockOrganizations []dotenv.UserOrganization
		mockStatusCode    int
		useAPIKey         bool
		wantOutput        []string
		wantError         bool
		errorContains     string
	}{
		{
			name: "successful user info",
			args: []string{},
			mockUser: &dotenv.User{
				ID:         "user-123",
				Email:      "test@example.com",
				Name:       "Test User",
				IsVerified: true,
				CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			mockOrganizations: []dotenv.UserOrganization{
				{
					ID:       "org-123",
					Name:     "Test Organization",
					Slug:     "test-org",
					Role:     "owner",
					JoinedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:       "org-456",
					Name:     "Another Org",
					Slug:     "another-org",
					Role:     "member",
					JoinedAt: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
				},
			},
			mockStatusCode: http.StatusOK,
			wantOutput: []string{
				"Authenticated User",
				"Name:     Test User",
				"Email:    test@example.com",
				"ID:       user-123",
				"Verified: true",
				"Test Organization (test-org) - owner",
				"Another Org (another-org) - member",
			},
		},
		{
			name: "verbose organization display",
			args: []string{"--verbose"},
			mockUser: &dotenv.User{
				ID:         "user-456",
				Email:      "verbose@example.com",
				Name:       "Verbose User",
				IsVerified: true,
				CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			mockOrganizations: []dotenv.UserOrganization{
				{
					ID:       "org-789",
					Name:     "Detailed Org",
					Slug:     "detailed-org",
					Role:     "admin",
					JoinedAt: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				},
			},
			mockStatusCode: http.StatusOK,
			wantOutput: []string{
				"NAME\tSLUG\tROLE\tID\tJOINED",
				"Detailed Org\tdetailed-org\tadmin\torg-789\t2024-01-10",
			},
		},
		{
			name: "no organizations",
			args: []string{},
			mockUser: &dotenv.User{
				ID:         "user-lonely",
				Email:      "lonely@example.com",
				Name:       "Lonely User",
				IsVerified: false,
				CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			mockOrganizations: []dotenv.UserOrganization{},
			mockStatusCode:    http.StatusOK,
			wantOutput: []string{
				"No organization memberships found",
			},
		},
		{
			name:      "api key authentication",
			args:      []string{},
			useAPIKey: true,
			wantOutput: []string{
				"Authentication: API Key",
				"Token Prefix: test-api-key",
				"API key authentication has limited user information",
			},
		},
		{
			name:           "unauthorized",
			args:           []string{},
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
				assert.Equal(t, "/api/v1/test-org/user", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockStatusCode == http.StatusOK && tt.mockUser != nil {
					// Build the response in the format expected by the SDK
					userResp := dotenv.UserResponse{}
					userResp.Data.Type = "users"
					userResp.Data.ID = tt.mockUser.ID
					userResp.Data.Attributes.Name = tt.mockUser.Name
					userResp.Data.Attributes.Email = tt.mockUser.Email
					userResp.Data.Attributes.CreatedAt = tt.mockUser.CreatedAt
					userResp.Data.Attributes.UpdatedAt = tt.mockUser.UpdatedAt
					userResp.Data.Attributes.IsVerified = tt.mockUser.IsVerified

					// Add organizations to included
					for _, org := range tt.mockOrganizations {
						userResp.Included = append(userResp.Included, struct {
							Type       string `json:"type"`
							ID         string `json:"id"`
							Attributes struct {
								ULID string `json:"ulid"`
								Name string `json:"name"`
								Slug string `json:"slug"`
								Role string `json:"role"`
							} `json:"attributes"`
						}{
							Type: "organizations",
							ID:   org.ID,
							Attributes: struct {
								ULID string `json:"ulid"`
								Name string `json:"name"`
								Slug string `json:"slug"`
								Role string `json:"role"`
							}{
								ULID: org.ULID,
								Name: org.Name,
								Slug: org.Slug,
								Role: org.Role,
							},
						})
					}

					json.NewEncoder(w).Encode(userResp)
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
			apiKey := tc.APIKey
			if tt.useAPIKey {
				apiKey = "test-api-key-12345"
			}

			config := map[string]interface{}{
				"version":         "1.0",
				"current_context": "test",
				"contexts": map[string]interface{}{
					"test": map[string]interface{}{
						"api_url":      server.URL,
						"organization": "test-org",
					},
				},
			}

			// Add API key to config if using API key auth
			if tt.useAPIKey {
				contexts := config["contexts"].(map[string]interface{})
				testContext := contexts["test"].(map[string]interface{})
				testContext["api_key"] = apiKey
			}

			tc.WriteConfig(t, config)

			// Create command
			rootCmd := cmd.NewRootCommand()
			args := []string{"auth", "info", "--config", tc.ConfigPath}
			if tt.useAPIKey {
				args = append(args, "--api-key", apiKey)
			}
			args = append(args, tt.args...)
			rootCmd.SetArgs(args)

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
				output := stdout.String()
				for _, expected := range tt.wantOutput {
					assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
				}
			}
		})
	}
}

func TestAuthInfoCommand_WithAccount(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	mockUser := &dotenv.User{
		ID:         "user-123",
		Email:      "oauth@example.com",
		Name:       "OAuth User",
		IsVerified: true,
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	mockOrganizations := []dotenv.UserOrganization{
		{
			ID:       "org-123",
			Name:     "OAuth Organization",
			Slug:     "oauth-org",
			Role:     "owner",
			JoinedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Build the response in the format expected by the SDK
		userResp := dotenv.UserResponse{}
		userResp.Data.Type = "users"
		userResp.Data.ID = mockUser.ID
		userResp.Data.Attributes.Name = mockUser.Name
		userResp.Data.Attributes.Email = mockUser.Email
		userResp.Data.Attributes.CreatedAt = mockUser.CreatedAt
		userResp.Data.Attributes.UpdatedAt = mockUser.UpdatedAt
		userResp.Data.Attributes.IsVerified = mockUser.IsVerified

		// Add organizations to included
		for _, org := range mockOrganizations {
			userResp.Included = append(userResp.Included, struct {
				Type       string `json:"type"`
				ID         string `json:"id"`
				Attributes struct {
					ULID string `json:"ulid"`
					Name string `json:"name"`
					Slug string `json:"slug"`
					Role string `json:"role"`
				} `json:"attributes"`
			}{
				Type: "organizations",
				ID:   org.ID,
				Attributes: struct {
					ULID string `json:"ulid"`
					Name string `json:"name"`
					Slug string `json:"slug"`
					Role string `json:"role"`
				}{
					ULID: org.ULID,
					Name: org.Name,
					Slug: org.Slug,
					Role: org.Role,
				},
			})
		}

		json.NewEncoder(w).Encode(userResp)
	}))
	defer server.Close()

	// Set up config with account information
	config := map[string]interface{}{
		"version":         "1.0",
		"current_account": "test-account",
		"accounts": map[string]interface{}{
			"test-account": map[string]interface{}{
				"name":      "test-account",
				"api_url":   server.URL,
				"auth_type": constants.AuthTypeOAuth,
				"auth": map[string]interface{}{
					"access_token":  "test-access-token",
					"refresh_token": "test-refresh-token",
					"token_type":    "Bearer",
					"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
				},
				"organizations": []interface{}{
					map[string]interface{}{
						"ulid": "org-123",
						"name": "OAuth Organization",
						"slug": "oauth-org",
					},
				},
				"current_organization": "org-123",
			},
		},
	}
	tc.WriteConfig(t, config)

	// Create command
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"auth", "info", "--config", tc.ConfigPath})

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Check output
	output := stdout.String()
	assert.Contains(t, output, "OAuth User")
	assert.Contains(t, output, "oauth@example.com")
	assert.Contains(t, output, "Authentication type: OAuth")
	assert.Contains(t, output, "Token expires:")
}

func TestAuthCommand_NoSubcommand(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	// Set up minimal config
	config := map[string]interface{}{
		"version": "1.0",
	}
	tc.WriteConfig(t, config)

	// Create command without subcommand
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"auth", "--config", tc.ConfigPath})

	// Capture output
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// Execute
	err := rootCmd.Execute()

	// Should show help text, not an error
	require.NoError(t, err)
	output := stdout.String()
	assert.Contains(t, output, "Authentication management")
	assert.Contains(t, output, "Available Commands:")
	assert.Contains(t, output, "info")
}
