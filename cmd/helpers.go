package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/dotenv/cli/internal/auth/oauth"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

// getAPIClient returns a configured API client
func getAPIClient() (*dotenv.Client, error) {
	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv("DOTENV_API_KEY")
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		options := []dotenv.ClientOption{
			dotenv.WithAPIKey(apiKey),
		}

		// Check for custom API URL
		apiURL := os.Getenv("DOTENV_API_URL")
		if apiURL == "" {
			apiURL = "https://api.dotenv.cloud"
		}
		options = append(options, dotenv.WithBaseURL(apiURL))

		// Check for organization from environment
		if org := os.Getenv("DOTENV_ORGANIZATION"); org != "" {
			options = append(options, dotenv.WithOrganization(org))
		}

		// Check for TLS skip verify (development mode)
		if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
			options = append(options, dotenv.WithInsecureSkipVerify())
		}

		return dotenv.NewClient(options...), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Create client options
	options := []dotenv.ClientOption{
		dotenv.WithBaseURL(account.APIURL),
	}

	// Add organization context
	orgULID := account.GetCurrentOrganizationULID()
	if orgULID == "" {
		return nil, fmt.Errorf("No organization selected. Run 'dotenv org list' to see available organizations.")
	}
	options = append(options, dotenv.WithOrganization(orgULID))

	// Check for TLS skip verify (development mode)
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		options = append(options, dotenv.WithInsecureSkipVerify())
	}

	// Handle authentication based on account type
	if account.IsOAuth() {
		// Check if token is expired
		if account.IsTokenExpired() {
			ui.PrintInfo("OAuth token expired. Attempting to refresh...")

			// Try to refresh the token
			configPath, err := config.ConfigPath()
			if err != nil {
				return nil, fmt.Errorf("failed to locate config directory for token refresh: %w", err)
			}
			am, err := config.NewAccountManager(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize account manager at '%s': %w", configPath, err)
			}

			if err := refreshOAuthToken(am, account); err != nil {
				return nil, fmt.Errorf("failed to refresh token: %w. Please login again with 'dotenv login'", err)
			}

			// Reload account after refresh
			account, err = am.GetCurrent()
			if err != nil {
				return nil, fmt.Errorf("failed to reload account '%s' after token refresh: %w", account.Name, err)
			}
		}

		options = append(options, dotenv.WithBearerToken(account.Auth.AccessToken))
	} else {
		// API key authentication
		apiKey := account.GetToken()
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found in account '%s'", account.Name)
		}
		options = append(options, dotenv.WithAPIKey(apiKey))
	}

	// Create client
	client := dotenv.NewClient(options...)

	// Check if organization data needs refresh
	if account.NeedsOrganizationRefresh() {
		// Create account manager
		configPath, err := config.ConfigPath()
		if err == nil {
			am, err := config.NewAccountManager(configPath)
			if err == nil {
				// Attempt to refresh organizations (non-blocking)
				go func() {
					ctx := context.Background()
					if account.IsOAuth() {
						orgs, _, err := client.Organizations.List(ctx, nil)
						if err == nil && len(orgs) > 0 {
							orgInfos := make([]config.OrgInfo, len(orgs))
							for i, org := range orgs {
								// Use ID if ULID is empty (API returns ULID in ID field)
								ulid := org.ULID
								if ulid == "" && org.ID != "" {
									ulid = org.ID
								}
								orgInfos[i] = config.OrgInfo{
									ULID: ulid,
									Name: org.Name,
								}
							}
							orgRemoved, _ := am.RefreshOrganizations(account.Name, orgInfos)
							if orgRemoved {
								// Note: We can't show the error here since this is in a goroutine
								// The error will be shown on the next command that needs the org
							}
						}
					} else if account.Organization != nil {
						// For API key accounts, fetch the current org details
						orgs, _, err := client.Organizations.List(ctx, nil)
						if err == nil && len(orgs) > 0 {
							// Find the current org by ULID
							for _, org := range orgs {
								// Use ID if ULID is empty (API returns ULID in ID field)
								ulid := org.ULID
								if ulid == "" && org.ID != "" {
									ulid = org.ID
								}
								if ulid == account.Organization.ULID {
									orgInfo := config.OrgInfo{
										ULID: ulid,
										Name: org.Name,
									}
									_, _ = am.RefreshOrganizations(account.Name, []config.OrgInfo{orgInfo})
									break
								}
							}
						}
					}
				}()
			}
		}
	}

	return client, nil
}

// getCurrentAccount returns the current account
func getCurrentAccount() (*config.Account, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	am, err := config.NewAccountManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	account, err := am.GetCurrent()
	if err != nil {
		return nil, fmt.Errorf("No accounts configured. Run 'dotenv login' to add an account")
	}

	// TODO: Apply any environment overrides if needed

	return account, nil
}

