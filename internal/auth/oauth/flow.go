package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/browser"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenv/cli/internal/client"
	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
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
	if af.clientFactory == nil {
		af.clientFactory = client.NewFactory(af.BaseURL)
	}

	pkce, err := GeneratePKCEChallenge()
	if err != nil {
		return fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}
	state, err := GenerateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	callbackServer := NewCallbackServer(state)
	if _, err = callbackServer.FindAvailablePort(); err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	if err := callbackServer.Start(serverCtx); err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	authURL := af.buildAuthorizationURL(callbackServer.GetCallbackURL(), state, pkce.Challenge, pkce.Method)
	af.presentAuthURL(authURL)
	ui.PrintInfof("Waiting for authentication...")

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case code := <-callbackServer.AuthCode:
		return af.handleAuthCode(timeoutCtx, am, code, pkce.Verifier)
	case err := <-callbackServer.AuthError:
		return fmt.Errorf("authentication failed: %w", err)
	case <-timeoutCtx.Done():
		return fmt.Errorf("authentication timed out")
	}
}

func (af *AuthFlow) presentAuthURL(authURL string) {
	if af.NoBrowser {
		fmt.Println("Please open the following URL in your browser:")
		fmt.Println()
		fmt.Println(authURL)
		fmt.Println()
		return
	}
	ui.PrintInfof("Opening browser for authentication...")
	if err := browser.OpenURL(authURL); err != nil {
		ui.PrintWarningf("Failed to open browser: %v", err)
		fmt.Println("Please open the following URL manually:")
		fmt.Println()
		fmt.Println(authURL)
		fmt.Println()
	}
}

func (af *AuthFlow) handleAuthCode(ctx context.Context, am *config.AccountManager, code, verifier string) error {
	ui.PrintInfof("Exchanging authorization code for tokens...")

	sdkClient := af.clientFactory.NewUnauthenticatedClient(af.BaseURL, config.ShouldSkipTLSVerify())
	req := dotenv.OAuthTokenAuthCodeRequest{
		Code:         code,
		CodeVerifier: verifier,
		ClientID:     constants.OAuthClientID,
	}
	tokenResp, exchangeResp, err := sdkClient.OAuth.ExchangeToken(ctx, req)
	if exchangeResp != nil {
		defer exchangeResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	ui.PrintInfof("Fetching user information...")
	userInfo, orgs, err := af.fetchUserAndOrganizations(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to fetch user info: %w", err)
	}
	if len(orgs) == 0 {
		return fmt.Errorf("no organizations found for this account")
	}

	ui.PrintSuccessf("Authentication successful!")
	ui.PrintInfof("Found %d organization(s)", len(orgs))

	selectedOrg, err := af.selectOrganization(orgs)
	if err != nil {
		return err
	}

	accountName, err := af.resolveAccountName(am, userInfo.Email)
	if err != nil {
		return err
	}

	orgInfos := orgsToOrgInfos(orgs)
	configTokenResp := config.TokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
	}

	if err := af.persistAccount(am, accountName, configTokenResp, orgInfos, selectedOrg); err != nil {
		return err
	}
	if err := am.Use(accountName); err != nil {
		return fmt.Errorf("failed to set current account: %w", err)
	}

	ui.PrintSuccessf("Login complete!")
	ui.PrintInfof("Current account: %s", accountName)
	ui.PrintInfof("Current organization: %s", selectedOrg.Name)
	if len(orgs) > 1 {
		ui.PrintInfof("\nTo switch organizations, use: dotenv org use <ulid>")
	}
	return nil
}

func (af *AuthFlow) selectOrganization(orgs []Organization) (Organization, error) {
	if len(orgs) == 1 {
		ulid := orgULID(orgs[0])
		ui.PrintInfof("Using organization: %s (%s)", orgs[0].Name, ulid)
		return orgs[0], nil
	}
	if !af.IsInteractive {
		return Organization{}, fmt.Errorf("multiple organizations found but no interactive selection available")
	}
	ui.PrintInfof("\nAvailable organizations:")
	orgOptions := make([]string, len(orgs))
	for i, org := range orgs {
		orgOptions[i] = fmt.Sprintf("%s (%s)", org.Name, orgULID(org))
	}
	selected, err := ui.Select("Select your organization", orgOptions)
	if err != nil {
		return Organization{}, err
	}
	for _, org := range orgs {
		if strings.Contains(selected, org.Name) {
			return org, nil
		}
	}
	return Organization{}, fmt.Errorf("selected organization not found")
}

func (af *AuthFlow) resolveAccountName(am *config.AccountManager, defaultAccountName string) (string, error) {
	if !af.IsInteractive {
		return defaultAccountName, nil
	}
	if existing, _ := am.Get(defaultAccountName); existing != nil {
		update, confirmErr := ui.Confirm(fmt.Sprintf("Update existing account '%s'?", defaultAccountName), true)
		if confirmErr != nil {
			return "", confirmErr
		}
		if update {
			return defaultAccountName, nil
		}
		return ui.Input("New account name", defaultAccountName+"-new", nil)
	}
	return ui.Input("Account name", defaultAccountName, nil)
}

func (af *AuthFlow) persistAccount(
	am *config.AccountManager, accountName string,
	tok config.TokenResponse, orgInfos []config.OrgInfo,
	selectedOrg Organization,
) error {
	selectedOrgULID := orgULID(selectedOrg)
	existingAccount, err := am.Get(accountName)
	if err == nil && existingAccount != nil {
		ui.PrintInfof("Updating existing account: %s", accountName)
		if err := am.RefreshToken(accountName, tok); err != nil {
			return fmt.Errorf("failed to update tokens: %w", err)
		}
		if _, err := am.RefreshOrganizations(accountName, orgInfos); err != nil {
			return fmt.Errorf("failed to update organizations: %w", err)
		}
		if err := am.SetOrganization(accountName, selectedOrgULID); err != nil {
			return fmt.Errorf("failed to set organization: %w", err)
		}
		return nil
	}
	if err := am.CreateWithOAuth(accountName, af.BaseURL, tok, orgInfos, selectedOrgULID); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

func orgULID(org Organization) string {
	if org.ULID != "" {
		return org.ULID
	}
	return org.ID
}

func orgsToOrgInfos(orgs []Organization) []config.OrgInfo {
	orgInfos := make([]config.OrgInfo, 0, len(orgs))
	for _, org := range orgs {
		orgInfos = append(orgInfos, config.OrgInfo{ULID: orgULID(org), Name: org.Name})
	}
	return orgInfos
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
	user, sdkOrgs, userResp, err := sdkClient.User.GetAuthenticatedUser(context.Background())
	if userResp != nil {
		defer userResp.Body.Close()
	}
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
