package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/browser"

	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

// AuthFlow handles the complete OAuth2 authentication flow
type AuthFlow struct {
	BaseURL       string
	CallbackPort  string
	NoBrowser     bool
	IsInteractive bool
	clientFactory *client.Factory
}

// OrganizationResponse represents the API response for user organizations
type OrganizationResponse struct {
	Organizations []Organization `json:"organizations"`
}

// Organization represents an organization the user has access to
type Organization struct {
	ID   string `json:"id"`   // ULID comes in ID field from API
	ULID string `json:"ulid"` // Not used but kept for compatibility
	Name string `json:"name"`
}

// Run executes the OAuth2 authentication flow
func (af *AuthFlow) Run(ctx context.Context, am *config.AccountManager) error {
	// Initialize client factory if not already set
	if af.clientFactory == nil {
		af.clientFactory = client.NewFactory(af.BaseURL)
	}

	// Generate PKCE challenge
	pkce, err := GeneratePKCEChallenge()
	if err != nil {
		return fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}

	// Generate state
	state, err := GenerateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	// Create callback server
	callbackServer := NewCallbackServer(state)

	// Find available port
	_, err = callbackServer.FindAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	// Start callback server
	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()

	if err := callbackServer.Start(serverCtx); err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	// Build authorization URL
	authURL := af.buildAuthorizationURL(callbackServer.GetCallbackURL(), state, pkce.Challenge, pkce.Method)

	// Open browser or print URL
	if af.NoBrowser {
		fmt.Println("Please open the following URL in your browser:")
		fmt.Println()
		fmt.Println(authURL)
		fmt.Println()
	} else {
		ui.PrintInfo("Opening browser for authentication...")
		if err := browser.OpenURL(authURL); err != nil {
			ui.PrintWarning("Failed to open browser: %v", err)
			fmt.Println("Please open the following URL manually:")
			fmt.Println()
			fmt.Println(authURL)
			fmt.Println()
		}
	}

	ui.PrintInfo("Waiting for authentication...")

	// Wait for auth result
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case code := <-callbackServer.AuthCode:
		// Exchange code for tokens
		ui.PrintInfo("Exchanging authorization code for tokens...")

		// Create unauthenticated SDK client for OAuth operations
		sdkClient := af.clientFactory.NewUnauthenticatedClient(af.BaseURL, config.ShouldSkipTLSVerify())

		// Use SDK to exchange code for tokens
		req := dotenv.OAuthTokenAuthCodeRequest{
			Code:         code,
			CodeVerifier: pkce.Verifier,
			ClientID:     constants.OAuthClientID,
		}

		tokenResp, _, err := sdkClient.OAuth.ExchangeToken(timeoutCtx, req)
		if err != nil {
			return fmt.Errorf("failed to exchange code: %w", err)
		}

		// Fetch user info and organizations
		ui.PrintInfo("Fetching user information...")

		userInfo, orgs, err := af.fetchUserAndOrganizations(tokenResp.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to fetch user info: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("no organizations found for this account")
		}

		// Authentication successful
		ui.PrintSuccess("Authentication successful!")
		ui.PrintInfo("Found %d organization(s)", len(orgs))

		// Default account name is user email
		defaultAccountName := userInfo.Email
		accountName := defaultAccountName

		// Let user select organization
		var selectedOrg Organization
		if len(orgs) > 1 && af.IsInteractive {
			ui.PrintInfo("\nAvailable organizations:")
			orgOptions := make([]string, len(orgs))
			for i, org := range orgs {
				// Show ULID from ID field
				ulid := org.ULID
				if ulid == "" && org.ID != "" {
					ulid = org.ID
				}
				orgOptions[i] = fmt.Sprintf("%s (%s)", org.Name, ulid)
			}

			selected, err := ui.Select("Select your organization", orgOptions)
			if err != nil {
				return err
			}

			// Find selected org by name
			for _, org := range orgs {
				if strings.Contains(selected, org.Name) {
					selectedOrg = org
					break
				}
			}
		} else if len(orgs) == 1 {
			// Single org, use it
			selectedOrg = orgs[0]
			// Show ULID from ID field
			ulid := selectedOrg.ULID
			if ulid == "" && selectedOrg.ID != "" {
				ulid = selectedOrg.ID
			}
			ui.PrintInfo("Using organization: %s (%s)", selectedOrg.Name, ulid)
		} else {
			// No orgs
			return fmt.Errorf("no organizations found for this account")
		}

		// Get account name from user if interactive
		if af.IsInteractive {
			// Check if this account already exists
			if existing, _ := am.Get(defaultAccountName); existing != nil {
				// Account exists, ask if they want to update it
				update, err := ui.Confirm(fmt.Sprintf("Update existing account '%s'?", defaultAccountName), true)
				if err != nil {
					return err
				}

				if update {
					accountName = defaultAccountName
				} else {
					// Let them enter a different name
					name, err := ui.Input("New account name", defaultAccountName+"-new", nil)
					if err != nil {
						return err
					}
					accountName = name
				}
			} else {
				// New account
				name, err := ui.Input("Account name", defaultAccountName, nil)
				if err != nil {
					return err
				}
				accountName = name
			}
		}

		// Convert organizations to config format
		var orgInfos []config.OrgInfo
		for _, org := range orgs {
			// Use ID if ULID is empty (API returns ULID in ID field)
			ulid := org.ULID
			if ulid == "" && org.ID != "" {
				ulid = org.ID
			}
			orgInfos = append(orgInfos, config.OrgInfo{
				ULID: ulid,
				Name: org.Name,
			})
		}

		// Create OAuth account with tokens and organizations
		configTokenResp := config.TokenResponse{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
			ExpiresIn:    tokenResp.ExpiresIn,
		}

		// Check if account already exists
		existingAccount, err := am.Get(accountName)
		if err == nil && existingAccount != nil {
			// Account exists, update it
			ui.PrintInfo("Updating existing account: %s", accountName)

			// Update tokens
			if err := am.RefreshToken(accountName, configTokenResp); err != nil {
				return fmt.Errorf("failed to update tokens: %w", err)
			}

			// Update organizations
			_, err := am.RefreshOrganizations(accountName, orgInfos)
			if err != nil {
				return fmt.Errorf("failed to update organizations: %w", err)
			}

			// Update current organization if needed
			// Use ID if ULID is empty (API returns ULID in ID field)
			selectedOrgULID := selectedOrg.ULID
			if selectedOrgULID == "" && selectedOrg.ID != "" {
				selectedOrgULID = selectedOrg.ID
			}
			if err := am.SetOrganization(accountName, selectedOrgULID); err != nil {
				return fmt.Errorf("failed to set organization: %w", err)
			}
		} else {
			// Create new account
			// Use ID if ULID is empty (API returns ULID in ID field)
			selectedOrgULID := selectedOrg.ULID
			if selectedOrgULID == "" && selectedOrg.ID != "" {
				selectedOrgULID = selectedOrg.ID
			}
			if err := am.CreateWithOAuth(accountName, af.BaseURL, configTokenResp, orgInfos, selectedOrgULID); err != nil {
				return fmt.Errorf("failed to create account: %w", err)
			}
		}

		// Set as current account
		if err := am.Use(accountName); err != nil {
			return fmt.Errorf("failed to set current account: %w", err)
		}

		ui.PrintSuccess("Login complete!")
		ui.PrintInfo("Current account: %s", accountName)
		ui.PrintInfo("Current organization: %s", selectedOrg.Name)

		if len(orgs) > 1 {
			ui.PrintInfo("\nTo switch organizations, use: dotenv org use <ulid>")
		}

		return nil

	case err := <-callbackServer.AuthError:
		return fmt.Errorf("authentication failed: %w", err)

	case <-timeoutCtx.Done():
		return fmt.Errorf("authentication timed out")
	}
}

