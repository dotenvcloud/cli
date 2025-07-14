package config

import (
	"fmt"
	"time"

	"github.com/dotenv/cli/internal/constants"
)

// AccountManager manages accounts in the configuration
type AccountManager struct {
	config *Config
	loader *Loader
}

// NewAccountManager creates a new account manager
func NewAccountManager(configPath string) (*AccountManager, error) {
	loader := NewLoader(configPath)

	config, err := loader.Load()
	if err != nil {
		// If config doesn't exist, create default
		if !loader.Exists() {
			config = DefaultConfig()
			if err := loader.Save(config); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Check for legacy config
	if config.HasLegacyConfig() {
		return nil, fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
	}

	return &AccountManager{
		config: config,
		loader: loader,
	}, nil
}

// Create creates a new account
func (am *AccountManager) Create(name string, apiURL string, authType string) error {
	if name == "" {
		return fmt.Errorf("account name cannot be empty")
	}

	if authType != constants.AuthTypeOAuth && authType != constants.AuthTypeAPIKey {
		return fmt.Errorf("invalid auth type: %s (must be 'oauth' or 'api_key')", authType)
	}

	if apiURL == "" {
		apiURL = constants.LegacyAPIURL
	}

	// Check if account already exists
	if _, exists := am.config.Accounts[name]; exists {
		return fmt.Errorf("Account name already exists: %s", name)
	}

	// Create new account
	account := Account{
		Name:      name,
		AuthType:  authType,
		APIURL:    apiURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	// Add to config
	if err := am.config.AddAccount(name, account); err != nil {
		return err
	}

	// Save config
	return am.loader.Save(am.config)
}

// CreateWithAPIKey creates a new API key account
func (am *AccountManager) CreateWithAPIKey(name string, apiURL string, apiKey string, orgInfo *OrgInfo) error {
	if err := am.Create(name, apiURL, constants.AuthTypeAPIKey); err != nil {
		return err
	}

	// Update with API key and organization
	updates := map[string]interface{}{
		"auth": AuthData{
			APIKey: apiKey,
		},
		"organization":            orgInfo,
		"organization_fetched_at": time.Now(),
	}

	return am.Update(name, updates)
}

// CreateWithOAuth creates a new OAuth account
func (am *AccountManager) CreateWithOAuth(name string, apiURL string, tokens TokenResponse, orgs []OrgInfo, selectedOrgULID string) error {
	if err := am.Create(name, apiURL, constants.AuthTypeOAuth); err != nil {
		return err
	}

	// Update with OAuth data
	updates := map[string]interface{}{
		"auth": AuthData{
			AccessToken:           tokens.AccessToken,
			RefreshToken:          tokens.RefreshToken,
			TokenType:             tokens.TokenType,
			ExpiresAt:             time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
			RefreshTokenExpiresAt: time.Now().Add(60 * 24 * time.Hour), // 60 days default (matching web app)
		},
		"organizations":            orgs,
		"current_organization":     selectedOrgULID,
		"organizations_fetched_at": time.Now(),
	}

	return am.Update(name, updates)
}

// Update updates an existing account
func (am *AccountManager) Update(name string, updates map[string]interface{}) error {
	account, exists := am.config.Accounts[name]
	if !exists {
		return fmt.Errorf("account '%s' not found", name)
	}

	// Update fields
	for key, value := range updates {
		switch key {
		case "auth_type":
			if v, ok := value.(string); ok {
				account.AuthType = v
			}
		case "api_url":
			if v, ok := value.(string); ok {
				account.APIURL = v
			}
		case "auth":
			if v, ok := value.(AuthData); ok {
				account.Auth = v
			}
		case "organizations":
			if v, ok := value.([]OrgInfo); ok {
				account.Organizations = v
			}
		case "current_organization":
			if v, ok := value.(string); ok {
				account.CurrentOrganization = v
			}
		case "organizations_fetched_at":
			if v, ok := value.(time.Time); ok {
				t := v
				account.OrganizationsFetchedAt = &t
			}
		case "organization":
			if v, ok := value.(*OrgInfo); ok {
				account.Organization = v
			}
		case "organization_fetched_at":
			if v, ok := value.(time.Time); ok {
				t := v
				account.OrganizationFetchedAt = &t
			}
		}
	}

	account.UpdatedAt = time.Now()
	account.LastUsed = time.Now()

	// Save updated account
	am.config.Accounts[name] = account
	return am.loader.Save(am.config)
}

// Use sets an account as the current active account
func (am *AccountManager) Use(name string) error {
	if err := am.config.SetCurrentAccount(name); err != nil {
		return err
	}

	// Update last used
	if account, exists := am.config.Accounts[name]; exists {
		account.LastUsed = time.Now()
		am.config.Accounts[name] = account
	}

	return am.loader.Save(am.config)
}

// List returns all account names
func (am *AccountManager) List() []string {
	return am.config.ListAccounts()
}

// Get returns an account by name
func (am *AccountManager) Get(name string) (*Account, error) {
	account, exists := am.config.Accounts[name]
	if !exists {
		return nil, fmt.Errorf("Account not found: %s", name)
	}
	return &account, nil
}

// GetCurrent returns the current account
func (am *AccountManager) GetCurrent() (*Account, error) {
	return am.config.GetCurrentAccount()
}

// Remove removes an account
func (am *AccountManager) Remove(name string) error {
	if err := am.config.RemoveAccount(name); err != nil {
		return err
	}
	return am.loader.Save(am.config)
}

// Rename renames an account
func (am *AccountManager) Rename(oldName, newName string) error {
	if err := am.config.RenameAccount(oldName, newName); err != nil {
		return err
	}
	return am.loader.Save(am.config)
}

// RefreshToken updates OAuth tokens
func (am *AccountManager) RefreshToken(name string, tokens TokenResponse) error {
	updates := map[string]interface{}{
		"auth": AuthData{
			AccessToken:           tokens.AccessToken,
			RefreshToken:          tokens.RefreshToken,
			TokenType:             tokens.TokenType,
			ExpiresAt:             time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
			RefreshTokenExpiresAt: time.Now().Add(60 * 24 * time.Hour), // 60 days default (matching web app)
		},
	}

	return am.Update(name, updates)
}

// SetOrganization sets the current organization for an OAuth account
func (am *AccountManager) SetOrganization(accountName string, orgULID string) error {
	account, exists := am.config.Accounts[accountName]
	if !exists {
		return fmt.Errorf("account '%s' not found", accountName)
	}

	if !account.IsOAuth() {
		return fmt.Errorf("cannot change organization for API key account")
	}

	if err := account.SetCurrentOrganization(orgULID); err != nil {
		return err
	}

	// Save updated account
	am.config.Accounts[accountName] = account
	return am.loader.Save(am.config)
}

// RefreshOrganizations updates the organization list for an account
// Returns true if the current organization was removed
func (am *AccountManager) RefreshOrganizations(accountName string, orgs []OrgInfo) (bool, error) {
	account, exists := am.config.Accounts[accountName]
	if !exists {
		return false, fmt.Errorf("account '%s' not found", accountName)
	}

	orgWasRemoved := false

	if account.IsOAuth() {
		account.Organizations = orgs
		now := time.Now()
		account.OrganizationsFetchedAt = &now

		// Verify current org still exists
		currentOrgExists := false
		for _, org := range orgs {
			if org.ULID == account.CurrentOrganization {
				currentOrgExists = true
				break
			}
		}

		if !currentOrgExists && account.CurrentOrganization != "" {
			orgWasRemoved = true
			if len(orgs) > 0 {
				// Current org no longer exists, reset to first
				account.CurrentOrganization = orgs[0].ULID
			} else {
				// No orgs available
				account.CurrentOrganization = ""
			}
		}
	} else {
		// For API key accounts, update single organization
		if len(orgs) > 0 {
			account.Organization = &orgs[0]
			now := time.Now()
			account.OrganizationFetchedAt = &now
		}
	}

	account.UpdatedAt = time.Now()
	am.config.Accounts[accountName] = account
	if err := am.loader.Save(am.config); err != nil {
		return orgWasRemoved, err
	}
	return orgWasRemoved, nil
}

// TokenResponse represents OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}
