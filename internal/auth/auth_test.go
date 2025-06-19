package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenv/cli/internal/auth"
)

func TestNewAuthFlow(t *testing.T) {
	flow := auth.NewAuthFlow()
	assert.NotNil(t, flow)
}

func TestStartAuth(t *testing.T) {
	ctx := context.Background()

	t.Run("successful auth", func(t *testing.T) {
		flow := auth.NewAuthFlow()

		// Start auth in background
		authChan := make(chan *auth.AuthResult, 1)
		go func() {
			result, err := flow.StartAuth(ctx)
			if err != nil {
				authChan <- &auth.AuthResult{Error: err}
			} else {
				authChan <- result
			}
		}()

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Simulate callback
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/callback?code=test-code&state=%s",
			auth.DefaultPort, "test-state"))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Get result
		select {
		case result := <-authChan:
			if result.Error != nil {
				t.Fatalf("Auth failed: %v", result.Error)
			}
			assert.Equal(t, "test-code", result.Code)
		case <-time.After(5 * time.Second):
			t.Fatal("Auth timeout")
		}
	})

	t.Run("missing code", func(t *testing.T) {
		flow := auth.NewAuthFlow()

		// Start auth in background
		authChan := make(chan *auth.AuthResult, 1)
		go func() {
			result, err := flow.StartAuth(ctx)
			if err != nil {
				authChan <- &auth.AuthResult{Error: err}
			} else {
				authChan <- result
			}
		}()

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Simulate callback without code
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/callback?state=test-state",
			auth.DefaultPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Get result
		select {
		case result := <-authChan:
			assert.Error(t, result.Error)
			assert.Contains(t, result.Error.Error(), "code not found")
		case <-time.After(5 * time.Second):
			t.Fatal("Auth timeout")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		flow := auth.NewAuthFlow()

		// Start auth in background
		authChan := make(chan error, 1)
		go func() {
			_, err := flow.StartAuth(ctx)
			authChan <- err
		}()

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context
		cancel()

		// Get result
		select {
		case err := <-authChan:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
		case <-time.After(5 * time.Second):
			t.Fatal("Cancellation timeout")
		}
	})
}

func TestExchangeCode(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		serverStatus  int
		serverResp    string
		expectedToken string
		expectedError string
	}{
		{
			name:         "successful exchange",
			code:         "valid-code",
			serverStatus: http.StatusOK,
			serverResp: `{
				"access_token": "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_test123",
				"token_type": "Bearer",
				"expires_in": 3600
			}`,
			expectedToken: "dotenv_01ARZ3NDEKTSV4RRFFQ69G5FAV_test123",
		},
		{
			name:          "invalid code",
			code:          "invalid-code",
			serverStatus:  http.StatusBadRequest,
			serverResp:    `{"error": "invalid_grant"}`,
			expectedError: "invalid_grant",
		},
		{
			name:          "server error",
			code:          "any-code",
			serverStatus:  http.StatusInternalServerError,
			serverResp:    "Internal Server Error",
			expectedError: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/oauth/token", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				// Check form data
				err := r.ParseForm()
				require.NoError(t, err)
				assert.Equal(t, tt.code, r.Form.Get("code"))
				assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResp))
			}))
			defer server.Close()

			// Override auth URL for testing
			origURL := auth.AuthURL
			auth.AuthURL = server.URL
			defer func() { auth.AuthURL = origURL }()

			flow := auth.NewAuthFlow()
			token, err := flow.ExchangeCode(context.Background(), tt.code)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token.AccessToken)
			}
		})
	}
}

func TestWaitForAuth(t *testing.T) {
	flow := auth.NewAuthFlow()

	t.Run("successful auth", func(t *testing.T) {
		// Start waiting in background
		resultChan := make(chan *auth.AuthResult, 1)
		go func() {
			result := flow.WaitForAuth(context.Background())
			resultChan <- result
		}()

		// Simulate successful auth
		time.Sleep(100 * time.Millisecond)
		flow.CompleteAuth("test-code")

		// Get result
		select {
		case result := <-resultChan:
			assert.NoError(t, result.Error)
			assert.Equal(t, "test-code", result.Code)
		case <-time.After(2 * time.Second):
			t.Fatal("Wait timeout")
		}
	})

	t.Run("auth error", func(t *testing.T) {
		flow := auth.NewAuthFlow()

		// Start waiting in background
		resultChan := make(chan *auth.AuthResult, 1)
		go func() {
			result := flow.WaitForAuth(context.Background())
			resultChan <- result
		}()

		// Simulate auth error
		time.Sleep(100 * time.Millisecond)
		flow.FailAuth(fmt.Errorf("auth failed"))

		// Get result
		select {
		case result := <-resultChan:
			assert.Error(t, result.Error)
			assert.Contains(t, result.Error.Error(), "auth failed")
		case <-time.After(2 * time.Second):
			t.Fatal("Wait timeout")
		}
	})
}

func TestBuildAuthURL(t *testing.T) {
	flow := auth.NewAuthFlow()
	authURL := flow.BuildAuthURL("test-state")

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Contains(t, parsed.Host, "dotenv.com")
	assert.Equal(t, "/oauth/authorize", parsed.Path)

	q := parsed.Query()
	assert.Equal(t, "test-state", q.Get("state"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.NotEmpty(t, q.Get("client_id"))
	assert.NotEmpty(t, q.Get("redirect_uri"))
}

func TestGenerateState(t *testing.T) {
	flow := auth.NewAuthFlow()

	// Generate multiple states
	states := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state := flow.GenerateState()
		assert.Len(t, state, 32) // 16 bytes in hex = 32 chars
		states[state] = true
	}

	// All should be unique
	assert.Len(t, states, 100)
}

func TestValidateState(t *testing.T) {
	flow := auth.NewAuthFlow()

	tests := []struct {
		name     string
		expected string
		got      string
		wantErr  bool
	}{
		{
			name:     "valid state",
			expected: "abc123",
			got:      "abc123",
			wantErr:  false,
		},
		{
			name:     "invalid state",
			expected: "abc123",
			got:      "xyz789",
			wantErr:  true,
		},
		{
			name:     "empty state",
			expected: "abc123",
			got:      "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := flow.ValidateState(tt.expected, tt.got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
