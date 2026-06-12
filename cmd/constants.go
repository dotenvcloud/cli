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

// Key-rotation history policies (mirror the API's history_policy values):
// what happens to backup versions still under the old key when a key rotates.
const (
	policyKeep      = "keep"
	policyReencrypt = "re_encrypt"
)

// Platform identifiers used by update / explore commands.
const platformWindows = "windows"
