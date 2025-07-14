package oauth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"

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
	// Create OAuth2 client
	client := NewOAuth2Client(constants.OAuthClientID, af.BaseURL)

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
	authURL := client.GetAuthorizationURL(AuthorizeParams{
		RedirectURI:         callbackServer.GetCallbackURL(),
		State:               state,
		CodeChallenge:       pkce.Challenge,
		CodeChallengeMethod: pkce.Method,
	})

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

		tokens, err := client.ExchangeCode(timeoutCtx, code, pkce.Verifier, callbackServer.GetCallbackURL())
		if err != nil {
			return fmt.Errorf("failed to exchange code: %w", err)
		}

		// Fetch user info and organizations
		ui.PrintInfo("Fetching user information...")

		userInfo, orgs, err := af.fetchUserAndOrganizations(tokens.AccessToken)
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
		tokenResp := config.TokenResponse{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			TokenType:    tokens.TokenType,
			ExpiresIn:    tokens.ExpiresIn,
		}

		// Check if account already exists
		existingAccount, err := am.Get(accountName)
		if err == nil && existingAccount != nil {
			// Account exists, update it
			ui.PrintInfo("Updating existing account: %s", accountName)

			// Update tokens
			if err := am.RefreshToken(accountName, tokenResp); err != nil {
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
			if err := am.CreateWithOAuth(accountName, af.BaseURL, tokenResp, orgInfos, selectedOrgULID); err != nil {
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

	// Create API client with the OAuth token
	client := dotenv.NewClient(
		dotenv.WithBearerToken(accessToken),
		dotenv.WithBaseURL(apiURL),
	)

	// For development
	if os.Getenv("DOTENV_TLS_SKIP_VERIFY") != "" {
		client.SetTLSSkipVerify(true)
	}

	// Get user info which includes organizations
	req, err := client.NewRequest(context.Background(), "GET", "/api/v1/user", nil)
	if err != nil {
		return userInfo, nil, fmt.Errorf("failed to create request: %w", err)
	}

	var userResp struct {
		ID            int64          `json:"id"`
		Email         string         `json:"email"`
		Name          string         `json:"name"`
		Organizations []Organization `json:"organizations"`
	}

	_, err = client.Do(context.Background(), req, &userResp)
	if err != nil {
		return userInfo, nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	userInfo.ID = userResp.ID
	userInfo.Email = userResp.Email
	userInfo.Name = userResp.Name

	return userInfo, userResp.Organizations, nil
}
