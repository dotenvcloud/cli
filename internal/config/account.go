package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/dotenv/cli/internal/constants"
)

// Account represents an authentication account (OAuth or API Key)
type Account struct {
	Name      string    `yaml:"name"`
	AuthType  string    `yaml:"auth_type"` // "oauth" or "api_key"
	APIURL    string    `yaml:"api_url"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
	LastUsed  time.Time `yaml:"last_used"`

	// Auth data (consistent for both OAuth and API Key)
	Auth AuthData `yaml:"auth"`

	// For OAuth accounts
	Organizations          []OrgInfo  `yaml:"organizations,omitempty"`
	CurrentOrganization    string     `yaml:"current_organization,omitempty"` // ULID reference
	OrganizationsFetchedAt *time.Time `yaml:"organizations_fetched_at,omitempty"`

	// For API Key accounts
	Organization          *OrgInfo   `yaml:"organization,omitempty"`
	OrganizationFetchedAt *time.Time `yaml:"organization_fetched_at,omitempty"`
}

// OrgInfo represents organization information
type OrgInfo struct {
	ULID string `yaml:"ulid"`
	Name string `yaml:"name"`
}

// IsOAuth returns true if this is an OAuth account
func (a *Account) IsOAuth() bool {
	return a.AuthType == constants.AuthTypeOAuth
}

// IsAPIKey returns true if this is an API key account
func (a *Account) IsAPIKey() bool {
	return a.AuthType == constants.AuthTypeAPIKey
}

// GetToken returns the appropriate authentication token
func (a *Account) GetToken() string {
	if a.IsOAuth() {
		return a.Auth.AccessToken
	}
	return a.Auth.APIKey
}

// GetTokenType returns the token type for Authorization header
func (a *Account) GetTokenType() string {
	if a.IsOAuth() {
		return a.Auth.TokenType // Usually "Bearer"
	}
	return "" // API keys don't use a type prefix
}

// IsTokenExpired checks if OAuth token is expired
func (a *Account) IsTokenExpired() bool {
	if !a.IsOAuth() {
		return false
	}
	return time.Now().After(a.Auth.ExpiresAt)
}

// IsRefreshTokenExpired checks if OAuth refresh token is expired
func (a *Account) IsRefreshTokenExpired() bool {
	if !a.IsOAuth() {
		return false
	}
	return time.Now().After(a.Auth.RefreshTokenExpiresAt)
}

// GetCurrentOrganization returns the current organization info
func (a *Account) GetCurrentOrganization() (*OrgInfo, error) {
	if a.IsOAuth() {
		if a.CurrentOrganization == "" {
			return nil, fmt.Errorf("no organization selected")
		}

		// Find org by ULID
		for _, org := range a.Organizations {
			if org.ULID == a.CurrentOrganization {
				return &org, nil
			}
		}
		return nil, fmt.Errorf("current organization '%s' not found in available organizations", a.CurrentOrganization)
	}

	// API key account
	if a.Organization == nil {
		return nil, fmt.Errorf("no organization info available")
	}
	return a.Organization, nil
}

// GetOrganizationIdentifier returns a valid identifier for API calls
// Since the web API only supports ULID, always return ULID
func (a *Account) GetOrganizationIdentifier() (string, error) {
	org, err := a.GetCurrentOrganization()
	if err != nil {
		return "", err
	}

	// Always use ULID since the web API doesn't support slugs
	if org.ULID != "" {
		return org.ULID, nil
	}

	// No valid identifier
	return "", fmt.Errorf("organization '%s' has no valid ULID. Please refresh your account data with 'dotenv account refresh'", org.Name)
}

// GetCurrentOrganizationULID returns the current organization ULID
func (a *Account) GetCurrentOrganizationULID() string {
	ulid, _ := a.GetOrganizationIdentifier()
	return ulid
}

// SetCurrentOrganization sets the current organization for OAuth accounts
func (a *Account) SetCurrentOrganization(ulid string) error {
	if !a.IsOAuth() {
		return fmt.Errorf("cannot change organization for API key account")
	}

	// Verify org exists
	found := false
	for _, org := range a.Organizations {
		if org.ULID == ulid {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("organization '%s' not found in available organizations", ulid)
	}

	a.CurrentOrganization = ulid
	a.UpdatedAt = time.Now()
	return nil
}

// NeedsOrganizationRefresh checks if organization data should be refreshed
func (a *Account) NeedsOrganizationRefresh() bool {
	var fetchedAt *time.Time

	if a.IsOAuth() {
		fetchedAt = a.OrganizationsFetchedAt
	} else {
		fetchedAt = a.OrganizationFetchedAt
	}

	if fetchedAt == nil {
		return true
	}

	// Refresh if older than configured interval
	return time.Since(*fetchedAt) > time.Duration(constants.OrganizationRefreshHours)*time.Hour
}

// ResolveOrganization finds an organization by ULID
func ResolveOrganization(identifier string, orgs []OrgInfo) (*OrgInfo, error) {
	if identifier == "" {
		return nil, fmt.Errorf("organization identifier cannot be empty")
	}

	// Check if it's a valid ULID (26 characters, uppercase alphanumeric)
	if len(identifier) == 26 && isValidULID(identifier) {
		// Find by ULID
		for _, org := range orgs {
			if org.ULID == identifier {
				return &org, nil
			}
		}
		return nil, fmt.Errorf("Organization not found: %s", identifier)
	}

	// Try exact name match first (case insensitive)
	lowerIdent := strings.ToLower(identifier)
	for _, org := range orgs {
		if strings.ToLower(org.Name) == lowerIdent {
			return &org, nil
		}
	}
	
	// If no exact match, try to find partial matches for suggestions
	var suggestions []string
	for _, org := range orgs {
		if strings.Contains(strings.ToLower(org.Name), lowerIdent) {
			suggestions = append(suggestions, fmt.Sprintf("%s (%s)", org.Name, org.ULID))
		}
	}

	if len(suggestions) > 0 {
		return nil, fmt.Errorf("Organization not found: %s. Did you mean one of: %s",
			identifier, strings.Join(suggestions, ", "))
	}

	return nil, fmt.Errorf("Organization not found: %s", identifier)
}

// isValidULID checks if a string looks like a ULID
func isValidULID(s string) bool {
	// ULIDs are 26 characters, using Crockford's base32
	if len(s) != 26 {
		return false
	}

	// Check if all characters are valid base32 (excluding I, L, O, U)
	// Accept both uppercase and lowercase
	validChars := "0123456789ABCDEFGHJKMNPQRSTVWXYZabcdefghjkmnpqrstvwxyz"
	for _, c := range s {
		if !strings.ContainsRune(validChars, c) {
			return false
		}
	}

	return true
}
