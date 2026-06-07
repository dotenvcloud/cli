package config

import "os"

// AuthSource identifies where the active credentials for a CLI invocation came
// from.
type AuthSource string

const (
	// AuthSourceEnv means credentials came from the environment / --api-key
	// flag (CI/CD). The local account store is not consulted.
	AuthSourceEnv AuthSource = "env"
	// AuthSourceAccount means credentials came from the stored account file.
	AuthSourceAccount AuthSource = "account"
	// AuthSourceNone means no credentials are available; the user must log in.
	AuthSourceNone AuthSource = "none"
)

// ActiveAuth is the single resolved answer to "who am I and which organization"
// for one CLI invocation. It is the one source of truth consulted by client
// construction, organization refresh, and the identity banner — so those never
// disagree (e.g. refreshing a stale stored account while an API key is set).
type ActiveAuth struct {
	Source AuthSource

	// APIKey is set when Source == AuthSourceEnv.
	APIKey string
	// Organization is the ULID to scope requests to (may be empty).
	Organization string
	// APIURL is the resolved base URL.
	APIURL string
	// Label is a human-friendly identifier for messaging (account name, or
	// "environment credentials").
	Label string
	// UsesAccountStore is true only when the stored account file is the active
	// credential source. Organization refresh / token refresh apply only then.
	UsesAccountStore bool
	// Account is non-nil only when Source == AuthSourceAccount.
	Account *Account
}

// UsingEnvCredential reports whether an explicit API key is active (via the
// --api-key flag value passed in, or the DOTENV_API_KEY env var). This is the
// single predicate for "are we in environment-credential (CI/CD) mode", used by
// the resolver, organization refresh, and identity reporting so they agree.
func UsingEnvCredential(flagAPIKey string) bool {
	return flagAPIKey != "" || os.Getenv(EnvAPIKey) != ""
}

// ResolveAuth decides the active identity with this precedence (per product
// spec):
//  1. An explicit API key (flag or DOTENV_API_KEY) → ephemeral environment
//     identity, announced; the account store is bypassed (CI/CD friendly).
//  2. Otherwise the current/default stored account.
//  3. Otherwise none — the caller should prompt the user to log in.
//
// flagAPIKey is the value of the --api-key flag (empty if unset). env is the
// loaded environment overrides. account is the current stored account (nil if
// none / not loaded).
func ResolveAuth(flagAPIKey string, env *EnvConfig, account *Account) ActiveAuth {
	apiKey := flagAPIKey
	if apiKey == "" {
		apiKey = env.APIKey
	}

	if apiKey != "" {
		apiURL := env.APIURL
		if apiURL == "" {
			apiURL = GetAPIURL("")
		}
		return ActiveAuth{
			Source:           AuthSourceEnv,
			APIKey:           apiKey,
			Organization:     env.Organization,
			APIURL:           apiURL,
			Label:            "environment credentials",
			UsesAccountStore: false,
		}
	}

	if account != nil {
		return ActiveAuth{
			Source:           AuthSourceAccount,
			Organization:     account.GetCurrentOrganizationULID(),
			APIURL:           account.APIURL,
			Label:            account.Name,
			UsesAccountStore: true,
			Account:          account,
		}
	}

	return ActiveAuth{Source: AuthSourceNone, UsesAccountStore: true}
}
