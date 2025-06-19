# Development Guide

This guide is for contributors who want to develop, test, and contribute to the DotEnv CLI.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Building from Source](#building-from-source)
- [Testing](#testing)
- [Code Style](#code-style)
- [Contributing Guidelines](#contributing-guidelines)
- [Release Process](#release-process)

## Architecture Overview

The DotEnv CLI is built with:
- **Go 1.21+** - Primary language
- **Cobra** - Command-line framework
- **Viper** - Configuration management
- **Standard crypto** - Encryption (AES-256-GCM)

### Key Components

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Commands  │────▶│    Core     │────▶│     API     │
│   (Cobra)   │     │   Logic     │     │   Client    │
└─────────────┘     └─────────────┘     └─────────────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Config    │     │   Crypto    │     │   Formats   │
│   (Viper)   │     │  (AES-GCM)  │     │ (ENV/JSON)  │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Development Setup

### Prerequisites

1. **Go 1.21+**
   ```bash
   # Check version
   go version
   
   # Install/update Go
   # macOS
   brew install go
   
   # Linux
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   ```

2. **Development Tools**
   ```bash
   # Install development dependencies
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   go install github.com/goreleaser/goreleaser@latest
   go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest
   ```

3. **Clone Repository**
   ```bash
   git clone https://github.com/dotenv/cli.git
   cd cli
   ```

### Initial Setup

```bash
# Install dependencies
go mod download

# Run initial build
make build

# Run tests
make test
```

## Project Structure

```
apps/cli/
├── cmd/                    # Command implementations
│   ├── root.go            # Root command setup
│   ├── init.go            # Init command
│   ├── login.go           # Login command
│   ├── pull.go            # Pull command
│   └── ...
├── internal/              # Internal packages
│   ├── api/              # API client
│   │   ├── client.go
│   │   └── models.go
│   ├── auth/             # Authentication
│   │   ├── auth.go
│   │   └── oauth.go
│   ├── config/           # Configuration
│   │   ├── config.go
│   │   └── context.go
│   ├── crypto/           # Encryption
│   │   ├── aes/
│   │   └── key/
│   ├── formats/          # File formats
│   │   ├── env/
│   │   ├── json/
│   │   └── yaml/
│   └── ui/               # User interface
│       ├── prompts.go
│       └── messages.go
├── test/                  # Test files
│   ├── integration/
│   └── mocks/
├── scripts/               # Build scripts
├── docs/                  # Documentation
├── go.mod                 # Go modules
├── go.sum                 # Go checksums
├── Makefile              # Build automation
└── main.go               # Entry point
```

## Building from Source

### Basic Build

```bash
# Build for current platform
make build

# Output: ./bin/dotenv
```

### Cross-Platform Build

```bash
# Build for all platforms
make build-all

# Outputs:
# ./bin/dotenv-darwin-amd64
# ./bin/dotenv-darwin-arm64
# ./bin/dotenv-linux-amd64
# ./bin/dotenv-linux-arm64
# ./bin/dotenv-windows-amd64.exe
```

### Development Build

```bash
# Build with race detector
make dev

# Build with debug symbols
make build-debug
```

### Build Tags

```bash
# Build without telemetry
go build -tags notelemetry

# Build with experimental features
go build -tags experimental
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run specific package tests
go test ./internal/crypto/...

# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestEncryption ./internal/crypto
```

### Integration Tests

```bash
# Run integration tests
make test-integration

# Run specific integration test
go test -tags=integration -run TestCLICommands ./test/integration
```

### Test Coverage

```bash
# Generate coverage report
make test-coverage

# View coverage in browser
go tool cover -html=coverage.out

# Check coverage threshold
make coverage-check
```

### Benchmarks

```bash
# Run benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkEncryption ./internal/crypto

# Compare benchmarks
go test -bench=. -benchmem ./... > new.txt
benchcmp old.txt new.txt
```

### Mock Generation

```bash
# Generate mocks
make mocks

# Manual mock generation
mockgen -source=internal/api/client.go -destination=test/mocks/api_mock.go
```

## Code Style

### Go Standards

We follow standard Go conventions:
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use meaningful variable names
- Keep functions small and focused

### Linting

```bash
# Run linters
make lint

# Run specific linter
golangci-lint run ./...

# Auto-fix issues
golangci-lint run --fix
```

### Code Organization

1. **Package Structure**
   ```go
   // Package comment describes the package
   package crypto
   
   import (
       "crypto/aes"
       "crypto/cipher"
   )
   
   // Public types and constants first
   type Encryptor interface {
       Encrypt(data []byte) ([]byte, error)
       Decrypt(data []byte) ([]byte, error)
   }
   
   // Then public functions
   func NewEncryptor(key []byte) (Encryptor, error) {
       // Implementation
   }
   
   // Private types and functions last
   type aesEncryptor struct {
       cipher cipher.Block
   }
   ```

2. **Error Handling**
   ```go
   // Define package errors
   var (
       ErrInvalidKey = errors.New("invalid encryption key")
       ErrDecryptionFailed = errors.New("decryption failed")
   )
   
   // Wrap errors with context
   if err != nil {
       return fmt.Errorf("failed to encrypt data: %w", err)
   }
   ```

3. **Comments**
   ```go
   // Encryptor provides methods for encrypting and decrypting data.
   // It uses AES-256-GCM for authenticated encryption.
   type Encryptor interface {
       // Encrypt encrypts the given data and returns the ciphertext.
       // The returned ciphertext includes the nonce and authentication tag.
       Encrypt(data []byte) ([]byte, error)
       
       // Decrypt decrypts the given ciphertext and returns the plaintext.
       // It verifies the authentication tag before returning the data.
       Decrypt(ciphertext []byte) ([]byte, error)
   }
   ```

## Contributing Guidelines

### Workflow

1. **Fork the repository**
2. **Create a feature branch**
   ```bash
   git checkout -b feature/my-feature
   ```

3. **Make changes**
   - Write tests first (TDD)
   - Implement feature
   - Update documentation

4. **Commit changes**
   ```bash
   git add .
   git commit -m "feat: add new encryption method"
   ```

5. **Push and create PR**
   ```bash
   git push origin feature/my-feature
   ```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add support for YAML format
fix: resolve decryption error with special characters
docs: update installation guide
test: add integration tests for push command
refactor: extract encryption logic to separate package
chore: update dependencies
```

### Pull Request Process

1. **Before submitting**:
   - Run `make test`
   - Run `make lint`
   - Update documentation
   - Add tests for new features

2. **PR Description**:
   ```markdown
   ## Description
   Brief description of changes
   
   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update
   
   ## Testing
   - [ ] Unit tests pass
   - [ ] Integration tests pass
   - [ ] Manual testing completed
   
   ## Checklist
   - [ ] Code follows style guidelines
   - [ ] Self-review completed
   - [ ] Documentation updated
   - [ ] Tests added/updated
   ```

### Code Review

- Address all feedback
- Keep discussions professional
- Update PR based on reviews
- Squash commits before merge

## Release Process

### Version Numbering

We use [Semantic Versioning](https://semver.org/):
- MAJOR: Breaking changes
- MINOR: New features
- PATCH: Bug fixes

### Release Steps

1. **Update version**
   ```bash
   # Update version in version.go
   VERSION=v1.2.0
   sed -i "s/Version = .*/Version = \"$VERSION\"/" internal/version/version.go
   ```

2. **Update changelog**
   ```bash
   git-chglog -o CHANGELOG.md --next-tag $VERSION
   ```

3. **Create release commit**
   ```bash
   git add .
   git commit -m "chore: release $VERSION"
   git tag $VERSION
   ```

4. **Build releases**
   ```bash
   make release
   ```

5. **Push changes**
   ```bash
   git push origin main
   git push origin $VERSION
   ```

### Release Automation

We use GitHub Actions for automated releases:

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v4
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Development Tips

### Debugging

1. **Enable debug logging**
   ```go
   if debug {
       log.SetLevel(log.DebugLevel)
   }
   ```

2. **Use delve debugger**
   ```bash
   dlv debug -- pull myproject
   ```

3. **Add debug prints**
   ```go
   log.Debugf("Processing %d secrets", len(secrets))
   ```

### Performance

1. **Profile CPU usage**
   ```bash
   go test -cpuprofile cpu.prof -bench .
   go tool pprof cpu.prof
   ```

2. **Profile memory usage**
   ```bash
   go test -memprofile mem.prof -bench .
   go tool pprof mem.prof
   ```

3. **Trace execution**
   ```bash
   go test -trace trace.out
   go tool trace trace.out
   ```

### Common Tasks

```bash
# Update dependencies
go get -u ./...
go mod tidy

# Check for security vulnerabilities
go list -json -m all | nancy sleuth

# Generate documentation
godoc -http=:6060

# Format code
gofmt -w .

# Check for common mistakes
go vet ./...
```

## Next Steps

1. Read [Architecture Documentation](../architecture/README.md)
2. Review [API Documentation](../api/README.md)
3. Join [Discord Community](https://discord.gg/dotenv)
4. Check [Open Issues](https://github.com/dotenv/cli/issues)