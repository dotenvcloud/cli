# Configuration Management Package

This package provides comprehensive configuration management for the DotEnv CLI, including secure storage of API keys, multi-context support, and environment variable overrides.

## Features

- **Multi-Context Support**: Manage multiple organizations/environments
- **Secure Storage**: API keys are encrypted using AES-256-GCM
- **Environment Overrides**: All settings can be overridden via environment variables
- **Atomic Operations**: Safe file writing with backup support
- **Thread Safety**: Concurrent access protection

## Usage

### Basic Usage

```go
import "github.com/dotenv/cli/internal/config"

// Create a new manager
mgr, err := config.NewManager("")
if err != nil {
    log.Fatal(err)
}

// Get current context
ctx, err := mgr.GetCurrentContext()
if err != nil {
    log.Fatal(err)
}

// Access API credentials
apiURL, _ := mgr.GetAPIURL()
apiKey, _ := mgr.GetAPIKey()
org, _ := mgr.GetOrganization()
```

### Context Management

```go
// Get context manager
cm := mgr.GetContextManager()

// Create a new context
err = cm.Create("production", "https://api.dotenv.cloud", "your-api-key", "my-org")

// Switch contexts
err = cm.Use("production")

// List all contexts
contexts := cm.List()
for _, ctx := range contexts {
    fmt.Println(ctx)
}

// Rename a context
err = cm.Rename("old-name", "new-name")

// Delete a context
err = cm.Delete("staging")
```

### Environment Variables

The following environment variables are supported:

- `DOTENV_CONFIG_DIR`: Custom configuration directory
- `DOTENV_API_KEY`: Override API key
- `DOTENV_API_URL`: Override API URL
- `DOTENV_ORGANIZATION`: Override organization
- `DOTENV_CONTEXT`: Override current context
- `DOTENV_DEBUG`: Enable debug mode
- `DOTENV_TLS_SKIP_VERIFY`: Skip TLS verification

### Configuration File

The configuration is stored in YAML format at `~/.dotenv/config.yaml`:

```yaml
version: "1.0"
telemetry_enabled: false
current_context: production
contexts:
  production:
    name: production
    api_url: https://api.dotenv.cloud
    api_key: <encrypted>
    organization: my-org
    created_at: 2025-06-18T10:00:00Z
    updated_at: 2025-06-18T10:00:00Z
    last_update: 2025-06-18T10:00:00Z
preferences:
  default_format: env
  color_output: true
  auto_update: true
```

## Security

- API keys are encrypted using AES-256-GCM with machine-specific keys
- Configuration files are created with 0600 permissions (owner read/write only)
- For production use, consider integrating with system keychains:
  - macOS: Keychain Services
  - Linux: Secret Service API
  - Windows: Credential Manager

## Testing

Run the test suite:

```bash
go test ./internal/config/...
```

Run integration tests:

```bash
INTEGRATION_TEST=1 go test ./internal/config/... -tags=integration
```