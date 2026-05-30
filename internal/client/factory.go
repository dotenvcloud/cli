package client

import (
	"context"
	"fmt"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	dotenv "github.com/dotenvcloud/sdk-go"
)

// Options contains configuration for creating SDK clients
type Options struct {
	// BaseURL is the API base URL
	BaseURL string

	// Authentication
	APIKey      string
	BearerToken string

	// Organization context
	Organization string

	// TLS configuration
	InsecureSkipVerify bool
}

// Factory provides methods for creating configured SDK clients
type Factory struct {
	defaultBaseURL string
}

// NewFactory creates a new SDK client factory
func NewFactory(defaultBaseURL string) *Factory {
	if defaultBaseURL == "" {
		defaultBaseURL = constants.DefaultAPIURL
	}
	return &Factory{
		defaultBaseURL: defaultBaseURL,
	}
}

// NewClient creates a new SDK client with the provided options
func (f *Factory) NewClient(opts Options) *dotenv.Client {
	clientOpts := []dotenv.ClientOption{}

	// Set base URL
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = f.defaultBaseURL
	}
	clientOpts = append(clientOpts, dotenv.WithBaseURL(baseURL))

	// Set authentication
	if opts.APIKey != "" {
		clientOpts = append(clientOpts, dotenv.WithAPIKey(opts.APIKey))
	} else if opts.BearerToken != "" {
		clientOpts = append(clientOpts, dotenv.WithBearerToken(opts.BearerToken))
	}

	// Set organization context
	if opts.Organization != "" {
		clientOpts = append(clientOpts, dotenv.WithOrganization(opts.Organization))
	}

	// Set TLS configuration
	if opts.InsecureSkipVerify {
		clientOpts = append(clientOpts, dotenv.WithInsecureSkipVerify())
	}

	return dotenv.NewClient(clientOpts...)
}

// NewUnauthenticatedClient creates an SDK client without authentication
func (f *Factory) NewUnauthenticatedClient(baseURL string, insecureSkipVerify bool) *dotenv.Client {
	return f.NewClient(Options{
		BaseURL:            baseURL,
		InsecureSkipVerify: insecureSkipVerify,
	})
}

// NewClientFromAccount creates an SDK client from a config.Account
func (f *Factory) NewClientFromAccount(account *config.Account, includeOrg bool) (*dotenv.Client, error) {
	opts := Options{
		BaseURL:            account.APIURL,
		InsecureSkipVerify: config.ShouldSkipTLSVerify(),
	}

	// Set authentication based on account type
	if account.IsOAuth() {
		if account.IsTokenExpired() {
			return nil, fmt.Errorf("OAuth token expired for account %s", account.Name)
		}
		opts.BearerToken = account.Auth.AccessToken
	} else {
		apiKey := account.GetToken()
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found for account %s", account.Name)
		}
		opts.APIKey = apiKey
	}

	// Add organization context if requested
	if includeOrg {
		orgULID := account.GetCurrentOrganizationULID()
		if orgULID == "" {
			return nil, fmt.Errorf("no organization selected for account %s", account.Name)
		}
		opts.Organization = orgULID
	}

	return f.NewClient(opts), nil
}

// NewClientFromAPIKey creates an SDK client with API key authentication
func (f *Factory) NewClientFromAPIKey(apiKey, baseURL, organization string) *dotenv.Client {
	opts := Options{
		APIKey:             apiKey,
		BaseURL:            baseURL,
		Organization:       organization,
		InsecureSkipVerify: config.ShouldSkipTLSVerify(),
	}
	return f.NewClient(opts)
}

// RefreshTokenAndCreateClient refreshes the token if needed and creates a client
func (f *Factory) RefreshTokenAndCreateClient(
	ctx context.Context,
	account *config.Account,
	am *config.AccountManager,
	includeOrg bool,
) (*dotenv.Client, error) {
	// Only refresh for OAuth accounts
	if account.IsOAuth() && account.IsTokenExpired() {
		// Create unauthenticated client for token refresh
		refreshClient := f.NewUnauthenticatedClient(account.APIURL, config.ShouldSkipTLSVerify())

		// Refresh token
		tokenResp, _, err := refreshClient.OAuth.RefreshToken(ctx, account.Auth.RefreshToken, constants.OAuthClientID)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Update account with new tokens
		configTokenResp := config.TokenResponse{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
			ExpiresIn:    tokenResp.ExpiresIn,
		}

		if updateErr := am.RefreshToken(account.Name, configTokenResp); updateErr != nil {
			return nil, fmt.Errorf("failed to update tokens: %w", updateErr)
		}

		// Reload account to get updated tokens
		account, err = am.Get(account.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to reload account after token refresh: %w", err)
		}
	}

	return f.NewClientFromAccount(account, includeOrg)
}