// displayAccountInfo displays the current account and organization info
func displayAccountInfo() error {
	account, err := getCurrentAccount()
	if err != nil {
		return err
	}

	orgName := ""
	if account.IsOAuth() {
		org, err := account.GetCurrentOrganization()
		if err == nil {
			orgName = org.Name
		}
	} else if account.Organization != nil {
		orgName = account.Organization.Name
	}

	if orgName != "" {
		ui.PrintInfo("[Account: %s | Organization: %s]", account.Name, orgName)
	} else {
		ui.PrintInfo("[Account: %s]", account.Name)
	}

	return nil
}

// ensureAuthenticated checks if we have valid credentials
func ensureAuthenticated() error {
	_, err := getCurrentAccount()
	return err
}

// refreshOAuthToken attempts to refresh an expired OAuth token
func refreshOAuthToken(am *config.AccountManager, account *config.Account) error {
	if !account.IsOAuth() {
		return fmt.Errorf("not an OAuth account")
	}

	if account.Auth.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// Create SDK client without authentication (OAuth token endpoint doesn't require auth)
	client := dotenv.NewClient(
		dotenv.WithBaseURL(account.APIURL),
	)

	// Check for TLS skip verify (development mode)
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		client = dotenv.NewClient(
			dotenv.WithBaseURL(account.APIURL),
			dotenv.WithInsecureSkipVerify(),
		)
	}

	// Attempt to refresh the token using SDK
	sdkTokenResp, _, err := client.OAuth.RefreshToken(context.Background(), account.Auth.RefreshToken, oauth.ClientID)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}

	// Update the stored tokens using AccountManager
	tokenResp := config.TokenResponse{
		AccessToken:  sdkTokenResp.AccessToken,
		RefreshToken: sdkTokenResp.RefreshToken,
		TokenType:    sdkTokenResp.TokenType,
		ExpiresIn:    sdkTokenResp.ExpiresIn,
	}

	// Save the new tokens
	return am.RefreshToken(account.Name, tokenResp)
}

// getAPIClientWithoutOrgContext returns a configured API client without organization context
// This is useful for operations like listing organizations where we don't need/have an org selected
func getAPIClientWithoutOrgContext() (*dotenv.Client, error) {
	// Check for command-line flag override first
	apiKey := viper.GetString("api_key")

	// If not set via flag, check environment variable
	if apiKey == "" {
		apiKey = os.Getenv("DOTENV_API_KEY")
	}

	// If API key is provided, bypass account system (for CI/CD)
	if apiKey != "" {
		options := []dotenv.ClientOption{
			dotenv.WithAPIKey(apiKey),
		}

		// Check for custom API URL
		apiURL := os.Getenv("DOTENV_API_URL")
		if apiURL == "" {
			apiURL = "https://api.dotenv.cloud"
		}
		options = append(options, dotenv.WithBaseURL(apiURL))

		// Check for TLS skip verify (development mode)
		if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
			options = append(options, dotenv.WithInsecureSkipVerify())
		}

		return dotenv.NewClient(options...), nil
	}

	// Get current account
	account, err := getCurrentAccount()
	if err != nil {
		return nil, err
	}

	// Create client options
	options := []dotenv.ClientOption{
		dotenv.WithBaseURL(account.APIURL),
	}

	// Check for TLS skip verify (development mode)
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		options = append(options, dotenv.WithInsecureSkipVerify())
	}

	// Handle authentication based on account type
	if account.IsOAuth() {
		// Check if token is expired
		if account.IsTokenExpired() {
			ui.PrintInfo("OAuth token expired. Attempting to refresh...")

			// Try to refresh the token
			configPath, err := config.ConfigPath()
			if err != nil {
				return nil, fmt.Errorf("failed to locate config directory for token refresh: %w", err)
			}
			am, err := config.NewAccountManager(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize account manager at '%s': %w", configPath, err)
			}

			if err := refreshOAuthToken(am, account); err != nil {
				return nil, fmt.Errorf("failed to refresh token: %w. Please login again with 'dotenv login'", err)
			}

			// Reload account after refresh
			account, err = am.GetCurrent()
			if err != nil {
				return nil, fmt.Errorf("failed to reload account '%s' after token refresh: %w", account.Name, err)
			}
		}

		options = append(options, dotenv.WithBearerToken(account.Auth.AccessToken))
	} else {
		// API key authentication
		apiKey := account.GetToken()
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found in account '%s'", account.Name)
		}
		options = append(options, dotenv.WithAPIKey(apiKey))
	}

	// Create client
	return dotenv.NewClient(options...), nil
}
