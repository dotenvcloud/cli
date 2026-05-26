package client

import (
	"testing"
	"time"

	"github.com/dotenv/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	tests := []struct {
		name           string
		defaultBaseURL string
		expected       string
	}{
		{
			name:           "with custom base URL",
			defaultBaseURL: "https://custom.api.com",
			expected:       "https://custom.api.com",
		},
		{
			name:           "with empty base URL",
			defaultBaseURL: "",
			expected:       "https://api.dotenv.cloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.defaultBaseURL)
			assert.NotNil(t, factory)
			assert.Equal(t, tt.expected, factory.defaultBaseURL)
		})
	}
}

func TestNewClient(t *testing.T) {
	factory := NewFactory("https://api.test.com")

	tests := []struct {
		name     string
		opts     Options
		validate func(t *testing.T, client interface{})
	}{
		{
			name: "with API key",
			opts: Options{
				BaseURL: "https://api.test.com",
				APIKey:  "test-api-key",
			},
			validate: func(t *testing.T, client interface{}) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with bearer token",
			opts: Options{
				BaseURL:     "https://api.test.com",
				BearerToken: "test-bearer-token",
			},
			validate: func(t *testing.T, client interface{}) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with organization",
			opts: Options{
				BaseURL:      "https://api.test.com",
				APIKey:       "test-api-key",
				Organization: "test-org",
			},
			validate: func(t *testing.T, client interface{}) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with insecure skip verify",
			opts: Options{
				BaseURL:            "https://api.test.com",
				APIKey:             "test-api-key",
				InsecureSkipVerify: true,
			},
			validate: func(t *testing.T, client interface{}) {
				assert.NotNil(t, client)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := factory.NewClient(tt.opts)
			tt.validate(t, client)
		})
	}
}

func TestNewUnauthenticatedClient(t *testing.T) {
	factory := NewFactory("https://api.test.com")

	tests := []struct {
		name               string
		baseURL            string
		insecureSkipVerify bool
	}{
		{
			name:               "with custom base URL",
			baseURL:            "https://custom.api.com",
			insecureSkipVerify: false,
		},
		{
			name:               "with insecure skip verify",
			baseURL:            "https://api.test.com",
			insecureSkipVerify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := factory.NewUnauthenticatedClient(tt.baseURL, tt.insecureSkipVerify)
			assert.NotNil(t, client)
		})
	}
}

func TestNewClientFromAPIKey(t *testing.T) {
	factory := NewFactory("https://api.test.com")

	tests := []struct {
		name         string
		apiKey       string
		baseURL      string
		organization string
	}{
		{
			name:         "with all parameters",
			apiKey:       "test-api-key",
			baseURL:      "https://api.test.com",
			organization: "test-org",
		},
		{
			name:         "without organization",
			apiKey:       "test-api-key",
			baseURL:      "https://api.test.com",
			organization: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := factory.NewClientFromAPIKey(tt.apiKey, tt.baseURL, tt.organization)
			assert.NotNil(t, client)
		})
	}
}

//nolint:funlen // table-driven test covers many account-shape variants; splitting hurts readability
func TestNewClientFromAccount(t *testing.T) {
	factory := NewFactory("https://api.test.com")

	tests := []struct {
		name        string
		account     *config.Account
		includeOrg  bool
		expectError bool
		errorMsg    string
	}{
		{
			name: "OAuth account with valid token",
			account: &config.Account{
				Name:     "test-oauth",
				APIURL:   "https://api.test.com",
				AuthType: "oauth",
				Auth: config.AuthData{
					AccessToken: "test-access-token",
					TokenType:   "Bearer",
					ExpiresAt:   *futureTime(3600), // 1 hour from now
				},
				Organizations: []config.OrgInfo{
					{ULID: "test-org", Name: "Test Org"},
				},
				CurrentOrganization: "test-org",
			},
			includeOrg:  true,
			expectError: false,
		},
		{
			name: "OAuth account with expired token",
			account: &config.Account{
				Name:     "test-oauth-expired",
				APIURL:   "https://api.test.com",
				AuthType: "oauth",
				Auth: config.AuthData{
					AccessToken: "test-access-token",
					TokenType:   "Bearer",
					ExpiresAt:   *pastTime(3600), // 1 hour ago
				},
			},
			includeOrg:  false,
			expectError: true,
			errorMsg:    "OAuth token expired",
		},
		{
			name: "API key account",
			account: &config.Account{
				Name:     "test-apikey",
				APIURL:   "https://api.test.com",
				AuthType: "apikey",
				Auth: config.AuthData{
					APIKey: "test-api-key",
				},
				Organization: &config.OrgInfo{
					ULID: "test-org",
					Name: "Test Org",
				},
			},
			includeOrg:  true,
			expectError: false,
		},
		{
			name: "API key account without key",
			account: &config.Account{
				Name:     "test-no-key",
				APIURL:   "https://api.test.com",
				AuthType: "apikey",
				Auth: config.AuthData{
					APIKey: "",
				},
			},
			includeOrg:  false,
			expectError: true,
			errorMsg:    "no API key found",
		},
		{
			name: "account without organization when required",
			account: &config.Account{
				Name:     "test-no-org",
				APIURL:   "https://api.test.com",
				AuthType: "apikey",
				Auth: config.AuthData{
					APIKey: "test-api-key",
				},
			},
			includeOrg:  true,
			expectError: true,
			errorMsg:    "no organization selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.NewClientFromAccount(tt.account, tt.includeOrg)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// Helper functions for testing
func futureTime(seconds int) *time.Time {
	t := time.Now().Add(time.Duration(seconds) * time.Second)
	return &t
}

func pastTime(seconds int) *time.Time {
	t := time.Now().Add(-time.Duration(seconds) * time.Second)
	return &t
}
