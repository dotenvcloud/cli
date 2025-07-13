package constants

// Authentication types
const (
	// AuthTypeOAuth represents OAuth authentication
	AuthTypeOAuth = "oauth"
	
	// AuthTypeAPIKey represents API key authentication
	AuthTypeAPIKey = "api_key"
)

// Token types
const (
	// TokenTypeBearer is the bearer token type used in OAuth
	TokenTypeBearer = "Bearer"
)

// OAuth constants
const (
	// OAuthClientID is the OAuth2 client ID for the CLI
	OAuthClientID = "dotenv-cli"
	
	// DefaultOAuthPort is the default port for OAuth callback
	DefaultOAuthPort = "8899"
	
	// OAuthTimeout is the timeout for OAuth authentication flow
	OAuthTimeoutMinutes = 5
)

// Organization constants
const (
	// SystemOrganizationULID is the ULID for the system organization
	SystemOrganizationULID = "system-organization-000000"
	
	// OrganizationRefreshHours is how often to refresh organization data
	OrganizationRefreshHours = 24
)

// Configuration file constants
const (
	// ConfigVersion is the current configuration version
	ConfigVersion = "1.0"
	
	// ConfigFileName is the name of the configuration file
	ConfigFileName = "config.yaml"
	
	// ConfigDirName is the name of the configuration directory
	ConfigDirName = ".dotenv"
)

// Error messages
const (
	// ErrNoCurrentAccount is returned when no account is selected
	ErrNoCurrentAccount = "no current account selected"
	
	// ErrNoOrganization is returned when no organization is selected
	ErrNoOrganization = "no organization selected"
	
	// ErrTokenExpired is returned when the OAuth token is expired
	ErrTokenExpired = "authentication token expired"
	
	// ErrInvalidAPIKey is returned when the API key format is invalid
	ErrInvalidAPIKey = "invalid API key format"
)