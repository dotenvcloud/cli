# Command Reference

Complete reference for all DotEnv CLI commands.

## Global Flags

These flags are available for all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--help` | `-h` | Show help for command |
| `--debug` | | Enable debug output |
| `--quiet` | | Suppress non-error output |
| `--no-color` | | Disable colored output |
| `--config` | | Specify config file location |

## Commands

### dotenv init

Initialize DotEnv CLI configuration.

```bash
dotenv init [flags]
```

**Flags:**
- `--force` - Overwrite existing configuration
- `--non-interactive` - Use defaults without prompting

**Interactive Prompts:**
1. API URL (default: https://api.dotenv.com)
2. Authentication method (browser/manual)
3. Organization selection
4. Telemetry opt-in

**Example:**
```bash
# First time setup
dotenv init

# Reinitialize
dotenv init --force

# Non-interactive with defaults
dotenv init --non-interactive
```

### dotenv login

Authenticate with DotEnv via browser.

```bash
dotenv login [flags]
```

**Flags:**
- `--no-browser` - Print URL instead of opening browser
- `--callback-port` - Specify callback port (default: random)
- `--token` - Use API token directly (non-interactive)

**Process:**
1. Starts local callback server
2. Opens browser to authorization page
3. User selects organizations
4. Receives API tokens
5. Stores encrypted credentials

**Example:**
```bash
# Normal login
dotenv login

# Manual login (for SSH sessions)
dotenv login --no-browser

# Token-based login
dotenv login --token=dotenv_xxx_yyy
```

### dotenv pull

Pull secrets from DotEnv.

```bash
dotenv pull [project]/[target]/[environment] [flags]
```

**Arguments:**
- `project` - Project slug (required)
- `target` - Target slug (optional)
- `environment` - Environment slug (optional)

**Flags:**
- `--output, -o` - Output to file instead of stdout
- `--format, -f` - Output format (env, json, yaml, shell, dockerfile)
- `--resolve, -r` - Resolve variable interpolation
- `--decrypt` - Decrypt values (default: true)
- `--include-comments` - Include comments in output
- `--overwrite` - Overwrite existing file without prompt

**Hierarchy:**
- Project secrets are base defaults
- Target secrets override project
- Environment secrets override target

**Examples:**
```bash
# Pull project secrets to stdout
dotenv pull myproject

# Pull to file
dotenv pull myproject --output=.env

# Pull specific environment
dotenv pull myproject/production/api

# Export as JSON
dotenv pull myproject --format=json

# Resolve variables
dotenv pull myproject --resolve

# Pull without decryption
dotenv pull myproject --decrypt=false
```

### dotenv push

Push secrets to DotEnv.

```bash
dotenv push [project]/[target]/[environment] [file] [flags]
```

**Arguments:**
- Path - Project/target/environment path
- File - File to push (for single file mode)

**Flags:**
- `--project` - Project-level secrets file
- `--target` - Target-level secrets file  
- `--env` - Environment-level secrets file
- `--force, -f` - Overwrite without confirmation
- `--encrypt` - Encrypt before upload (default: true)
- `--dry-run` - Show what would be pushed without pushing
- `--validate` - Validate format before pushing

**Modes:**
1. **Single file**: Specify path and file
2. **Multiple files**: Specify project and use flags

**Examples:**
```bash
# Push single file
dotenv push myproject .env
dotenv push myproject/staging .env.staging

# Push multiple files with hierarchy
dotenv push myproject \
  --project=.env.defaults \
  --target=.env.production \
  --env=.env.production.local

# Force overwrite
dotenv push myproject .env --force

# Dry run
dotenv push myproject .env --dry-run
```

### dotenv list

List DotEnv resources.

```bash
dotenv list [resource] [parent] [flags]
```

**Resources:**
- `contexts` - Local CLI contexts
- `organizations` - Your organizations
- `projects` - Projects in organization
- `targets` - Targets in project
- `environments` - Environments in target

**Flags:**
- `--organization` - Override current organization
- `--format, -f` - Output format (table, json, yaml)
- `--filter` - Filter results by name pattern

**Examples:**
```bash
# List contexts
dotenv list contexts

# List projects
dotenv list projects

# List targets
dotenv list targets myproject

# List environments
dotenv list environments myproject/production

# JSON output
dotenv list projects --format=json