// fetchUserAndOrganizations fetches the user info and organizations
func (af *AuthFlow) fetchUserAndOrganizations(accessToken string) (userInfo struct {
	ID    int64
	Email string
	Name  string
}, orgs []Organization, err error) {
	// For development, use API subdomain for API calls
	apiURL := af.BaseURL
	if strings.Contains(af.BaseURL, "dotenv.test") {
		apiURL = constants.TestAPIURL
	}

	// Create API client with the OAuth token using factory
	sdkClient := af.clientFactory.NewClient(client.Options{
		BaseURL:            apiURL,
		BearerToken:        accessToken,
		InsecureSkipVerify: config.ShouldSkipTLSVerify(),
	})

	// Use the SDK's UserService to get user info
	user, sdkOrgs, _, err := sdkClient.User.GetAuthenticatedUser(context.Background())
	if err != nil {
		return userInfo, nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	// Convert user ID from string to int64
	var userID int64
	if _, err := fmt.Sscanf(user.ID, "%d", &userID); err != nil {
		// If conversion fails, just use 0
		userID = 0
	}

	userInfo.ID = userID
	userInfo.Email = user.Email
	userInfo.Name = user.Name

	// Convert SDK organizations to CLI organizations
	orgs = make([]Organization, len(sdkOrgs))
	for i, sdkOrg := range sdkOrgs {
		orgs[i] = Organization{
			ID:   sdkOrg.ID,   // This contains the ULID from the API
			ULID: sdkOrg.ULID, // Also populate ULID field for compatibility
			Name: sdkOrg.Name,
		}
	}

	return userInfo, orgs, nil
}

// buildAuthorizationURL constructs the OAuth authorization URL
func (af *AuthFlow) buildAuthorizationURL(redirectURI, state, codeChallenge, challengeMethod string) string {
	// Store original base URL for OAuth URL construction
	oauthBaseURL := af.BaseURL

	// Remove /api/v1 suffix if present
	oauthBaseURL = strings.TrimSuffix(oauthBaseURL, "/api/v1")
	oauthBaseURL = strings.TrimSuffix(oauthBaseURL, "/api")

	// For development, use the web URL for OAuth endpoints
	if strings.Contains(oauthBaseURL, "dotenv.test") || strings.Contains(oauthBaseURL, "api.dotenv.test") {
		oauthBaseURL = "https://dotenv.test"
	}

	u, err := url.Parse(oauthBaseURL)
	if err != nil {
		// Fallback to default URL if parsing fails
		u = &url.URL{
			Scheme: "https",
			Host:   "api.dotenv.cloud",
		}
	}
	u.Path = "/oauth/authorize"

	q := u.Query()
	q.Set("client_id", constants.OAuthClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", challengeMethod)

	u.RawQuery = q.Encode()
	return u.String()
}
