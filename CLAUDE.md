# DotEnv CLI - Development Context

This is the Go CLI for DotEnv, providing fast and secure access to environment variables.

## Overview

The DotEnv CLI is a standalone binary written in Go that allows developers to:
- Pull secrets from DotEnv cloud
- Push local .env files to the cloud
- Manage projects, targets, and environments
- Handle encryption locally for maximum security

## Technology Stack

- **Language**: Go 1.21+
- **CLI Framework**: Cobra
- **Configuration**: Viper
- **HTTP Client**: Standard library with retry logic
- **Encryption**: crypto/aes (AES-256-GCM)

## Project Structure

```
apps/cli/
├── cmd/                    # Command implementations
│   ├── root.go            # Main command setup
│   ├── init.go            # Initialize configuration
│   ├── login.go           # Authentication
│   ├── pull.go            # Pull secrets
│   ├── push.go            # Push secrets
│   ├── list.go            # List resources
│   ├── export.go          # Export in various formats
│   └── config.go          # Configuration management
├── internal/              # Internal packages
│   ├── api/              # API client
│   │   ├── client.go     # HTTP client wrapper
│   │   ├── auth.go       # Authentication logic
│   │   └── errors.go     # Error handling
│   ├── crypto/           # Encryption/decryption
│   │   ├── encryptor.go  # AES-256-GCM implementation
│   │   └── keys.go       # Key management
│   ├── config/           # Configuration management
│   │   ├── config.go     # Config structure
│   │   └── loader.go     # Config file handling
│   └── formats/          # Format handlers
│       ├── env.go        # .env format
│       ├── json.go       # JSON format
│       └── yaml.go       # YAML format
├── pkg/                   # Public packages
│   └── dotenv/           # Public API
├── main.go               # Entry point
├── go.mod                # Go modules
└── go.sum                # Dependency lock
```

## Core Commands

### init
```bash
dotenv init
```
- Creates ~/.dotenv/config.yml
- Sets up authentication
- Configures default organization

### login
```bash
dotenv login
dotenv login --api-key=xxx
```
- Interactive login flow
- Stores credentials securely
- Supports multiple profiles

### pull
```bash
dotenv pull [project]/[target]/[environment]
dotenv pull myapp/production/web
dotenv pull myapp --all
dotenv pull myapp/production/web --format=env > .env
dotenv pull myapp/production/web --client-key=./key.pem
```
- Pulls secrets from specified hierarchy
- Supports multiple output formats
- Handles client-side decryption

### push
```bash
dotenv push .env [project]/[target]/[environment]
dotenv push secrets.json myapp/staging/api --format=json
```
- Pushes local secrets to cloud
- Validates format before upload
- Supports encryption before push

### list
```bash
dotenv list projects
dotenv list targets myapp
dotenv list environments myapp/production
```
- Lists available resources
- Hierarchical navigation
- Filters and search

### export
```bash
dotenv export myapp/production/web --format=docker
dotenv export myapp/production/web --format=kubernetes
dotenv export myapp/production/web --format=systemd
```
- Exports for different platforms
- Platform-specific formats
- Integration helpers

## Configuration

### Config File Location
```yaml
# ~/.dotenv/config.yml
current_context: production
contexts:
  production:
    api_url: https://api.dotenv.cloud
    api_key: ${DOTENV_API_KEY}  # Can use env vars
    organization: acme-corp
    default_project: web-app
  staging:
    api_url: https://staging-api.dotenv.cloud
    api_key: stored-securely
    organization: acme-corp-staging
```

### Environment Variables
```bash
DOTENV_API_KEY=xxx         # API key
DOTENV_API_URL=xxx         # API base URL
DOTENV_CONTEXT=production  # Active context
```

Client-managed encryption keys are resolved per command (shared by `pull` and
`push` via `resolveEncryptionKey` in `cmd/clientkey.go`), in this order of
preference — safest first:

1. `--client-key=<file>` — a file path (recommended; no warning).
2. `--client-key=<value>` — the key value itself (warned: leaks via shell
   history / process list). A value that looks like a path but doesn't exist is
   an error, not a silent key, to catch typos.
3. `DOTENV_CLIENT_KEY=<value>` — env var holding the key value (warned: leaks
   via the process environment). Consulted only when a client key is actually
   needed (client-managed project), so a stray global value can't override a
   server-managed project.
4. Interactive prompt — when none of the above is provided on a client-managed
   project.

Keys are used as raw strings (never hex/base64-decoded) per the platform
crypto contract.

## Encryption Implementation

### Encryption Flow
1. Generate 32-byte key (if not provided)
2. Generate 12-byte nonce for GCM
3. Encrypt using AES-256-GCM
4. Concatenate: nonce + ciphertext + tag
5. Base64 encode for transport

### Key Management
- Support for key files
- Environment variable keys
- Interactive key input
- Key derivation from passwords

### Code Example
```go
func (e *Encryptor) Encrypt(plaintext []byte, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", err
    }
    
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

## API Integration

### Authentication
- Bearer token in Authorization header
- Organization context in headers
- Automatic token refresh

### Error Handling
- Exponential backoff for retries
- Meaningful error messages
- Status code mapping

### Rate Limiting
- Respect rate limit headers
- Automatic throttling
- Queue management

## Development Guidelines

### Code Style
- Follow standard Go conventions
- Use `gofmt` and `golint`
- Write idiomatic Go code
- Comprehensive error handling

### Testing
- Unit tests for all packages
- Integration tests for commands
- Mock API responses
- Test encryption/decryption

### Building
```bash
# Development
go build -o dotenv

