package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dotenv/cli/internal/config"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

// ErrClientManagedKey indicates that the project uses client-managed encryption
var ErrClientManagedKey = errors.New("project uses client-managed encryption")

// HandleAPIError provides user-friendly error messages with suggested actions
func HandleAPIError(err error, account *config.Account) error {
	if err == nil {
		return nil
	}

	// Check for client-managed key error — either CLI's sentinel or the SDK
	// typed sentinel exposed by F-19.
	if errors.Is(err, ErrClientManagedKey) || errors.Is(err, dotenv.ErrClientManagedEncryption) {
		return fmt.Errorf("this project uses client-managed encryption. " +
			"Please provide your encryption key using --client-key flag or you will be prompted for it")
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
	case resourceOrganization:
		return fmt.Errorf("organization '%s' not found. Run 'dotenv org list' to see available organizations", err.ID)
	case resourceProject:
		return fmt.Errorf("project '%s' not found. Run 'dotenv list projects' to see available projects", err.ID)
	case resourceTarget:
		return fmt.Errorf("target '%s' not found. Run 'dotenv list targets <project>' to see available targets", err.ID)
	case resourceEnvironment:
		return fmt.Errorf(
			"environment '%s' not found. Run 'dotenv list environments <project>/<target>' to see available environments",
			err.ID,
		)
	case "secret":
		return fmt.Errorf("secret '%s' not found", err.ID)
	default:
		if err.ID != "" {
			return fmt.Errorf("%s '%s' not found", err.Resource, err.ID)
		}
		return fmt.Errorf("%s not found", err.Resource)
	}
}

func handleUnauthorizedError(err *dotenv.ErrUnauthorized, account *config.Account) error {
	if account != nil && account.IsOAuth() && account.IsTokenExpired() {
		return fmt.Errorf("your session has expired. Please run 'dotenv login' to authenticate again")
	}

	if err.Message != "" {
		return fmt.Errorf("authentication failed: %s", err.Message)
	}

	return fmt.Errorf("authentication failed. Please check your credentials or run 'dotenv login'")
}

func handleForbiddenError(err *dotenv.ErrForbidden) error {
	switch err.Resource {
	case resourceOrganization:
		return fmt.Errorf("access denied to organization '%s'. Contact your organization admin for access", err.ID)
	case resourceProject:
		return fmt.Errorf("access denied to project '%s'. You may need additional permissions", err.ID)
	default:
		if err.ID != "" {
			return fmt.Errorf("access denied to %s '%s'", err.Resource, err.ID)
		}
		return fmt.Errorf("access denied to %s", err.Resource)
	}
}

// ShowErrorWithHelp displays an error with helpful suggestions
func ShowErrorWithHelp(err error) {
	if err == nil {
		return
	}

	ui.PrintErrorf("%v", err)

	// Add contextual help based on error message
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no organization selected"):
		ui.PrintInfof("\nTo fix this:")
		ui.PrintInfof("1. Run 'dotenv org list' to see available organizations")
		ui.PrintInfof("2. Run 'dotenv org use <organization>' to select one")

	case strings.Contains(msg, "not found"):
		ui.PrintInfof("\nTo troubleshoot:")
		ui.PrintInfof("1. Check your spelling and try again")
		ui.PrintInfof("2. Run 'dotenv list' commands to see available resources")
		ui.PrintInfof("3. Ensure you have the correct organization selected")

	case strings.Contains(msg, "access denied"):
		ui.PrintInfof("\nTo resolve:")
		ui.PrintInfof("1. Contact your organization administrator for access")
		ui.PrintInfof("2. Check that you're using the correct organization")
		ui.PrintInfof("3. Verify your account has the necessary permissions")

	case strings.Contains(msg, "session has expired"):
		ui.PrintInfof("\nYour authentication has expired. Please run:")
		ui.PrintInfof("  dotenv login")

	case strings.Contains(msg, "client-managed encryption"):
		ui.PrintInfof("\nThis project uses client-managed encryption keys.")
		ui.PrintInfof("To provide your key:")
		ui.PrintInfof("1. Use the --client-key flag: dotenv pull <project> --client-key=path/to/key")
		ui.PrintInfof("2. Or wait to be prompted for the key")
		ui.PrintInfof("3. The key should be in base64, hex, or raw format")
	}
}
