# Configuration Guide

This guide covers all aspects of configuring the DotEnv CLI for your specific needs.

## Table of Contents

- [Configuration Overview](#configuration-overview)
- [Config File Structure](#config-file-structure)
- [Managing Contexts](#managing-contexts)
- [Security Settings](#security-settings)
- [User Preferences](#user-preferences)
- [Environment Variables](#environment-variables)
- [Advanced Configuration](#advanced-configuration)

## Configuration Overview

DotEnv CLI uses a layered configuration system:

1. **Config File** (`~/.dotenv/config.yaml`) - Primary configuration
2. **Environment Variables** - Override config file settings
3. **Command Flags** - Override everything for single commands

### Configuration Location

- **Default**: `~/.dotenv/config.yaml`
- **Custom**: Set with `--config` flag or `DOTENV_CONFIG_FILE` env var
- **Permissions**: 0600 (read/write by owner only)

## Config File Structure

### Complete Example

```yaml
# Configuration version
version: "1.0"

# Telemetry settings
telemetry_enabled: true
analytics_id: 550e8400-e29b-41d4-a716-446655440000

# Current active context
current_context: production

# Contexts (one per organization)
contexts:
  production:
    name: production
    api_url: https://api.dotenv.com
    api_key: <encrypted>
    organization: acme-corp
    created_at: 2024-01-01T00:00:00Z
    updated_at: 2024-01-15T10:30:00Z
    metadata:
      user_email: john@acme.com
      organization_id: org_abc123
      permissions:
        - secrets:read
        - secrets:write
        - projects:manage
        - teams:read
  
  staging:
    name: staging
    api_url: https://api.dotenv.com
    api_key: <encrypted>
    organization: acme-staging
    created_at: 2024-01-01T00:00:00Z
    updated_at: 2024-01-15T10:30:00Z

# User preferences
preferences:
  # Display settings
  default_format: env
  color_output: true
  timestamp_format: "2006-01-02 15:04:05"
  
  # Update settings
  auto_update: true
  update_channel: stable
  update_check_interval: 24
  
  # Command defaults
  default_pull_options:
    resolve_variables: false
    output_format: env
    decrypt: true
    include_comments: false
  
  default_push_options:
    encrypt: true
    force: false
    validate: true
  
  # Performance
  cache:
    enabled: true
    ttl: 300
    max_size: 100

# Last update check
last_update_check: 2024-01-15T00:00:00Z

# Cache settings
cache:
  directory: ~/.dotenv/cache
  max_age: 3600
```

## Managing Contexts

### What is a Context?

A context represents authenticated access to a specific organization. Think of it as a "profile" for accessing different organizations.

### Creating Contexts

Contexts are created automatically when you:

```bash
# Initial setup
dotenv init
dotenv login

# Adding additional organizations
dotenv login --add-context
```

### Switching Contexts

```bash
# View current context
dotenv config get current_context

# List all contexts
dotenv list contexts

# Output:
# NAME         ORGANIZATION    API URL                   CURRENT
# production   acme-corp       https://api.dotenv.com    *
# staging      acme-staging    https://api.dotenv.com
# personal     john-doe        https://api.dotenv.com

# Switch context
dotenv use-context staging
```

### Managing Multiple Organizations

Best practices for multiple contexts:

```yaml
contexts:
  # Work - Production
  work-prod:
    organization: acme-corp
    api_key: <encrypted>
  
  # Work - Development
  work-dev:
    organization: acme-corp-dev
    api_key: <encrypted>
  
  # Personal projects
  personal:
    organization: john-doe
    api_key: <encrypted>
  
  # Client project
  client-abc:
    organization: client-abc
    api_key: <encrypted>
```

Quick switching with aliases:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias work='dotenv use-context work-prod'
alias personal='dotenv use-context personal'
alias client='dotenv use-context client-abc'
```

## Security Settings

### API Key Encryption

API keys are encrypted using AES-256-GCM with a machine-specific key:

```yaml
# Never edit encrypted keys manually!
api_key: "gAAAAABh...encrypted...data...=="
```

The encryption key is derived from:
- User home directory path
- System username  
- Machine ID
- Static salt

### Security Best Practices

1. **File Permissions**
   ```bash
   # Check permissions
   ls -la ~/.dotenv/config.yaml
   # Should show: -rw------- (0600)
   
   # Fix if needed
   chmod 600 ~/.dotenv/config.yaml
   ```

2. **No Version Control**
   ```gitignore
   # .gitignore
   ~/.dotenv/
   .dotenv/
   ```

3. **Regular Key Rotation**
   ```bash
   # Refresh credentials
   dotenv refresh
   
   # Rotate for specific context
   dotenv refresh --context=production
   ```

### Secure Development Mode

For development with self-signed certificates:

```yaml
# Only for development!
contexts:
  dev-local:
    api_url: https://localhost:8443
    insecure_skip_verify: true  # Skip TLS verification
```

Or via environment variable:
```bash
export DOTENV_TLS_SKIP_VERIFY=true
```

## User Preferences

### Display Preferences

```yaml
preferences:
  # Output format (env, json, yaml, shell, dockerfile)
  default_format: env
  
  # Enable/disable colors
  color_output: true
  
  # Timestamp format (Go time format)
  timestamp_format: "2006-01-02 15:04:05"
  
  # Table output settings
  table_style: simple  # simple, grid, markdown
  
  # Pager for long output
  use_pager: true
  pager_command: less -R
```

### Update Preferences

```yaml
preferences:
  # Auto-update checking
  auto_update: true
  
  # Update channel (stable, beta, nightly)
  update_channel: stable
  
  # Check interval in hours
  update_check_interval: 24
  
  # Auto-install updates
  auto_install_updates: false
```

### Command Defaults

Set default options for commands:

```yaml
preferences:
  # Pull command defaults
  default_pull_options:
    resolve_variables: true
    output_format: env
    decrypt: true
    include_comments: true
    create_backup: true
  
  # Push command defaults  
  default_push_options:
    encrypt: true
    force: false
    validate: true
    create_backup: true
  
  # Export command defaults
  default_export_options:
    format: json
    pretty_print: true
```

## Environment Variables

Environment variables override config file settings:

### Authentication

```bash
# Override API key
export DOTENV_API_KEY="dotenv_xxx_yyy"

# Override organization
export DOTENV_ORGANIZATION="different-org"

# Override context
export DOTENV_CONTEXT="staging"
```

### API Settings

```bash
# API endpoint
export DOTENV_API_URL="https://api.staging.dotenv.com"

# Request timeout (seconds)
export DOTENV_TIMEOUT=30

# Retry attempts
export DOTENV_RETRY_ATTEMPTS=3
```

### Display Settings

```bash
# Disable colors
export DOTENV_NO_COLOR=true

# Enable debug output
export DOTENV_DEBUG=true

# Suppress all output
export DOTENV_QUIET=true

# Set log level (debug, info, warn, error)
export DOTENV_LOG_LEVEL=debug
```

### Security Settings

```bash
# Skip TLS verification (development only!)
export DOTENV_TLS_SKIP_VERIFY=true

# Custom CA certificate
export DOTENV_CA_CERT=/path/to/ca.crt

# Client certificate authentication
export DOTENV_CLIENT_CERT=/path/to/client.crt
export DOTENV_CLIENT_KEY=/path/to/client.key
```

## Advanced Configuration

### Custom Config Location

```bash
# Via environment variable
export DOTENV_CONFIG_FILE=/custom/path/config.yaml

# Via command flag
dotenv --config=/custom/path/config.yaml pull myproject
```

### Per-Project Configuration

Create `.dotenv.yaml` in your project:

```yaml
# .dotenv.yaml (project root)
project: web-app
target: production
environment: api
auto_pull: true
pull_on_start: true
```

Then commands use project settings:
```bash
cd /path/to/project
dotenv pull  # Automatically uses web-app/production/api
```

### Config Management Commands

```bash
# View all configuration
dotenv config list

# Get specific value
dotenv config get current_context
dotenv config get contexts.production.organization

# Set configuration value
dotenv config set telemetry_enabled false
dotenv config set preferences.default_format json

# Edit in your editor
dotenv config edit

# Validate configuration
dotenv config validate

# Show configuration path
dotenv config path
```

### Cache Configuration

```yaml
cache:
  # Enable/disable caching
  enabled: true
  
  # Cache directory
  directory: ~/.dotenv/cache
  
  # TTL in seconds
  ttl: 300
  
  # Maximum cache size in MB
  max_size: 100
  
  # Cache specific operations
  cache_operations:
    - list_projects
    - list_targets
    - list_environments
```

Clear cache:
```bash
dotenv cache clear
```

### Proxy Configuration

For corporate environments:

```yaml
contexts:
  corporate:
    api_url: https://api.dotenv.com
    proxy:
      http_proxy: http://proxy.corp.com:8080
      https_proxy: http://proxy.corp.com:8080
      no_proxy: localhost,127.0.0.1,.corp.com
```

Or via environment:
```bash
export HTTP_PROXY=http://proxy.corp.com:8080
export HTTPS_PROXY=http://proxy.corp.com:8080
export NO_PROXY=localhost,127.0.0.1
```

### Plugin Configuration

Enable experimental features:

```yaml
plugins:
  enabled: true
  directory: ~/.dotenv/plugins
  auto_load: true
  
experimental_features:
  - secret_scanning
  - auto_rotation
  - compliance_mode
```

## Troubleshooting Configuration

### Reset Configuration

```bash
# Backup current config
cp ~/.dotenv/config.yaml ~/.dotenv/config.yaml.backup

# Reset to defaults
rm ~/.dotenv/config.yaml
dotenv init
```

### Validate Configuration

```bash
# Check syntax
dotenv config validate

# Test API connection
dotenv config test

# Debug configuration loading
DOTENV_DEBUG=true dotenv config list
```

### Common Issues

**Corrupted Config File**
```bash
# Check YAML syntax
python -c "import yaml; yaml.safe_load(open('$HOME/.dotenv/config.yaml'))"

# Restore from backup
cp ~/.dotenv/config.yaml.backup ~/.dotenv/config.yaml
```

**Lost Credentials**
```bash
# Re-authenticate
dotenv login
```

**Permission Issues**
```bash
# Fix permissions
chmod 600 ~/.dotenv/config.yaml
chmod 700 ~/.dotenv
```

## Migration and Backup

### Export Configuration

```bash
# Export (without sensitive data)
dotenv config export > config-backup.yaml

# Export with encrypted credentials
dotenv config export --include-credentials > config-full.yaml
```

### Import Configuration

```bash
# Import configuration
dotenv config import config-backup.yaml

# Merge with existing
dotenv config import --merge config-backup.yaml
```

### Sync Between Machines

Since API keys are machine-specific, you need to re-authenticate on each machine:

```bash
# Machine 1: Export without credentials
dotenv config export > dotenv-config.yaml

# Machine 2: Import and re-authenticate
dotenv config import dotenv-config.yaml
dotenv login
```

## Best Practices

1. **Use contexts** for different organizations/environments
2. **Set preferences** to match your workflow
3. **Regular backups** of configuration
4. **Secure permissions** on config files
5. **Rotate credentials** periodically
6. **Use environment variables** for temporary overrides
7. **Document** custom configurations for your team

## Next Steps

- Learn about [Security Best Practices](security.md)
- Set up [CI/CD Integration](ci-cd-integration.md)
- Explore [Advanced Usage](../examples/advanced-usage.md)