package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Validator validates configuration values
type Validator struct {
	apiKeyPattern *regexp.Regexp
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		// API key format: dotenv_[org_ulid]_[token]
		apiKeyPattern: regexp.MustCompile(`^dotenv_[0-9A-Z]{26}_[a-zA-Z0-9]+$`),
	}
}

// ValidateAPIKey validates an API key format
func (v *Validator) ValidateAPIKey(key string) error {
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if !v.apiKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid API key format")
	}

	return nil
}

// ValidateAPIURL validates an API URL
func (v *Validator) ValidateAPIURL(apiURL string) error {
	if apiURL == "" {
		return nil // Will use default
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("API URL must use http or https scheme")
	}

	if u.Host == "" {
		return fmt.Errorf("API URL must have a host")
	}

	// Allow dotenv.test for development
	if u.Host == "dotenv.test" || u.Host == "api.dotenv.test" {
		return nil
	}

	return nil
}

// ValidateOrganization validates an organization slug
func (v *Validator) ValidateOrganization(org string) error {
	if org == "" {
		return fmt.Errorf("organization cannot be empty")
	}

	if len(org) < 3 || len(org) > 50 {
		return fmt.Errorf("organization must be between 3 and 50 characters")
	}

	// Must be lowercase alphanumeric with hyphens
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(org) {
		return fmt.Errorf("organization must be lowercase alphanumeric with hyphens")
	}

	// Cannot start or end with hyphen
	if strings.HasPrefix(org, "-") || strings.HasSuffix(org, "-") {
		return fmt.Errorf("organization cannot start or end with hyphen")
	}

	// Cannot have consecutive hyphens
	if strings.Contains(org, "--") {
		return fmt.Errorf("organization cannot have consecutive hyphens")
	}

	return nil
}

// ValidateAccountName validates an account name
func (v *Validator) ValidateAccountName(name string) error {
	if name == "" {
		return fmt.Errorf("account name cannot be empty")
	}

	if len(name) > 100 {
		return fmt.Errorf("account name too long (max 100 characters)")
	}

	// Allow alphanumeric, hyphens, underscores, dots, and @ for email-like names
	if !regexp.MustCompile(`^[a-zA-Z0-9._@-]+$`).MatchString(name) {
		return fmt.Errorf("invalid account name: %s (allowed: a-z, A-Z, 0-9, -, _, .)", name)
	}

	return nil
}

// ValidateConfig validates the entire configuration
func (v *Validator) ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	if config.Version == "" {
		return fmt.Errorf("configuration version is missing")
	}

	// Check for legacy configuration
	if config.HasLegacyConfig() {
		return fmt.Errorf("old configuration format detected. Please run 'dotenv init' to set up the new account system")
	}

	// Validate each account
	for name, account := range config.Accounts {
		if err := v.ValidateAccountName(name); err != nil {
			return fmt.Errorf("account '%s': %w", name, err)
		}

		if err := v.ValidateAPIURL(account.APIURL); err != nil {
			return fmt.Errorf("account '%s': %w", name, err)
		}

		// Validate account based on type
		if account.AuthType == "oauth" {
			// OAuth accounts must have organizations
			if len(account.Organizations) == 0 {
				return fmt.Errorf("account '%s': OAuth account has no organizations", name)
			}
		} else if account.AuthType == "api_key" {
			// API key accounts must have organization info
			if account.Organization == nil {
				return fmt.Errorf("account '%s': API key account missing organization info", name)
			}
			if err := v.ValidateOrganization(account.Organization.Slug); err != nil {
				return fmt.Errorf("account '%s': %w", name, err)
			}
		}

		// Note: Don't validate API key here as it might be encrypted
	}

	// Validate current account exists
	if config.CurrentAccount != "" {
		if _, exists := config.Accounts[config.CurrentAccount]; !exists {
			return fmt.Errorf("current account '%s' does not exist", config.CurrentAccount)
		}
	}

	return nil
}
