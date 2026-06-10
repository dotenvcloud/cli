package cmd

// Output format constants shared across list / org / pull / push commands.
const (
	formatJSON = "json"
	formatYAML = "yaml"
	formatENV  = "env"
)

// Resource type constants used in error handling and CLI argument switches.
const (
	resourceOrganization = "organization"
	resourceProject      = "project"
	resourceTarget       = "target"
	resourceEnvironment  = "environment"
	resourceTargets      = "targets"
	resourceEnvironments = "environments"
	resourceAccounts     = "accounts"
	resourceAll          = "all"
)

// Key custody modes: who holds the encryption key. Both modes encrypt with the
// same unified PBKDF2 data key; the difference is only where the key lives.
const (
	managedServer = "server"
	managedClient = "client"
)

// Platform identifiers used by update / explore commands.
const platformWindows = "windows"
