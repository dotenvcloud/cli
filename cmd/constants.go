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

// Platform identifiers used by update / explore commands.
const platformWindows = "windows"