# Filter results
dotenv list projects --filter="web-*"
```

### dotenv export

Export secrets in various formats.

```bash
dotenv export [project]/[target]/[environment] [flags]
```

**Flags:**
- `--format, -f` - Export format
- `--output, -o` - Output file
- `--template` - Custom template file

**Formats:**
- `env` - Standard .env format (default)
- `json` - JSON object
- `yaml` - YAML format
- `shell` - Shell export statements
- `dockerfile` - Dockerfile ENV instructions
- `kubernetes` - Kubernetes Secret YAML
- `systemd` - Systemd environment file

**Examples:**
```bash
# Export as shell script
dotenv export myproject --format=shell

# Export as Dockerfile
dotenv export myproject --format=dockerfile --output=Dockerfile.env

# Export as JSON
dotenv export myproject/production --format=json > secrets.json

# Export as Kubernetes Secret
dotenv export myproject --format=kubernetes | kubectl apply -f -
```

### dotenv refresh

Refresh API credentials.

```bash
dotenv refresh [flags]
```

**Flags:**
- `--context` - Refresh specific context
- `--all` - Refresh all contexts

**Use Cases:**
- Organization permissions changed
- API token expired
- Need to update access

**Example:**
```bash
# Refresh current context
dotenv refresh

# Refresh specific context
dotenv refresh --context=production

# Refresh all contexts
dotenv refresh --all
```

### dotenv update

Update CLI to latest version.

```bash
dotenv update [flags]
```

**Flags:**
- `--check` - Only check for updates
- `--version` - Install specific version
- `--channel` - Update channel (stable, beta, nightly)

**Process:**
1. Checks GitHub releases
2. Downloads appropriate binary
3. Replaces current installation
4. Verifies update

**Examples:**
```bash
# Check for updates
dotenv update --check

# Update to latest
dotenv update

# Install specific version
dotenv update --version=1.2.0

# Update to beta channel
dotenv update --channel=beta
```

### dotenv use-context

Switch between configured contexts.

```bash
dotenv use-context [context-name]
```

**Arguments:**
- `context-name` - Name of context to switch to

**Example:**
```bash
# Switch context
dotenv use-context production

# List contexts first
dotenv list contexts
dotenv use-context staging
```

### dotenv version

Show version information.

```bash
dotenv version [flags]
```

**Flags:**
- `--short, -s` - Show version number only
- `--json` - Output as JSON

**Output includes:**
- Version number
- Git commit hash
- Build date
- Go version

**Example:**
```bash
# Full version info
dotenv version

# Just version number
dotenv version --short

# JSON output
dotenv version --json
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Command syntax error |
| 3 | Authentication required |
| 4 | Resource not found |
| 5 | Permission denied |
| 6 | Network error |
| 7 | Decryption failed |

## Environment Variables

Override configuration with environment variables:

| Variable | Description |
|----------|-------------|
| `DOTENV_API_KEY` | API key override |
| `DOTENV_API_URL` | API URL override |
| `DOTENV_ORGANIZATION` | Organization override |
| `DOTENV_CONTEXT` | Context override |
| `DOTENV_DEBUG` | Enable debug mode |
| `DOTENV_NO_COLOR` | Disable colors |
| `DOTENV_QUIET` | Suppress output |
| `DOTENV_TLS_SKIP_VERIFY` | Skip TLS verification |

## Command Aliases

Common workflow aliases:

```bash
# Add to ~/.bashrc or ~/.zshrc

# Quick pull to .env
alias dep="dotenv pull"

# Pull specific environments
alias dep-prod="dotenv pull myproject/production --output=.env"
alias dep-stage="dotenv pull myproject/staging --output=.env"

# Push with confirmation
alias deps="dotenv push"

# List shortcuts
alias del="dotenv list"
```

## Scripting Examples

### Check if secrets exist

```bash
if dotenv pull myproject --quiet; then
  echo "Secrets retrieved successfully"
else
  echo "Failed to retrieve secrets"
  exit 1
fi
```

### Export for Docker build

```bash
#!/bin/bash
dotenv pull myproject --format=dockerfile > .env.docker
docker build --env-file .env.docker -t myapp .
rm .env.docker
```

### CI/CD Integration

```bash
# In CI pipeline
set -e
dotenv pull myproject/production --output=.env
npm test
npm run build
```

### Batch Operations

```bash
# Pull multiple projects
for project in frontend backend api; do
  dotenv pull $project --output=.env.$project
done

# Push to multiple environments
for env in staging production; do
  dotenv push myproject/$env .env.$env
done
```