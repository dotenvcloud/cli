package errors

import (
	"fmt"
)

// AuthError represents an authentication error
type AuthError struct {
	Type    string
	Message string
	Err     error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError creates a new authentication error
func NewAuthError(authType, message string, err error) *AuthError {
	return &AuthError{
		Type:    authType,
		Message: message,
		Err:     err,
	}
}

// TokenExpiredError represents an expired token error
type TokenExpiredError struct {
	AccountName string
	ExpiresAt   string
}

func (e *TokenExpiredError) Error() string {
	return fmt.Sprintf("token expired for account '%s' at %s", e.AccountName, e.ExpiresAt)
}

// OrganizationNotFoundError represents an organization not found error
type OrganizationNotFoundError struct {
	Identifier  string
	Suggestions []string
}

func (e *OrganizationNotFoundError) Error() string {
	if len(e.Suggestions) > 0 {
		return fmt.Sprintf("organization not found: %s. Did you mean one of: %v", e.Identifier, e.Suggestions)
	}
	return fmt.Sprintf("organization not found: %s", e.Identifier)
}

// NoCurrentAccountError represents no current account error
type NoCurrentAccountError struct{}

func (e *NoCurrentAccountError) Error() string {
	return "no current account selected. Run 'dotenv account add' to add an account"
}

// NoOrganizationSelectedError represents no organization selected error
type NoOrganizationSelectedError struct {
	AccountName string
}

func (e *NoOrganizationSelectedError) Error() string {
	return fmt.Sprintf("no organization selected for account '%s'. Run 'dotenv org use <ulid>' to select one", e.AccountName)
}

// InvalidAPIKeyError represents an invalid API key format error
type InvalidAPIKeyError struct {
	Reason string
}

func (e *InvalidAPIKeyError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid API key format: %s", e.Reason)
	}
	return "invalid API key format"
}

// AccountNotFoundError represents an account not found error
type AccountNotFoundError struct {
	AccountName string
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("account '%s' not found", e.AccountName)
}

// ConfigurationError represents a configuration error
type ConfigurationError struct {
	Field   string
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in '%s': %s", e.Field, e.Message)
}
