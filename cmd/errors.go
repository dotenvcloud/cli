package cmd

import (
	"errors"
	"fmt"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/dotenv/sdk-go"
)

// ErrClientManagedKey indicates that the project uses client-managed encryption
var ErrClientManagedKey = errors.New("project uses client-managed encryption")

// HandleAPIError provides user-friendly error messages with suggested actions
func HandleAPIError(err error, account *config.Account) error {
	if err == nil {
		return nil
	}

	// Check for client-managed key error
	if errors.Is(err, ErrClientManagedKey) {
		return fmt.Errorf("this project uses client-managed encryption. Please provide your encryption key using --client-key flag or you will be prompted for it")
	}

	// Check for specific error types
	switch {
	case dotenv.IsNotFound(err):
		if e, ok := err.(*dotenv.ErrNotFound); ok {
			return handleNotFoundError(e)
		}

	case dotenv.IsUnauthorized(err):
		if e, ok := err.(*dotenv.ErrUnauthorized); ok {
			return handleUnauthorizedError(e, account)
		}

	case dotenv.IsForbidden(err):
		if e, ok := err.(*dotenv.ErrForbidden); ok {
			return handleForbiddenError(e)
		}

	case dotenv.IsRateLimited(err):
		if e, ok := err.(*dotenv.ErrRateLimited); ok {
			return fmt.Errorf("rate limit exceeded. Please wait %d seconds before trying again", e.RetryAfter)
		}

	case dotenv.IsValidation(err):
		if e, ok := err.(*dotenv.ErrValidation); ok {
			return fmt.Errorf("validation error: %v", e.Errors)
		}
	}

	// Default error
	return err
}

func handleNotFoundError(err *dotenv.ErrNotFound) error {
	switch err.Resource {
	case "organization":
		return fmt.Errorf("Organization '%s' not found. Run 'dotenv org list' to see available organizations", err.ID)
	case "project":
		return fmt.Errorf("Project '%s' not found. Run 'dotenv list projects' to see available projects", err.ID)
	case "target":
		return fmt.Errorf("Target '%s' not found. Run 'dotenv list targets <project>' to see available targets", err.ID)
	case "environment":
		return fmt.Errorf("Environment '%s' not found. Run 'dotenv list environments <project>/<target>' to see available environments", err.ID)
	case "secret":
		return fmt.Errorf("Secret '%s' not found", err.ID)
	default:
		if err.ID != "" {
			return fmt.Errorf("%s '%s' not found", err.Resource, err.ID)
		}
		return fmt.Errorf("%s not found", err.Resource)
	}
}

func handleUnauthorizedError(err *dotenv.ErrUnauthorized, account *config.Account) error {
	if account != nil && account.IsOAuth() && account.IsTokenExpired() {
		return fmt.Errorf("Your session has expired. Please run 'dotenv login' to authenticate again")
	}

	if err.Message != "" {
		return fmt.Errorf("Authentication failed: %s", err.Message)
	}

	return fmt.Errorf("Authentication failed. Please check your credentials or run 'dotenv login'")
}

func handleForbiddenError(err *dotenv.ErrForbidden) error {
	switch err.Resource {
	case "organization":
		return fmt.Errorf("Access denied to organization '%s'. Contact your organization admin for access", err.ID)
	case "project":
		return fmt.Errorf("Access denied to project '%s'. You may need additional permissions", err.ID)
	default:
		if err.ID != "" {
			return fmt.Errorf("Access denied to %s '%s'", err.Resource, err.ID)
		}
		return fmt.Errorf("Access denied to %s", err.Resource)
	}
}

// ShowErrorWithHelp displays an error with helpful suggestions
func ShowErrorWithHelp(err error) {
	if err == nil {
		return
	}

	ui.PrintError("%v", err)

	// Add contextual help based on error message
	switch {
	case contains(err.Error(), "No organization selected"):
		ui.PrintInfo("\nTo fix this:")
		ui.PrintInfo("1. Run 'dotenv org list' to see available organizations")
		ui.PrintInfo("2. Run 'dotenv org use <organization>' to select one")

	case contains(err.Error(), "not found"):
		ui.PrintInfo("\nTo troubleshoot:")
		ui.PrintInfo("1. Check your spelling and try again")
		ui.PrintInfo("2. Run 'dotenv list' commands to see available resources")
		ui.PrintInfo("3. Ensure you have the correct organization selected")

	case contains(err.Error(), "Access denied"):
		ui.PrintInfo("\nTo resolve:")
		ui.PrintInfo("1. Contact your organization administrator for access")
		ui.PrintInfo("2. Check that you're using the correct organization")
		ui.PrintInfo("3. Verify your account has the necessary permissions")

	case contains(err.Error(), "session has expired"):
		ui.PrintInfo("\nYour authentication has expired. Please run:")
		ui.PrintInfo("  dotenv login")

	case contains(err.Error(), "client-managed encryption"):
		ui.PrintInfo("\nThis project uses client-managed encryption keys.")
		ui.PrintInfo("To provide your key:")
		ui.PrintInfo("1. Use the --client-key flag: dotenv pull <project> --client-key=path/to/key")
		ui.PrintInfo("2. Or wait to be prompted for the key")
		ui.PrintInfo("3. The key should be in base64, hex, or raw format")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			(len(s) > len(substr) && findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
