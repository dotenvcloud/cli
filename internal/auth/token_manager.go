package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/errors"
	"github.com/dotenv/cli/internal/utils"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

// TokenManager handles OAuth token refresh and validation
type TokenManager struct {
	accountManager *config.AccountManager
	clientFactory  *client.Factory
}

// NewTokenManager creates a new token manager
func NewTokenManager(am *config.AccountManager) *TokenManager {
	return &TokenManager{
		accountManager: am,
		clientFactory:  client.NewFactory(config.GetAPIURL("")),
	}
}

// RefreshTokenIfNeeded checks if the token needs refresh and refreshes it if necessary
func (tm *TokenManager) RefreshTokenIfNeeded(ctx context.Context, account *config.Account) error {
	if !account.IsOAuth() {
		return nil // API key accounts don't need token refresh
	}

	if !account.IsTokenExpired() {
		return nil // Token is still valid
	}

	if account.IsRefreshTokenExpired() {
		return &errors.TokenExpiredError{
			AccountName: account.Name,
			ExpiresAt:   account.Auth.RefreshTokenExpiresAt.Format(time.RFC3339),
		}
	}

	return tm.RefreshToken(ctx, account)
}

// RefreshToken refreshes the OAuth token for the given account
func (tm *TokenManager) RefreshToken(ctx context.Context, account *config.Account) error {
	// Create unauthenticated SDK client for token refresh
	sdkClient := tm.clientFactory.NewUnauthenticatedClient(account.APIURL, config.ShouldSkipTLSVerify())

	// Refresh token using SDK
	tokenResp, _, err := sdkClient.OAuth.RefreshToken(ctx, account.Auth.RefreshToken, constants.OAuthClientID)
	if err != nil {
		return errors.NewAuthError(constants.AuthTypeOAuth, "failed to refresh token", err)
	}

	// Update account with new tokens
	configTokenResp := config.TokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
	}

	if err := tm.accountManager.RefreshToken(account.Name, configTokenResp); err != nil {
		return errors.NewAuthError(constants.AuthTypeOAuth, "failed to update tokens", err)
	}

	return nil
}

// RefreshOrganizationsIfNeeded checks if organizations need refresh and refreshes them if necessary
func (tm *TokenManager) RefreshOrganizationsIfNeeded(ctx context.Context, account *config.Account) (bool, error) {
	if !account.NeedsOrganizationRefresh() {
		return false, nil
	}

	// Create client with appropriate authentication
	var sdkClient *dotenv.Client
	var err error

	if account.IsOAuth() {
		// Ensure token is valid first
		if refreshErr := tm.RefreshTokenIfNeeded(ctx, account); refreshErr != nil {
			return false, refreshErr
		}

		// Reload account to get refreshed tokens
		account, err = tm.accountManager.Get(account.Name)
		if err != nil {
			return false, fmt.Errorf("failed to reload account after token refresh: %w", err)
		}
	}

	// Create client from account (without organization context)
	sdkClient, err = tm.clientFactory.NewClientFromAccount(account, false)
	if err != nil {
		return false, fmt.Errorf("failed to create API client: %w", err)
	}

	// Fetch organizations
	orgs, _, err := sdkClient.Organizations.List(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to fetch organizations: %w", err)
	}

	if len(orgs) == 0 {
		return false, fmt.Errorf("no organizations found")
	}

	// Convert to OrgInfo format
	orgInfos := make([]config.OrgInfo, 0, len(orgs))
	for _, org := range orgs {
		orgInfos = append(orgInfos, config.OrgInfo{
			ULID: utils.GetULIDFromOrg(org),
			Name: org.Name,
		})
	}

	// Update account with new organizations
	return tm.accountManager.RefreshOrganizations(account.Name, orgInfos)
}
