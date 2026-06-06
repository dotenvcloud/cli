package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/dotenvcloud/cli/internal/auth/oauth"
	"github.com/dotenvcloud/cli/internal/config"
)

// BrowserLoginOptions contains options for browser-based login
type BrowserLoginOptions struct {
	APIUrl        string
	CallbackPort  string
	NoBrowser     bool
	IsInteractive bool // Set to false in CI environments
}

// DoBrowserLogin performs OAuth2 browser-based authentication flow
func DoBrowserLogin(ctx context.Context, am *config.AccountManager, opts BrowserLoginOptions) error {
	// In CI environment, skip browser login
	if os.Getenv("CI") != "" || !opts.IsInteractive {
		return fmt.Errorf("browser login not available in non-interactive environment")
	}

	// Use the new OAuth flow
	authFlow := oauth.AuthFlow{
		BaseURL:       opts.APIUrl,
		CallbackPort:  opts.CallbackPort,
		NoBrowser:     opts.NoBrowser,
		IsInteractive: opts.IsInteractive,
	}

	return authFlow.Run(ctx, am)
}