# Production (all platforms)
make build-all

# Specific platform
GOOS=linux GOARCH=amd64 go build -o dotenv-linux-amd64
```

### Dependencies
- Minimal external dependencies
- Vendor dependencies for reliability
- Security audit all dependencies

## User Experience

### Installation

Two release channels — stable (tagged) and nightly (rolling main HEAD).

```bash
# Stable, macOS
brew install dotenv-cli

# Stable, Linux/Windows
curl -sSL https://dotenv.cloud/install.sh | bash

# Nightly (bleeding-edge main HEAD)
curl -sSL https://dotenv.cloud/install.sh | bash -s -- --nightly

# Go
go install github.com/dotenvcloud/cli@latest
```

Stable comes from `release.yml` (GoReleaser on `v*` tag). Nightly comes from `nightly.yml` (rolling pre-release on every main-CI success). See `docs/INSTALLATION.md` for full channel docs.

### First Run Experience
1. `dotenv init` - Interactive setup
2. `dotenv login` - Authenticate
3. `dotenv pull myapp` - Get secrets
4. Ready to use!

### Performance Goals
- Startup time: < 10ms
- Command execution: < 100ms
- Binary size: < 10MB
- Memory usage: < 20MB

## Integration with Web App

### API Endpoints Used
- `GET /api/v1/{project}/secrets`
- `POST /api/v1/secrets/retrieve`
- `GET /api/v1/organization`
- `GET /api/v1/{project}/encryption-key`

### Shared Logic
- Encryption algorithms (via sdk-go)
- API client patterns
- Error codes and messages

## Security Considerations

### Credential Storage
- Use system keychain when available
- Encrypted file storage as fallback
- Never store in plain text
- Support hardware tokens

### Network Security
- TLS 1.3 minimum
- Certificate pinning option
- Proxy support
- DNS over HTTPS

### Local Security
- Secure memory for keys
- Clear sensitive data after use
- File permission checks
- Audit logging

## Common Tasks

### Adding a New Command
1. Create cmd/newcommand.go
2. Implement cobra.Command
3. Add to root command
4. Write tests
5. Update documentation

### Updating API Client
1. Check OpenAPI spec changes
2. Update client methods
3. Handle new error codes
4. Test against staging API

### Release Process
1. Update version in main.go
2. Run all tests
3. Build for all platforms
4. Create GitHub release
5. Update Homebrew formula

## Debugging

### Verbose Output
```bash
dotenv pull myapp -v        # Verbose
dotenv pull myapp -vv       # Very verbose
dotenv pull myapp --debug   # Debug mode
```

### Common Issues
- **Authentication failures**: Check API key and organization
- **Encryption errors**: Verify key format and encoding
- **Network issues**: Check proxy settings and connectivity
- **Permission denied**: Verify file permissions and paths

## MCP Tool Usage for This CLI Project

This CLI project has its own Serena MCP server configured at `apps/cli/.serena`, providing project-specific code analysis and editing capabilities.

### Available MCP Tools for This Project

The **mcp__serena-cli__** server provides these tools specifically for this CLI codebase:

- `mcp__serena-cli__find_symbol` - Find symbols in this CLI project
- `mcp__serena-cli__find_referencing_symbols` - Find references within this codebase
- `mcp__serena-cli__get_symbols_overview` - Get overview of CLI code structure
- `mcp__serena-cli__replace_symbol_body` - Replace entire functions/methods
- `mcp__serena-cli__insert_before_symbol` - Insert code before symbols
- `mcp__serena-cli__insert_after_symbol` - Insert code after symbols
- `mcp__serena-cli__replace_regex` - Precise edits within this project
- `mcp__serena-cli__search_for_pattern` - Search patterns in CLI code
- `mcp__serena-cli__list_memories` - List CLI project-specific memories
- `mcp__serena-cli__write_memory` - Store CLI development decisions
- `mcp__serena-cli__read_memory` - Retrieve CLI project context

### When to Use This Project's MCP

**Use mcp__serena-cli__ for:**
- Navigating this CLI's package structure
- Understanding Cobra command implementations in this project
- Refactoring within the CLI codebase
- Storing/retrieving CLI-specific patterns and decisions
- Analyzing this project's error handling patterns
- Finding crypto implementations specific to this CLI

**Examples:**
```go
// Find Cobra commands in this CLI
mcp__serena-cli__find_symbol with name_path "Command" 

// Find this CLI's encryption implementation
mcp__serena-cli__find_symbol with name_path "Encrypt" relative_path "internal/crypto"

// Find references to config in this project
mcp__serena-cli__find_referencing_symbols with name_path "Config" relative_path "internal/config/config.go"

// Get overview of this CLI's API client
mcp__serena-cli__get_symbols_overview with relative_path "internal/api"
```

### Best Practices

1. **Use this project's MCP** for all semantic operations within apps/cli/
2. **Store project decisions** in mcp__serena-cli__ memories
3. **Don't mix with other project MCPs** - each has its own context
4. **Use standard tools** when working across multiple projects
5. **Check references** within this project before changing APIs

## Future Enhancements
- Shell integration for auto-loading
- Secret rotation commands
- Team collaboration features
- Audit log viewing
- Template support