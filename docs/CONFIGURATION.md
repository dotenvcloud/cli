# Configuration Guide

This guide covers DotEnv CLI configuration in detail.

## Configuration File

The CLI stores configuration in `~/.dotenv/config.yaml`.

### File Structure

```yaml
# Configuration version
version: "1.0"

# Telemetry settings
telemetry_enabled: true

# Current active context
current_context: production

# Contexts (organizations)
contexts:
  production:
    name: production
    api_url: https://api.dotenv.com
    api_key: <encrypted>
    organization: acme-corp
    created_at: 2024-01-01T00:00:00Z
    updated_at: 2024-01-01T00:00:00Z
    metadata:
      user_email: user@example.com
      organization_id: org_123
      permissions:
        - secrets:read
        - secrets:write
        - projects:read
  
  staging:
    name: staging
    api_url: https://api.dotenv.com
    api_key: <encrypted>
    organization: acme-corp-staging
    created_at: 2024-01-01T00:00:00Z
    updated_at: 2024-01-01T00:00:00Z

# User preferences
preferences:
  default_format: env
  color_output: true
  auto_update: true
  update_channel: stable
  default_pull_options:
    resolve_variables: false
    output_format: env

# Last update check
last_update_check: 2024-01-01T00:00:00Z
```

## Managing Contexts

### What is a Context?

A context represents access to a specific organization. Each context contains:
- API endpoint URL
- Encrypted API key
- Organization identifier
- Metadata about permissions

### Creating Contexts

Contexts are created automatically when you:

1. Run `dotenv init` and authenticate
2. Run `dotenv login` and select organizations
3. Manually edit the config file

### Switching Contexts

```bash
# View current context
dotenv config get current_context

# List all contexts
dotenv list contexts

# Switch context
dotenv use-context staging
```

### Context Naming

Best practices for context names:
- Use descriptive names: `production`, `staging`, `dev`
- Include organization: `acme-prod`, `acme-dev`
- Include region: `us-east-1`, `eu-west-1`

## Security

### API Key Storage

API keys are encrypted using AES-256-GCM with a machine-specific key derived from:
- User home directory
- Username
- Machine ID
- Static salt

**Important**: Keys are tied to your machine. Config files cannot be shared between machines.

### Secure Practices

1. **File Permissions**: Config file is created with 0600 (owner read/write only)
2. **No Plain Text**: API keys are never stored in plain text
3. **No Sharing**: Don't commit config files to version control
4. **Regular Rotation**: Refresh credentials periodically

## Environment Variables

Environment variables override config file settings:

```bash
# Override API key
export DOTENV_API_KEY="dotenv_xxx_yyy"

# Override organization
export DOTENV_ORGANIZATION="different-org"

# Override API URL
export DOTENV_API_URL="https://api.staging.dotenv.com"

# Skip TLS verification
export DOTENV_TLS_SKIP_VERIFY=true
```

### Precedence Order

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

## Advanced Configuration

### Custom Config Location

```bash
# Use custom config file
dotenv --config=/path/to/config.yaml pull myproject

# Set via environment
export DOTENV_CONFIG_FILE=/path/to/config.yaml
```

### Multiple Organizations

Managing multiple organizations effectively:

```yaml
contexts:
  # Personal projects
  personal:
    organization: john-doe
    api_key: <encrypted>
  
  # Work projects - production
  work-prod:
    organization: acme-corp
    api_key: <encrypted>
  
  # Work projects - staging  
  work-stage:
    organization: acme-corp-staging
    api_key: <encrypted>
```

### Per-Project Configuration

Create `.dotenv` file in project root:

```yaml
# .dotenv (project-specific)
project: myapp
target: production
environment: api
```

Then pull without specifying:
```bash
cd /path/to/project
dotenv pull  # Uses settings from .dotenv
```

## Telemetry Configuration

### Opting In/Out

During `dotenv init`:
```
Enable telemetry? [y/N]
```

Or manually:
```bash
# Enable
dotenv config set telemetry_enabled true

# Disable
dotenv config set telemetry_enabled false
```

### What's Collected

When enabled, we collect:
- Command usage (which commands, how often)
- Performance metrics (command duration)
- Error types (not error messages)
- CLI version and OS

Never collected:
- Secret values
- File contents
- Project/organization names
- Personal information

### Anonymous ID

A random UUID is generated for telemetry:
```yaml
preferences:
  analytics_id: 550e8400-e29b-41d4-a716-446655440000
```

## Preferences

### Display Preferences

```yaml
preferences:
  # Default output format for pull command
  default_format: env  # env, json, yaml
  
  # Enable colored output
  color_output: true
  
  # Timestamp format
  timestamp_format: "2006-01-02 15:04:05"
```

### Update Preferences

```yaml
preferences:
  # Automatically check for updates
  auto_update: true
  
  # Update channel
  update_channel: stable  # stable, beta, nightly
  
  # Check frequency (hours)
  update_check_interval: 24
```

### Default Command Options

```yaml
preferences:
  default_pull_options:
    resolve_variables: true
    output_format: env
    decrypt: true
  
  default_push_options:
    encrypt: true
    force: false
```

## Troubleshooting Configuration

### Reset Configuration

```bash
# Backup current config
cp ~/.dotenv/config.yaml ~/.dotenv/config.yaml.backup

# Reinitialize
dotenv init --force
```

### Validate Configuration

```bash
# Check config syntax
dotenv config validate

# Test API connection
dotenv config test
```

### Common Issues

**"No current context"**
```bash
dotenv login
# or
dotenv use-context <name>
```

**"Invalid API key"**
```bash
dotenv refresh
```

**"Config file corrupted"**
```bash
# Remove and reinitialize
rm ~/.dotenv/config.yaml
dotenv init
```

## Config Command Reference

```bash
# Get configuration value
dotenv config get <key>

# Set configuration value
dotenv config set <key> <value>

# List all configuration
dotenv config list

# Validate configuration
dotenv config validate

# Edit configuration in editor
dotenv config edit
```

### Examples

```bash
# Check telemetry status
dotenv config get telemetry_enabled

# Change default format
dotenv config set preferences.default_format json

# View current context details
dotenv config get contexts.$(dotenv config get current_context)
```

## Migration from Other Tools

### From .env Files

No migration needed - DotEnv CLI works with standard .env files:

```bash
# Push existing .env
dotenv push myproject .env
```

### From Other Secret Managers

Export to .env format first:

```bash
# Example: From HashiCorp Vault
vault kv get -format=json secret/myapp | jq -r '.data.data | to_entries[] | "\(.key)=\(.value)"' > .env
dotenv push myproject .env
```

## Best Practices

1. **One Context Per Environment**: Keep production and staging separate
2. **Regular Refresh**: Run `dotenv refresh` periodically
3. **Secure Storage**: Ensure ~/.dotenv has proper permissions
4. **No Config in Repos**: Add to .gitignore: `~/.dotenv/`
5. **Environment Isolation**: Use different contexts for different environments

## Next Steps

- Learn about [Encryption](encryption.md)
- Set up [CI/CD Integration](ci-cd-integration.md)
- Explore [Advanced Usage](../examples/advanced-usage.md)