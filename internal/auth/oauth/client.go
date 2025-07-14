package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dotenv/cli/internal/constants"
)

// OAuth2Client handles OAuth2 authentication flow
type OAuth2Client struct {
	ClientID     string
	BaseURL      string
	CallbackPort string
	HTTPClient   *http.Client
}

// TokenResponse represents the OAuth2 token response
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in,omitempty"`
	Scope            string `json:"scope,omitempty"`
}

// AuthorizeParams contains parameters for the authorization request
type AuthorizeParams struct {
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
}

// NewOAuth2Client creates a new OAuth2 client
func NewOAuth2Client(clientID, baseURL string) *OAuth2Client {
	if baseURL == "" {
		baseURL = constants.DefaultAPIURL
	}

	// Remove /api/v1 suffix if present
	baseURL = strings.TrimSuffix(baseURL, "/api/v1")
	baseURL = strings.TrimSuffix(baseURL, "/api")

	// For development, use the web URL for OAuth endpoints
	if strings.Contains(baseURL, "dotenv.test") || strings.Contains(baseURL, "api.dotenv.test") {
		baseURL = "https://dotenv.test"
	}

	return &OAuth2Client{
		ClientID: clientID,
		BaseURL:  baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAuthorizationURL builds the authorization URL
func (c *OAuth2Client) GetAuthorizationURL(params AuthorizeParams) string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = "/oauth/authorize"

	q := u.Query()
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", params.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", params.State)
	q.Set("code_challenge", params.CodeChallenge)
	q.Set("code_challenge_method", params.CodeChallengeMethod)

	if params.Scope != "" {
		q.Set("scope", params.Scope)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeCode exchanges an authorization code for tokens
func (c *OAuth2Client) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	// For development, the token endpoint is on the API subdomain
	tokenURL := c.BaseURL
	if strings.Contains(c.BaseURL, "dotenv.test") {
		tokenURL = constants.TestAPIURL
	} else if c.BaseURL == constants.DefaultAPIURL {
		// Production already has the right URL
		tokenURL = c.BaseURL
	}
	tokenURL = fmt.Sprintf("%s/api/oauth/token", tokenURL)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.ClientID},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshToken refreshes an access token using a refresh token
func (c *OAuth2Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	// For development, the token endpoint is on the API subdomain
	tokenURL := c.BaseURL
	if strings.Contains(c.BaseURL, "dotenv.test") {
		tokenURL = constants.TestAPIURL
	} else if c.BaseURL == constants.DefaultAPIURL {
		// Production already has the right URL
		tokenURL = c.BaseURL
	}
	tokenURL = fmt.Sprintf("%s/api/oauth/token", tokenURL)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("token refresh failed: %s - %s", errResp.Error, errResp.ErrorDescription)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}
